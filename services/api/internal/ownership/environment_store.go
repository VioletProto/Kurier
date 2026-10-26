package ownership

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	runner "github.com/VioletProto/Kurier/services/request-runtime"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

func (st *Store) environmentStageGuard(stage record) types.TransactWriteItem {
	names := map[string]string{"#s": "state"}
	values := map[string]types.AttributeValue{":active": s("active"), ":g": s(stage.RecoveryGeneration)}
	expr := "#s = :active AND recoveryGeneration = :g"
	keys := []string{}
	versions := runner.EnvironmentStageVersions()
	for k := range versions {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for i, k := range keys {
		tag := fmt.Sprintf(":cap%d", i)
		expr += " AND " + k + " = " + tag
		values[tag] = n(versions[k])
	}
	return types.TransactWriteItem{ConditionCheck: &types.ConditionCheck{TableName: &st.table, Key: key(stage.PK, "META"), ConditionExpression: &expr, ExpressionAttributeNames: names, ExpressionAttributeValues: values}}
}
func (st *Store) environmentContext(ctx context.Context, user, project string) (record, record, record, error) {
	stage, u, p, e := st.requestContext(ctx, user, project)
	if e != nil {
		return stage, u, p, e
	}
	if stage.ProjectEnvironmentsSchemaVersion != 1 {
		return stage, u, p, unavailable()
	}
	return stage, u, p, nil
}
func (st *Store) environment(ctx context.Context, project, id string) (record, runner.Environment, error) {
	if !bindingUUID.MatchString(id) {
		return record{}, runner.Environment{}, missing()
	}
	r, e := st.get(ctx, "P#"+project, "ENV#"+id)
	var env runner.Environment
	if e != nil {
		return r, env, e
	}
	if r.Kind != "environment" || r.State != "active" || r.ProjectID != project || r.EnvironmentID != id || runner.StrictJSON([]byte(r.ConfigurationJSON), &env) != nil || env.EnvironmentID != id || env.ProjectID != project || env.Revision != r.Revision || env.Validate() != nil {
		return r, env, missing()
	}
	return r, env, nil
}

type environmentWrite struct {
	Name      *string         `json:"name"`
	Variables json.RawMessage `json:"variables"`
}
type variableWrite struct {
	VariableID  string                  `json:"variableId,omitempty"`
	Name        string                  `json:"name"`
	Enabled     bool                    `json:"enabled"`
	Sensitive   bool                    `json:"sensitive"`
	Value       *runner.Primitive       `json:"value,omitempty"`
	ValueType   string                  `json:"valueType,omitempty"`
	SecretWrite *environmentSecretWrite `json:"secretWrite,omitempty"`
}
type environmentSecretWrite struct {
	Action    string            `json:"action"`
	Value     *runner.Primitive `json:"value,omitempty"`
	SecretRef *runner.Ref       `json:"secretRef,omitempty"`
}

func environmentError(e error) error {
	var x *runner.Error
	if errors.As(e, &x) {
		return apiError(x.Status, x.Code, "Environment operation could not be completed.")
	}
	return e
}
func (st *Store) saveEnvironment(ctx context.Context, user, project, id string, raw []byte, expected int64, create bool) (runner.Environment, error) {
	stage, u, p, e := st.environmentContext(ctx, user, project)
	if e != nil {
		return runner.Environment{}, e
	}
	var input environmentWrite
	var fields map[string]json.RawMessage
	if runner.StrictJSON(raw, &input) != nil || runner.StrictJSON(raw, &fields) != nil || fields == nil || len(fields) == 0 {
		return runner.Environment{}, invalidConfiguration()
	}
	for k := range fields {
		if k != "name" && k != "variables" {
			return runner.Environment{}, invalidConfiguration()
		}
	}
	var old runner.Environment
	var r record
	now := timestamp(st.now())
	if create {
		id = newID()
		old = runner.Environment{EnvironmentID: id, ProjectID: project, Variables: []runner.EnvironmentVariable{}, CreatedAt: now}
		if input.Name == nil || input.Variables == nil {
			return old, invalidConfiguration()
		}
	} else {
		r, old, e = st.environment(ctx, project, id)
		if e != nil {
			return old, e
		}
		if r.Revision != expected {
			return old, requestStale()
		}
	}
	if !create && old.Revision >= 9007199254740991 {
		return old, conflict()
	}
	env := old
	env.UpdatedAt = now
	if input.Name != nil {
		env.Name = strings.TrimSpace(*input.Name)
	} else if fields["name"] != nil {
		return old, invalidConfiguration()
	}
	if utf8.RuneCountInString(env.Name) < 1 || utf8.RuneCountInString(env.Name) > 100 || control(env.Name, false) {
		return old, invalidConfiguration()
	}
	actions := []types.TransactWriteItem{st.stageGuard(stage), st.userGuard(u), st.requestGate(p, "active")}
	if input.Variables != nil {
		var writes []variableWrite
		var rows []map[string]json.RawMessage
		if runner.StrictJSON(input.Variables, &writes) != nil || runner.StrictJSON(input.Variables, &rows) != nil || writes == nil || len(writes) > 32 {
			return old, invalidConfiguration()
		}
		previous := map[string]runner.EnvironmentVariable{}
		for _, v := range old.Variables {
			previous[v.VariableID] = v
		}
		env.Variables = []runner.EnvironmentVariable{}
		names := map[string]bool{}
		ids := map[string]bool{}
		protected := 0
		for i, w := range writes {
			allowed := map[string]bool{"variableId": true, "name": true, "enabled": true, "sensitive": true, "value": true}
			if w.Sensitive {
				delete(allowed, "value")
				allowed["valueType"] = true
				allowed["secretWrite"] = true
			}
			for k := range rows[i] {
				if !allowed[k] {
					return old, invalidConfiguration()
				}
			}

			if rows[i]["enabled"] == nil || string(rows[i]["enabled"]) == "null" || rows[i]["sensitive"] == nil || string(rows[i]["sensitive"]) == "null" || !environmentVariableName(w.Name) || names[w.Name] || rows[i]["variableId"] != nil && w.VariableID == "" {
				return old, invalidConfiguration()
			}
			names[w.Name] = true
			typ := w.ValueType
			if !w.Sensitive {
				if w.Value == nil || w.ValueType != "" || w.SecretWrite != nil || credentialName(w.Name) {
					return old, invalidConfiguration()
				}
				typ = w.Value.Type
				value, e := w.Value.Raw()
				if e != nil {
					return old, environmentError(e)
				}
				if typ == "string" {
					var text string
					json.Unmarshal(value, &text)
					if obviousCredential(text) {
						return old, invalidConfiguration()
					}
				}
			}
			prior, exists := previous[w.VariableID]
			if w.VariableID != "" {
				priorType := prior.ValueType
				if !prior.Sensitive && prior.Value != nil {
					priorType = prior.Value.Type
				}
				if !exists || create || prior.Name != w.Name || prior.Sensitive != w.Sensitive || priorType != typ || ids[w.VariableID] {
					return old, invalidConfiguration()
				}
			} else {
				w.VariableID = newID()
			}
			ids[w.VariableID] = true
			v := runner.EnvironmentVariable{VariableID: w.VariableID, Name: w.Name, Enabled: w.Enabled, Sensitive: w.Sensitive, Value: w.Value}
			if w.Sensitive {
				protected++
				if protected > 16 || w.Value != nil || w.SecretWrite == nil {
					return old, invalidConfiguration()
				}
				sw := w.SecretWrite
				v.ValueType = typ
				v.Masked = true
				switch sw.Action {
				case "set":
					if sw.Value == nil || sw.SecretRef != nil || sw.Value.Type != typ {
						return old, invalidConfiguration()
					}
					value, e := sw.Value.Raw()
					if e != nil {
						return old, environmentError(e)
					}
					text := string(value)
					kind := "jsonScalar"
					if typ == "string" {
						kind = "string"
						json.Unmarshal(value, &text)
					}
					if runner.ValidateTypedSecret(kind, typ, text) != nil {
						return old, invalidConfiguration()
					}
					secretID := newID()
					encrypted, e := st.cipher.encrypt(ctx, project, secretID, 0, []byte(text))
					if e != nil {
						return old, e
					}
					secret := protectedRecord{ScalarType: typ, PK: p.PK, SK: "SECRET#" + secretID, Kind: "savedSecret", SchemaVersion: 2, SecretID: secretID, ProjectID: project, State: "active", ValueKind: kind, HeaderSafe: kind == "string" && !control(text, true), QuerySafe: !control(text, false), CreatedAt: now, UpdatedAt: now, Envelope: encrypted}
					action, e := st.secretPut(secret, true)
					if e != nil {
						return old, unavailable()
					}
					actions = append(actions, action)
					v.SecretRef = &runner.Ref{SecretID: secretID}
				case "preserve", "secretRef":
					if sw.Value != nil || sw.SecretRef == nil {
						return old, invalidConfiguration()
					}
					if sw.Action == "preserve" && (!exists || prior.SecretRef == nil || prior.SecretRef.SecretID != sw.SecretRef.SecretID) {
						return old, invalidConfiguration()
					}
					secret, e := st.secret(ctx, project, sw.SecretRef.SecretID)
					if e != nil || secret.State != "active" || secret.metadata().ScalarType != typ || secret.ValueKind == "jsonBody" {
						return old, invalidConfiguration()
					}
					v.SecretRef = sw.SecretRef
					// Deduplicate shared source checks in this replacing collection.
					key := secret.SecretID
					already := false
					for _, a := range actions {
						if a.ConditionCheck != nil && a.ConditionCheck.Key["SK"] != nil {
							if x, ok := a.ConditionCheck.Key["SK"].(*types.AttributeValueMemberS); ok && x.Value == "SECRET#"+key {
								already = true
							}
						}
					}
					if !already {
						actions = append(actions, st.secretSourceGuard(secret))
					}
				default:
					return old, invalidConfiguration()
				}
			}
			env.Variables = append(env.Variables, v)
		}
	}
	if !create {
		env.Revision++
	}
	if e = env.Validate(); e != nil {
		return old, environmentError(e)
	}
	encoded, _ := json.Marshal(env)
	if len(encoded) > 32*1024 {
		return old, apiError(413, "payload_too_large", "Environment exceeds its size limit.")
	}
	r = record{PK: p.PK, SK: "ENV#" + id, Kind: "environment", SchemaVersion: 1, ProjectID: project, EnvironmentID: id, Revision: env.Revision, State: "active", ConfigurationJSON: string(encoded), CreatedAt: env.CreatedAt, UpdatedAt: now}
	condition := "attribute_not_exists(PK)"
	var values map[string]types.AttributeValue
	if !create {
		condition = "revision = :r AND #s = :active AND #k = :kind"
		values = map[string]types.AttributeValue{":r": n(expected), ":active": s("active"), ":kind": s("environment")}
	}
	action, e := st.put(r, condition, values)
	if e != nil {
		return old, e
	}
	if !create {
		action.Put.ExpressionAttributeNames = map[string]string{"#s": "state", "#k": "kind"}
	}
	actions = append(actions, action)
	if e = st.transact(ctx, actions); e != nil {
		return old, st.environmentFailure(ctx, user, project, id, expected, create, e)
	}
	return env, nil
}
func environmentVariableName(name string) bool {
	if len(name) < 1 || len(name) > 64 {
		return false
	}
	for i, c := range name {
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c == '_' || i > 0 && c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}
func (st *Store) environmentFailure(ctx context.Context, user, project, id string, expected int64, create bool, original error) error {
	if !isConditional(original) {
		return unavailable()
	}
	if _, _, _, e := st.environmentContext(ctx, user, project); e != nil {
		return e
	}
	if !create {
		r, _, e := st.environment(ctx, project, id)
		if e != nil {
			return e
		}
		if r.Revision != expected {
			return requestStale()
		}
	}
	return conflict()
}
func (a *Server) environmentEndpoint(w http.ResponseWriter, r *http.Request, u User) error {
	project, id := r.PathValue("projectId"), r.PathValue("environmentId")
	if _, _, _, e := a.store.environmentContext(r.Context(), u.UserID, project); e != nil {
		return e
	}
	if r.Method == "GET" && id == "" {
		return a.listEnvironments(w, r, u)
	}
	if r.Method == "GET" {
		_, env, e := a.store.environment(r.Context(), project, id)
		if e != nil {
			return e
		}
		if _, _, _, e = a.store.environmentContext(r.Context(), u.UserID, project); e != nil {
			return e
		}
		w.Header().Set("ETag", strconv.Quote(strconv.FormatInt(env.Revision, 10)))
		success(w, 200, map[string]any{"environment": env})
		return nil
	}
	create := r.Method == "POST"
	expected := int64(0)
	if !create {
		var e error
		expected, e = precondition(r)
		if e == nil && expected > 9007199254740991 {
			return invalidConfiguration()
		}
		if e != nil {
			return e
		}
	}
	if r.Method == "DELETE" {
		if r.ContentLength != 0 || r.TransferEncoding != nil {
			return invalidConfiguration()
		}
		if e := a.store.deleteEnvironment(r.Context(), u.UserID, project, id, expected); e != nil {
			return e
		}
		w.WriteHeader(204)
		return nil
	}
	raw, e := readRequestJSON(w, r)
	if e != nil {
		return e
	}
	env, e := a.store.saveEnvironment(r.Context(), u.UserID, project, id, raw, expected, create)
	if e != nil {
		return e
	}
	status := 200
	if create {
		status = 201
		w.Header().Set("Location", "/api/v1/projects/"+project+"/environments/"+env.EnvironmentID)
	}
	w.Header().Set("ETag", strconv.Quote(strconv.FormatInt(env.Revision, 10)))
	success(w, status, map[string]any{"environment": env})
	return nil
}
func (st *Store) deleteEnvironment(ctx context.Context, user, project, id string, expected int64) error {
	stage, u, p, e := st.environmentContext(ctx, user, project)
	if e != nil {
		return e
	}
	r, e := st.get(ctx, p.PK, "ENV#"+id)
	if e != nil {
		return e
	}
	if r.Kind == "environmentTombstone" && r.State == "deleted" && r.EnvironmentID == id && r.Revision == expected+1 {
		return nil
	}
	if r.Kind != "environment" || r.State != "active" || r.EnvironmentID != id {
		return missing()
	}
	if r.Revision != expected {
		return requestStale()
	}
	if r.Revision >= 9007199254740991 {
		return conflict()
	}
	tomb := record{PK: r.PK, SK: r.SK, Kind: "environmentTombstone", SchemaVersion: 1, ProjectID: project, EnvironmentID: id, State: "deleted", Revision: expected + 1}
	action, e := st.put(tomb, "revision = :r AND #s = :active AND #k = :kind", map[string]types.AttributeValue{":r": n(expected), ":active": s("active"), ":kind": s("environment")})
	if e != nil {
		return e
	}
	action.Put.ExpressionAttributeNames = map[string]string{"#s": "state", "#k": "kind"}
	if e = st.transact(ctx, []types.TransactWriteItem{st.stageGuard(stage), st.userGuard(u), st.requestGate(p, "active"), action}); e != nil {
		return st.environmentFailure(ctx, user, project, id, expected, false, e)
	}
	return nil
}

type environmentSummary struct {
	EnvironmentID string `json:"environmentId"`
	ProjectID     string `json:"projectId"`
	Revision      int64  `json:"revision"`
	Name          string `json:"name"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
}

func (a *Server) listEnvironments(w http.ResponseWriter, r *http.Request, u User) error {
	project := r.PathValue("projectId")
	stage, _, _, e := a.store.environmentContext(r.Context(), u.UserID, project)
	if e != nil {
		return e
	}
	q, e := url.ParseQuery(r.URL.RawQuery)
	if e != nil {
		return invalidConfiguration()
	}
	limit := 25
	for k, v := range q {
		if len(v) != 1 || k != "limit" && k != "cursor" {
			return invalidConfiguration()
		}
	}
	if q.Has("limit") {
		limit, e = strconv.Atoi(q.Get("limit"))
		if e != nil || limit < 1 || limit > 100 {
			return invalidConfiguration()
		}
	}
	now := a.store.now()
	cursor := projectCursor{Owner: u.UserID, Stage: a.store.stage, Generation: stage.RecoveryGeneration, Type: "environments:" + project, Index: "Control", IssuedAt: now.Unix(), ExpiresAt: now.Add(24 * time.Hour).Unix()}
	var start map[string]types.AttributeValue
	if q.Has("cursor") {
		cursor, e = decodeCursor(q.Get("cursor"), a.cursorSecret)
		if e != nil {
			return e
		}
		if cursor.Owner != u.UserID || cursor.Stage != a.store.stage || cursor.Generation != stage.RecoveryGeneration || cursor.Type != "environments:"+project || cursor.Index != "Control" || cursor.ExpiresAt <= now.Unix() || cursor.IssuedAt > now.Unix() || cursor.ExpiresAt-cursor.IssuedAt != 86400 || cursor.Last.PK != "P#"+project || !strings.HasPrefix(cursor.Last.SK, "ENV#") {
			return invalidConfiguration()
		}
		start = key(cursor.Last.PK, cursor.Last.SK)
	}
	items := []environmentSummary{}
	bytesRead := 0
	for i := 0; i < 5; i++ {
		page, e := a.store.db.Query(r.Context(), &dynamodb.QueryInput{TableName: &a.store.table, KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :prefix)"), ExpressionAttributeValues: map[string]types.AttributeValue{":pk": s("P#" + project), ":prefix": s("ENV#")}, ConsistentRead: aws.Bool(true), ExclusiveStartKey: start, Limit: aws.Int32(int32(limit - len(items)))})
		if e != nil {
			return unavailable()
		}
		for _, item := range page.Items {
			raw, _ := json.Marshal(item)
			bytesRead += len(raw)
			if bytesRead > 2*1024*1024 {
				return unavailable()
			}
			var row record
			if attributevalue.UnmarshalMap(item, &row) != nil {
				return unavailable()
			}
			if row.Kind != "environment" || row.State != "active" {
				continue
			}
			_, env, e := a.store.environment(r.Context(), project, row.EnvironmentID)
			if e != nil {
				continue
			}
			items = append(items, environmentSummary{env.EnvironmentID, project, env.Revision, env.Name, env.CreatedAt, env.UpdatedAt})
		}
		start = page.LastEvaluatedKey
		if len(start) == 0 || len(items) == limit {
			break
		}
	}
	var next *string
	if len(start) > 0 {
		v, ok := start["SK"].(*types.AttributeValueMemberS)
		if !ok {
			return unavailable()
		}
		cursor.Last = cursorKey{PK: "P#" + project, SK: v.Value}
		token, e := encodeCursor(cursor, a.cursorSecret)
		if e != nil {
			return unavailable()
		}
		next = &token
	}
	if _, _, _, e = a.store.environmentContext(r.Context(), u.UserID, project); e != nil {
		return e
	}
	success(w, 200, map[string]any{"items": items, "nextCursor": next})
	return nil
}

func (w *environmentSecretWrite) UnmarshalJSON(raw []byte) error {
	type plain environmentSecretWrite
	var v plain
	var fields map[string]json.RawMessage
	if runner.StrictJSON(raw, &v) != nil || runner.StrictJSON(raw, &fields) != nil || len(fields) != 2 || fields["action"] == nil {
		return invalidConfiguration()
	}
	if v.Action == "set" {
		if fields["value"] == nil || string(fields["value"]) == "null" || fields["secretRef"] != nil {
			return invalidConfiguration()
		}
	} else if v.Action == "preserve" || v.Action == "secretRef" {
		if fields["secretRef"] == nil || string(fields["secretRef"]) == "null" || fields["value"] != nil {
			return invalidConfiguration()
		}
	} else {
		return invalidConfiguration()
	}
	*w = environmentSecretWrite(v)
	return nil
}
