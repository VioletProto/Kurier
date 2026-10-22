// This package is a disposable library experiment, never an application importer.
package selection

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"

	execution "github.com/VioletProto/Kurier/services/request-runtime"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/oasdiff/yaml3"
	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

const sourceLimit = 2 << 20

func fixture(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/pets.yaml")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// Inspect the syntax tree BEFORE reference resolution. In particular, a property
// named "oneOf" is data, whereas a schema keyword oneOf changes semantics.
// This deliberately tiny grammar preserves the ORIGINAL RESTRICTED experiment.
// It is research evidence, not the complete import MVP's feature allowlist.
// request_generation_test.go separately probes the approved generation needs.
var grammar = map[string]map[string]string{
	"root":       {"openapi": "scalar", "info": "info", "servers": "servers", "paths": "paths", "components": "components"},
	"info":       {"title": "scalar", "version": "scalar", "description": "scalar"},
	"server":     {"url": "scalar", "description": "scalar"},
	"path":       {"get": "operation", "post": "operation", "put": "operation", "patch": "operation", "delete": "operation", "summary": "scalar", "description": "scalar"},
	"operation":  {"operationId": "scalar", "summary": "scalar", "description": "scalar", "responses": "responses"},
	"components": {"schemas": "schemas", "responses": "responsesByName"},
	"response":   {"$ref": "ref", "description": "scalar", "headers": "headers", "content": "content"},
	"header":     {"description": "scalar", "required": "scalar", "schema": "schema"},
	"media":      {"schema": "schema"},
	"schema":     {"$ref": "ref", "type": "scalar", "nullable": "scalar", "description": "scalar", "title": "scalar", "required": "strings", "properties": "schemas", "items": "schema", "additionalProperties": "scalar"},
}

func preflight(raw []byte) ([]byte, string) {
	if len(raw) > sourceLimit {
		return nil, "source_limit"
	}
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	var doc, extra yaml.Node
	if dec.Decode(&doc) != nil || dec.Decode(&extra) != io.EOF || len(doc.Content) != 1 {
		return nil, "syntax_invalid"
	}
	nodes, operations := 0, 0
	var visit func(*yaml.Node, string, int) string
	visit = func(n *yaml.Node, kind string, depth int) string {
		nodes++
		if depth > 32 || nodes > 20000 {
			return "structure_limit"
		}
		if n.Kind == yaml.AliasNode || n.Anchor != "" {
			return "yaml_alias_unsupported"
		}
		if kind == "scalar" {
			if n.Kind != yaml.ScalarNode || (n.Tag != "!!str" && n.Tag != "!!bool" && n.Tag != "!!int" && n.Tag != "!!float") {
				return "shape_invalid"
			}
			return ""
		}
		if kind == "ref" {
			if n.Kind != yaml.ScalarNode || n.Tag != "!!str" || !strings.HasPrefix(n.Value, "#/components/") {
				return "reference_unsupported"
			}
			return ""
		}
		if kind == "servers" || kind == "strings" {
			if n.Kind != yaml.SequenceNode {
				return "shape_invalid"
			}
			child := "scalar"
			if kind == "servers" {
				child = "server"
			}
			for _, v := range n.Content {
				if code := visit(v, child, depth+1); code != "" {
					return code
				}
			}
			return ""
		}
		if n.Kind != yaml.MappingNode {
			return "shape_invalid"
		}
		seen := map[string]bool{}
		for i := 0; i < len(n.Content); i += 2 {
			k, v := n.Content[i], n.Content[i+1]
			if k.Kind != yaml.ScalarNode || k.Tag != "!!str" || seen[k.Value] {
				return "key_invalid"
			}
			seen[k.Value] = true
			child := ""
			switch kind {
			case "paths":
				if !strings.HasPrefix(k.Value, "/") || strings.ContainsAny(k.Value, "{}") {
					return "path_unsupported"
				}
				child = "path"
			case "schemas":
				child = "schema"
			case "responsesByName":
				child = "response"
			case "headers":
				child = "header"
			case "content":
				if k.Value != "application/json" {
					return "media_unsupported"
				}
				child = "media"
			case "responses":
				if k.Value != "default" && (len(k.Value) != 3 || k.Value[0] < '1' || k.Value[0] > '5' || k.Value[1] < '0' || k.Value[1] > '9' || k.Value[2] < '0' || k.Value[2] > '9') {
					return "status_unsupported"
				}
				child = "response"
			default:
				child = grammar[kind][k.Value]
			}
			if child == "" {
				return "feature_unsupported"
			}
			if kind == "path" && child == "operation" {
				operations++
				if operations > 100 {
					return "operation_limit"
				}
			}
			if kind == "root" && k.Value == "openapi" && v.Value != "3.0.0" && v.Value != "3.0.1" && v.Value != "3.0.2" && v.Value != "3.0.3" && v.Value != "3.0.4" {
				return "dialect_unsupported"
			}
			if kind == "schema" && k.Value == "additionalProperties" && v.Tag != "!!bool" {
				return "feature_unsupported"
			}
			if kind == "schema" && k.Value == "type" && !strings.Contains("|object|array|string|integer|number|boolean|", "|"+v.Value+"|") {
				return "feature_unsupported"
			}
			if code := visit(v, child, depth+1); code != "" {
				return code
			}
		}
		if seen["$ref"] && len(seen) != 1 {
			return "reference_sibling_unsupported"
		}
		return ""
	}
	if code := visit(doc.Content[0], "root", 0); code != "" {
		return nil, code
	}
	// Preserve numbers as JSON numbers through a YAML-node conversion.
	var value any
	if doc.Decode(&value) != nil {
		return nil, "syntax_invalid"
	}
	b, err := json.Marshal(value)
	if err != nil {
		return nil, "syntax_invalid"
	}
	if code := checkReferences(value); code != "" {
		return nil, code
	}
	return b, ""
}

// Traverse without expanding into a new document. Count repeated dereferences
// as work, and reject cycles rather than relying on a validator's recursion.
func checkReferences(root any) string {
	work := 0
	active := map[string]bool{}
	var walk func(any, int) string
	walk = func(v any, depth int) string {
		work++
		if depth > 32 || work > 20000 {
			return "reference_limit"
		}
		switch x := v.(type) {
		case map[string]any:
			if ref, ok := x["$ref"].(string); ok {
				if active[ref] {
					return "reference_cycle"
				}
				var target any = root
				for _, part := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
					m, ok := target.(map[string]any)
					if !ok {
						return "reference_invalid"
					}
					part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
					target, ok = m[part]
					if !ok {
						return "reference_invalid"
					}
				}
				active[ref] = true
				code := walk(target, depth+1)
				delete(active, ref)
				return code
			}
			for _, child := range x {
				if code := walk(child, depth+1); code != "" {
					return code
				}
			}
		case []any:
			for _, child := range x {
				if code := walk(child, depth+1); code != "" {
					return code
				}
			}
		}
		return ""
	}
	return walk(root, 0)
}

