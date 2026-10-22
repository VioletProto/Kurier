package execution

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	runner "github.com/VioletProto/Kurier/services/request-runtime"
	wire "github.com/VioletProto/Kurier/services/request-runtime/mcpwire"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dt "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"slices"
	"strings"
	"time"
)

// readSet observes authoritative reads, then conditions the exact observed fields
// in the data-release audit transaction. It never writes or caches resource data.
type mcpReadSet struct {
	Database
	table   string
	items   map[string]map[string]dt.AttributeValue
	changed bool
}

func (d *mcpReadSet) GetItem(ctx context.Context, in *dynamodb.GetItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	out, e := d.Database.GetItem(ctx, in, opts...)
	if e != nil || out == nil || len(out.Item) == 0 {
		return out, e
	}
	if aws.ToString(in.TableName) != d.table {
		return nil, safeError()
	}
	pkv, pok := out.Item["PK"].(*dt.AttributeValueMemberS)
	skv, sok := out.Item["SK"].(*dt.AttributeValueMemberS)
	if !pok || !sok {
		return nil, safeError()
	}
	pk := pkv.Value
	sk := skv.Value
	k := pk + "\x00" + sk
	if old, ok := d.items[k]; ok {
		a, _ := json.Marshal(old)
		b, _ := json.Marshal(out.Item)
		if string(a) != string(b) {
			d.changed = true
		}
	} else {
		d.items[k] = out.Item
	}
	return out, e
}
func (d *mcpReadSet) guards(s *Service) ([]dt.TransactWriteItem, error) {
	if d.changed || len(d.items) > 72 {
		return nil, safeError()
	}
	out := []dt.TransactWriteItem{}
	for _, m := range d.items {
		names := map[string]string{}
		values := map[string]dt.AttributeValue{}
		parts := []string{}
		for i, k := range []string{"version", "kind", "revision", "state", "deletionEpoch", "disabled", "recoveryGeneration", "mcpReadOnlySchemaVersion", "savedRequestsSchemaVersion", "workflowAssertionsSchemaVersion", "assertionSemanticsVersion", "normalExpiresAt", "pinned", "evidenceAvailability", "tokenHash", "expiresAt"} {
			n := fmt.Sprintf("#f%d", i)
			names[n] = k
			if v, ok := m[k]; ok {
				val := fmt.Sprintf(":f%d", i)
				values[val] = v
				parts = append(parts, n+" = "+val)
			} else {
				parts = append(parts, "attribute_not_exists("+n+")")
			}
		}
		out = append(out, dt.TransactWriteItem{ConditionCheck: &dt.ConditionCheck{TableName: &s.Table, Key: map[string]dt.AttributeValue{"PK": m["PK"], "SK": m["SK"]}, ConditionExpression: aws.String(strings.Join(parts, " AND ")), ExpressionAttributeNames: names, ExpressionAttributeValues: values}})
	}
	return out, nil
}

type mcpCursor struct {
	Credential string `json:"credential"`
	Version    int64  `json:"version"`
	Generation string `json:"generation"`
	Tool       string `json:"tool"`
	Project    string `json:"project"`
	Filter     string `json:"filter"`
	High       string `json:"high"`
	LastID     string `json:"lastId"`
	LastTime   string `json:"lastTime"`
	Expires    int64  `json:"expires"`
	Offset     int    `json:"offset"`
}

