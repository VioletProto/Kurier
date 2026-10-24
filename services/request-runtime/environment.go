package execution

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"
)

// Primitive keeps numbers as authored decimal text across every public boundary.
type Primitive struct {
	Type    string          `json:"type"`
	Value   json.RawMessage `json:"value,omitempty"`
	Decimal string          `json:"decimal,omitempty"`
}

func (p *Primitive) UnmarshalJSON(raw []byte) error {
	type plain Primitive
	var v plain
	var fields map[string]json.RawMessage
	if StrictJSON(raw, &fields) != nil || StrictJSON(raw, &v) != nil {
		return failure(400, "variable_type")
	}
	expected := 1
	if v.Type == "number" {
		expected = 2
		if fields["decimal"] == nil || fields["value"] != nil {
			return failure(400, "variable_type")
		}
	}
	if v.Type == "string" || v.Type == "boolean" {
		expected = 2
		if fields["value"] == nil || fields["decimal"] != nil {
			return failure(400, "variable_type")
		}
	}
	if fields["type"] == nil || len(fields) != expected {
		return failure(400, "variable_type")
	}
	*p = Primitive(v)
	_, e := p.Raw()
	return e
}
func (p Primitive) Raw() ([]byte, error) {
	if p.Type == "number" && p.Value != nil || p.Type != "number" && p.Decimal != "" {
		return nil, failure(400, "variable_type")
	}
	var raw []byte
	switch p.Type {
	case "number":
		if _, e := ParseAssertionDecimal(p.Decimal); e != nil {
			return nil, failure(400, "variable_type")
		}
		raw = []byte(p.Decimal)
	case "null":
		if len(p.Value) > 0 || p.Decimal != "" {
			return nil, failure(400, "variable_type")
		}
		raw = []byte("null")
	case "string", "boolean":
		var v any
		if StrictJSON(p.Value, &v) != nil {
			return nil, failure(400, "variable_type")
		}
		switch v.(type) {
		case string:
			if p.Type != "string" {
				return nil, failure(400, "variable_type")
			}
		case bool:
			if p.Type != "boolean" {
				return nil, failure(400, "variable_type")
			}
		default:
			return nil, failure(400, "variable_type")
		}
		raw = append([]byte(nil), p.Value...)
	default:
		return nil, failure(400, "variable_type")
	}
	if len(raw) > 2048 {
		return nil, failure(413, "variable_limit")
	}
	return raw, nil
}
func ValidateTypedSecret(kind, scalarType, value string) error {
	if kind == "string" && scalarType == "string" {
		if value == "" {
			return failure(400, "variable_type")
		}
		raw, _ := json.Marshal(value)
		_, e := (Primitive{Type: "string", Value: raw}).Raw()
		return e
	}
	if kind != "jsonScalar" || scalarType == "string" {
		return failure(400, "variable_type")
	}
	var v any
	if len(value) > 2048 || StrictJSON([]byte(value), &v) != nil {
		return failure(400, "variable_type")
	}
	typ := ""
	switch x := v.(type) {
	case json.Number:
		typ = "number"
		if _, e := ParseAssertionDecimal(string(x)); e != nil {
			return failure(400, "variable_type")
		}
	case bool:
		typ = "boolean"
	case nil:
		typ = "null"
	}
	if typ != scalarType || typ == "" {
		return failure(400, "variable_type")
	}
	return nil
}

type EnvironmentContext struct {
	EnvironmentID string `json:"environmentId"`
	Revision      int64  `json:"revision"`
}

func (c *EnvironmentContext) UnmarshalJSON(raw []byte) error {
	type plain EnvironmentContext
	var v plain
	var m map[string]json.RawMessage
	if StrictJSON(raw, &m) != nil || StrictJSON(raw, &v) != nil || len(m) != 2 || m["environmentId"] == nil || m["revision"] == nil || bytes.Equal(m["revision"], []byte("null")) || !workflowUUID.MatchString(v.EnvironmentID) || v.Revision < 0 || v.Revision > 9007199254740991 {
		return failure(400, "invalid_request")
	}
	*c = EnvironmentContext(v)
	return nil
}

type FrozenVariable struct {
	VariableID string     `json:"variableId"`
	Name       string     `json:"name"`
	ValueType  string     `json:"valueType"`
	Sensitive  bool       `json:"sensitive"`
	Value      *Primitive `json:"value,omitempty"`
	SecretRef  *Ref       `json:"secretRef,omitempty"`
}
type TemplateSnapshot struct {
	SchemaVersion      int                 `json:"schemaVersion"`
	EnvironmentContext *EnvironmentContext `json:"environmentContext"`
	Variables          []FrozenVariable    `json:"variables"`
}

