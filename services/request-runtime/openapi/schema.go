package openapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"sort"
	"strconv"
	"strings"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

func schemaFrom(v any) (Schema, error) {
	raw := object(v)
	if raw == nil {
		return Schema{}, fail("schema_invalid")
	}
	s := Schema{Type: str(raw["type"]), Nullable: boolean(raw["nullable"])}
	if a, ok := raw["required"].([]any); ok {
		for _, v := range a {
			s.Required = append(s.Required, str(v))
		}
	}
	if p, ok := raw["properties"].(map[string]any); ok {
		s.Properties = map[string]Schema{}
		for name, v := range p {
			child, e := schemaFrom(v)
			if e != nil {
				return Schema{}, e
			}
			s.Properties[name] = child
		}
	}
	if v, ok := raw["items"]; ok {
		child, e := schemaFrom(v)
		if e != nil {
			return Schema{}, e
		}
		s.Items = &child
	}
	if b, ok := raw["additionalProperties"].(bool); ok {
		s.AdditionalProperties = &b
	}
	work := 0
	if e := validSchema(s, 0, &work); e != nil {
		return Schema{}, e
	}
	if value, ok := raw["example"]; ok {
		if e := validateValue(s, encoded(value)); e != nil {
			return Schema{}, e
		}
	}
	return s, nil
}
func validSchema(s Schema, depth int, work *int) error {
	*work++
	if depth > 24 || *work > 4000 {
		return fail("schema_limit")
	}
	if !strings.Contains("|string|integer|number|boolean|object|array|", "|"+s.Type+"|") {
		return fail("schema_unsupported")
	}
	if s.Type != "object" && (len(s.Properties) > 0 || len(s.Required) > 0 || s.AdditionalProperties != nil) {
		return fail("schema_invalid")
	}
	if s.Type == "array" && s.Items == nil || s.Type != "array" && s.Items != nil {
		return fail("schema_invalid")
	}
	seen := map[string]bool{}
	for _, r := range s.Required {
		if r == "" || len(r) > 128 || seen[r] {
			return fail("schema_invalid")
		}
		seen[r] = true
		if _, ok := s.Properties[r]; !ok {
			return fail("schema_invalid")
		}
	}
	for k, c := range s.Properties {
		if k == "" || len(k) > 128 || unsafeText(k) {
			return fail("schema_invalid")
		}
		if e := validSchema(c, depth+1, work); e != nil {
			return e
		}
	}
	if s.Items != nil {
		return validSchema(*s.Items, depth+1, work)
	}
	return nil
}
func standardSchema(s Schema) map[string]any {
	types := []string{s.Type}
	if s.Nullable {
		types = append(types, "null")
	}
	m := map[string]any{"type": types}
	if len(s.Required) > 0 {
		m["required"] = s.Required
	}
	if s.AdditionalProperties != nil {
		m["additionalProperties"] = *s.AdditionalProperties
	}
	if len(s.Properties) > 0 {
		p := map[string]any{}
		for k, v := range s.Properties {
			p[k] = standardSchema(v)
		}
		m["properties"] = p
	}
	if s.Items != nil {
		m["items"] = standardSchema(*s.Items)
	}
	return m
}
func compile(s Schema) (*jsonschema.Schema, error) {
	work := 0
	if e := validSchema(s, 0, &work); e != nil {
		return nil, e
	}
	b := encoded(standardSchema(s))
	if len(b) > BundleLimit {
		return nil, fail("bundle_limit")
	}
	v, e := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if e != nil {
		return nil, fail("schema_invalid")
	}
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	if e = c.AddResource("urn:kurier:openapi:response", v); e != nil {
		return nil, fail("schema_invalid")
	}
	compiled, e := c.Compile("urn:kurier:openapi:response")
	if e != nil {
		return nil, fail("schema_invalid")
	}
	return compiled, nil
}
func validateValue(s Schema, raw json.RawMessage) error {
	if len(raw) > 8192 {
		return fail("example_limit")
	}
	v, e := strictJSON(raw)
	if e != nil {
		return fail("example_invalid")
	}
	compiled, e := compile(s)
	if e != nil {
		return e
	}
	if e = compiled.Validate(v); e != nil {
		return fail("example_invalid")
	}
	return nil
}

func unavailable(code string) Result {
	return Result{SemanticsVersion: 1, Status: "not_evaluated", Checks: []Check{{"status", "not_evaluated", code}, {"headers", "not_evaluated", code}, {"body", "not_evaluated", code}}}
}

