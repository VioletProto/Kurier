package openapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

const fixture = `{"openapi":"3.0.4","info":{"title":"Test API","version":"1"},"servers":[{"url":"https://example.test/v1"}],"paths":{"/pets/{id}":{"parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"string"}}],"post":{"parameters":[{"name":"tags","in":"query","schema":{"type":"array","items":{"type":"string"}},"example":["blue","green"]},{"name":"X-Region","in":"header","required":true,"schema":{"type":"string","example":"ohio"}}],"requestBody":{"required":true,"content":{"application/json":{"schema":{"type":"object","required":["ratio"],"properties":{"ratio":{"type":"number"}}},"example":{"ratio":0.10000000000000001}}}},"responses":{"200":{"description":"Success","headers":{"X-Trace":{"required":true,"schema":{"type":"string"}}},"content":{"application/json":{"schema":{"type":"object","required":["id"],"properties":{"id":{"type":"integer"},"oneOf":{"type":"string"}}}}}},"204":{"description":"Empty"}}}}}}`

func parsed(t *testing.T) Document {
	t.Helper()
	d, e := Parse([]byte(fixture), "json", Policy{})
	if e != nil {
		t.Fatal(e)
	}
	return d
}
func code(err error) string {
	if e, ok := err.(*Error); ok {
		return e.Code
	}
	if err != nil {
		return "unexpected"
	}
	return ""
}

func TestImportGenerationAndJSONYAMLExportPrecision(t *testing.T) {
	d := parsed(t)
	if len(d.Operations) != 1 {
		t.Fatal("operation missing")
	}
	op := d.Operations[0]
	if _, e := Generate(op, Choices{}); code(e) != "inputs_required" {
		t.Fatal("missing path input accepted", e)
	}
	g, e := Generate(op, Choices{Parameters: map[string]json.RawMessage{"p0": json.RawMessage(`"blue fox/a?b"`)}})
	if e != nil {
		t.Fatal(e)
	}
	if g.URL != "https://example.test/v1/pets/blue%20fox%2Fa%3Fb" || len(g.QueryParameters) != 2 || g.QueryParameters[1].Value != "green" || g.Headers[0].Value != "ohio" || !strings.Contains(g.Body.Text, "0.10000000000000001") {
		t.Fatal("generation changed logical values")
	}
	for _, format := range []string{"json", "yaml"} {
		b, e := Export(d, format)
		if e != nil {
			t.Fatal(e)
		}
		again, e := Parse(b, format, Policy{})
		if e != nil {
			t.Fatal(format, e, string(b))
		}
		generated, e := Generate(again.Operations[0], Choices{Parameters: map[string]json.RawMessage{"p0": json.RawMessage(`"1"`)}})
		if e != nil || !strings.Contains(generated.Body.Text, "0.10000000000000001") {
			t.Fatal("export changed decimal", format, e)
		}
	}
}

