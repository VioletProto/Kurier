// Package mcpwire defines the bounded read-only bridge shared by API and stdio.
package mcpwire

import (
	"bytes"
	"encoding/json"
	"errors"
	runner "github.com/VioletProto/Kurier/services/request-runtime"
	"regexp"
	"slices"
	"strings"
)

const InputLimit = 16 * 1024
const OutputLimit = 256 * 1024
const CaptureLimit = 96 * 1024

var Names = []string{"list_projects", "list_saved_requests", "list_executions", "get_execution", "list_workflow_runs", "get_workflow_run", "get_execution_evidence"}
var uuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func ValidID(s string) bool { return uuid.MatchString(s) }

type Invocation struct {
	Tool  string          `json:"tool"`
	Input json.RawMessage `json:"input"`
}
type Input struct {
	ProjectID     string `json:"projectId,omitempty"`
	RequestID     string `json:"requestId,omitempty"`
	ExecutionID   string `json:"executionId,omitempty"`
	WorkflowID    string `json:"workflowId,omitempty"`
	WorkflowRunID string `json:"workflowRunId,omitempty"`
	Status        string `json:"status,omitempty"`
	State         string `json:"state,omitempty"`
	View          string `json:"view,omitempty"`
	Limit         int    `json:"limit,omitempty"`
	Cursor        string `json:"cursor,omitempty"`
}

func fields(tool string) (required, optional []string) {
	switch tool {
	case "list_projects":
		return nil, []string{"limit", "cursor"}
	case "list_saved_requests":
		return []string{"projectId"}, []string{"limit", "cursor"}
	case "list_executions":
		return []string{"projectId"}, []string{"limit", "cursor", "requestId", "status"}
	case "get_execution":
		return []string{"projectId", "executionId"}, nil
	case "list_workflow_runs":
		return []string{"projectId"}, []string{"limit", "cursor", "workflowId", "state"}
	case "get_workflow_run":
		return []string{"projectId", "workflowRunId"}, nil
	case "get_execution_evidence":
		return []string{"projectId", "executionId"}, []string{"view"}
	}
	return nil, nil
}
func Schema(tool string) map[string]any {
	req, opt := fields(tool)
	props := map[string]any{}
	for _, k := range append(append([]string{}, req...), opt...) {
		p := map[string]any{"type": "string", "minLength": 1, "maxLength": 36, "pattern": uuid.String()}
		switch k {
		case "limit":
			p = map[string]any{"type": "integer", "minimum": 1, "maximum": 25}
			if tool == "list_projects" {
				p["maximum"] = 10
			}
			if tool == "list_workflow_runs" {
				p["maximum"] = 5
			}
		case "cursor":
			p = map[string]any{"type": "string", "minLength": 1, "maxLength": 8192}
		case "view":
			p = map[string]any{"type": "string", "enum": []string{"summary", "complete"}}
		case "status":
			p = map[string]any{"type": "string", "enum": []string{"queued", "claimed", "running", "completed", "failed"}}
		case "state":
			p = map[string]any{"type": "string", "enum": []string{"queued", "running", "completed", "failed"}}
		}
		props[k] = p
	}
	if req == nil {
		req = []string{}
	}
	return map[string]any{"type": "object", "properties": props, "required": req, "additionalProperties": false}
}
func Parse(tool string, raw []byte) (Input, error) {
	bad := errors.New("invalid_request")
	var in Input
	if !slices.Contains(Names, tool) || len(raw) > InputLimit || runner.NonNullObject(raw) != nil || runner.StrictJSON(raw, &in) != nil {
		return in, bad
	}
	var m map[string]json.RawMessage
	_ = json.Unmarshal(raw, &m)
	req, opt := fields(tool)
	for k, v := range m {
		if bytes.Equal(bytes.TrimSpace(v), []byte("null")) || !slices.Contains(req, k) && !slices.Contains(opt, k) {
			return in, bad
		}
		if k != "limit" {
			var v string
			if json.Unmarshal(m[k], &v) != nil || v == "" {
				return in, bad
			}
		}
	}
	for _, k := range req {
		if m[k] == nil {
			return in, bad
		}
	}
	for _, v := range []string{in.ProjectID, in.RequestID, in.ExecutionID, in.WorkflowID, in.WorkflowRunID} {
		if v != "" && !ValidID(v) {
			return in, bad
		}
	}
	max := 25
	if tool == "list_projects" {
		max = 10
	}
	if tool == "list_workflow_runs" {
		max = 5
	}
	if m["limit"] != nil && (in.Limit < 1 || in.Limit > max) || len(in.Cursor) > 8192 || in.View != "" && in.View != "summary" && in.View != "complete" || in.Status != "" && !slices.Contains([]string{"queued", "claimed", "running", "completed", "failed"}, in.Status) || in.State != "" && !slices.Contains([]string{"queued", "running", "completed", "failed"}, in.State) {
		return in, bad
	}
	if in.Limit == 0 {
		in.Limit = max
	}
	if in.View == "" {
		in.View = "summary"
	}
	return in, nil
}

