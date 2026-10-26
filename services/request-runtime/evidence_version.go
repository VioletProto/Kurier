package execution

import (
	"encoding/json"
	"strings"
)

func (e *Evidence) UnmarshalJSON(raw []byte) error {
	type plain Evidence
	var v plain
	var fields map[string]json.RawMessage
	if StrictJSON(raw, &fields) != nil || StrictJSON(raw, &v) != nil {
		return workflowInvalid()
	}
	for k := range fields {
		if (strings.EqualFold(k, "templateSemanticsVersion") || strings.EqualFold(k, "environmentContext")) && k != "templateSemanticsVersion" && k != "environmentContext" {
			return workflowInvalid()
		}
	}
	if v.SchemaVersion == 3 && fields["templateSemanticsVersion"] == nil {
		return workflowInvalid()
	}
	if v.SchemaVersion < 3 && (fields["templateSemanticsVersion"] != nil || fields["environmentContext"] != nil) || v.SchemaVersion == 3 && (v.TemplateSemanticsVersion != 1 || fields["environmentContext"] == nil) {
		return workflowInvalid()
	}
	if v.SchemaVersion == 1 && fields["assertions"] != nil {
		return workflowInvalid()
	}
	*e = Evidence(v)
	return ValidateEvidenceVersion(e)
}

// LegacyEvidence freezes the existing MCP capture schema. Do not embed Evidence:
// adding owner evidence fields must not expand generated MCP or local schemas.
type LegacyEvidence struct {
	SchemaVersion int             `json:"schemaVersion"`
	ProjectID     string          `json:"projectId"`
	ExecutionID   string          `json:"executionId"`
	Source        Source          `json:"source"`
	Target        string          `json:"target"`
	SubmittedAt   string          `json:"submittedAt"`
	StartedAt     *string         `json:"startedAt"`
	CompletedAt   string          `json:"completedAt"`
	Status        string          `json:"status"`
	Outcome       Outcome         `json:"outcome"`
	Request       RequestCapture  `json:"request"`
	Response      ResponseCapture `json:"response"`
	Timing        Timing          `json:"timing"`
	Redaction     Redaction       `json:"redaction"`
}

func (e Evidence) LegacyProjection() LegacyEvidence {
	return LegacyEvidence{1, e.ProjectID, e.ExecutionID, e.Source, e.Target, e.SubmittedAt, e.StartedAt, e.CompletedAt, e.Status, e.Outcome, e.Request, e.Response, e.Timing, e.Redaction}
}

func ValidateEvidenceVersion(e *Evidence) error {
	if e.SchemaVersion == 3 {
		if e.TemplateSemanticsVersion != 1 || e.EnvironmentContext != nil && (!workflowUUID.MatchString(e.EnvironmentContext.EnvironmentID) || e.EnvironmentContext.Revision < 0 || e.EnvironmentContext.Revision > 9007199254740991) {
			return workflowInvalid()
		}
		if e.Assertions == nil {
			return nil
		}
		copy := *e
		copy.SchemaVersion = 2
		copy.TemplateSemanticsVersion = 0
		copy.EnvironmentContext = nil
		return ValidateEvidenceVersion(&copy)
	}
	if e.TemplateSemanticsVersion != 0 || e.EnvironmentContext != nil {
		return workflowInvalid()
	}
	if e.SchemaVersion == 1 && e.Assertions == nil {
		return nil
	}
	if e.SchemaVersion != 2 || e.Target != "cloud" || e.Assertions == nil {
		return workflowInvalid()
	}
	a := e.Assertions
	raw, err := json.Marshal(a)
	if err != nil || len(raw) > AssertionResultLimit || a.SchemaVersion != 1 || a.Results == nil || len(a.Results) > 8 {
		return workflowInvalid()
	}
	if len(a.Results) == 0 && a.State != "not_configured" || len(a.Results) > 0 && a.State != "evaluated" {
		return workflowInvalid()
	}
	ids := map[string]bool{}
	for _, r := range a.Results {
		if !workflowUUID.MatchString(r.AssertionID) || ids[r.AssertionID] {
			return workflowInvalid()
		}
		ids[r.AssertionID] = true
		if r.Kind != "statusEquals" && r.Kind != "jsonEquals" && r.Kind != "jsonExists" {
			return workflowInvalid()
		}
		valid := false
		switch r.Outcome {
		case "passed":
			valid = r.Kind == "jsonExists" && r.Reason == "exists" || r.Kind != "jsonExists" && r.Reason == "matched"
		case "failed":
			valid = r.Kind == "statusEquals" && r.Reason == "status_mismatch" || r.Kind != "statusEquals" && r.Reason == "path_missing" || r.Kind == "jsonEquals" && (r.Reason == "type_mismatch" || r.Reason == "value_mismatch")
		case "not_evaluated":
			switch r.Reason {
			case "not_dispatched", "evaluation_unavailable":
				valid = true
			case "status_unavailable":
				valid = r.Kind == "statusEquals"
			case "response_unavailable", "body_absent", "body_unsupported", "body_limit_exceeded", "json_invalid", "json_number_unsupported":
				valid = r.Kind != "statusEquals"
			case "sensitive_input":
				valid = r.Kind == "jsonEquals"
			}
		}
		if !valid {
			return workflowInvalid()
		}
	}
	if e.Status == "completed" && !a.Passed() {
		return workflowInvalid()
	}
	return nil
}

func ValidateFrozenAssertions(e *Evidence, step WorkflowStep, semantics int) error {
	if ValidateEvidenceVersion(e) != nil {
		return workflowInvalid()
	}
	if semantics == 0 {
		if (e.SchemaVersion != 1 && e.SchemaVersion != 3) || e.Assertions != nil || len(step.Assertions) != 0 {
			return workflowInvalid()
		}
		return nil
	}
	if semantics != 1 || (e.SchemaVersion != 2 && e.SchemaVersion != 3) || e.Assertions == nil || len(e.Assertions.Results) != len(step.Assertions) {
		return workflowInvalid()
	}
	for i, a := range step.Assertions {
		r := e.Assertions.Results[i]
		if a.AssertionID != r.AssertionID || a.Kind != r.Kind {
			return workflowInvalid()
		}
	}
	return nil
}
