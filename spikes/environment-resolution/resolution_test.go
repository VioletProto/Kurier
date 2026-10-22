// Package environments contains review experiments only, not an application
// resolver, repository, encryption layer, or production protocol extension.
package environments

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"testing"

	execution "github.com/VioletProto/Kurier/services/request-runtime"
)

var namePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,63}$`)

type scalar struct {
	Raw       json.RawMessage
	Protected bool
	Enabled   bool
}

func compact(v any) []byte {
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	_ = e.Encode(v)
	return bytes.TrimSuffix(b.Bytes(), []byte{'\n'})
}

func parseScalar(raw []byte) (any, error) {
	if len(raw) > 2048 {
		return nil, errors.New("variable_limit")
	}
	var v any
	if execution.StrictJSON(raw, &v) != nil {
		return nil, errors.New("variable_invalid")
	}
	switch x := v.(type) {
	case nil, string, bool:
	case json.Number:
		if _, err := execution.ParseAssertionDecimal(string(x)); err != nil {
			return nil, errors.New("variable_invalid")
		}
	default:
		return nil, errors.New("variable_type")
	}
	return v, nil
}

type part struct{ literal, name string }

// A bounded single pass; values are never parsed as templates recursively.
func scan(text string) ([]part, error) {
	if len(text) > 65536 {
		return nil, errors.New("template_limit")
	}
	parts := []part{}
	refs := 0
	for len(text) > 0 {
		i := strings.Index(text, "{{")
		if i < 0 {
			if strings.Contains(text, "}}") {
				return nil, errors.New("template_invalid")
			}
			parts = append(parts, part{literal: text})
			break
		}
		if strings.Contains(text[:i], "}}") {
			return nil, errors.New("template_invalid")
		}
		parts = append(parts, part{literal: text[:i]})
		text = text[i+2:]
		end := strings.Index(text, "}}")
		if end < 0 || !namePattern.MatchString(text[:end]) {
			return nil, errors.New("template_invalid")
		}
		refs++
		if refs > 32 {
			return nil, errors.New("template_limit")
		}
		parts = append(parts, part{name: text[:end]})
		text = text[end+2:]
	}
	return parts, nil
}

func render(text, location string, vars map[string]scalar) (any, bool, error) {
	parts, err := scan(text)
	if err != nil {
		return nil, false, err
	}
	sole := len(parts) == 2 && parts[0].literal == "" && parts[1].name != ""
	var b strings.Builder
	protected := false
	for _, p := range parts {
		piece := p.literal
		if p.name != "" {
			s, ok := vars[p.name]
			if !ok || !s.Enabled {
				return nil, false, errors.New("variable_missing")
			}
			value, err := parseScalar(s.Raw)
			if err != nil {
				return nil, false, err
			}
			protected = protected || s.Protected
			if sole && location == "json" {
				return value, protected, nil
			}
			if str, ok := value.(string); ok {
				piece = str
			} else if sole && (location == "query" || location == "header") && value != nil {
				piece = string(s.Raw)
			} else {
				return nil, false, errors.New("variable_type")
			}
		}
		if b.Len()+len(piece) > 65536 {
			return nil, false, errors.New("resolved_limit")
		}
		b.WriteString(piece)
	}
	value := b.String()
	if location == "header" && (len(value) > 2048 || !execution.HeaderValueValid(value)) {
		return nil, false, errors.New("header_invalid")
	}
	return value, protected, nil
}

// JSON string VALUES only: property names and untouched number tokens survive.
func renderJSON(raw []byte, vars map[string]scalar) ([]byte, bool, error) {
	if len(raw) > 65536 {
		return nil, false, errors.New("resolved_limit")
	}
	var root any
	if execution.StrictJSON(raw, &root) != nil {
		return nil, false, errors.New("json_invalid")
	}
	protected := false
	var visit func(any) (any, error)
	visit = func(v any) (any, error) {
		switch x := v.(type) {
		case string:
			r, s, err := render(x, "json", vars)
			protected = protected || s
			return r, err
		case map[string]any:
			for k, child := range x {
				if strings.Contains(k, "{{") || strings.Contains(k, "}}") {
					return nil, errors.New("template_target_invalid")
				}
				r, err := visit(child)
				if err != nil {
					return nil, err
				}
				x[k] = r
			}
		case []any:
			for i, child := range x {
				r, err := visit(child)
				if err != nil {
					return nil, err
				}
				x[i] = r
			}
		}
		return v, nil
	}
	root, err := visit(root)
	if err != nil {
		return nil, false, err
	}
	out := compact(root)
	if len(out) > 65536 {
		return nil, false, errors.New("resolved_limit")
	}
	return out, protected, nil
}

// URL experiment: a public leading base URL plus public whole path segments.
// This does syntax/taint checks only; it does not perform DNS or authorize a target.
func renderURL(text string, vars map[string]scalar) (string, error) {
	if len(text) > 65536 {
		return "", errors.New("template_limit")
	}
	if strings.HasPrefix(text, "{{") {
		i := strings.Index(text, "}}")
		if i < 0 {
			return "", errors.New("template_invalid")
		}
		name := text[2:i]
		if !namePattern.MatchString(name) {
			return "", errors.New("template_invalid")
		}
		v, ok := vars[name]
		if !ok || !v.Enabled {
			return "", errors.New("variable_missing")
		}
		if v.Protected {
			return "", errors.New("sensitive_url")
		}
		value, err := parseScalar(v.Raw)
		base, ok := value.(string)
		if err != nil || !ok || strings.ContainsAny(base, "{}") {
			return "", errors.New("url_invalid")
		}
		u, err := url.Parse(base)
		if err != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Host == "" || u.Scheme != "http" && u.Scheme != "https" {
			return "", errors.New("url_invalid")
		}
		suffix := text[i+2:]
		if suffix != "" && !strings.HasPrefix(suffix, "/") {
			return "", errors.New("url_invalid")
		}
		text = strings.TrimSuffix(base, "/") + suffix
	}
	u, err := url.Parse(text)
	if err != nil || u.User != nil || u.Host == "" || strings.ContainsAny(u.Host, "{}") || u.RawQuery != "" || u.Fragment != "" || u.Scheme != "http" && u.Scheme != "https" {
		return "", errors.New("url_invalid")
	}
	// Read template delimiters before net/url percent-escapes them. Percent
	// escapes authored as literal data do not turn into template syntax.
	prefix := len(u.Scheme) + 3
	start := strings.Index(text[prefix:], "/")
	sourcePath := ""
	if start >= 0 {
		sourcePath = text[prefix+start:]
	}
	segments := strings.Split(sourcePath, "/")
	for i, p := range segments {
		if !strings.ContainsAny(p, "{}") {
			decoded, err := url.PathUnescape(p)
			if err != nil {
				return "", errors.New("url_invalid")
			}
			segments[i] = url.PathEscape(decoded)
			continue
		}
		if !strings.HasPrefix(p, "{{") || !strings.HasSuffix(p, "}}") {
			return "", errors.New("url_invalid")
		}
		name := p[2 : len(p)-2]
		if !namePattern.MatchString(name) {
			return "", errors.New("template_invalid")
		}
		v, ok := vars[name]
		if !ok || !v.Enabled {
			return "", errors.New("variable_missing")
		}
		if v.Protected {
			return "", errors.New("sensitive_url")
		}
		x, err := parseScalar(v.Raw)
		s, ok := x.(string)
		if err != nil || !ok || s == "." || s == ".." || strings.ContainsAny(s, "/\\%?#{}") {
			return "", errors.New("url_invalid")
		}
		segments[i] = url.PathEscape(s)
	}
	path := strings.Join(segments, "/")
	out := u.Scheme + "://" + u.Host + path
	if len(out) > 65536 {
		return "", errors.New("resolved_limit")
	}
	return out, nil
}

func TestTypedJSONAndSinglePass(t *testing.T) {
	vars := map[string]scalar{
		"n": {json.RawMessage(`0.10000000000000001`), false, true}, "b": {json.RawMessage(`true`), false, true},
		"z": {json.RawMessage(`null`), false, true}, "s": {json.RawMessage(`"1"`), false, true},
		"inert": {json.RawMessage(`"{{missing}}"`), false, true},
	}
	out, secret, err := renderJSON([]byte(`{"n":"{{n}}","b":"{{b}}","z":"{{z}}","s":"{{s}}","literal":1E-01,"inert":"{{inert}}"}`), vars)
	if err != nil || secret || string(out) != `{"b":true,"inert":"{{missing}}","literal":1E-01,"n":0.10000000000000001,"s":"1","z":null}` {
		t.Fatal("typed JSON or exact lexeme changed")
	}
	for _, tc := range []struct {
		text, location string
		valid          bool
	}{
		{"{{n}}", "query", true}, {"{{b}}", "header", true}, {"{{z}}", "query", false},
		{"prefix {{n}}", "json", false}, {"prefix {{b}}", "text", false}, {"prefix {{s}}", "text", true},
		{"{{unknown}}", "text", false}, {"{{ n }}", "text", false}, {"{{n.x}}", "text", false},
		{"{{n", "text", false}, {"n}}", "text", false}, {"{{{n}}}", "text", false},
	} {
		_, _, err := render(tc.text, tc.location, vars)
		if (err == nil) != tc.valid {
			t.Fatal("grammar/type compatibility differs")
		}
	}
	vars["s"] = scalar{json.RawMessage(`"quote\" newline\n"`), false, true}
	out, _, err = renderJSON([]byte(`{"value":"{{s}}"}`), vars)
	var strict any
	if err != nil || execution.StrictJSON(out, &strict) != nil {
		t.Fatal("JSON substitution escaped unsafely")
	}
	if _, _, err := renderJSON([]byte(`{"{{s}}":"literal"}`), vars); err == nil {
		t.Fatal("JSON property name template accepted")
	}
	// A normal generic JSON decode loses this distinction. The accepted strict
	// helper plus json.Number/typed decimal transport preserves the token.
	var generic any
	_ = json.Unmarshal([]byte(`0.10000000000000001`), &generic)
	if generic != float64(0.1) {
		t.Fatal("generic float behavior changed")
	}
}

func TestBoundariesAndHeaderInjection(t *testing.T) {
	for _, raw := range []string{`{"x":1}`, `[1]`, `1e99999999`, `9007199254740992`, `0.0000000000000000001e-308`, `"\ud800"`} {
		if _, err := parseScalar([]byte(raw)); err == nil {
			t.Fatal("unsupported variable accepted")
		}
	}
	if _, err := parseScalar(append([]byte{'"'}, append(bytes.Repeat([]byte{'a'}, 2046), '"')...)); err != nil {
		t.Fatal("exact scalar cap rejected")
	}
	if _, err := parseScalar(append([]byte{'"'}, append(bytes.Repeat([]byte{'a'}, 2047), '"')...)); err == nil {
		t.Fatal("scalar overrun accepted")
	}
	vars := map[string]scalar{"x": {json.RawMessage(`"abc\r\nInjected: yes"`), true, true}}
	if _, _, err := render("Bearer {{x}}", "header", vars); err == nil || err.Error() != "header_invalid" {
		t.Fatal("header injection or unsafe diagnostic")
	}
	vars["x"] = scalar{json.RawMessage(`"a"`), false, false}
	if _, _, err := render("{{x}}", "query", vars); err == nil || err.Error() != "variable_missing" {
		t.Fatal("disabled is not missing")
	}
	if _, err := scan(strings.Repeat("{{x}}", 33)); err == nil {
		t.Fatal("token work unbounded")
	}
	vars["x"] = scalar{json.RawMessage(`"` + strings.Repeat("a", 2046) + `"`), false, true}
	if _, _, err := render(strings.Repeat("{{x}}", 32)+strings.Repeat("a", 65), "text", vars); err == nil {
		t.Fatal("expanded byte cap bypass")
	}
}

func TestURLParsingAndSecretBoundary(t *testing.T) {
	// net/url percent-escapes braces in EscapedPath; tokenization must precede it.
	u, _ := url.Parse("https://example.invalid/{{path}}")
	if !strings.Contains(u.EscapedPath(), "%7B") {
		t.Fatal("Go URL behavior changed")
	}
	vars := map[string]scalar{"baseUrl": {json.RawMessage(`"https://example.invalid/v1"`), false, true}, "path": {json.RawMessage(`"blue fox"`), false, true}}
	if got, err := renderURL("{{baseUrl}}/pets", vars); err != nil || got != "https://example.invalid/v1/pets" {
		t.Fatal("public base URL resolution failed")
	}
	if got, err := renderURL("{{baseUrl}}/pets/{{path}}", vars); err != nil || got != "https://example.invalid/v1/pets/blue%20fox" {
		t.Fatal("whole path segment was not encoded")
	}
	vars["baseUrl"] = scalar{json.RawMessage(`"https://example.invalid"`), true, true}
	if _, err := renderURL("{{baseUrl}}/pets", vars); err == nil || err.Error() != "sensitive_url" {
		t.Fatal("protected URL source accepted")
	}
	for _, raw := range []string{`"https://u:synthetic@example.invalid"`, `"https://example.invalid?key=synthetic"`, `"https://example.invalid#fragment"`} {
		vars["baseUrl"] = scalar{json.RawMessage(raw), false, true}
		if _, err := renderURL("{{baseUrl}}/pets", vars); err == nil {
			t.Fatal("unsafe base URL accepted")
		}
	}
	if _, err := renderURL("https://{{path}}/pets", vars); err == nil {
		t.Fatal("authority interpolation accepted")
	}
	if _, err := renderURL("https://example.invalid/?q={{path}}", vars); err == nil {
		t.Fatal("inline query interpolation accepted")
	}
	vars["path"] = scalar{json.RawMessage(`"synthetic-secret"`), true, true}
	if _, err := renderURL("https://example.invalid/{{path}}", vars); err == nil || err.Error() != "sensitive_url" {
		t.Fatal("protected path accepted")
	}
	for _, s := range []string{`"../admin"`, `".."`, `"%2Fadmin"`, `"a?b"`} {
		vars["path"] = scalar{json.RawMessage(s), false, true}
		if _, err := renderURL("https://example.invalid/{{path}}", vars); err == nil {
			t.Fatal("path structure injection accepted")
		}
	}
	if _, err := execution.ValidateDestination("http://127.0.0.1:3000/pets"); err == nil {
		t.Fatal("environment bypassed cloud destination policy")
	}
	if _, err := execution.ValidateLocalURL("http://127.0.0.1:3000/pets", execution.Destination{Origin: "http://127.0.0.1:3000", AddressClass: "loopback"}); err != nil {
		t.Fatal("exact-origin local policy rejected fixture")
	}
	// Separate query rows are encoded as data once by net/url.
	query := url.Values{}
	query.Add("q", "a&admin=true")
	query.Add("q", "+ / %")
	back, err := url.ParseQuery(query.Encode())
	if err != nil || len(back["q"]) != 2 || back["q"][0] != "a&admin=true" || back["admin"] != nil {
		t.Fatal("query value injected structure")
	}
}

func TestFrozenPublicBufferIndependence(t *testing.T) {
	// Fixture snapshots demonstrate Go slice aliasing and an explicit copy. This
	// is not a database/CAS or actual encrypted-admission proof.
	live := map[string]scalar{"n": {json.RawMessage(`0.10000000000000001`), false, true}}
	frozen := map[string]scalar{}
	for name, v := range live {
		v.Raw = append(json.RawMessage{}, v.Raw...)
		frozen[name] = v
	}
	live["n"].Raw[0] = '1'
	if string(frozen["n"].Raw) != `0.10000000000000001` {
		t.Fatal("freeze aliases live bytes")
	}
}

func TestSharedRedactionPrimitivesAndLegacyReaders(t *testing.T) {
	vars := map[string]scalar{"token": {json.RawMessage(`"synthetic-environment-canary"`), true, true}}
	raw, protected, err := render("Bearer {{token}}", "header", vars)
	if err != nil || !protected {
		t.Fatal("sensitivity lost during rendering")
	}
	corpus := map[string]string{"raw": "synthetic-environment-canary", "rendered": raw.(string)}
	if strings.Contains(execution.Scrub("echo Bearer synthetic-environment-canary", corpus), "synthetic-environment-canary") {
		t.Fatal("reflection canary survived shared sanitizer")
	}
	for _, name := range []string{"Authorization", "Cookie", "X-Api-Key", "password"} {
		if !execution.CredentialName(name) {
			t.Fatal("credential recognition changed")
		}
	}
	// Existing strict wire readers deny new fields; rollout cannot assume this
	// is an additive reader-compatible change or turn markers on automatically.
	if _, err := execution.ParseOptions([]byte(`{"environment":{"environmentId":"e","revision":0}}`)); err == nil {
		t.Fatal("legacy submit silently accepts environment")
	}
	var step execution.WorkflowStep
	if execution.StrictJSON([]byte(`{"stepId":"s","requestId":"r","requestRevision":0,"environment":{"mode":"inherit"}}`), &step) == nil {
		t.Fatal("legacy step silently accepts environment")
	}
	var config execution.Configuration
	if execution.StrictJSON([]byte(`{"templateSemanticsVersion":1}`), &config) == nil {
		t.Fatal("legacy plan silently accepts marker")
	}
}