func (s *TemplateSnapshot) Validate() error {
	if s != nil && s.EnvironmentContext != nil && (!workflowUUID.MatchString(s.EnvironmentContext.EnvironmentID) || s.EnvironmentContext.Revision < 0 || s.EnvironmentContext.Revision > 9007199254740991) {
		return failure(400, "source_unavailable")
	}
	if s == nil || s.SchemaVersion != 1 || s.Variables == nil || len(s.Variables) > 32 || s.EnvironmentContext == nil && len(s.Variables) > 0 {
		return failure(400, "source_unavailable")
	}
	names := map[string]bool{}
	ids := map[string]bool{}
	previous := ""
	for _, v := range s.Variables {
		if !workflowUUID.MatchString(v.VariableID) || !variableName.MatchString(v.Name) || names[v.Name] || ids[v.VariableID] || v.Name < previous {
			return failure(400, "source_unavailable")
		}
		names[v.Name] = true
		ids[v.VariableID] = true
		previous = v.Name
		if v.Sensitive {
			if v.Value != nil || v.SecretRef == nil || !workflowUUID.MatchString(v.SecretRef.SecretID) || !strings.Contains("|string|number|boolean|null|", "|"+v.ValueType+"|") {
				return failure(400, "source_unavailable")
			}
		} else {
			if v.SecretRef != nil || v.Value == nil || v.Value.Type != v.ValueType {
				return failure(400, "source_unavailable")
			}
			if _, e := v.Value.Raw(); e != nil {
				return e
			}
		}
	}
	return nil
}
func SortFrozenVariables(v []FrozenVariable) {
	sort.Slice(v, func(i, j int) bool { return v[i].Name < v[j].Name })
}
func (c *Configuration) UnmarshalJSON(raw []byte) error {
	type plain Configuration
	var v plain
	var m map[string]json.RawMessage
	if StrictJSON(raw, &m) != nil || StrictJSON(raw, &v) != nil {
		return failure(400, "invalid_request")
	}
	for k := range m {
		if strings.EqualFold(k, "templateSemanticsVersion") && k != "templateSemanticsVersion" {
			return failure(400, "invalid_request")
		}
	}
	if marker, ok := m["templateSemanticsVersion"]; ok && (string(marker) != "1" || v.TemplateSemanticsVersion != 1) {
		return failure(400, "invalid_request")
	}
	*c = Configuration(v)
	return nil
}

// EnvironmentStageVersions is the complete accepted capability tuple. It is
// used only by guarded readers; applying it to a stage requires separate review.
func EnvironmentStageVersions() map[string]int64 {
	return map[string]int64{
		"savedRequestsSchemaVersion": 9, "projectEnvironmentsSchemaVersion": 1,
		"protectedSecretsSchemaVersion": 2, "localAgentExecutionsSchemaVersion": 2,
		"cloudExecutionsSchemaVersion": 1, "executionInputsSchemaVersion": 1,
		"evidencePinningSchemaVersion": 1, "sequentialWorkflowsSchemaVersion": 1,
		"mcpReadOnlySchemaVersion": 1, "workflowAssertionsSchemaVersion": 1,
	}
}

type EnvironmentVariable struct {
	VariableID string     `json:"variableId"`
	Name       string     `json:"name"`
	Enabled    bool       `json:"enabled"`
	Sensitive  bool       `json:"sensitive"`
	Value      *Primitive `json:"value,omitempty"`
	ValueType  string     `json:"valueType,omitempty"`
	Masked     bool       `json:"masked,omitempty"`
	SecretRef  *Ref       `json:"secretRef,omitempty"`
}
type Environment struct {
	EnvironmentID string                `json:"environmentId"`
	ProjectID     string                `json:"projectId"`
	Revision      int64                 `json:"revision"`
	Name          string                `json:"name"`
	Variables     []EnvironmentVariable `json:"variables"`
	CreatedAt     string                `json:"createdAt"`
	UpdatedAt     string                `json:"updatedAt"`
}

type StepEnvironment struct {
	Mode          string `json:"mode"`
	EnvironmentID string `json:"environmentId,omitempty"`
	Revision      int64  `json:"revision,omitempty"`
}

func (s *StepEnvironment) UnmarshalJSON(raw []byte) error {
	type plain StepEnvironment
	var v plain
	var m map[string]json.RawMessage
	if StrictJSON(raw, &v) != nil || StrictJSON(raw, &m) != nil {
		return failure(400, "invalid_request")
	}
	if m["mode"] == nil {
		return failure(400, "invalid_request")
	}
	if v.Mode == "select" {
		if len(m) != 3 || m["environmentId"] == nil || m["revision"] == nil || string(m["revision"]) == "null" || !workflowUUID.MatchString(v.EnvironmentID) || v.Revision < 0 || v.Revision > 9007199254740991 {
			return failure(400, "invalid_request")
		}
	} else if (v.Mode != "inherit" && v.Mode != "none") || len(m) != 1 {
		return failure(400, "invalid_request")
	}
	*s = StepEnvironment(v)
	return nil
}
func (s StepEnvironment) MarshalJSON() ([]byte, error) {
	if s.Mode == "select" {
		return json.Marshal(struct {
			Mode          string `json:"mode"`
			EnvironmentID string `json:"environmentId"`
			Revision      int64  `json:"revision"`
		}{s.Mode, s.EnvironmentID, s.Revision})
	}
	return json.Marshal(struct {
		Mode string `json:"mode"`
	}{s.Mode})
}