// Validate evaluates raw response bytes without retaining them or exposing
// validator diagnostics. The caller marks any sensitive/reflected response as
// suppressed before this function can compute secret-dependent match bits.
func Validate(op Operation, status int, headers http.Header, body []byte, suppressed bool) Result {
	if ValidateOperation(op) != nil || status < 100 || status > 599 {
		return unavailable("evaluation_unavailable")
	}
	if suppressed {
		return unavailable("sensitive_response")
	}
	result := Result{SemanticsVersion: 1, Status: "passed", Checks: []Check{{Kind: "status", Status: "passed"}, {Kind: "headers", Status: "passed"}, {Kind: "body", Status: "passed"}}}
	response, ok := op.Responses[strconv.Itoa(status)]
	if !ok {
		response, ok = op.Responses["default"]
	}
	if !ok {
		result.Status = "failed"
		result.Checks[0] = Check{"status", "failed", "undocumented_status"}
		result.Checks[1] = Check{"headers", "not_evaluated", "evaluation_unavailable"}
		result.Checks[2] = Check{"body", "not_evaluated", "evaluation_unavailable"}
		return result
	}
	if response.Schema != nil && sensitiveSchema(*response.Schema) {
		return unavailable("sensitive_response")
	}
	names := make([]string, 0, len(response.Headers))
	for k := range response.Headers {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, name := range names {
		if credentialName(name) {
			result.Checks[1] = Check{"headers", "not_evaluated", "sensitive_response"}
			break
		}
		if response.Headers[name].Required && len(headers.Values(name)) == 0 {
			result.Checks[1] = Check{"headers", "failed", "missing_required_header"}
			break
		}
	}
	if response.Schema != nil {
		media, _, mediaError := mime.ParseMediaType(headerValue(headers, "Content-Type"))
		if mediaError != nil || media != "application/json" || len(body) == 0 || len(body) > 2<<20 {
			result.Checks[2] = Check{"body", "not_evaluated", "body_unavailable"}
		} else {
			v, e := strictJSON(body)
			if e != nil {
				result.Checks[2] = Check{"body", "failed", "primitive_type_mismatch"}
			} else if sensitiveExample(v, Policy{}) {
				result.Checks[2] = Check{"body", "not_evaluated", "sensitive_response"}
			} else {
				compiled, e := compile(*response.Schema)
				if e != nil {
					result.Checks[2] = Check{"body", "not_evaluated", "evaluation_unavailable"}
				} else if e = compiled.Validate(v); e != nil {
					code := "primitive_type_mismatch"
					var ve *jsonschema.ValidationError
					if errors.As(e, &ve) {
						code = validationCode(ve)
					}
					result.Checks[2] = Check{"body", "failed", code}
				}
			}
		}
	}
	for _, c := range result.Checks {
		if c.Status != "passed" {
			result.Status = "failed"
			break
		}
	}
	return result
}

func headerValue(headers http.Header, name string) string {
	for k, values := range headers {
		if strings.EqualFold(k, name) && len(values) > 0 {
			return values[0]
		}
	}
	return ""
}
func sensitiveSchema(s Schema) bool {
	for name, child := range s.Properties {
		if credentialName(name) || sensitiveSchema(child) {
			return true
		}
	}
	for _, name := range s.Required {
		if credentialName(name) {
			return true
		}
	}
	return s.Items != nil && sensitiveSchema(*s.Items)
}
func validationCode(v *jsonschema.ValidationError) string {
	// Error text/instance paths are deliberately never read or returned.
	kind := v.ErrorKind.KeywordPath()
	if len(kind) > 0 {
		switch kind[len(kind)-1] {
		case "required":
			return "missing_required_property"
		case "additionalProperties":
			return "additional_property"
		}
	}
	for _, cause := range v.Causes {
		if code := validationCode(cause); code != "primitive_type_mismatch" {
			return code
		}
	}
	return "primitive_type_mismatch"
}

func ValidResult(r Result) bool {
	if r.SemanticsVersion != 1 || len(r.Checks) != 3 {
		return false
	}
	kinds := []string{"status", "headers", "body"}
	overall := "passed"
	for i, c := range r.Checks {
		if c.Kind != kinds[i] {
			return false
		}
		switch c.Status {
		case "passed":
			if c.Code != "" {
				return false
			}
		case "failed":
			overall = "failed"
			if !(i == 0 && c.Code == "undocumented_status" || i == 1 && c.Code == "missing_required_header" || i == 2 && (c.Code == "missing_required_property" || c.Code == "primitive_type_mismatch" || c.Code == "additional_property")) {
				return false
			}
		case "not_evaluated":
			if c.Code == "body_unavailable" && i != 2 {
				return false
			}
			if overall != "failed" {
				overall = "not_evaluated"
			}
			if !strings.Contains("|sensitive_response|not_dispatched|evaluation_unavailable|body_unavailable|", "|"+c.Code+"|") {
				return false
			}
		default:
			return false
		}
	}
	// Configured unavailable checks fail an otherwise dispatched operation.
	return r.Status == overall || r.Status == "failed" && overall == "not_evaluated"
}
