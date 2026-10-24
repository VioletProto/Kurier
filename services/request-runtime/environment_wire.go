package execution

import (
	"encoding/json"
	"reflect"
	"strings"
)

// Explicit null context is required by new versions; legacy serializers retain
// their exact field order and omitted fields by bypassing this helper.
func withEnvironmentContext(raw []byte, ctx *EnvironmentContext) ([]byte, error) {
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		return nil, failure(400, "invalid_request")
	}
	value, e := json.Marshal(ctx)
	if e != nil {
		return nil, e
	}
	m["environmentContext"] = value
	return json.Marshal(m)
}
func (e Evidence) MarshalJSON() ([]byte, error) {
	type plain Evidence
	raw, err := json.Marshal(plain(e))
	if err != nil || e.SchemaVersion != 3 {
		return raw, err
	}
	return withEnvironmentContext(raw, e.EnvironmentContext)
}
func (v View) MarshalJSON() ([]byte, error) {
	type plain View
	raw, err := json.Marshal(plain(v))
	if err != nil || v.TemplateSemanticsVersion != 1 {
		return raw, err
	}
	return withEnvironmentContext(raw, v.EnvironmentContext)
}
func (w Work) MarshalJSON() ([]byte, error) {
	type plain Work
	raw, err := json.Marshal(plain(w))
	if err != nil || w.SchemaVersion != 2 {
		return raw, err
	}
	return withEnvironmentContext(raw, w.EnvironmentContext)
}
func (r Result) MarshalJSON() ([]byte, error) {
	type plain Result
	raw, err := json.Marshal(plain(r))
	if err != nil || r.SchemaVersion != 2 {
		return raw, err
	}
	return withEnvironmentContext(raw, r.EnvironmentContext)
}
func validateContextVersion(fields map[string]json.RawMessage, version, semantics int) error {
	for k := range fields {
		if (strings.EqualFold(k, "templateSemanticsVersion") || strings.EqualFold(k, "environmentContext")) && k != "templateSemanticsVersion" && k != "environmentContext" {
			return failure(400, "invalid_request")
		}
	}
	if version == 1 {
		if fields["templateSemanticsVersion"] != nil || fields["environmentContext"] != nil {
			return failure(400, "invalid_request")
		}
		return nil
	}
	if version != 2 || semantics != 1 || fields["templateSemanticsVersion"] == nil || fields["environmentContext"] == nil {
		return failure(400, "invalid_request")
	}
	return nil
}
func (w *Work) UnmarshalJSON(raw []byte) error {
	type plain Work
	var v plain
	var fields map[string]json.RawMessage
	if StrictJSON(raw, &fields) != nil || StrictJSON(raw, &v) != nil {
		return failure(400, "invalid_request")
	}
	if e := validateContextVersion(fields, v.SchemaVersion, v.TemplateSemanticsVersion); e != nil {
		return e
	}
	if v.SchemaVersion == 2 {
		required := strings.Fields("schemaVersion projectId jobId executionId localAgentId recoveryGeneration leaseId fence nonce dispatchIntentAt executeNotAfter deadline plan protectedInputs destination templateSemanticsVersion environmentContext")
		if len(fields) != len(required) {
			return failure(400, "invalid_request")
		}
		for _, k := range required {
			if fields[k] == nil || k != "environmentContext" && string(fields[k]) == "null" {
				return failure(400, "invalid_request")
			}
		}
	}
	*w = Work(v)
	return ValidateWorkContext(*w)
}
func (r *Result) UnmarshalJSON(raw []byte) error {
	type plain Result
	var v plain
	var fields map[string]json.RawMessage
	if StrictJSON(raw, &fields) != nil || StrictJSON(raw, &v) != nil {
		return failure(400, "invalid_request")
	}
	if e := validateContextVersion(fields, v.SchemaVersion, v.TemplateSemanticsVersion); e != nil {
		return e
	}
	if v.SchemaVersion == 2 {
		required := strings.Fields("schemaVersion leaseId fence nonce capture templateSemanticsVersion environmentContext")
		if len(fields) != len(required) {
			return failure(400, "invalid_request")
		}
		for _, k := range required {
			if fields[k] == nil || k != "environmentContext" && string(fields[k]) == "null" {
				return failure(400, "invalid_request")
			}
		}
	}
	*r = Result(v)
	return nil
}
func ValidateWorkContext(w Work) error {
	if w.SchemaVersion == 1 && w.TemplateSemanticsVersion == 0 && w.EnvironmentContext == nil && w.Plan.TemplateSnapshot == nil && w.Plan.Configuration.TemplateSemanticsVersion == 0 {
		return nil
	}
	if w.SchemaVersion != 2 || w.TemplateSemanticsVersion != 1 || w.Plan.Configuration.TemplateSemanticsVersion != 1 || w.Plan.TemplateSnapshot.Validate() != nil || !reflect.DeepEqual(w.EnvironmentContext, w.Plan.TemplateSnapshot.EnvironmentContext) {
		return failure(400, "invalid_request")
	}
	return nil
}
func ValidateResultContext(r Result, p Plan) error {
	if p.TemplateSnapshot == nil {
		if r.SchemaVersion == 1 && r.TemplateSemanticsVersion == 0 && r.EnvironmentContext == nil {
			return nil
		}
		return failure(400, "invalid_request")
	}
	if r.SchemaVersion != 2 || r.TemplateSemanticsVersion != 1 || !reflect.DeepEqual(r.EnvironmentContext, p.TemplateSnapshot.EnvironmentContext) {
		return failure(400, "invalid_request")
	}
	return nil
}

type AgentCapabilities struct {
	SchemaVersion              int   `json:"schemaVersion"`
	WorkVersions               []int `json:"workVersions"`
	ResultVersions             []int `json:"resultVersions"`
	TemplateSemanticsVersions  []int `json:"templateSemanticsVersions"`
	EnvironmentContextVersions []int `json:"environmentContextVersions"`
}

func EnvironmentAgentCapabilities() *AgentCapabilities {
	return &AgentCapabilities{1, []int{1, 2}, []int{1, 2}, []int{1}, []int{1}}
}
func (c *AgentCapabilities) Valid() bool { return reflect.DeepEqual(c, EnvironmentAgentCapabilities()) }

func (c *AgentCapabilities) UnmarshalJSON(raw []byte) error {
	type plain AgentCapabilities
	var v plain
	var fields map[string]json.RawMessage
	if StrictJSON(raw, &v) != nil || StrictJSON(raw, &fields) != nil || len(fields) != 5 {
		return failure(400, "invalid_request")
	}
	for _, k := range []string{"schemaVersion", "workVersions", "resultVersions", "templateSemanticsVersions", "environmentContextVersions"} {
		if fields[k] == nil || string(fields[k]) == "null" {
			return failure(400, "invalid_request")
		}
	}
	*c = AgentCapabilities(v)
	if !c.Valid() {
		return failure(400, "invalid_request")
	}
	return nil
}
