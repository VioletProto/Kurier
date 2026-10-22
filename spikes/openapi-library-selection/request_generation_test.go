package selection

import (
	"bytes"
	"context"
	"encoding/json"
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
)

// Separate compatibility probes: the old static-only preflight is deliberately
// unchanged. These tests are not a production generator or an expanded ingress
// validator; all input here is a bounded synthetic fixture, never an upload.
func generationFixture(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile("testdata/request-generation.yaml")
	if err != nil || len(raw) > sourceLimit {
		t.Fatal("fixture unavailable")
	}
	return raw
}

func fixtureJSON(t *testing.T, raw []byte) []byte {
	t.Helper()
	// yaml3 -> generic -> JSON would round example numbers. Render YAML scalars
	// into raw JSON tokens for this fixture, preserving numeric token spellings.
	var doc yaml.Node
	if yaml.Unmarshal(raw, &doc) != nil {
		t.Fatal("fixture syntax invalid")
	}
	work := 0
	var encode func(*yaml.Node, int) []byte
	encode = func(n *yaml.Node, depth int) []byte {
		work++
		if depth > 32 || work > 20000 {
			t.Fatal("fixture work bound")
		}
		switch n.Kind {
		case yaml.MappingNode:
			parts := [][]byte{}
			for i := 0; i < len(n.Content); i += 2 {
				key, _ := json.Marshal(n.Content[i].Value)
				parts = append(parts, append(append(key, ':'), encode(n.Content[i+1], depth+1)...))
			}
			return append(append([]byte{'{'}, bytes.Join(parts, []byte{','})...), '}')
		case yaml.SequenceNode:
			parts := [][]byte{}
			for _, child := range n.Content {
				parts = append(parts, encode(child, depth+1))
			}
			return append(append([]byte{'['}, bytes.Join(parts, []byte{','})...), ']')
		case yaml.ScalarNode:
			if n.Tag == "!!int" || n.Tag == "!!float" || n.Tag == "!!bool" || n.Tag == "!!null" {
				return []byte(n.Value)
			}
			v, _ := json.Marshal(n.Value)
			return v
		default:
			t.Fatal("unsupported fixture syntax")
			return nil
		}
	}
	out := encode(doc.Content[0], 0)
	var strict any
	if execution.StrictJSON(out, &strict) != nil || len(out) > sourceLimit {
		t.Fatal("fixture JSON invalid")
	}
	return out
}

func TestGenerationLibraryCompatibility(t *testing.T) {
	y := generationFixture(t)
	for name, raw := range map[string][]byte{"yaml": y, "json": fixtureJSON(t, y)} {
		t.Run(name, func(t *testing.T) {
			d, err := kin(raw)
			if err != nil {
				t.Fatal("kin generation fixture rejected")
			}
			path := d.Paths.Value("/pets/{petId}")
			op := path.Post
			if len(path.Parameters) != 2 || path.Parameters[0].Value.Name != "petId" || !path.Parameters[0].Value.Required {
				t.Fatal("path parameter reference lost")
			}
			if path.Parameters[0].Value.Schema.Value.Example != "blue fox" {
				t.Fatal("schema example fallback unavailable")
			}
			if op.Parameters[0].Value.Name != "limit" || op.Parameters[0].Value.Example != float64(5) {
				t.Fatal("operation override data lost")
			}
			for _, p := range append(append(openapi3.Parameters{}, path.Parameters...), op.Parameters...) {
				method, err := p.Value.SerializationMethod()
				if err != nil {
					t.Fatal("serialization defaults unavailable")
				}
				want := "simple"
				explode := false
				if p.Value.In == "query" {
					want = "form"
					explode = true
				}
				if method.Style != want || method.Explode != explode {
					t.Fatal("unexpected serialization default")
				}
			}
			if op.Parameters[1].Value.Examples["basic"].Value.Value.([]any)[0] != "blue" || op.Parameters[3].Value.Examples["ohio"].Value.Value != "ohio" {
				t.Fatal("parameter example reference lost")
			}
			body := op.RequestBody.Value
			if body.Required || body.Content["application/json"].Schema.Value.Properties["name"].Value == nil || body.Content["application/json"].Examples["basic"].Value == nil {
				t.Fatal("optional JSON body/example refs lost")
			}
			compiled, err := compiledSchema(body.Content["application/json"].Schema.Value)
			if err != nil {
				t.Fatal("request schema subset incompatible")
			}
			var value any
			if execution.StrictJSON([]byte(`{"name":"blue fox","active":true,"ratio":0.10000000000000001}`), &value) != nil || compiled.Validate(value) != nil {
				t.Fatal("exact request example validation failed")
			}
			if compiled.Validate(map[string]any{"name": true}) == nil {
				t.Fatal("invalid request example accepted")
			}
			cfg := datamodel.NewDocumentConfiguration()
			cfg.AllowFileReferences = false
			cfg.AllowRemoteReferences = false
			other, err := libopenapi.NewDocumentWithConfiguration(raw, cfg)
			if err != nil {
				t.Fatal("alternate parser rejected generation fixture")
			}
			model, err := other.BuildV3Model()
			if err != nil {
				t.Fatal("alternate generation model failed")
			}
			otherPath := model.Model.Paths.PathItems.GetOrZero("/pets/{petId}")
			if otherPath.Parameters[0].Name != "petId" || otherPath.Post.Parameters[3].Examples.GetOrZero("ohio").Value.Value != "ohio" || otherPath.Post.RequestBody.Content.GetOrZero("application/json").Examples.GetOrZero("basic") == nil {
				t.Fatal("alternate reference data lost")
			}
		})
	}
}