func kin(raw []byte) (*openapi3.T, error) {
	l := openapi3.NewLoader()
	l.IsExternalRefsAllowed = false
	// A custom loader bypasses IsExternalRefsAllowed: it must deny EVERYTHING.
	l.ReadFromURIFunc = func(*openapi3.Loader, *url.URL) ([]byte, error) { return nil, errors.New("reference_denied") }
	d, err := l.LoadFromData(raw)
	if err != nil {
		return nil, err
	}
	return d, d.Validate(context.Background())
}

func TestParserComparison(t *testing.T) {
	y := fixture(t)
	j, code := preflight(y)
	if code != "" {
		t.Fatal(code)
	}
	for name, raw := range map[string][]byte{"yaml": y, "json": j} {
		t.Run(name, func(t *testing.T) {
			d, err := kin(raw)
			if err != nil {
				t.Fatal("kin failed synthetic fixture")
			}
			if d.Paths.Value("/pets").Get.Responses.Status(200).Value.Content["application/json"].Schema.Value.Properties["id"].Value == nil {
				t.Fatal("kin local reference unresolved")
			}
			cfg := datamodel.NewDocumentConfiguration()
			cfg.AllowFileReferences = false
			cfg.AllowRemoteReferences = false
			other, err := libopenapi.NewDocumentWithConfiguration(raw, cfg)
			if err != nil {
				t.Fatal("libopenapi parse failed")
			}
			model, err := other.BuildV3Model()
			if err != nil || model == nil {
				t.Fatal("libopenapi model failed")
			}
			op := model.Model.Paths.PathItems.GetOrZero("/pets").Get
			if op.Responses.Codes.GetOrZero("200").Content.GetOrZero("application/json").Schema.Schema().Properties.GetOrZero("id").Schema().Type[0] != "integer" {
				t.Fatal("libopenapi local reference unresolved")
			}
		})
	}
	for _, raw := range [][]byte{[]byte("{broken"), []byte("openapi: [")} {
		if _, err := kin(raw); err == nil {
			t.Fatal("kin accepted malformed document")
		}
		if _, err := libopenapi.NewDocument(raw); err == nil {
			t.Fatal("libopenapi accepted malformed document")
		}
	}
	// Building a model is not semantic validation.
	bad := bytes.Replace(j, []byte(`"info":{`), []byte(`"info":{"unexpected":true,`), 1)
	if _, err := kin(bad); err == nil {
		t.Fatal("kin accepted unknown structural field")
	}
	other, err := libopenapi.NewDocument(bad)
	if err != nil {
		t.Fatal("unexpected parser failure")
	}
	if _, err = other.BuildV3Model(); err != nil {
		t.Fatal("model build behavior changed: reassess comparison")
	}
}