func (s *Service) mcpAEAD() (cipher.AEAD, error) {
	if len(s.SigningKey) < 32 {
		return nil, safeError()
	}
	key := sha256.Sum256(append([]byte("kurier/mcp-cursor/v1/"+s.Stage+"/"), s.SigningKey...))
	b, e := aes.NewCipher(key[:])
	if e != nil {
		return nil, e
	}
	return cipher.NewGCM(b)
}
func (s *Service) mcpCursorEncode(c mcpCursor) (*string, error) {
	a, e := s.mcpAEAD()
	if e != nil {
		return nil, safeError()
	}
	nonce := make([]byte, a.NonceSize())
	if _, e = rand.Read(nonce); e != nil {
		return nil, safeError()
	}
	raw, _ := json.Marshal(c)
	v := base64.RawURLEncoding.EncodeToString(a.Seal(nonce, nonce, raw, []byte("mcp-pagination-v1")))
	return &v, nil
}
func (s *Service) mcpCursorDecode(raw string, c *mcpCursor) error {
	a, e := s.mcpAEAD()
	if e != nil {
		return safeError()
	}
	b, e := base64.RawURLEncoding.DecodeString(raw)
	if e != nil || len(b) < a.NonceSize() {
		return failure(400, "invalid_request")
	}
	b, e = a.Open(nil, b[:a.NonceSize()], b[a.NonceSize():], []byte("mcp-pagination-v1"))
	if e != nil || StrictJSON(b, c) != nil {
		return failure(400, "invalid_request")
	}
	return nil
}
func mcpRetention(r *Retention) *wire.Retention {
	if r == nil {
		return nil
	}
	return &wire.Retention{NormalExpiresAt: r.NormalExpiresAt, Pinned: r.Pinned, PinnedAt: r.PinnedAt, EvidenceAvailability: r.EvidenceAvailability, State: r.State, EffectiveStatus: r.EffectiveStatus}
}
func mcpExecution(v View) wire.Execution {
	return wire.Execution{ProjectID: v.ProjectID, ExecutionID: v.ExecutionID, Source: v.Source, RunID: v.RunID, Position: v.Position, Target: v.Target, Status: v.Status, Version: v.Version, SubmittedAt: v.SubmittedAt, StartedAt: v.StartedAt, CompletedAt: v.CompletedAt, Summary: v.Summary, Retention: mcpRetention(v.Retention)}
}
func mcpRun(project string, r WorkflowRun, detail bool) wire.Run {
	v := wire.Run{ProjectID: project, WorkflowRunID: r.WorkflowRunID, WorkflowID: r.WorkflowID, WorkflowRevision: r.WorkflowRevision, Target: r.Target, State: r.State, Version: r.Version, SubmittedAt: r.SubmittedAt, StartedAt: r.StartedAt, CompletedAt: r.CompletedAt, NormalExpiresAt: r.NormalExpiresAt, CurrentPosition: r.CurrentPosition, FailedPosition: r.FailedPosition}
	if detail {
		v.Steps = []wire.Step{}
		for _, step := range r.Steps {
			v.Steps = append(v.Steps, wire.Step{StepID: step.StepID, Position: step.Position, RequestID: step.RequestID, RequestRevision: step.RequestRevision, State: step.State, ExecutionID: step.ExecutionID, Summary: step.Summary, Retention: mcpRetention(step.Retention), EvidenceExpired: step.EvidenceExpired})
		}
	}
	return v
}