func TestInvalidGenerationDocumentsAndInlineBodyExample(t *testing.T) {
	j := fixtureJSON(t, generationFixture(t))
	invalidPath := bytes.Replace(j, []byte(`"name":"petId","in":"path","required":true`), []byte(`"name":"petId","in":"path","required":false`), 1)
	if bytes.Equal(j, invalidPath) {
		t.Fatal("fixture mutation did not apply")
	}
	if _, err := kin(invalidPath); err == nil {
		t.Fatal("optional path declaration accepted")
	}
	inline := bytes.Replace(j, []byte(`"examples":{"basic":{"$ref":"#/components/examples/Update"}}`), []byte(`"example":{"name":"direct"}`), 1)
	if bytes.Equal(j, inline) {
		t.Fatal("inline fixture mutation did not apply")
	}
	d, err := kin(inline)
	if err != nil {
		t.Fatal("inline media example rejected")
	}
	media := d.Paths.Value("/pets/{petId}").Post.RequestBody.Value.Content["application/json"]
	if media.Example.(map[string]any)["name"] != "direct" {
		t.Fatal("inline body example lost")
	}
	// Required flag is separate from the optional-body compatibility cases.
	d.Paths.Value("/pets/{petId}").Post.RequestBody.Value.Required = true
	req, _ := http.NewRequest("POST", "https://example.invalid/v1/pets/blue", nil)
	req.Header.Set("X-Region", "ohio")
	path := d.Paths.Value("/pets/{petId}")
	if openapi3filter.ValidateRequest(context.Background(), &openapi3filter.RequestValidationInput{Request: req, PathParams: map[string]string{"petId": "blue"}, Route: &routers.Route{Spec: d, PathItem: path, Operation: path.Post}}) == nil {
		t.Fatal("missing required body accepted")
	}
}