// Translate ONLY the allowlisted subset, after resolving and bounding refs.
// No arbitrary OAS keywords are forwarded to a standard JSON Schema compiler.
func standardSchema(s *openapi3.Schema, depth int, work *int) (map[string]any, error) {
	*work++
	if s == nil || depth > 32 || *work > 20000 || len(s.Type.Slice()) != 1 {
		return nil, errors.New("schema_limit")
	}
	types := append([]string{}, s.Type.Slice()...)
	if s.Nullable {
		types = append(types, "null")
	}
	out := map[string]any{"type": types}
	if len(s.Required) > 0 {
		out["required"] = s.Required
	}
	if s.AdditionalProperties.Has != nil {
		out["additionalProperties"] = *s.AdditionalProperties.Has
	}
	if len(s.Properties) > 0 {
		props := map[string]any{}
		for name, ref := range s.Properties {
			child, err := standardSchema(ref.Value, depth+1, work)
			if err != nil {
				return nil, err
			}
			props[name] = child
		}
		out["properties"] = props
	}
	if s.Items != nil {
		child, err := standardSchema(s.Items.Value, depth+1, work)
		if err != nil {
			return nil, err
		}
		out["items"] = child
	}
	return out, nil
}

func compiledSchema(s *openapi3.Schema) (*jsonschema.Schema, error) {
	work := 0
	translated, err := standardSchema(s, 0, &work)
	if err != nil {
		return nil, err
	}
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	encoded, err := json.Marshal(translated)
	if err != nil || len(encoded) > 32*1024 {
		return nil, errors.New("bundle_limit")
	}
	normalized, err := jsonschema.UnmarshalJSON(bytes.NewReader(encoded))
	if err != nil {
		return nil, err
	}
	if err = c.AddResource("urn:selection:response", normalized); err != nil {
		return nil, err
	}
	return c.Compile("urn:selection:response")
}

func supportedNumbers(v any) bool {
	switch x := v.(type) {
	case json.Number:
		_, err := execution.ParseAssertionDecimal(string(x))
		return err == nil
	case map[string]any:
		for _, child := range x {
			if !supportedNumbers(child) {
				return false
			}
		}
	case []any:
		for _, child := range x {
			if !supportedNumbers(child) {
				return false
			}
		}
	}
	return true
}