// MCPInvoke never dispatches execution or calls KMS. A conditioned audit gates all data.
func (s *Service) MCPInvoke(ctx context.Context, token, tool string, raw []byte) (wire.Envelope, string, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	_, cred, e := s.authenticateMCP(ctx, token)
	if e != nil {
		return wire.Envelope{}, "", e
	}
	now := s.now()
	minute := now.UTC().Format("2006-01-02T15:04")
	short := fmt.Sprint(now.Unix() / 10)
	for _, r := range []struct {
		k, w string
		n    int64
	}{{"credential-" + cred.MCPTokenID, minute, 60}, {"burst-" + cred.MCPTokenID, short, 10}, {"account", minute, 120}} {
		if e = s.mcpRate(ctx, cred.OwnerID, r.k, r.w, r.n, 1); e != nil {
			return s.mcpAttempt(ctx, token, tool, raw, e)
		}
	}
	if tool == "get_execution_evidence" {
		in, _ := wire.Parse(tool, raw)
		if in.View == "complete" {
			if e = s.mcpRate(ctx, cred.OwnerID, "evidence-"+cred.MCPTokenID, minute, 10, 1); e != nil {
				return s.mcpAttempt(ctx, token, tool, raw, e)
			}
		}
	}
	for i := 0; i < 3; i++ {
		env, until, e := s.mcpAttempt(ctx, token, tool, raw, nil)
		if pinConflict(e) {
			continue
		}
		return env, until, e
	}
	return wire.Envelope{}, "", safeError()
}
func (s *Service) mcpAttempt(ctx context.Context, token, tool string, raw []byte, forced error) (wire.Envelope, string, error) {
	tap := &mcpReadSet{Database: s.DB, table: s.Table, items: map[string]map[string]dt.AttributeValue{}}
	ss := *s
	ss.DB = tap
	stage, c, e := ss.authenticateMCP(ctx, token)
	if e != nil {
		return wire.Envelope{}, "", e
	}
	readStart := s.now()
	in, parseErr := wire.Parse(tool, raw)
	var data *wire.Data
	var readErr error
	if forced != nil {
		readErr = forced
	} else if parseErr != nil {
		readErr = failure(400, "invalid_request")
	} else if in.ProjectID != "" && !slices.Contains(c.MCPProjectIDs, in.ProjectID) {
		readErr = failure(404, "not_found")
	} else {
		data, readErr = ss.mcpData(ctx, c, tool, in, tap)
	}
	now := s.now()
	if c.ExpiresAt <= stamp(now.Add(5*time.Second)) {
		return wire.Envelope{}, "", failure(401, "unauthenticated")
	}
	until := now.Add(5 * time.Second)
	// Every retained row influencing the result must remain logically live at release.
	for _, m := range tap.items {
		var r row
		_ = attributevalue.UnmarshalMap(m, &r)
		if r.Kind == "executionRetention" && r.State == "live" && !r.Pinned && r.NormalExpiresAt > stamp(readStart) {
			expiry, er := time.Parse(time.RFC3339Nano, r.NormalExpiresAt)
			if er != nil {
				return wire.Envelope{}, "", safeError()
			}
			if !expiry.After(now) {
				return wire.Envelope{}, "", failure(404, "not_found")
			}
			if expiry.Before(until) {
				until = expiry
			}
		}
	}
	env := wire.Envelope{Data: data}
	code := "ok"
	if readErr != nil {
		code = mcpCode(readErr)
		env = wire.Envelope{Error: MCPError(readErr)}
	} else {
		data.ServerTime = stamp(now)
		data.SchemaVersion = 1
		if !wire.Fits(env) {
			readErr = failure(413, "evidence_too_large")
			code = "evidence_too_large"
			env = wire.Envelope{Error: MCPError(readErr)}
		}
	}
	pk := "U#" + c.OwnerID
	project := ""
	if readErr == nil && in.ProjectID != "" {
		pk = "P#" + in.ProjectID
		project = in.ProjectID
	}
	audit := row{PK: pk, SK: "AUDIT#" + stamp(now) + "#" + id(), Kind: "executionMCPAudit", SchemaVersion: 1, OwnerID: c.OwnerID, ProjectID: project, MCPTokenID: c.MCPTokenID, MCPTool: tool, MCPCode: code, MCPRequestID: id(), CreatedAt: stamp(now), ExpiresAt: stamp(now.Add(90 * 24 * time.Hour)), DPK: dueShard(c.OwnerID), DSK: stamp(now.Add(90*24*time.Hour)) + "#" + id()}
	if project == "" {
		audit.Kind = "mcpAudit"
	}
	if !slices.Contains(wire.Names, tool) {
		audit.MCPTool = "invalid"
	}
	envError := env.Error
	if envError != nil {
		envError.RequestID = audit.MCPRequestID
	}
	actions, e := tap.guards(s)
	if e != nil {
		return wire.Envelope{}, "", e
	}
	put, e := s.put(s.Table, audit, true)
	if e != nil {
		return wire.Envelope{}, "", e
	}
	actions = append(actions, put)
	// Rate accounting precedes disclosure; rejected/uncertain reads may consume budget.
	bytes, _ := json.Marshal(env)
	if data != nil && readErr == nil {
		if e = s.mcpRate(ctx, c.OwnerID, "bytes", now.UTC().Format("2006-01-02T15:04"), 16*1024*1024, int64(len(bytes)*3+1024)); e != nil {
			return s.mcpAttempt(ctx, token, tool, raw, e)
		}
	}
	if e = s.transact(ctx, actions); e != nil {
		return wire.Envelope{}, "", e
	}
	if s.now().After(until) {
		return wire.Envelope{}, "", safeError()
	}
	_ = stage
	return env, stamp(until), nil
}
func mcpCode(e error) string {
	var v *Error
	if errors.As(e, &v) {
		return v.Code
	}
	return "service_unavailable"
}
func MCPError(e error) *wire.Error {
	code := mcpCode(e)
	msg := map[string]string{"internal_error": "Unexpected service failure.", "invalid_request": "Invalid read request.", "unauthenticated": "Valid MCP credential required.", "not_found": "Resource not found.", "conflict": "Evidence is not published.", "payload_too_large": "Request too large.", "evidence_too_large": "Complete evidence exceeds the MCP limit; request summary view.", "rate_limited": "Read budget exceeded; wait before retrying.", "evidence_unavailable": "Published evidence unavailable.", "service_unavailable": "Service temporarily unavailable."}[code]
	if msg == "" {
		code = "service_unavailable"
		msg = "Service temporarily unavailable."
	}
	retry := 0
	if code == "rate_limited" {
		retry = 60
	}
	if code == "service_unavailable" {
		retry = 2
	}
	return &wire.Error{Code: code, Message: msg, Details: []any{}, RequestID: id(), RetryAfterSeconds: retry}
}
func (s *Service) mcpData(ctx context.Context, c row, tool string, in wire.Input, tap *mcpReadSet) (*wire.Data, error) {
	out := &wire.Data{Items: []json.RawMessage{}}
	if in.ProjectID != "" {
		if _, e := s.project(ctx, c.OwnerID, in.ProjectID); e != nil {
			return nil, e
		}
	}
	switch tool {
	case "get_execution":
		v, e := s.Detail(ctx, c.OwnerID, in.ProjectID, in.ExecutionID)
		if e != nil {
			return nil, e
		}
		x := mcpExecution(v)
		out.Execution = &x
		return out, nil
	case "get_workflow_run":
		r, e := s.RunDetail(ctx, c.OwnerID, in.ProjectID, in.WorkflowRunID)
		if e != nil {
			return nil, e
		}
		v := mcpRun(in.ProjectID, r, true)
		out.Run = &v
		return out, nil
	case "get_execution_evidence":
		before, e := s.Detail(ctx, c.OwnerID, in.ProjectID, in.ExecutionID)
		if e != nil {
			return nil, e
		}
		if before.Retention == nil {
			return nil, failure(409, "conflict")
		}
		if before.Retention.EvidenceAvailability != "available" {
			return nil, failure(503, "evidence_unavailable")
		}
		raw, e := s.Evidence(ctx, c.OwnerID, in.ProjectID, in.ExecutionID)
		if e != nil {
			return nil, e
		}
		var stored struct {
			Data struct {
				Evidence Evidence `json:"evidence"`
			} `json:"data"`
		}
		if StrictJSON(raw, &stored) != nil {
			return nil, failure(503, "evidence_unavailable")
		}
		capture := stored.Data.Evidence
		if runner.ValidateEvidenceVersion(&capture) != nil || capture.ProjectID != in.ProjectID || capture.ExecutionID != in.ExecutionID {
			return nil, failure(503, "evidence_unavailable")
		}
		v, e := s.Detail(ctx, c.OwnerID, in.ProjectID, in.ExecutionID)
		if e != nil {
			return nil, e
		}
		if v.Retention == nil || v.Retention.EvidenceAvailability != "available" {
			return nil, failure(503, "evidence_unavailable")
		}
		delivery := &wire.Evidence{View: in.View, Retention: mcpRetention(v.Retention), OmittedFields: []string{}}
		out.Evidence = delivery
		if in.View == "complete" {
			b, _ := json.Marshal(capture.LegacyProjection())
			if len(b) > wire.CaptureLimit {
				return nil, failure(413, "evidence_too_large")
			}
			delivery.Complete = true
			legacy := capture.LegacyProjection()
			delivery.Capture = &legacy
			if capture.SchemaVersion == 2 {
				delivery.OmissionReason = pointer("assertions_not_exposed")
				delivery.OmittedFields = []string{"assertions"}
			}
		} else {
			delivery.OmissionReason = pointer("mcp_summary_view")
			delivery.OmittedFields = []string{"request.url", "request.headers", "request.body.text", "response.headers", "response.body.text"}
			if capture.SchemaVersion == 2 {
				delivery.OmittedFields = append(delivery.OmittedFields, "assertions")
			}
			delivery.Summary = &wire.CaptureSummary{ProjectID: capture.ProjectID, ExecutionID: capture.ExecutionID, Status: capture.Status, Outcome: capture.Outcome, SubmittedAt: capture.SubmittedAt, StartedAt: capture.StartedAt, CompletedAt: capture.CompletedAt, HTTPStatus: capture.Response.HTTPStatus, Timing: capture.Timing, RequestBody: wire.BodySummary{Kind: capture.Request.Body.Kind, OmissionReason: capture.Request.Body.OmissionReason}, ResponseBody: wire.BodySummary{Kind: capture.Response.Body.Kind, OmissionReason: capture.Response.Body.OmissionReason}, Redaction: capture.Redaction}
		}
		return out, nil
	}
	cur := mcpCursor{Credential: c.MCPTokenID, Version: c.Version, Generation: c.RecoveryGeneration, Tool: tool, Project: in.ProjectID, Filter: in.RequestID + "/" + in.Status + "/" + in.WorkflowID + "/" + in.State, High: stamp(s.now()) + "#~", Expires: s.now().Add(24 * time.Hour).Unix()}
	if in.Cursor != "" {
		var decoded mcpCursor
		if e := s.mcpCursorDecode(in.Cursor, &decoded); e != nil {
			return nil, e
		}
		if decoded.Credential != cur.Credential || decoded.Version != cur.Version || decoded.Generation != cur.Generation || decoded.Tool != tool || decoded.Project != in.ProjectID || decoded.Filter != cur.Filter || decoded.Expires <= s.now().Unix() || decoded.Expires > s.now().Add(24*time.Hour).Unix() || decoded.High == "" {
			return nil, failure(400, "invalid_request")
		}
		cur = decoded
	}
	if tool == "list_projects" {
		if cur.Offset < 0 || cur.Offset > len(c.MCPProjectIDs) {
			return nil, failure(400, "invalid_request")
		}
		for cur.Offset < len(c.MCPProjectIDs) && len(out.Items) < in.Limit {
			pid := c.MCPProjectIDs[cur.Offset]
			cur.Offset++
			p, e := s.project(ctx, c.OwnerID, pid)
			if missingRow(e) {
				continue
			}
			if e != nil {
				return nil, e
			}
			out.Items = append(out.Items, mcpRaw(wire.Project{ProjectID: pid, Name: p.Name, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt}))
		}
		if cur.Offset < len(c.MCPProjectIDs) {
			out.NextCursor, _ = s.mcpCursorEncode(cur)
		}
		return out, nil
	}
	prefix, collection := "REQ#", "REQUEST"
	if tool == "list_executions" {
		prefix, collection = "EXEC#", "EXEC"
	}
	if tool == "list_workflow_runs" {
		prefix, collection = "RUN#", "RUN"
	}
	var start map[string]dt.AttributeValue
	if cur.LastID != "" {
		if !wire.ValidID(cur.LastID) || cur.LastTime == "" {
			return nil, failure(400, "invalid_request")
		}
		start = map[string]dt.AttributeValue{"PK": str("P#" + in.ProjectID), "SK": str(prefix + cur.LastID), "LPK": str("P#" + in.ProjectID + "#" + collection), "LSK": str(cur.LastTime)}
	}
	for page := 0; page < 5; page++ {
		q, e := s.DB.Query(ctx, &dynamodb.QueryInput{TableName: &s.Table, IndexName: aws.String("GSI1"), KeyConditionExpression: aws.String("LPK = :p AND LSK <= :hi"), ExpressionAttributeValues: map[string]dt.AttributeValue{":p": str("P#" + in.ProjectID + "#" + collection), ":hi": str(cur.High)}, ScanIndexForward: aws.Bool(false), Limit: aws.Int32(25), ExclusiveStartKey: start})
		if e != nil {
			return nil, safeError()
		}
		for idx, item := range q.Items {
			if len(out.Items) >= in.Limit || len(tap.items) > 40 && tool == "list_workflow_runs" || len(tap.items) > 65 {
				out.NextCursor, e = s.mcpCursorEncode(cur)
				return out, e
			}
			var candidate row
			if attributevalue.UnmarshalMap(item, &candidate) != nil || candidate.PK != "P#"+in.ProjectID || !strings.HasPrefix(candidate.SK, prefix) {
				return nil, safeError()
			}
			rid := strings.TrimPrefix(candidate.SK, prefix)
			if !wire.ValidID(rid) {
				return nil, safeError()
			}
			var value any
			switch tool {
			case "list_saved_requests":
				r, er := s.get(ctx, s.Table, candidate.PK, candidate.SK)
				e = er
				if e == nil {
					if r.Kind != "request" || r.ProjectID != in.ProjectID || r.State != "active" {
						e = failure(404, "not_found")
					} else {
						var cfg Configuration
						if json.Unmarshal([]byte(r.ConfigurationJSON), &cfg) != nil {
							return nil, safeError()
						}
						value = wire.Request{ProjectID: in.ProjectID, RequestID: r.RequestID, Name: cfg.Name, Method: cfg.Method, Revision: r.Revision, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
					}
				}
			case "list_executions":
				v, er := s.Detail(ctx, c.OwnerID, in.ProjectID, rid)
				e = er
				if e == nil && (in.RequestID == "" || v.Source.RequestID == in.RequestID) && (in.Status == "" || v.Status == in.Status) {
					value = mcpExecution(v)
				}
			case "list_workflow_runs":
				v, er := s.RunDetail(ctx, c.OwnerID, in.ProjectID, rid)
				e = er
				if e == nil && (in.WorkflowID == "" || v.WorkflowID == in.WorkflowID) && (in.State == "" || v.State == in.State) {
					value = mcpRun(in.ProjectID, v, false)
				}
			default:
				return nil, failure(400, "invalid_request")
			}
			if e != nil && !missingRow(e) {
				return nil, e
			}
			if value != nil {
				out.Items = append(out.Items, mcpRaw(value))
			}
			cur.LastID = rid
			cur.LastTime = candidate.LSK
			if len(out.Items) == in.Limit {
				if idx < len(q.Items)-1 || len(q.LastEvaluatedKey) > 0 {
					out.NextCursor, e = s.mcpCursorEncode(cur)
				}
				return out, e
			}
		}
		if len(q.LastEvaluatedKey) == 0 {
			return out, nil
		}
		start = q.LastEvaluatedKey
	}
	var e error
	out.NextCursor, e = s.mcpCursorEncode(cur)
	return out, e
}
