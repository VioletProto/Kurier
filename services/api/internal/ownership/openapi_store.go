package ownership

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	runner "github.com/VioletProto/Kurier/services/request-runtime"
	oapi "github.com/VioletProto/Kurier/services/request-runtime/openapi"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Only sanitized source, bounded operation bundles and public choices reach these
// rows. Raw uploads, policy secret values and parser errors never reach storage.
type openapiRecord struct {
	PK              string `dynamodbav:"PK"`
	SK              string `dynamodbav:"SK"`
	Kind            string `dynamodbav:"kind"`
	SchemaVersion   int    `dynamodbav:"schemaVersion"`
	ProjectID       string `dynamodbav:"projectId"`
	ImportID        string `dynamodbav:"importId"`
	OperationID     string `dynamodbav:"operationId,omitempty"`
	Revision        int64  `dynamodbav:"revision"`
	State           string `dynamodbav:"state,omitempty"`
	OperationCount  int    `dynamodbav:"operationCount,omitempty"`
	SourceChunks    int    `dynamodbav:"sourceChunks,omitempty"`
	SourceBytes     int    `dynamodbav:"sourceBytes,omitempty"`
	DiagnosticsJSON string `dynamodbav:"diagnosticsJSON,omitempty"`
	OperationJSON   string `dynamodbav:"operationJSON,omitempty"`
	ChoicesJSON     string `dynamodbav:"choicesJSON,omitempty"`
	RequestID       string `dynamodbav:"requestId,omitempty"`
	Data            string `dynamodbav:"data,omitempty"`
	CreatedAt       string `dynamodbav:"createdAt,omitempty"`
	UpdatedAt       string `dynamodbav:"updatedAt,omitempty"`
	LPK             string `dynamodbav:"LPK,omitempty"`
	LSK             string `dynamodbav:"LSK,omitempty"`
}
type OpenAPIImportSummary struct {
	ImportID       string `json:"importId"`
	ProjectID      string `json:"projectId"`
	Revision       int64  `json:"revision"`
	State          string `json:"state"`
	OperationCount int    `json:"operationCount"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}
type OpenAPICandidate struct {
	Operation   oapi.Operation `json:"operation"`
	Choices     oapi.Choices   `json:"choices"`
	ChoicesJSON string         `json:"choicesJSON"`
	RequestID   string         `json:"requestId"`
	Ready       bool           `json:"ready"`
}
type OpenAPIImport struct {
	OpenAPIImportSummary
	ReadyOperationCount int                `json:"readyOperationCount"`
	NextOperationOffset *int               `json:"nextOperationOffset"`
	Diagnostics         []oapi.Diagnostic  `json:"diagnostics"`
	Operations          []OpenAPICandidate `json:"operations"`
}

func (r openapiRecord) summary() OpenAPIImportSummary {
	return OpenAPIImportSummary{r.ImportID, r.ProjectID, r.Revision, r.State, r.OperationCount, r.CreatedAt, r.UpdatedAt}
}
func openapiKey(id string) string    { return "OPENAPI#" + id + "#META" }
func openapiPrefix(id string) string { return "OPENAPI#" + id + "#" }
func openapiError(err error) error {
	var e *oapi.Error
	if errors.As(err, &e) {
		return apiError(400, e.Code, "OpenAPI operation could not be completed. Check the supported format and required inputs.")
	}
	return unavailable()
}
func (st *Store) openapiContext(ctx context.Context, user, project string) (record, record, record, error) {
	stage, u, p, e := st.requestContext(ctx, user, project)
	if e != nil {
		return stage, u, p, e
	}
	if stage.OpenAPISchemaVersion != 1 || stage.SavedRequestsSchemaVersion != 10 {
		return stage, u, p, unavailable()
	}
	return stage, u, p, nil
}
func (st *Store) openapiGet(ctx context.Context, project, id, sk string) (openapiRecord, error) {
	var r openapiRecord
	if !bindingUUID.MatchString(id) {
		return r, missing()
	}
	out, e := st.db.GetItem(ctx, &dynamodb.GetItemInput{TableName: &st.table, Key: key("P#"+project, sk), ConsistentRead: aws.Bool(true)})
	if e != nil {
		return r, unavailable()
	}
	if len(out.Item) == 0 {
		return r, missing()
	}
	return decodeOpenAPI(out.Item, project, id, sk)
}
func decodeOpenAPI(item map[string]types.AttributeValue, project, id, sk string) (openapiRecord, error) {
	var r openapiRecord
	if attributevalue.UnmarshalMap(item, &r) != nil || r.SchemaVersion != 1 || r.PK != "P#"+project || r.ProjectID != project || r.ImportID != id || r.SK != sk || r.Revision < 0 || r.Revision > 9007199254740991 {
		return r, unavailable()
	}
	// Re-encoding also rejects unrecognized persisted attributes.
	canonical, e := attributevalue.MarshalMap(r)
	if e != nil || !sameAttributeMap(canonical, item) {
		return r, unavailable()
	}
	if sk == openapiKey(id) {
		if r.Kind != "openapiImport" || r.OperationCount < 1 || r.OperationCount > 100 || r.SourceChunks < 1 || r.SourceChunks > 44 || r.SourceBytes < 1 || r.SourceBytes > 2*1024*1024 {
			return r, unavailable()
		}
		switch r.State {
		case "creating", "preparing", "publishing", "ready", "deleting":
		default:
			return r, unavailable()
		}
	}
	return r, nil
}
func sameAttributeMap(a, b map[string]types.AttributeValue) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return bytes.Equal(x, y)
}
func (st *Store) openapiPut(r openapiRecord, condition string, values map[string]types.AttributeValue) (types.TransactWriteItem, error) {
	item, e := attributevalue.MarshalMap(r)
	if e != nil || itemBytes(item) > 96*1024 {
		return types.TransactWriteItem{}, unavailable()
	}
	return types.TransactWriteItem{Put: &types.Put{TableName: &st.table, Item: item, ConditionExpression: &condition, ExpressionAttributeValues: values}}, nil
}
func (st *Store) openapiGuard(r openapiRecord) types.TransactWriteItem {
	return types.TransactWriteItem{ConditionCheck: &types.ConditionCheck{TableName: &st.table, Key: key(r.PK, r.SK), ConditionExpression: aws.String("#k = :k AND schemaVersion = :schema AND #s = :s AND revision = :r"), ExpressionAttributeNames: map[string]string{"#k": "kind", "#s": "state"}, ExpressionAttributeValues: map[string]types.AttributeValue{":k": s("openapiImport"), ":schema": n(1), ":s": s(r.State), ":r": n(r.Revision)}}}
}
func (st *Store) openapiChange(ctx context.Context, user, project string, old, next openapiRecord, additional []types.TransactWriteItem) error {
	stage, u, p, e := st.openapiContext(ctx, user, project)
	if e != nil {
		return e
	}
	if old.Revision >= 9007199254740991 {
		return conflict()
	}
	next.Revision = old.Revision + 1
	next.UpdatedAt = timestamp(st.now())
	action, e := st.openapiPut(next, "revision = :r AND #s = :s AND #k = :k", map[string]types.AttributeValue{":r": n(old.Revision), ":s": s(old.State), ":k": s("openapiImport")})
	if e != nil {
		return e
	}
	action.Put.ExpressionAttributeNames = map[string]string{"#s": "state", "#k": "kind"}
	return st.transact(ctx, append([]types.TransactWriteItem{st.stageGuard(stage), st.userGuard(u), st.requestGate(p, "active"), action}, additional...))
}
func (st *Store) openapiRows(ctx context.Context, project, id string) ([]openapiRecord, error) {
	rows := []openapiRecord{}
	var start map[string]types.AttributeValue
	for page := 0; page < 32; page++ {
		out, e := st.db.Query(ctx, &dynamodb.QueryInput{TableName: &st.table, KeyConditionExpression: aws.String("PK = :p AND begins_with(SK, :s)"), ExpressionAttributeValues: map[string]types.AttributeValue{":p": s("P#" + project), ":s": s(openapiPrefix(id))}, ConsistentRead: aws.Bool(true), ExclusiveStartKey: start, Limit: aws.Int32(25)})
		if e != nil {
			return nil, unavailable()
		}
		for _, item := range out.Items {
			var k struct{ SK string }
			if attributevalue.UnmarshalMap(item, &k) != nil {
				return nil, unavailable()
			}
			r, e := decodeOpenAPI(item, project, id, k.SK)
			if e != nil {
				return nil, e
			}
			rows = append(rows, r)
			if len(rows) > 145 {
				return nil, unavailable()
			}
		}
		start = out.LastEvaluatedKey
		if len(start) == 0 {
			return rows, nil
		}
	}
	return nil, unavailable()
}
func (st *Store) openapiDetail(ctx context.Context, user, project, id string) (OpenAPIImport, error) {
	var out OpenAPIImport
	if _, _, _, e := st.openapiContext(ctx, user, project); e != nil {
		return out, e
	}
	root, e := st.openapiGet(ctx, project, id, openapiKey(id))
	if e != nil {
		return out, e
	}
	out.OpenAPIImportSummary = root.summary()
	out.Diagnostics = []oapi.Diagnostic{}
	out.Operations = []OpenAPICandidate{}
	if root.DiagnosticsJSON != "" && runner.StrictJSON([]byte(root.DiagnosticsJSON), &out.Diagnostics) != nil {
		return out, unavailable()
	}
	rows, e := st.openapiRows(ctx, project, id)
	if e != nil {
		return out, e
	}
	for _, r := range rows {
		if r.Kind != "openapiOperation" {
			continue
		}
		var op oapi.Operation
		choices, choicesErr := decodeOpenAPIChoices([]byte(r.ChoicesJSON))
		if runner.StrictJSON([]byte(r.OperationJSON), &op) != nil || choicesErr != nil || op.OperationID != r.OperationID || r.SK != openapiPrefix(id)+"OP#"+r.OperationID {
			return out, unavailable()
		}
		generated, e := oapi.Generate(op, choices)
		if e == nil {
			configuration := generatedConfiguration(generated, nil)
			e = validateConfiguration(&configuration)
		}
		if e == nil {
			out.ReadyOperationCount++
		}
		out.Operations = append(out.Operations, OpenAPICandidate{op, choices, r.ChoicesJSON, r.RequestID, e == nil})
	}
	if (root.State == "preparing" || root.State == "publishing" || root.State == "ready") && len(out.Operations) != root.OperationCount {
		return out, unavailable()
	}
	latest, e := st.openapiGet(ctx, project, id, openapiKey(id))
	if e != nil || latest.Revision != root.Revision || latest.State != root.State {
		return out, conflict()
	}
	if _, _, _, e = st.openapiContext(ctx, user, project); e != nil {
		return OpenAPIImport{}, e
	}
	return out, nil
}
func (st *Store) createOpenAPI(ctx context.Context, user, project, format, source string, policy oapi.Policy) (OpenAPIImport, error) {
	var empty OpenAPIImport
	stage, u, p, e := st.openapiContext(ctx, user, project)
	if e != nil {
		return empty, e
	}
	doc, e := oapi.Parse([]byte(source), format, policy)
	if e != nil {
		return empty, openapiError(e)
	}
	if len(doc.Source) == 0 || len(doc.Source) > 2*1024*1024 || len(doc.Operations) == 0 || len(doc.Operations) > 100 {
		return empty, invalidConfiguration()
	}
	for _, operation := range doc.Operations {
		raw, e := json.Marshal(operation)
		if e != nil || len(raw) > 32*1024 {
			return empty, invalidConfiguration()
		}
	}
	id := newID()
	now := timestamp(st.now())
	diagnostics, _ := json.Marshal(doc.Diagnostics)
	root := openapiRecord{PK: p.PK, SK: openapiKey(id), Kind: "openapiImport", SchemaVersion: 1, ProjectID: project, ImportID: id, State: "creating", OperationCount: len(doc.Operations), SourceBytes: len(doc.Source), SourceChunks: (len(doc.Source) + 48*1024 - 1) / (48 * 1024), DiagnosticsJSON: string(diagnostics), CreatedAt: now, UpdatedAt: now, LPK: p.PK + "#OPENAPI", LSK: now + "#" + id}
	action, e := st.openapiPut(root, "attribute_not_exists(PK)", nil)
	if e != nil {
		return empty, e
	}
	if e = st.transact(ctx, []types.TransactWriteItem{st.stageGuard(stage), st.userGuard(u), st.requestGate(p, "active"), action}); e != nil {
		return empty, e
	}
	rows := []openapiRecord{}
	for i, op := range doc.Operations {
		raw, e := json.Marshal(op)
		if e != nil || len(raw) > 32*1024 {
			return empty, unavailable()
		}
		rows = append(rows, openapiRecord{PK: p.PK, SK: openapiPrefix(id) + "OP#" + op.OperationID, Kind: "openapiOperation", SchemaVersion: 1, ProjectID: project, ImportID: id, OperationID: op.OperationID, OperationJSON: string(raw), ChoicesJSON: "{}", RequestID: newID(), Revision: int64(i)})
	}
	for i, offset := 0, 0; offset < len(doc.Source); i, offset = i+1, offset+48*1024 {
		end := offset + 48*1024
		if end > len(doc.Source) {
			end = len(doc.Source)
		}
		rows = append(rows, openapiRecord{PK: p.PK, SK: fmt.Sprintf("%sSOURCE#%03d", openapiPrefix(id), i), Kind: "openapiSource", SchemaVersion: 1, ProjectID: project, ImportID: id, Data: base64.StdEncoding.EncodeToString(doc.Source[offset:end])})
	}
	for start := 0; start < len(rows); start += 20 {
		end := start + 20
		if end > len(rows) {
			end = len(rows)
		}
		stage, u, p, e = st.openapiContext(ctx, user, project)
		if e != nil {
			return empty, e
		}
		actions := []types.TransactWriteItem{st.stageGuard(stage), st.userGuard(u), st.requestGate(p, "active"), st.openapiGuard(root)}
		for _, row := range rows[start:end] {
			a, e := st.openapiPut(row, "attribute_not_exists(PK)", nil)
			if e != nil {
				return empty, e
			}
			actions = append(actions, a)
		}
		if e = st.transact(ctx, actions); e != nil {
			return empty, e
		}
	}
	next := root
	next.State = "preparing"
	if e = st.openapiChange(ctx, user, project, root, next, nil); e != nil {
		return empty, e
	}
	return st.openapiDetail(ctx, user, project, id)
}
func (st *Store) updateOpenAPICandidate(ctx context.Context, user, project, id, operation string, choices oapi.Choices, expected int64) (OpenAPIImport, error) {
	var empty OpenAPIImport
	if _, _, _, e := st.openapiContext(ctx, user, project); e != nil {
		return empty, e
	}
	root, e := st.openapiGet(ctx, project, id, openapiKey(id))
	if e != nil {
		return empty, e
	}
	if root.Revision != expected {
		return empty, requestStale()
	}
	if root.State != "preparing" {
		return empty, conflict()
	}
	op, e := st.openapiGet(ctx, project, id, openapiPrefix(id)+"OP#"+operation)
	if e != nil {
		return empty, e
	}
	var definition oapi.Operation
	if op.Kind != "openapiOperation" || runner.StrictJSON([]byte(op.OperationJSON), &definition) != nil {
		return empty, unavailable()
	}
	if !publicOpenAPIChoices(choices) {
		return empty, invalidConfiguration()
	}
	generated, e := oapi.Generate(definition, choices)
	if e != nil {
		var oe *oapi.Error
		if !errors.As(e, &oe) || oe.Code != "inputs_required" {
			return empty, openapiError(e)
		}
	} else {
		c := generatedConfiguration(generated, nil)
		if e = validateConfiguration(&c); e != nil {
			return empty, e
		}
	}
	raw, e := json.Marshal(choices)
	if e != nil || len(raw) > 64*1024 {
		return empty, invalidConfiguration()
	}
	op.ChoicesJSON = string(raw)
	action, e := st.openapiPut(op, "#k = :k AND operationJSON = :o", map[string]types.AttributeValue{":k": s("openapiOperation"), ":o": s(op.OperationJSON)})
	if e != nil {
		return empty, e
	}
	action.Put.ExpressionAttributeNames = map[string]string{"#k": "kind"}
	if e = st.openapiChange(ctx, user, project, root, root, []types.TransactWriteItem{action}); e != nil {
		return empty, e
	}
	return st.openapiDetail(ctx, user, project, id)
}
func decodeOpenAPIChoices(raw []byte) (oapi.Choices, error) {
	var choices oapi.Choices
	var fields map[string]json.RawMessage
	if runner.StrictJSON(raw, &fields) != nil || fields == nil {
		return choices, invalidConfiguration()
	}
	for key, value := range fields {
		switch key {
		case "body":
		case "server", "parameters", "secretRefs", "bodySecretRef":
			if string(value) == "null" {
				return choices, invalidConfiguration()
			}
		default:
			return choices, invalidConfiguration()
		}
	}
	if runner.StrictJSON(raw, &choices) != nil {
		return choices, invalidConfiguration()
	}
	return choices, nil
}
func publicOpenAPIChoices(choices oapi.Choices) bool {
	values := []json.RawMessage{}
	for _, v := range choices.Parameters {
		values = append(values, v)
	}
	if len(choices.Body) > 0 {
		values = append(values, choices.Body)
	}
	for _, raw := range values {
		var v any
		d := json.NewDecoder(bytes.NewReader(raw))
		d.UseNumber()
		if d.Decode(&v) != nil || !publicJSON(v) {
			return false
		}
	}
	return true
}
func generatedConfiguration(g oapi.Generated, ref *runner.OperationRef) RequestConfiguration {
	c := RequestConfiguration{Name: g.Name, Method: g.Method, URL: g.URL, OperationRef: ref, Headers: []RequestField{}, QueryParameters: []RequestField{}}
	convert := func(f oapi.Field) RequestField {
		v := RequestField{Name: f.Name, Value: f.Value, Enabled: f.Enabled, Sensitive: f.Sensitive}
		if f.SecretRef != nil {
			v.BindingID = newID()
			v.SecretWrite = &SecretWrite{Action: "secretRef", SecretRef: &SecretReference{SecretID: f.SecretRef.SecretID}}
		}
		return v
	}
	for _, f := range g.Headers {
		c.Headers = append(c.Headers, convert(f))
	}
	for _, f := range g.QueryParameters {
		c.QueryParameters = append(c.QueryParameters, convert(f))
	}
	if g.Body != nil {
		c.Body = &RequestBody{Type: g.Body.Type, Text: g.Body.Text, Sensitive: g.Body.Sensitive}
		if g.Body.SecretRef != nil {
			c.Body.BindingID = newID()
			c.Body.SecretWrite = &SecretWrite{Action: "secretRef", SecretRef: &SecretReference{SecretID: g.Body.SecretRef.SecretID}}
		}
	}
	return c
}
func (st *Store) publishOpenAPI(ctx context.Context, user, project, id string, expected int64) (OpenAPIImport, error) {
	var empty OpenAPIImport
	detail, e := st.openapiDetail(ctx, user, project, id)
	if e != nil {
		return empty, e
	}
	root, e := st.openapiGet(ctx, project, id, openapiKey(id))
	if e != nil {
		return empty, e
	}
	if root.Revision != expected {
		return empty, requestStale()
	}
	if detail.Revision != root.Revision {
		return empty, conflict()
	}
	if root.State != "preparing" {
		return empty, conflict()
	}
	// Resolve all candidates before claiming publication; incomplete input never
	// creates executable rows. Secret refs are checked by the existing authoring path.
	configs := make([]RequestConfiguration, len(detail.Operations))
	for i, candidate := range detail.Operations {
		g, e := oapi.Generate(candidate.Operation, candidate.Choices)
		if e != nil {
			return empty, openapiError(e)
		}
		configs[i] = generatedConfiguration(g, &runner.OperationRef{ImportID: id, OperationID: candidate.Operation.OperationID})
		if e = validateConfiguration(&configs[i]); e != nil {
			return empty, e
		}
	}
	claimed := root
	claimed.State = "publishing"
	if e = st.openapiChange(ctx, user, project, root, claimed, nil); e != nil {
		return empty, e
	}
	root, e = st.openapiGet(ctx, project, id, openapiKey(id))
	if e != nil {
		return empty, e
	}
	// Hidden request rows are staged in bounded batches. No owner request reader
	// or execution admission can use them until the single ready transition.
	for i := 0; i < len(configs); {
		stage, u, p, e := st.openapiContext(ctx, user, project)
		if e != nil {
			return empty, e
		}
		actions := []types.TransactWriteItem{st.stageGuard(stage), st.userGuard(u), st.requestGate(p, "active"), st.openapiGuard(root)}
		seen := map[string][]byte{}
		count := 0
		for i < len(configs) && count < 20 {
			c := configs[i]
			cloneRaw, _ := json.Marshal(c)
			if json.Unmarshal(cloneRaw, &c) != nil {
				return empty, unavailable()
			}
			secretActions, e := st.protectIfEnabled(ctx, stage, project, &c, nil)
			if e != nil {
				return empty, e
			}
			extra := []types.TransactWriteItem{}
			for _, a := range secretActions {
				if a.ConditionCheck == nil {
					return empty, unavailable()
				}
				kraw, _ := json.Marshal(a.ConditionCheck.Key)
				k := *a.ConditionCheck.TableName + string(kraw)
				v, _ := json.Marshal(a)
				if prior, ok := seen[k]; ok {
					if !bytes.Equal(prior, v) {
						return empty, conflict()
					}
					continue
				}
				seen[k] = v
				extra = append(extra, a)
			}
			if len(actions)+len(extra)+1 > 95 {
				break
			}
			raw, e := configurationJSON(c)
			if e != nil {
				return empty, e
			}
			now := timestamp(st.now())
			rid := detail.Operations[i].RequestID
			r := record{PK: p.PK, SK: "REQ#" + rid, Kind: "request", SchemaVersion: 4, ProjectID: project, ImportID: id, RequestID: rid, State: "active", ConfigurationJSON: string(raw), CreatedAt: now, UpdatedAt: now, LPK: p.PK + "#REQUEST", LSK: now + "#" + rid}
			a, e := st.put(r, "attribute_not_exists(PK)", nil)
			if e != nil {
				return empty, e
			}
			actions = append(actions, extra...)
			actions = append(actions, a)
			configs[i] = c
			i++
			count++
		}
		if count == 0 {
			return empty, unavailable()
		}
		if e = st.transact(ctx, actions); e != nil {
			return empty, e
		}
	}
	next := root
	next.State = "ready"
	if e = st.openapiChange(ctx, user, project, root, next, nil); e != nil {
		return empty, e
	}
	return st.openapiDetail(ctx, user, project, id)
}
func (st *Store) openapiSource(ctx context.Context, user, project, id, format string) ([]byte, error) {
	if _, _, _, e := st.openapiContext(ctx, user, project); e != nil {
		return nil, e
	}
	root, e := st.openapiGet(ctx, project, id, openapiKey(id))
	if e != nil {
		return nil, e
	}
	if root.State != "preparing" && root.State != "ready" {
		return nil, conflict()
	}
	var raw []byte
	for i := 0; i < root.SourceChunks; i++ {
		r, e := st.openapiGet(ctx, project, id, fmt.Sprintf("%sSOURCE#%03d", openapiPrefix(id), i))
		if e != nil || r.Kind != "openapiSource" {
			return nil, unavailable()
		}
		chunk, e := base64.StdEncoding.DecodeString(r.Data)
		if e != nil || len(chunk) > 48*1024 {
			return nil, unavailable()
		}
		raw = append(raw, chunk...)
	}
	if len(raw) != root.SourceBytes {
		return nil, unavailable()
	}
	// Reparse sanitized persisted source to reject corrupt/incompatible stored data.
	doc, e := oapi.Parse(raw, "json", oapi.Policy{})
	if e != nil {
		return nil, unavailable()
	}
	out, e := oapi.Export(doc, format)
	if e != nil {
		return nil, openapiError(e)
	}
	latest, e := st.openapiGet(ctx, project, id, openapiKey(id))
	if e != nil || latest.Revision != root.Revision {
		return nil, conflict()
	}
	if _, _, _, e = st.openapiContext(ctx, user, project); e != nil {
		return nil, e
	}
	return out, nil
}
func (st *Store) requestImportReady(ctx context.Context, r record) error {
	if r.ImportID == "" {
		return nil
	}
	root, e := st.openapiGet(ctx, r.ProjectID, r.ImportID, openapiKey(r.ImportID))
	if e != nil || root.State != "ready" {
		return missing()
	}
	return nil
}
func (st *Store) requestOperationGuards(ctx context.Context, stage record, project string, c RequestConfiguration) ([]types.TransactWriteItem, error) {
	if c.OperationRef == nil {
		return nil, nil
	}
	if stage.OpenAPISchemaVersion != 1 {
		return nil, unavailable()
	}
	root, e := st.openapiGet(ctx, project, c.OperationRef.ImportID, openapiKey(c.OperationRef.ImportID))
	if e != nil || root.State != "ready" {
		return nil, missing()
	}
	op, e := st.openapiGet(ctx, project, root.ImportID, openapiPrefix(root.ImportID)+"OP#"+c.OperationRef.OperationID)
	if e != nil || op.Kind != "openapiOperation" {
		return nil, missing()
	}
	var definition oapi.Operation
	if runner.StrictJSON([]byte(op.OperationJSON), &definition) != nil || definition.Method != c.Method {
		return nil, invalidConfiguration()
	}
	return []types.TransactWriteItem{st.openapiGuard(root), {ConditionCheck: &types.ConditionCheck{TableName: &st.table, Key: key(op.PK, op.SK), ConditionExpression: aws.String("#k = :k AND operationJSON = :o"), ExpressionAttributeNames: map[string]string{"#k": "kind"}, ExpressionAttributeValues: map[string]types.AttributeValue{":k": s("openapiOperation"), ":o": s(op.OperationJSON)}}}}, nil
}

// Keep imports' cursor format owner/stage/project-bound, just like request lists.
func (st *Store) listOpenAPI(ctx context.Context, user, project, raw string, secret []byte) ([]OpenAPIImportSummary, *string, error) {
	stage, _, _, e := st.openapiContext(ctx, user, project)
	if e != nil {
		return nil, nil, e
	}
	now := st.now()
	c := projectCursor{Owner: user, Stage: st.stage, Generation: stage.RecoveryGeneration, Type: "openapi:" + project, Index: "GSI1", HighWater: timestamp(now) + "#~", IssuedAt: now.Unix(), ExpiresAt: now.Add(24 * 60 * 60 * 1000000000).Unix()}
	var start map[string]types.AttributeValue
	if raw != "" {
		c, e = decodeCursor(raw, secret)
		if e != nil || c.Owner != user || c.Stage != st.stage || c.Generation != stage.RecoveryGeneration || c.Type != "openapi:"+project || c.Index != "GSI1" || c.ExpiresAt <= now.Unix() || c.IssuedAt > now.Unix() || c.ExpiresAt-c.IssuedAt != 86400 || c.HighWater == "" || c.Last.PK != "P#"+project || !strings.HasPrefix(c.Last.SK, "OPENAPI#") || c.Last.LPK != "P#"+project+"#OPENAPI" || c.Last.LSK == "" || c.Last.LSK > c.HighWater {
			return nil, nil, invalidConfiguration()
		}
		start = map[string]types.AttributeValue{"PK": s(c.Last.PK), "SK": s(c.Last.SK), "LPK": s(c.Last.LPK), "LSK": s(c.Last.LSK)}
	}
	out, e := st.db.Query(ctx, &dynamodb.QueryInput{TableName: &st.table, IndexName: aws.String("GSI1"), KeyConditionExpression: aws.String("LPK = :p AND LSK <= :h"), ExpressionAttributeValues: map[string]types.AttributeValue{":p": s("P#" + project + "#OPENAPI"), ":h": s(c.HighWater)}, ScanIndexForward: aws.Bool(false), ExclusiveStartKey: start, Limit: aws.Int32(25)})
	if e != nil {
		return nil, nil, unavailable()
	}
	items := []OpenAPIImportSummary{}
	for _, item := range out.Items {
		var k cursorKey
		if attributevalue.UnmarshalMap(item, &k) != nil {
			return nil, nil, unavailable()
		}
		parts := strings.Split(k.SK, "#")
		if len(parts) != 3 || parts[0] != "OPENAPI" || parts[2] != "META" {
			return nil, nil, unavailable()
		}
		r, e := st.openapiGet(ctx, project, parts[1], k.SK)
		if e != nil {
			var ae *APIError
			if errors.As(e, &ae) && ae.Status == 404 {
				continue
			}
			return nil, nil, e
		}
		items = append(items, r.summary())
	}
	var next *string
	if len(out.LastEvaluatedKey) > 0 {
		if attributevalue.UnmarshalMap(out.LastEvaluatedKey, &c.Last) != nil {
			return nil, nil, unavailable()
		}
		v, e := encodeCursor(c, secret)
		if e != nil {
			return nil, nil, e
		}
		next = &v
	}
	if _, _, _, e = st.openapiContext(ctx, user, project); e != nil {
		return nil, nil, e
	}
	return items, next, nil
}