// Explicit status selection avoids the filter's early returns. Production would
// compile once at import, never once per response as this tiny experiment does.
func evaluate(d *openapi3.T, status int, h http.Header, body string) string {
	r := d.Paths.Value("/pets").Get.Responses.Status(status)
	if r == nil {
		r = d.Paths.Value("/pets").Get.Responses.Default()
	}
	if r == nil {
		return "status_undocumented"
	}
	for name, header := range r.Value.Headers {
		if header.Value.Required && len(h.Values(name)) == 0 {
			return "header_required"
		}
	}
	if len(r.Value.Content) == 0 {
		return "passed"
	}
	if len(body) > sourceLimit {
		return "body_limit"
	}
	if h.Get("Content-Type") != "application/json" {
		return "body_unsupported"
	}
	var v any
	if execution.StrictJSON([]byte(body), &v) != nil {
		return "json_invalid"
	}
	if !supportedNumbers(v) {
		return "json_number_unsupported"
	}
	s, err := compiledSchema(r.Value.Content["application/json"].Schema.Value)
	if err != nil {
		return "evaluation_unavailable"
	}
	err = s.Validate(v)
	if err != nil {
		return "body_schema_mismatch"
	}
	return "passed"
}

func TestResponseCases(t *testing.T) {
	d, err := kin(fixture(t))
	if err != nil {
		t.Fatal("fixture invalid")
	}
	for _, tc := range []struct {
		name   string
		status int
		body   string
		header bool
		want   string
	}{
		{"valid", 200, `{"id":1,"name":"pet","active":true,"score":1.25,"tags":["a"],"nickname":null}`, true, "passed"},
		{"undocumented", 201, `{}`, true, "status_undocumented"},
		{"redirect skip guarded", 307, `{}`, true, "status_undocumented"},
		{"not modified skip guarded", 304, ``, true, "status_undocumented"},
		{"required property", 200, `{"id":1}`, true, "body_schema_mismatch"},
		{"integer string", 200, `{"id":"1","name":"pet"}`, true, "body_schema_mismatch"},
		{"fractional integer", 200, `{"id":1.5,"name":"pet"}`, true, "body_schema_mismatch"},
		{"precision fractional integer", 200, `{"id":1.00000000000000001,"name":"pet"}`, true, "body_schema_mismatch"},
		{"integral exponent", 200, `{"id":1e0,"name":"pet"}`, true, "passed"},
		{"numeric work bound", 200, `{"id":1e999999999,"name":"pet"}`, true, "json_number_unsupported"},
		{"string number", 200, `{"id":1,"name":2}`, true, "body_schema_mismatch"},
		{"boolean string", 200, `{"id":1,"name":"pet","active":"true"}`, true, "body_schema_mismatch"},
		{"number string", 200, `{"id":1,"name":"pet","score":"1.25"}`, true, "body_schema_mismatch"},
		{"array item", 200, `{"id":1,"name":"pet","tags":[1]}`, true, "body_schema_mismatch"},
		{"non nullable", 200, `{"id":1,"name":null}`, true, "body_schema_mismatch"},
		{"additional property", 200, `{"id":1,"name":"pet","extra":true}`, true, "body_schema_mismatch"},
		{"required header", 200, `{"id":1,"name":"pet"}`, false, "header_required"},
		{"empty declared", 204, ``, false, "passed"},
		{"invalid json", 200, `{`, true, "json_invalid"},
		{"duplicate json", 200, `{"id":1,"id":2,"name":"pet"}`, true, "json_invalid"},
		{"invalid surrogate", 200, `{"id":1,"name":"\ud800"}`, true, "json_invalid"},
		{"trailing json", 200, `{"id":1,"name":"pet"} {}`, true, "json_invalid"},
		{"body cap", 200, strings.Repeat(" ", sourceLimit+1), true, "body_limit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := http.Header{"Content-Type": {"application/json"}}
			if tc.header {
				h.Set("x-trace", "synthetic")
			}
			if got := evaluate(d, tc.status, h, tc.body); got != tc.want {
				t.Fatalf("got %s want %s", got, tc.want)
			}
		})
	}
	// Default explicitly documents otherwise unlisted codes.
	op := d.Paths.Value("/pets").Get
	op.Responses.Set("default", op.Responses.Status(204))
	if evaluate(d, 418, http.Header{}, "") != "passed" {
		t.Fatal("default not honored")
	}
	op.Responses.Set("304", op.Responses.Status(200))
	if evaluate(d, 304, http.Header{}, "") != "header_required" {
		t.Fatal("declared 304 bypassed required headers")
	}
}