func TestStrictSyntaxAndUnsupportedFeatures(t *testing.T) {
	cases := map[string]struct{ raw, format string }{
		"json yaml superset":                {"openapi: 3.0.4", "json"},
		"duplicate JSON":                    {strings.Replace(fixture, `"openapi":"3.0.4"`, `"openapi":"3.0.4","openapi":"3.0.4"`, 1), "json"},
		"unpaired surrogate":                {strings.Replace(fixture, `"Test API"`, `"\ud800"`, 1), "json"},
		"schema composition":                {strings.Replace(fixture, `"ratio":{"type":"number"}`, `"ratio":{"type":"number","oneOf":[]}`, 1), "json"},
		"external ref":                      {strings.Replace(fixture, `"id":{"type":"integer"}`, `"id":{"$ref":"https://example.test/private"}`, 1), "json"},
		"ref sibling":                       {strings.Replace(fixture, `"id":{"type":"integer"}`, `"id":{"$ref":"#/components/schemas/ID","description":"bad"}`, 1), "json"},
		"YAML duplicate":                    {"openapi: 3.0.4\nopenapi: 3.0.4\n", "yaml"},
		"YAML alias":                        {"openapi: 3.0.4\ninfo: &a {title: API, version: '1'}\npaths: *a", "yaml"},
		"YAML tag":                          {"openapi: !private 3.0.4", "yaml"},
		"YAML documents":                    {"openapi: 3.0.4\n---\nopenapi: 3.0.4", "yaml"},
		"YAML nonJSON numeric":              {"openapi: 3.0.4\ninfo: {title: API, version: '1'}\npaths: {}\ncomponents: {examples: {x: {value: 0x10}}}", "yaml"},
		"query array serialization":         {strings.Replace(fixture, `"name":"tags","in":"query"`, `"name":"tags","in":"query","explode":false`, 1), "json"},
		"server credential":                 {strings.Replace(fixture, "https://example.test/v1", "https://user:pass@example.test/v1", 1), "json"},
		"missing required path declaration": {strings.Replace(fixture, `"name":"id","in":"path","required":true`, `"name":"id","in":"path","required":false`, 1), "json"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, e := Parse([]byte(tc.raw), tc.format, Policy{}); e == nil {
				t.Fatal("unsafe input accepted")
			} else if strings.Contains(e.Error(), "private") || strings.Contains(e.Error(), "Test API") {
				t.Fatal("input-dependent error")
			}
		})
	}
	if _, e := Parse(make([]byte, SourceLimit+1), "json", Policy{}); code(e) != "source_limit" {
		t.Fatal(e)
	}
}

func TestLocalReferencesAndCycleBudget(t *testing.T) {
	raw := strings.Replace(fixture, `"id":{"type":"integer"}`, `"id":{"$ref":"#/components/schemas/ID"}`, 1)
	raw = strings.TrimSuffix(raw, "}") + `,"components":{"schemas":{"ID":{"type":"integer"}}}}`
	if _, e := Parse([]byte(raw), "json", Policy{}); e != nil {
		t.Fatal("local ref failed", e)
	}
	for _, component := range []string{`{"ID":{"$ref":"#/components/schemas/ID"}}`, `{"ID":{"$ref":"#/components/schemas/Missing"}}`} {
		bad := strings.Replace(raw, `{"ID":{"type":"integer"}}`, component, 1)
		if _, e := Parse([]byte(bad), "json", Policy{}); e == nil {
			t.Fatal("invalid ref accepted")
		}
	}
}

func TestSecretOmissionBeforeRetentionAndProtectedCompletion(t *testing.T) {
	const canary = "synthetic-private-canary"
	raw := strings.Replace(fixture, `"ratio":0.10000000000000001`, `"ratio":0.10000000000000001,"token":"`+canary+`"`, 1)
	d, e := Parse([]byte(raw), "json", Policy{SensitiveValues: []string{canary}})
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(string(d.Source), canary) || d.Operations[0].RequestBody.Example != nil || len(d.Diagnostics) != 1 {
		t.Fatal("sensitive example retained")
	}
	raw = strings.Replace(fixture, `"name":"X-Region"`, `"name":"X-API-Key"`, 1)
	d, e = Parse([]byte(raw), "json", Policy{})
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(string(d.Source), "ohio") {
		t.Fatal("credential example retained")
	}
	choices := Choices{Parameters: map[string]json.RawMessage{"p0": json.RawMessage(`"one"`)}, SecretRefs: map[string]string{"p2": "a1234567-1234-4234-8234-123456789abc"}}
	g, e := Generate(d.Operations[0], choices)
	if e != nil {
		t.Fatal(e)
	}
	if g.Headers[0].Value != "" || g.Headers[0].SecretRef == nil || !g.Headers[0].Sensitive {
		t.Fatal("protected completion became public")
	}
	choices.Parameters["p2"] = json.RawMessage(`"private"`)
	if _, e = Generate(d.Operations[0], choices); code(e) != "source_unsafe" {
		t.Fatal("plaintext protected input accepted")
	}
}

