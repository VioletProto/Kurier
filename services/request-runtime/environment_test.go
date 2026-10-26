package execution

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func testPrimitive(typ, text string) *Primitive {
	p := &Primitive{Type: typ}
	if typ == "number" {
		p.Decimal = text
	} else if typ != "null" {
		p.Value = json.RawMessage(text)
	}
	return p
}
func environmentPlan(t *testing.T, body string, variables []FrozenVariable) Plan {
	t.Helper()
	SortFrozenVariables(variables)
	p := fixturePlan()
	p.Options.AllowInsecureSecrets = true
	p.Configuration.TemplateSemanticsVersion = 1
	p.Configuration.Method = "POST"
	p.Configuration.Body = &RequestBody{Type: "json", Text: body}
	p.TemplateSnapshot = &TemplateSnapshot{SchemaVersion: 1, EnvironmentContext: &EnvironmentContext{EnvironmentID: NewID(), Revision: 7}, Variables: variables}
	return p
}
func TestEnvironmentTypedResolutionFreezeAndContainment(t *testing.T) {
	secretID := NewID()
	p := environmentPlan(t, `{"n":"{{n}}","b":"{{b}}","z":"{{z}}","token":"{{token}}","inert":"{{inert}}","untouched":1E-01}`, []FrozenVariable{
		{VariableID: NewID(), Name: "n", ValueType: "number", Value: testPrimitive("number", "0.10000000000000001")},
		{VariableID: NewID(), Name: "b", ValueType: "boolean", Value: testPrimitive("boolean", "true")},
		{VariableID: NewID(), Name: "z", ValueType: "null", Value: testPrimitive("null", "")},
		{VariableID: NewID(), Name: "inert", ValueType: "string", Value: testPrimitive("string", `"{{unresolved}}"`)},
		{VariableID: NewID(), Name: "token", ValueType: "string", Sensitive: true, SecretRef: &Ref{SecretID: secretID}},
	})
	p.Configuration.Headers = []Field{{Name: "Authorization", Value: "Bearer {{token}}", Enabled: true}}
	p.Configuration.QueryParameters = []Field{{Name: "echo", Value: "prefix {{token}} suffix", Enabled: true}}
	frozen, _ := json.Marshal(p)
	values := map[string]string{secretID: "synthetic-environment-canary"}
	resolved, corpus, e := ResolveTemplatePlan(p, values)
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(resolved.Configuration.Body.Text, `"n":0.10000000000000001`) || !strings.Contains(resolved.Configuration.Body.Text, `"untouched":1E-01`) || !strings.Contains(resolved.Configuration.Body.Text, `"z":null`) || !strings.Contains(resolved.Configuration.Body.Text, `{{unresolved}}`) {
		t.Fatal("JSON type, exact lexeme or single pass changed")
	}
	after, _ := json.Marshal(p)
	if !bytes.Equal(frozen, after) || len(values) != 1 {
		t.Fatal("resolver mutated frozen plan/inputs")
	}
	if !resolved.Configuration.Headers[0].Sensitive || corpus[resolved.Configuration.Headers[0].SecretRef.SecretID] != "Bearer synthetic-environment-canary" {
		t.Fatal("derived header sensitivity lost")
	}
	prepared := fixturePrepared(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !bytes.Equal(body, []byte(resolved.Configuration.Body.Text)) || r.Header.Get("Authorization") != "Bearer synthetic-environment-canary" || r.URL.Query().Get("echo") != "prefix synthetic-environment-canary suffix" {
			t.Error("wrong outbound request")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"reflected":"synthetic-environment-canary"}`))
	}), p, values)
	evidence := NewEvidence(NewID(), NewID(), stampTime(), p)
	prepared.Run(context.Background(), evidence)
	raw, _ := json.Marshal(evidence)
	if bytes.Contains(raw, []byte("synthetic-environment-canary")) || evidence.SchemaVersion != 3 || evidence.EnvironmentContext == nil || evidence.EnvironmentContext.Revision != 7 || ValidateEvidenceVersion(evidence) != nil {
		t.Fatal("environment identity/redaction failed")
	}
}
func TestEnvironmentTypedDTOAndPolicyRejections(t *testing.T) {
	for _, raw := range []string{`{"TYPE":"number","decimal":"1"}`, `{"type":"number","decimal":"9007199254740992"}`, `{"type":"number","decimal":1}`, `{"type":"string","value":null}`, `{"type":"null","value":null}`, `{"type":"boolean","value":"false"}`, `{"type":"number","decimal":"1","type":"string"}`} {
		var p Primitive
		if StrictJSON([]byte(raw), &p) == nil {
			t.Fatal("invalid typed primitive accepted")
		}
	}
	for _, raw := range []string{`{}`, `{"environment":null}`, `{"environment":{"environmentId":"11111111-1111-4111-8111-111111111111","revision":0}}`} {
		if _, e := ParseOptions([]byte(raw)); e != nil {
			t.Fatal("valid selection rejected", e)
		}
	}
	for _, raw := range []string{`{"timeoutSeconds":null}`, `{"environment":{"environmentId":"11111111-1111-4111-8111-111111111111"}}`, `{"environment":{"environmentId":"11111111-1111-4111-8111-111111111111","revision":null}}`, `{"environment":{"environmentId":"11111111-1111-4111-8111-111111111111","revision":-1}}`} {
		if _, e := ParseOptions([]byte(raw)); e == nil {
			t.Fatal("invalid selection accepted")
		}
	}
	for _, tc := range []struct {
		body string
		vars []FrozenVariable
	}{
		{`{"{{key}}":"v"}`, nil},
		{`"{{missing}}"`, nil},
		{`"mixed {{n}}"`, []FrozenVariable{{VariableID: NewID(), Name: "n", ValueType: "number", Value: testPrimitive("number", "1")}}},
	} {
		p := environmentPlan(t, tc.body, tc.vars)
		if p.TemplateSnapshot.Variables == nil {
			p.TemplateSnapshot.Variables = []FrozenVariable{}
		}
		if _, _, e := ResolveTemplatePlan(p, map[string]string{}); e == nil {
			t.Fatal("invalid template accepted")
		}
	}
}
func TestEnvironmentVersionedWireIdentity(t *testing.T) {
	p := environmentPlan(t, `{}`, []FrozenVariable{})
	p.TemplateSnapshot.EnvironmentContext = nil
	e := NewEvidence(NewID(), NewID(), stampTime(), p)
	e.CompletedAt = stampTime()
	raw, _ := json.Marshal(e)
	var copy Evidence
	if StrictJSON(raw, &copy) != nil || !bytes.Contains(raw, []byte(`"environmentContext":null`)) {
		t.Fatal("version3 null context lost")
	}
	for _, key := range []string{"environmentContext", "templateSemanticsVersion"} {
		var m map[string]json.RawMessage
		json.Unmarshal(raw, &m)
		delete(m, key)
		bad, _ := json.Marshal(m)
		if StrictJSON(bad, &copy) == nil {
			t.Fatal("missing version3 context accepted")
		}
	}
	nonce := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))
	result := Result{SchemaVersion: 2, LeaseID: NewID(), Fence: 1, Nonce: nonce, TemplateSemanticsVersion: 1}
	before, err := CanonicalResult(result)
	if err != nil {
		t.Fatal(err)
	}
	result.EnvironmentContext = &EnvironmentContext{EnvironmentID: NewID(), Revision: 1}
	after, err := CanonicalResult(result)
	if err != nil || bytes.Equal(before, after) {
		t.Fatal("canonical identity ignored environment")
	}
	if ValidateResultContext(result, p) == nil {
		t.Fatal("result context mismatch accepted")
	}
	result.EnvironmentContext = nil
	if ValidateResultContext(result, p) != nil {
		t.Fatal("matching null context rejected")
	}
	legacy := NewEvidence(NewID(), NewID(), stampTime(), fixturePlan())
	legacyRaw, _ := json.Marshal(legacy)
	if bytes.Contains(legacyRaw, []byte("environmentContext")) || bytes.Contains(legacyRaw, []byte("templateSemanticsVersion")) {
		t.Fatal("legacy evidence changed")
	}
}

func TestEnvironmentURLBoundariesAndNoSensitivePublicStructure(t *testing.T) {
	base := FrozenVariable{VariableID: NewID(), Name: "base", ValueType: "string", Value: testPrimitive("string", `"https://example.test/api"`)}
	path := FrozenVariable{VariableID: NewID(), Name: "path", ValueType: "string", Value: testPrimitive("string", `"a b"`)}
	p := environmentPlan(t, `{}`, []FrozenVariable{base, path})
	p.Configuration.URL = "{{base}}/pets/{{path}}"
	if rendered, e := ResolvedTemplateURL(p); e != nil || rendered != "https://example.test/api/pets/a%20b" {
		t.Fatal("URL components changed", e)
	}
	for _, value := range []string{".", "..", "a/b", "%2f", "a?b", "a#b", "{{other}}"} {
		raw, _ := json.Marshal(value)
		for i := range p.TemplateSnapshot.Variables {
			if p.TemplateSnapshot.Variables[i].Name == "path" {
				p.TemplateSnapshot.Variables[i].Value.Value = raw
			}
		}
		if _, e := ResolvedTemplateURL(p); e == nil {
			t.Fatal("dynamic path changed URL structure")
		}
	}
	secret := NewID()
	p = environmentPlan(t, `{"public-canary":"{{token}}"}`, []FrozenVariable{{VariableID: NewID(), Name: "token", ValueType: "string", Sensitive: true, SecretRef: &Ref{SecretID: secret}}})
	if _, _, e := ResolveTemplatePlan(p, map[string]string{secret: "public-canary"}); e == nil {
		t.Fatal("known protected value accepted as a public JSON key")
	}
	p = environmentPlan(t, `{}`, []FrozenVariable{{VariableID: NewID(), Name: "token", ValueType: "string", Sensitive: true, SecretRef: &Ref{SecretID: secret}}})
	p.Configuration.Headers = []Field{{Name: "X-Value", Value: "{{token}}", Enabled: true}}
	for _, value := range []string{"line\r\nbreak", "bad\x7fvalue", strings.Repeat("x", 2049)} {
		if _, _, e := ResolveTemplatePlan(p, map[string]string{secret: value}); e == nil {
			t.Fatal("unsafe rendered header accepted")
		}
	}
}

func TestEnvironmentWorkAndSnapshotVersionPresence(t *testing.T) {
	for _, raw := range []string{
		`{"schemaVersion":1,"environmentContext":null,"variables":[]}`,
		`{"schemaVersion":1,"variables":[]}`,
		`{"schemaVersion":1,"environmentContext":null,"variables":null}`,
		`{"schemaVersion":2,"environmentContext":null,"variables":[]}`,
	} {
		var s TemplateSnapshot
		e := StrictJSON([]byte(raw), &s)
		if (e == nil) != (raw == `{"schemaVersion":1,"environmentContext":null,"variables":[]}`) {
			t.Fatal("snapshot version/presence acceptance mismatch")
		}
	}
	p := environmentPlan(t, `{}`, []FrozenVariable{})
	p.TemplateSnapshot.EnvironmentContext = nil
	w := Work{SchemaVersion: 2, TemplateSemanticsVersion: 1, Plan: p}
	raw, _ := json.Marshal(w)
	var decoded Work
	if StrictJSON(raw, &decoded) != nil {
		t.Fatal("valid Work2 null context rejected")
	}
	var fields map[string]json.RawMessage
	json.Unmarshal(raw, &fields)
	delete(fields, "environmentContext")
	bad, _ := json.Marshal(fields)
	if StrictJSON(bad, &decoded) == nil {
		t.Fatal("missing Work2 context accepted")
	}
	fields["environmentContext"] = json.RawMessage(`null`)
	fields["schemaVersion"] = json.RawMessage(`1`)
	bad, _ = json.Marshal(fields)
	if StrictJSON(bad, &decoded) == nil {
		t.Fatal("Work1 accepted context fields")
	}
}

func TestEnvironmentProtectedPrimitiveJSONFramingIsNotStringData(t *testing.T) {
	secret := NewID()
	p := environmentPlan(t, `{}`, []FrozenVariable{{VariableID: NewID(), Name: "n", ValueType: "number", Sensitive: true, SecretRef: &Ref{SecretID: secret}}})
	p.Configuration.Headers = []Field{{Name: "X-Number", Value: "{{n}}", Enabled: true}}
	resolved, values, e := ResolveTemplatePlan(p, map[string]string{secret: " 1.0000000000000001 "})
	if e != nil {
		t.Fatal(e)
	}
	if values[resolved.Configuration.Headers[0].SecretRef.SecretID] != "1.0000000000000001" {
		t.Fatal("numeric token changed or framing became header content")
	}
}