func TestNativeFilterHazardsAndDiagnostics(t *testing.T) {
	d, err := kin(fixture(t))
	if err != nil {
		t.Fatal("fixture invalid")
	}
	for _, status := range []int{201, 304, 307} {
		input := &openapi3filter.ResponseValidationInput{RequestValidationInput: &openapi3filter.RequestValidationInput{Request: &http.Request{Method: "GET"}, Route: &routers.Route{Spec: d, Operation: d.Paths.Value("/pets").Get}}, Status: status, Options: &openapi3filter.Options{IncludeResponseStatus: true}}
		err := openapi3filter.ValidateResponse(context.Background(), input)
		if (err != nil) != (status == 201) {
			t.Fatal("native status behavior changed: reassess guard")
		}
	}
	s := d.Components.Schemas["Pet"].Value
	err = s.VisitJSON(map[string]any{"id": "SYNTHETIC_DIAGNOSTIC_SENTINEL", "name": "pet"})
	var se *openapi3.SchemaError
	if !errors.As(err, &se) || se.SchemaField != "type" || strings.Join(se.JSONPointer(), "/") != "id" {
		t.Fatal("expected structured type/path diagnostic")
	}
	if !strings.Contains(err.Error(), "SYNTHETIC_DIAGNOSTIC_SENTINEL") {
		t.Fatal("error rendering changed: reassess containment")
	}
	// Native 3.0 validation rounds this valid numeric token to integral float64.
	if s.Properties["id"].Value.VisitJSON(json.Number("1.00000000000000001")) != nil {
		t.Fatal("native numeric behavior changed: reassess alternative")
	}
	standard, err := compiledSchema(s)
	if err != nil {
		t.Fatal("subset compile failed")
	}
	err = standard.Validate(map[string]any{"id": "SYNTHETIC_DIAGNOSTIC_SENTINEL", "name": "pet"})
	var validation *jsonschema.ValidationError
	if !errors.As(err, &validation) || len(validation.Causes) == 0 {
		t.Fatal("structured standard schema diagnostic missing")
	}
	if strings.Join(validation.Causes[0].InstanceLocation, "/") != "id" {
		t.Fatal("standard schema instance pointer missing")
	}
	// Never publish Error(), SchemaError.Value, schema, authored path, or actuals.
}

func TestTranslationBudgets(t *testing.T) {
	d, err := kin(fixture(t))
	if err != nil {
		t.Fatal("fixture invalid")
	}
	s := d.Components.Schemas["Pet"].Value
	s.Properties[strings.Repeat("x", 32*1024)] = s.Properties["name"]
	if _, err = compiledSchema(s); err == nil {
		t.Fatal("oversize translated schema accepted")
	}
	s.Properties = map[string]*openapi3.SchemaRef{"recursive": {Value: s}}
	if _, err = compiledSchema(s); err == nil {
		t.Fatal("unbounded translation accepted")
	}
}