func TestRelativeOrMissingServerRequiresOwnerInput(t *testing.T) {
	for _, raw := range []string{strings.Replace(fixture, `"servers":[{"url":"https://example.test/v1"}],`, "", 1), strings.Replace(fixture, "https://example.test/v1", "/v1", 1)} {
		d, e := Parse([]byte(raw), "json", Policy{})
		if e != nil {
			t.Fatal(e)
		}
		if len(d.Operations[0].Servers) != 0 {
			t.Fatal("relative URL became executable")
		}
		choices := Choices{Server: "https://example.test/v1", Parameters: map[string]json.RawMessage{"p0": json.RawMessage(`"one"`)}}
		if _, e = Generate(d.Operations[0], choices); e != nil {
			t.Fatal(e)
		}
	}
}

func TestResponseChecksAndNoRawDiagnostics(t *testing.T) {
	op := parsed(t).Operations[0]
	cases := []struct {
		name    string
		status  int
		headers http.Header
		body    string
		want    string
	}{
		{"passed", 200, http.Header{"X-Trace": []string{"ok"}, "Content-Type": []string{"application/json"}}, `{"id":2}`, ""},
		{"status", 201, http.Header{}, `{"id":2}`, "undocumented_status"},
		{"header", 200, http.Header{}, `{"id":2}`, "missing_required_header"},
		{"required", 200, http.Header{"X-Trace": []string{"ok"}, "Content-Type": []string{"application/json"}}, `{}`, "missing_required_property"},
		{"type", 200, http.Header{"X-Trace": []string{"ok"}, "Content-Type": []string{"application/json"}}, `{"id":"synthetic-private-canary"}`, "primitive_type_mismatch"},
		{"fractional integer", 200, http.Header{"X-Trace": []string{"ok"}, "Content-Type": []string{"application/json"}}, `{"id":2.5}`, "primitive_type_mismatch"},
		{"bodyless", 204, http.Header{}, "", ""},
		{"secret", 200, http.Header{"X-Trace": []string{"ok"}, "Content-Type": []string{"application/json"}}, `{"id":2,"password":"synthetic-private-canary"}`, "sensitive_response"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := Validate(op, tc.status, tc.headers, []byte(tc.body), false)
			if !ValidResult(result) {
				t.Fatal("invalid result", result)
			}
			b, _ := json.Marshal(result)
			if strings.Contains(string(b), "synthetic-private-canary") {
				t.Fatal("secret exposed")
			}
			if tc.want == "" && result.Status != "passed" || tc.want != "" && !strings.Contains(string(b), tc.want) {
				t.Fatal("wrong validation", string(b))
			}
		})
	}
	if r := Validate(op, 200, http.Header{}, []byte(`{"id":2}`), true); r.Status != "not_evaluated" || !ValidResult(r) {
		t.Fatal("suppressed response evaluated")
	}
}

func TestOperationCeilingAndTamperedPersistedBundles(t *testing.T) {
	for _, count := range []int{100, 101} {
		paths := []string{}
		for i := 0; i < count; i++ {
			paths = append(paths, fmt.Sprintf(`"/p%d":{"get":{"responses":{"200":{"description":"ok"}}}}`, i))
		}
		raw := `{"openapi":"3.0.4","info":{"title":"API","version":"1"},"paths":{` + strings.Join(paths, ",") + `}}`
		d, e := Parse([]byte(raw), "json", Policy{})
		if count == 100 && (e != nil || len(d.Operations) != count) || count == 101 && code(e) != "operation_limit" {
			t.Fatal(count, e)
		}
	}
	op := parsed(t).Operations[0]
	op.Parameters[0].Schema.Items = &op.Parameters[0].Schema
	if e := ValidateOperation(op); e == nil {
		t.Fatal("cyclic persisted schema accepted")
	}
}