// Environment selection alone permits explicit null; all other policy nulls
// keep the accepted rejection rule.
func NonNullEnvironmentPolicy(raw []byte) error {
	var fields map[string]json.RawMessage
	if StrictJSON(raw, &fields) != nil || fields == nil {
		return failure(400, "invalid_request")
	}
	for k := range fields {
		if strings.EqualFold(k, "environment") && k != "environment" {
			return failure(400, "invalid_request")
		}
	}
	if string(fields["environment"]) == "null" {
		delete(fields, "environment")
	}
	normalized, _ := json.Marshal(fields)
	return NonNullObject(normalized)
}

func (v *EnvironmentVariable) UnmarshalJSON(raw []byte) error {
	type plain EnvironmentVariable
	var p plain
	var fields map[string]json.RawMessage
	if StrictJSON(raw, &p) != nil || StrictJSON(raw, &fields) != nil {
		return failure(400, "invalid_request")
	}
	required := []string{"variableId", "name", "enabled", "sensitive", "value"}
	if p.Sensitive {
		required = []string{"variableId", "name", "enabled", "sensitive", "valueType", "masked", "secretRef"}
	}
	if len(fields) != len(required) {
		return failure(400, "invalid_request")
	}
	for _, k := range required {
		if fields[k] == nil || string(fields[k]) == "null" {
			return failure(400, "invalid_request")
		}
	}
	*v = EnvironmentVariable(p)
	return nil
}
func (s *TemplateSnapshot) UnmarshalJSON(raw []byte) error {
	type plain TemplateSnapshot
	var p plain
	var fields map[string]json.RawMessage
	if StrictJSON(raw, &p) != nil || StrictJSON(raw, &fields) != nil || len(fields) != 3 || fields["schemaVersion"] == nil || fields["variables"] == nil || fields["environmentContext"] == nil {
		return failure(400, "invalid_request")
	}
	*s = TemplateSnapshot(p)
	return s.Validate()
}
func (e Environment) Validate() error {
	if !workflowUUID.MatchString(e.EnvironmentID) || !workflowUUID.MatchString(e.ProjectID) || e.Revision < 0 || e.Revision > 9007199254740991 || e.Variables == nil || len(e.Variables) > 32 || e.CreatedAt == "" || e.UpdatedAt == "" || len([]rune(e.Name)) < 1 || len([]rune(e.Name)) > 100 {
		return failure(400, "invalid_request")
	}
	names := map[string]bool{}
	ids := map[string]bool{}
	protected := 0
	for _, v := range e.Variables {
		if !workflowUUID.MatchString(v.VariableID) || !variableName.MatchString(v.Name) || names[v.Name] || ids[v.VariableID] {
			return failure(400, "invalid_request")
		}
		names[v.Name] = true
		ids[v.VariableID] = true
		if v.Sensitive {
			protected++
			if protected > 16 || v.Value != nil || v.SecretRef == nil || !workflowUUID.MatchString(v.SecretRef.SecretID) || !v.Masked || !strings.Contains("|string|number|boolean|null|", "|"+v.ValueType+"|") {
				return failure(400, "invalid_request")
			}
		} else {
			if v.SecretRef != nil || v.ValueType != "" || v.Masked || v.Value == nil || credentialName(v.Name) {
				return failure(400, "invalid_request")
			}
			if _, err := v.Value.Raw(); err != nil {
				return err
			}
		}
	}
	raw, err := json.Marshal(e)
	if err != nil || len(raw) > 32*1024 {
		return failure(413, "variable_limit")
	}
	return nil
}

func (s StepEnvironment) Validate() error {
	if s.Mode == "select" {
		if !workflowUUID.MatchString(s.EnvironmentID) || s.Revision < 0 || s.Revision > 9007199254740991 {
			return failure(400, "invalid_request")
		}
		return nil
	}
	if (s.Mode != "inherit" && s.Mode != "none") || s.EnvironmentID != "" || s.Revision != 0 {
		return failure(400, "invalid_request")
	}
	return nil
}

func (v *FrozenVariable) UnmarshalJSON(raw []byte) error {
	type plain FrozenVariable
	var value plain
	var fields map[string]json.RawMessage
	if StrictJSON(raw, &value) != nil || StrictJSON(raw, &fields) != nil || len(fields) != 5 {
		return failure(400, "invalid_request")
	}
	required := []string{"variableId", "name", "valueType", "sensitive", "value"}
	if value.Sensitive {
		required[4] = "secretRef"
	}
	for _, k := range required {
		if fields[k] == nil || string(fields[k]) == "null" {
			return failure(400, "invalid_request")
		}
	}
	*v = FrozenVariable(value)
	return nil
}