func TestBoundaryPreflight(t *testing.T) {
	y := fixture(t)
	j, code := preflight(y)
	if code != "" {
		t.Fatal(code)
	}
	for _, tc := range []struct {
		name string
		raw  []byte
		want string
	}{
		{"oversize", bytes.Repeat([]byte(" "), sourceLimit+1), "source_limit"},
		{"malformed", []byte("{broken"), "syntax_invalid"},
		{"duplicate", []byte("openapi: 3.0.3\nopenapi: 3.0.3"), "key_invalid"},
		{"multiple documents", append(append([]byte{}, y...), []byte("\n---\n{}")...), "syntax_invalid"},
		{"3.1", bytes.Replace(j, []byte("3.0.3"), []byte("3.1.0"), 1), "dialect_unsupported"},
		{"remote", bytes.ReplaceAll(j, []byte("#/components/schemas/Pet"), []byte("https://example.invalid/schema")), "reference_unsupported"},
		{"relative file", bytes.ReplaceAll(j, []byte("#/components/schemas/Pet"), []byte("./schema.yaml#/Pet")), "reference_unsupported"},
		{"absolute file", bytes.ReplaceAll(j, []byte("#/components/schemas/Pet"), []byte("file:///tmp/schema")), "reference_unsupported"},
		{"schema keyword", bytes.Replace(j, []byte(`"nullable":true`), []byte(`"nullable":true,"oneOf":[]`), 1), "feature_unsupported"},
		{"format", bytes.Replace(j, []byte(`"nullable":true`), []byte(`"nullable":true,"format":"email"`), 1), "feature_unsupported"},
		{"ref sibling", bytes.Replace(j, []byte(`"$ref":"#/components/schemas/Pet"`), []byte(`"$ref":"#/components/schemas/Pet","type":"object"`), 1), "reference_sibling_unsupported"},
		{"alias", []byte("openapi: &v 3.0.3\ninfo: *v"), "yaml_alias_unsupported"},
		{"extension", bytes.Replace(j, []byte(`"info":{`), []byte(`"x-unknown":true,"info":{`), 1), "feature_unsupported"},
		{"recursive schema", bytes.Replace(j, []byte(`"type":"integer"`), []byte(`"$ref":"#/components/schemas/Pet"`), 1), "reference_cycle"},
		{"missing ref", bytes.ReplaceAll(j, []byte("#/components/schemas/Pet"), []byte("#/components/schemas/Missing")), "reference_invalid"},
		{"boolean schema", bytes.Replace(j, []byte(`"items":{"type":"string"}`), []byte(`"items":true`), 1), "shape_invalid"},
		{"union schema", bytes.Replace(j, []byte(`"type":"integer"`), []byte(`"type":["integer","null"]`), 1), "feature_unsupported"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, got := preflight(tc.raw); got != tc.want {
				t.Fatalf("got %s want %s", got, tc.want)
			}
		})
	}
	for _, ref := range []string{"https://example.invalid/schema", "file:///tmp/schema", "./schema.yaml", "#/components/schemas/Missing"} {
		raw := bytes.ReplaceAll(j, []byte("#/components/schemas/Pet"), []byte(ref))
		if _, err := kin(raw); err == nil {
			t.Fatal("unresolved reference accepted")
		}
	}
	// A schema with an invalid required type must fail semantic validation.
	if _, err := kin(bytes.Replace(j, []byte(`"required":["id","name"]`), []byte(`"required":true`), 1)); err == nil {
		t.Fatal("invalid schema accepted")
	}
}