func TestContextAwareAnnotationsReferencesAndSensitiveDefault(t *testing.T) {
	raw := strings.Replace(fixture, `"properties":{"ratio":{"type":"number"}}`, `"properties":{"ratio":{"type":"number"},"description":{"type":"string"},"title":{"type":"string"},"summary":{"type":"string"},"operationId":{"type":"string"},"$ref":{"type":"string"}}`, 1)
	raw = strings.Replace(raw, `"ratio":0.10000000000000001`, `"ratio":0.10000000000000001,"description":"literal","$ref":"literal-data"`, 1)
	d, e := Parse([]byte(raw), "json", Policy{})
	if e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"description", "title", "summary", "operationId", "$ref"} {
		if _, ok := d.Operations[0].RequestBody.Schema.Properties[name]; !ok {
			t.Fatal("literal schema property removed", name)
		}
	}
	if !strings.Contains(string(d.Operations[0].RequestBody.Example), `"$ref":"literal-data"`) {
		t.Fatal("example data interpreted as reference")
	}
	if _, e = Parse(d.Source, "json", Policy{}); e != nil {
		t.Fatal("sanitized source not importable", e)
	}
	for _, format := range []string{"json", "yaml"} {
		exported, e := Export(d, format)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = Parse(exported, format, Policy{}); e != nil {
			t.Fatal("literal source export not importable", format, e)
		}
	}
	raw = strings.Replace(fixture, `"schema":{"type":"string","example":"ohio"}`, `"schema":{"type":"string","example":"fallback"},"examples":{"a":{"value":"synthetic-private-canary"},"b":{"value":"safe-later"}}`, 1)
	d, e = Parse([]byte(raw), "json", Policy{SensitiveValues: []string{"synthetic-private-canary"}})
	if e != nil {
		t.Fatal(e)
	}
	if d.Operations[0].Parameters[2].Example != nil {
		t.Fatal("unsafe selected example silently switched")
	}
	if _, e = Generate(d.Operations[0], Choices{Parameters: map[string]json.RawMessage{"p0": json.RawMessage(`"one"`)}}); code(e) != "inputs_required" {
		t.Fatal("unsafe selection did not require input", e)
	}
	raw = strings.Replace(fixture, `"id":{"type":"integer"}`, `"id":{"type":"integer"},"synthetic-private-canary":{"type":"string"}`, 1)
	if _, e = Parse([]byte(raw), "json", Policy{SensitiveValues: []string{"synthetic-private-canary"}}); code(e) != "source_unsafe" {
		t.Fatal("designated structural key retained", e)
	}
}

func TestResponsePrivacyDefaultStatusAndNullableConstraints(t *testing.T) {
	op := parsed(t).Operations[0]
	s := Schema{Type: "object", Required: []string{"password"}, Properties: map[string]Schema{"password": {Type: "string"}}}
	r := op.Responses["200"]
	r.Schema = &s
	op.Responses["200"] = r
	for _, body := range []string{`{}`, `{"password":"synthetic-canary"}`} {
		result := Validate(op, 200, http.Header{"Content-Type": []string{"application/json"}}, []byte(body), false)
		if result.Status != "not_evaluated" || result.Checks[2].Code != "sensitive_response" {
			t.Fatal("sensitive presence affected validation", result)
		}
	}
	op = parsed(t).Operations[0]
	for _, media := range []string{"text/plain", "", "application/xml"} {
		result := Validate(op, 200, http.Header{"X-Trace": []string{"ok"}, "Content-Type": []string{media}}, []byte(`{"id":2}`), false)
		if result.Checks[2].Code != "body_unavailable" {
			t.Fatal("nonJSON response validated", media, result)
		}
	}
	r = op.Responses["200"]
	op.Responses["default"] = r
	if result := Validate(op, 307, http.Header{"Content-Type": []string{"application/json"}}, []byte(`{}`), false); result.Checks[1].Code != "missing_required_header" || result.Checks[2].Code != "missing_required_property" {
		t.Fatal("redirect/default skipped validation", result)
	}
	s = Schema{Type: "array", Items: &Schema{Type: "integer", Nullable: true}}
	if e := validateValue(s, json.RawMessage(`[null,1]`)); e != nil {
		t.Fatal(e)
	}
	if e := validateValue(s, json.RawMessage(`[null,1.5]`)); e == nil {
		t.Fatal("fractional integer accepted")
	}
	no := false
	s = Schema{Type: "object", AdditionalProperties: &no, Properties: map[string]Schema{"id": {Type: "integer"}}}
	if e := validateValue(s, json.RawMessage(`{"extra":1}`)); e == nil {
		t.Fatal("additional properties weakened")
	}
}