type Credential struct {
	TokenID    string   `json:"tokenId"`
	ProjectIDs []string `json:"projectIds"`
	CreatedAt  string   `json:"createdAt"`
	ExpiresAt  string   `json:"expiresAt"`
	RevokedAt  *string  `json:"revokedAt"`
}
type Error struct {
	Code              string `json:"code"`
	Message           string `json:"message"`
	Details           []any  `json:"details"`
	RequestID         string `json:"requestId"`
	RetryAfterSeconds int    `json:"retryAfterSeconds"`
}
type Envelope struct {
	Data  *Data  `json:"data,omitempty"`
	Error *Error `json:"error,omitempty"`
}
type Data struct {
	SchemaVersion int               `json:"schemaVersion"`
	ServerTime    string            `json:"serverTime"`
	Items         []json.RawMessage `json:"items"`
	NextCursor    *string           `json:"nextCursor"`
	Execution     *Execution        `json:"execution,omitempty"`
	Run           *Run              `json:"run,omitempty"`
	Evidence      *Evidence         `json:"evidence,omitempty"`
}
type Project struct {
	ProjectID string `json:"projectId"`
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}
type Request struct {
	RequestID string `json:"requestId"`
	ProjectID string `json:"projectId"`
	Name      string `json:"name"`
	Method    string `json:"method"`
	Revision  int64  `json:"revision"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}
type Retention struct {
	NormalExpiresAt      string  `json:"normalExpiresAt"`
	Pinned               bool    `json:"pinned"`
	PinnedAt             *string `json:"pinnedAt"`
	EvidenceAvailability string  `json:"evidenceAvailability"`
	State                string  `json:"state"`
	EffectiveStatus      string  `json:"effectiveStatus"`
}
type Execution struct {
	ProjectID   string          `json:"projectId"`
	ExecutionID string          `json:"executionId"`
	Source      runner.Source   `json:"source"`
	RunID       *string         `json:"runId"`
	Position    *int            `json:"position"`
	Target      string          `json:"target"`
	Status      string          `json:"status"`
	Version     int64           `json:"version"`
	SubmittedAt string          `json:"submittedAt"`
	StartedAt   *string         `json:"startedAt"`
	CompletedAt *string         `json:"completedAt"`
	Summary     *runner.Summary `json:"summary"`
	Retention   *Retention      `json:"retention"`
}
type Run struct {
	ProjectID        string  `json:"projectId"`
	WorkflowRunID    string  `json:"workflowRunId"`
	WorkflowID       string  `json:"workflowId"`
	WorkflowRevision int64   `json:"workflowRevision"`
	Target           string  `json:"target"`
	State            string  `json:"state"`
	Version          int64   `json:"version"`
	SubmittedAt      string  `json:"submittedAt"`
	StartedAt        *string `json:"startedAt"`
	CompletedAt      *string `json:"completedAt"`
	NormalExpiresAt  *string `json:"normalExpiresAt"`
	CurrentPosition  int     `json:"currentPosition"`
	FailedPosition   *int    `json:"failedPosition"`
	Steps            []Step  `json:"steps,omitempty"`
}
type Step struct {
	StepID          string          `json:"stepId"`
	Position        int             `json:"position"`
	RequestID       string          `json:"requestId"`
	RequestRevision int64           `json:"requestRevision"`
	State           string          `json:"state"`
	ExecutionID     *string         `json:"executionId"`
	Summary         *runner.Summary `json:"summary"`
	Retention       *Retention      `json:"retention"`
	EvidenceExpired bool            `json:"evidenceExpired"`
}
type Evidence struct {
	View           string           `json:"view"`
	Complete       bool             `json:"complete"`
	Truncated      bool             `json:"truncated"`
	OmissionReason *string          `json:"omissionReason"`
	OmittedFields  []string         `json:"omittedFields"`
	Retention      *Retention       `json:"retention"`
	Capture        *runner.Evidence `json:"capture,omitempty"`
	Summary        *CaptureSummary  `json:"summary,omitempty"`
}
type CaptureSummary struct {
	ProjectID    string           `json:"projectId"`
	ExecutionID  string           `json:"executionId"`
	Status       string           `json:"status"`
	Outcome      runner.Outcome   `json:"outcome"`
	SubmittedAt  string           `json:"submittedAt"`
	StartedAt    *string          `json:"startedAt"`
	CompletedAt  string           `json:"completedAt"`
	HTTPStatus   *int             `json:"httpStatus"`
	Timing       runner.Timing    `json:"timing"`
	RequestBody  BodySummary      `json:"requestBody"`
	ResponseBody BodySummary      `json:"responseBody"`
	Redaction    runner.Redaction `json:"redaction"`
}
type BodySummary struct {
	Kind           string  `json:"kind"`
	OmissionReason *string `json:"omissionReason"`
}

// Fits counts duplicate structured/text content and worst-case bounded JSON-RPC ID.
func Fits(v any) bool {
	raw, e := json.Marshal(v)
	if e != nil {
		return false
	}
	outer := map[string]any{"jsonrpc": "2.0", "id": string(bytes.Repeat([]byte("x"), 128)), "result": map[string]any{"resultType": "complete", "structuredContent": json.RawMessage(raw), "content": []any{map[string]any{"type": "text", "text": string(raw)}}, "isError": false}}
	b, e := json.Marshal(outer)
	return e == nil && len(b)+1 <= OutputLimit
}

// Decode rejects unexpected fields, including in list items. No internal row is forwarded.
func Decode(tool string, raw []byte) (Envelope, error) {
	var env Envelope
	if len(raw) > OutputLimit || runner.StrictJSON(raw, &env) != nil {
		return env, errors.New("service_unavailable")
	}
	if (env.Data == nil) == (env.Error == nil) {
		return env, errors.New("service_unavailable")
	}
	if env.Error != nil {
		if len(env.Error.Details) != 0 || len(env.Error.Message) > 256 || len(env.Error.RequestID) > 128 {
			return env, errors.New("service_unavailable")
		}
		return env, nil
	}
	bad := errors.New("service_unavailable")
	d := env.Data
	if strings.HasPrefix(tool, "list_") {
		if d.Items == nil || d.Execution != nil || d.Run != nil || d.Evidence != nil {
			return env, bad
		}
	} else {
		if len(d.Items) > 0 || d.NextCursor != nil {
			return env, bad
		}
		switch tool {
		case "get_execution":
			if d.Execution == nil || d.Run != nil || d.Evidence != nil {
				return env, bad
			}
		case "get_workflow_run":
			if d.Run == nil || d.Execution != nil || d.Evidence != nil {
				return env, bad
			}
		case "get_execution_evidence":
			if d.Evidence == nil || d.Run != nil || d.Execution != nil {
				return env, bad
			}
			v := d.Evidence
			if v.Truncated || v.View == "complete" && (!v.Complete || v.Capture == nil || v.Summary != nil) || v.View == "summary" && (v.Complete || v.Summary == nil || v.Capture != nil) || v.View != "complete" && v.View != "summary" {
				return env, bad
			}
		default:
			return env, bad
		}
	}
	if env.Data.SchemaVersion != 1 || !Fits(env) {
		return env, errors.New("service_unavailable")
	}
	for _, b := range env.Data.Items {
		var v any
		switch tool {
		case "list_projects":
			v = &Project{}
		case "list_saved_requests":
			v = &Request{}
		case "list_executions":
			v = &Execution{}
		case "list_workflow_runs":
			v = &Run{}
		default:
			return env, errors.New("service_unavailable")
		}
		if runner.StrictJSON(b, v) != nil {
			return env, errors.New("service_unavailable")
		}
	}
	return env, nil
}