func TestWorkBudgetsAndKeywordContext(t *testing.T) {
	root := map[string]any{}
	j, code := preflight(fixture(t))
	if code != "" || json.Unmarshal(j, &root) != nil {
		t.Fatal("fixture failed")
	}
	for _, version := range []string{"3.0.0", "3.0.1", "3.0.2", "3.0.3", "3.0.4"} {
		raw := bytes.Replace(j, []byte("3.0.3"), []byte(version), 1)
		if _, code := preflight(raw); code != "" {
			t.Fatal("supported dialect rejected")
		}
		if _, err := kin(raw); err != nil {
			t.Fatal("supported dialect failed semantic validation")
		}
	}
	exact := append(append([]byte{}, j...), bytes.Repeat([]byte(" "), sourceLimit-len(j))...)
	if _, code := preflight(exact); code != "" {
		t.Fatal("exact source cap rejected")
	}
	escaped := bytes.ReplaceAll(j, []byte(`"id"`), []byte(`"id/~"`))
	escaped = bytes.ReplaceAll(escaped, []byte("#/components/schemas/Pet"), []byte("#/components/schemas/Pet/properties/id~1~0"))
	if _, code := preflight(escaped); code != "" {
		t.Fatal("escaped component ref rejected")
	}
	if _, err := kin(escaped); err != nil {
		t.Fatal("escaped component ref failed semantic validation")
	}
	paths := map[string]any{}
	for i := 0; i < 100; i++ {
		paths["/p"+strings.Repeat("x", i)] = map[string]any{"get": map[string]any{"responses": map[string]any{"204": map[string]any{"description": "empty"}}}}
	}
	root["paths"] = paths
	raw, _ := json.Marshal(root)
	if _, code := preflight(raw); code != "" {
		t.Fatal("100 operations rejected")
	}
	paths["/overflow"] = paths["/p"]
	raw, _ = json.Marshal(root)
	if _, code := preflight(raw); code != "operation_limit" {
		t.Fatal("101 operations accepted")
	}
	deep := `{"type":"string"}`
	for i := 0; i < 40; i++ {
		deep = `{"type":"array","items":` + deep + `}`
	}
	raw = bytes.Replace(j, []byte(`"items":{"type":"string"}`), []byte(`"items":`+deep), 1)
	if _, code := preflight(raw); code != "structure_limit" {
		t.Fatal("deep schema accepted")
	}
	// Same word in a property position must not be treated as a schema keyword.
	raw = bytes.ReplaceAll(j, []byte(`"nickname"`), []byte(`"oneOf"`))
	if _, code := preflight(raw); code != "" {
		t.Fatal("property name confused with keyword")
	}
	// Shared references can imply exponential work without any cycle.
	next := map[string]any{"type": "string"}
	schemas := map[string]any{"base": next}
	ref := "base"
	for i := 0; i < 16; i++ {
		name := "n" + strings.Repeat("x", i)
		schemas[name] = map[string]any{"properties": map[string]any{"a": map[string]any{"$ref": "#/components/schemas/" + ref}, "b": map[string]any{"$ref": "#/components/schemas/" + ref}}}
		ref = name
	}
	if code := checkReferences(map[string]any{"components": map[string]any{"schemas": schemas}}); code != "reference_limit" {
		t.Fatal("reference work not bounded")
	}
}

func TestStandaloneJSONSchemaDialect(t *testing.T) {
	compile := func(raw string) *jsonschema.Schema {
		t.Helper()
		c := jsonschema.NewCompiler()
		c.DefaultDraft(jsonschema.Draft2020)
		v, err := jsonschema.UnmarshalJSON(strings.NewReader(raw))
		if err != nil {
			t.Fatal("fixture JSON invalid")
		}
		if c.AddResource("urn:selection", v) != nil {
			t.Fatal("resource failed")
		}
		s, err := c.Compile("urn:selection")
		if err != nil {
			t.Fatal("compile failed")
		}
		return s
	}
	s := compile(`{"type":"object","required":["id"],"properties":{"id":{"$ref":"#/$defs/id"}},"$defs":{"id":{"type":"integer"}}}`)
	for _, tc := range []struct {
		raw   string
		valid bool
	}{{`{"id":1}`, true}, {`{}`, false}, {`{"id":"1"}`, false}} {
		v, _ := jsonschema.UnmarshalJSON(strings.NewReader(tc.raw))
		if (s.Validate(v) == nil) != tc.valid {
			t.Fatal("standalone response criteria failed")
		}
	}
	// OAS nullable is silently an unknown annotation in standard JSON Schema.
	if compile(`{"type":"string","nullable":true}`).Validate(nil) == nil {
		t.Fatal("dialect behavior changed")
	}
	if compile(`{"type":["string","null"]}`).Validate(nil) != nil {
		t.Fatal("2020-12 union failed")
	}
	c := jsonschema.NewCompiler()
	if c.AddResource("urn:external", map[string]any{"$ref": "https://example.invalid/schema"}) != nil {
		t.Fatal("resource failed")
	}
	if _, err := c.Compile("urn:external"); err == nil {
		t.Fatal("unconfigured external loader accepted")
	}
}