func TestAggregateBudgetCountsUnselectedSchemaAndResponseExamples(t *testing.T) {
	root, _ := strictJSON([]byte(fixture))
	source := object(root)
	operation := object(object(object(source["paths"])["/pets/{id}"])["post"])
	parameters := []any{}
	for _, name := range []string{"a", "b"} {
		parameters = append(parameters, map[string]any{"name": name, "in": "query", "schema": map[string]any{"type": "string"}, "examples": map[string]any{"a": map[string]any{"value": "selected"}, "b": map[string]any{"value": strings.Repeat("x", 2000)}, "c": map[string]any{"value": strings.Repeat("y", 2000)}, "d": map[string]any{"value": strings.Repeat("z", 2000)}}})
	}
	operation["parameters"] = parameters
	if _, e := Parse(encoded(source), "json", Policy{}); e != nil {
		t.Fatal("below-limit examples rejected", e)
	}
	media := object(object(object(object(operation["responses"])["200"])["content"])["application/json"])
	media["example"] = map[string]any{"id": json.Number("2"), "pad": strings.Repeat("p", 5000)}
	if _, e := Parse(encoded(source), "json", Policy{}); code(e) != "example_limit" {
		t.Fatal("response/unselected aggregate limit missing", e)
	}
	delete(media, "example")
	requestMedia := object(object(object(operation["requestBody"])["content"])["application/json"])
	object(requestMedia["schema"])["example"] = map[string]any{"ratio": json.Number("0.1"), "pad": strings.Repeat("p", 5000)}
	if _, e := Parse(encoded(source), "json", Policy{}); code(e) != "example_limit" {
		t.Fatal("unselected schema aggregate limit missing", e)
	}
	delete(object(requestMedia["schema"]), "example")
	object(object(parameters[0])["examples"])["b"] = map[string]any{"value": strings.Repeat("x", 2049)}
	if _, e := Parse(encoded(source), "json", Policy{}); code(e) != "example_limit" {
		t.Fatal("unselected scalar bound missing", e)
	}
}

func TestPartialCompletionCannotHideUnsafeSuppliedChoices(t *testing.T) {
	raw := strings.Replace(fixture, `"name":"X-Region"`, `"name":"X-API-Key"`, 1)
	d, e := Parse([]byte(raw), "json", Policy{})
	if e != nil {
		t.Fatal(e)
	}
	op := d.Operations[0]
	for _, choices := range []Choices{
		{Parameters: map[string]json.RawMessage{"p2": json.RawMessage(`"private"`)}},
		{Parameters: map[string]json.RawMessage{"unknown": json.RawMessage(`"private"`)}},
		{Parameters: map[string]json.RawMessage{"p0": json.RawMessage(`".."`)}},
		{SecretRefs: map[string]string{"unknown": "a1234567-1234-4234-8234-123456789abc"}},
		{Body: json.RawMessage(`{"ratio":0.1,"password":"private"}`)},
	} {
		for _, missingServer := range []bool{false, true} {
			if missingServer {
				op.Servers = nil
			}
			if _, e := Generate(op, choices); e == nil || code(e) == "inputs_required" {
				t.Fatal("unsafe partial completion accepted", choices, e)
			}
		}
	}
}