func TestParameterAndOptionalBodyWireCompatibility(t *testing.T) {
	d, err := kin(generationFixture(t))
	if err != nil {
		t.Fatal("fixture invalid")
	}
	path := d.Paths.Value("/pets/{petId}")
	for _, tc := range []struct {
		name, query, header, body string
		valid                     bool
	}{
		{"optional body absent", "limit=5&tags=blue&tags=green&active=true", "ohio", "", true},
		{"optional JSON example", "limit=5", "ohio", `{"name":"blue fox","active":true}`, true},
		{"required header absent", "limit=5", "", "", false},
		{"integer query type", "limit=wrong", "ohio", "", false},
		{"boolean query type", "active=wrong", "ohio", "", false},
		{"JSON property type", "", "ohio", `{"name":true}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Standard URI escaping, plus repeated ordered query values; no HTTP send.
			endpoint := "https://example.invalid/v1/pets/" + url.PathEscape("blue fox") + "?" + tc.query
			req, err := http.NewRequest("POST", endpoint, nil)
			if err != nil {
				t.Fatal("synthetic request invalid")
			}
			if tc.header != "" {
				req.Header.Set("X-Region", tc.header)
			}
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
				req.Body = io.NopCloser(strings.NewReader(tc.body))
			}
			input := &openapi3filter.RequestValidationInput{Request: req, PathParams: map[string]string{"petId": "blue fox"}, Route: &routers.Route{Spec: d, PathItem: path, Operation: path.Post}, Options: &openapi3filter.Options{SkipSettingDefaults: true}}
			if (openapi3filter.ValidateRequest(context.Background(), input) == nil) != tc.valid {
				t.Fatal("wire compatibility outcome differs")
			}
			if req.URL.EscapedPath() != "/v1/pets/blue%20fox" || len(req.URL.Query()["tags"]) > 2 {
				t.Fatal("URI encoding changed")
			}
		})
	}
}

// Feature-policy probe only: the libraries support more than Kurier proposes.
func parameterProfile(p *openapi3.Parameter) string {
	if p == nil || p.Schema == nil || p.Schema.Value == nil || len(p.Content) > 0 || p.AllowReserved || p.AllowEmptyValue {
		return "parameter_unsupported"
	}
	s := p.Schema.Value
	method, err := p.SerializationMethod()
	if err != nil || s.Nullable {
		return "parameter_unsupported"
	}
	primitive := func(s *openapi3.Schema) bool {
		return s != nil && !s.Nullable && (s.Type.Is("string") || s.Type.Is("integer") || s.Type.Is("number") || s.Type.Is("boolean"))
	}
	if p.In == "query" && method.Style == "form" && method.Explode {
		if primitive(s) || s.Type.Is("array") && s.Items != nil && primitive(s.Items.Value) {
			return ""
		}
	}
	if (p.In == "path" || p.In == "header") && method.Style == "simple" && !method.Explode && primitive(s) && (p.In != "path" || p.Required) {
		return ""
	}
	return "parameter_unsupported"
}

func TestExplicitParameterSerializationBoundary(t *testing.T) {
	for _, tc := range []struct {
		in, style, typ string
		explode, valid bool
	}{
		{"path", "simple", "string", false, true}, {"query", "form", "string", true, true},
		{"query", "form", "array", true, true}, {"header", "simple", "integer", false, true},
		{"path", "label", "string", false, false}, {"path", "matrix", "string", false, false},
		{"query", "deepObject", "object", true, false}, {"query", "spaceDelimited", "array", false, false},
		{"query", "pipeDelimited", "array", false, false}, {"query", "form", "array", false, false},
		{"header", "simple", "array", false, false}, {"cookie", "form", "string", true, false},
	} {
		t.Run(tc.in+tc.style+tc.typ, func(t *testing.T) {
			s := &openapi3.Schema{Type: &openapi3.Types{tc.typ}}
			if tc.typ == "array" {
				s.Items = &openapi3.SchemaRef{Value: &openapi3.Schema{Type: &openapi3.Types{"string"}}}
			}
			p := &openapi3.Parameter{Name: "synthetic", In: tc.in, Required: tc.in == "path", Style: tc.style, Explode: &tc.explode, Schema: &openapi3.SchemaRef{Value: s}}
			if (parameterProfile(p) == "") != tc.valid {
				t.Fatal("serialization policy differs")
			}
		})
	}
}

func TestExamplePrecisionAndNoExternalResolution(t *testing.T) {
	j := fixtureJSON(t, generationFixture(t))
	var raw map[string]json.RawMessage
	if execution.StrictJSON(j, &raw) != nil || !bytes.Contains(j, []byte("0.10000000000000001")) {
		t.Fatal("source number lexeme lost")
	}
	d, err := kin(j)
	if err != nil {
		t.Fatal("fixture invalid")
	}
	v := d.Components.Examples["Update"].Value.Value.(map[string]any)["ratio"]
	if v != float64(0.1) {
		t.Fatal("model example number representation changed: reassess exact source extraction")
	}
	for _, ref := range []string{"https://example.invalid/example", "file:///tmp/example", "./example.yaml"} {
		external := bytes.ReplaceAll(j, []byte("#/components/examples/Update"), []byte(ref))
		if _, err := kin(external); err == nil {
			t.Fatal("external example ref accepted")
		}
	}
	// externalValue is an annotation URL, not a $ref. The loader preserves it
	// without dereferencing; semantic example validation is a separate step.
	// Production feature preflight must explicitly reject this feature.
	ext := bytes.Replace(j, []byte(`"value":"ohio"`), []byte(`"externalValue":"https://example.invalid/example"`), 1)
	loader := openapi3.NewLoader()
	called := false
	loader.ReadFromURIFunc = func(*openapi3.Loader, *url.URL) ([]byte, error) { called = true; return nil, io.ErrUnexpectedEOF }
	other, err := loader.LoadFromData(ext)
	if err != nil || called || other.Components.Examples["Region"].Value.ExternalValue == "" {
		t.Fatal("externalValue behavior changed")
	}
}

func TestExplicitNullExamplePresence(t *testing.T) {
	j := fixtureJSON(t, generationFixture(t))
	j = bytes.Replace(j, []byte(`"examples":{"basic":{"$ref":"#/components/examples/Update"}}`), []byte(`"example":null`), 1)
	j = bytes.Replace(j, []byte(`"type":"object","required"`), []byte(`"type":"object","nullable":true,"required"`), 1)
	var root map[string]any
	if execution.StrictJSON(j, &root) != nil {
		t.Fatal("null fixture invalid")
	}
	body := root["components"].(map[string]any)["requestBodies"].(map[string]any)["Update"].(map[string]any)["content"].(map[string]any)["application/json"].(map[string]any)
	if value, present := body["example"]; !present || value != nil {
		t.Fatal("explicit null presence lost in source")
	}
	d, err := kin(j)
	if err != nil {
		t.Fatal("direct null example load failed")
	}
	media := d.Components.RequestBodies["Update"].Value.Content["application/json"]
	compiled, err := compiledSchema(media.Schema.Value)
	if err != nil || compiled.Validate(nil) != nil {
		t.Fatal("nullable body example validation failed")
	}
	if media.Example != nil {
		t.Fatal("nil model representation changed: reassess presence handling")
	}
}

// This is a conservative omission-policy probe, not the production sanitizer.
// Credential-name and designated-value handling reuse accepted runtime helpers.
func publicExample(raw []byte, name string, designated map[string]string) ([]byte, string) {
	if len(raw) > 8*1024 {
		return nil, "example_limit"
	}
	var v any
	if execution.StrictJSON(raw, &v) != nil {
		return nil, "example_invalid"
	}
	if execution.CredentialName(name) || execution.Scrub(string(raw), designated) != string(raw) {
		return nil, "example_omitted_sensitive"
	}
	var safe func(any) bool
	safe = func(v any) bool {
		switch x := v.(type) {
		case string:
			text := strings.ToLower(strings.TrimSpace(x))
			if strings.HasPrefix(text, "bearer ") || strings.HasPrefix(text, "basic ") || strings.Contains(text, "private key-----") {
				return false
			}
		case map[string]any:
			for k, child := range x {
				if execution.CredentialName(k) || !safe(child) {
					return false
				}
			}
		case []any:
			for _, child := range x {
				if !safe(child) {
					return false
				}
			}
		}
		return true
	}
	if !safe(v) {
		return nil, "example_omitted_sensitive"
	}
	return raw, ""
}

func TestExamplesStayOutsidePublicSecretStorage(t *testing.T) {
	for _, tc := range []struct {
		name, raw  string
		designated map[string]string
		want       string
	}{
		{"name", `"blue fox"`, nil, ""},
		{"Authorization", `"synthetic credential fixture"`, nil, "example_omitted_sensitive"},
		{"Cookie", `"synthetic cookie fixture"`, nil, "example_omitted_sensitive"},
		{"X-Api-Key", `"synthetic key fixture"`, nil, "example_omitted_sensitive"},
		{"body", `{"nested":{"password":"synthetic fixture"}}`, nil, "example_omitted_sensitive"},
		{"body", `{"label":"designated-fixture"}`, map[string]string{"synthetic-id": "designated-fixture"}, "example_omitted_sensitive"},
		{"body", `{"label":"public"}`, nil, ""},
		{"body", `{"label":"Bearer synthetic-fixture"}`, nil, "example_omitted_sensitive"},
		{"body", strings.Repeat(" ", 8*1024+1), nil, "example_limit"},
		{"body", `{"name":true,"name":false}`, nil, "example_invalid"},
	} {
		out, code := publicExample([]byte(tc.raw), tc.name, tc.designated)
		if code != tc.want || code != "" && out != nil {
			t.Fatal("example containment or fixed diagnostic failed")
		}
	}
}
