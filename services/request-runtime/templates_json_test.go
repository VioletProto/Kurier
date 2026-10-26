package execution

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestRenderTemplateJSONDecodedValues(t *testing.T) {
	vars := map[string]templateScalar{
		"n":      {Raw: json.RawMessage(`0.10000000000000001`), Enabled: true},
		"b":      {Raw: json.RawMessage(`false`), Enabled: true},
		"z":      {Raw: json.RawMessage(`null`), Enabled: true},
		"inert":  {Raw: json.RawMessage(`"{{missing}}"`), Enabled: true},
		"secret": {Raw: json.RawMessage(`"synthetic-json-canary"`), Enabled: true, Protected: true},
	}
	for _, tc := range []struct {
		name, body, want string
		protected        bool
	}{
		{"unicode object", `{"n":"\u007b\u007bn\u007d\u007d","untouched":1E-01}`, `{"n":0.10000000000000001,"untouched":1E-01}`, false},
		{"mixed braces", `"\u007b{n}\u007d"`, `0.10000000000000001`, false},
		{"escaped opening", `"\u007b\u007bn}}"`, `0.10000000000000001`, false},
		{"escaped closing", `"{{n\u007d\u007d"`, `0.10000000000000001`, false},
		{"unicode root", `"\u007b\u007bn\u007d\u007d"`, `0.10000000000000001`, false},
		{"typed array", `["\u007b\u007bb\u007d\u007d","\u007b\u007bz\u007d\u007d"]`, `[false,null]`, false},
		{"array and single pass", `["\u007b\u007bn\u007d\u007d","\u007b{inert}\u007d",-0.0]`, `[0.10000000000000001,"{{missing}}",-0.0]`, false},
		{"protected", `"prefix \u007b\u007bsecret\u007d\u007d suffix"`, `"prefix synthetic-json-canary suffix"`, true},
		{"unchanged bytes", " \n{ \"z\":1E-01, \"a\":\"\\u0061\", \"slash\":\"\\/\" }\t", " \n{ \"z\":1E-01, \"a\":\"\\u0061\", \"slash\":\"\\/\" }\t", false},
		{"escaped backslashes are literal", ` ["\\u007b\\u007bn\\u007d\\u007d",1.00] `, ` ["\\u007b\\u007bn\\u007d\\u007d",1.00] `, false},
		{"unchanged root", " \"\\u0061\\/\"\n", " \"\\u0061\\/\"\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := []byte(tc.body)
			out, protected, err := renderTemplateJSON(raw, vars)
			if err != nil || string(out) != tc.want || protected != tc.protected {
				t.Fatalf("unexpected rendering: %q, protected=%v, err=%v", out, protected, err)
			}
			if string(raw) != tc.body {
				t.Fatal("input bytes mutated")
			}
		})
	}
}

func TestRenderTemplateJSONDecodedRejectionsAndLimits(t *testing.T) {
	vars := map[string]templateScalar{"s": {Raw: json.RawMessage(`"` + strings.Repeat("x", 2048) + `"`), Enabled: true}}
	for _, body := range []string{
		`{"\u007b\u007bs\u007d\u007d":"value"}`,
		`"\u007b\u007bmissing\u007d\u007d"`,
		`"\u007d\u007d"`,
		`"\u007b\u007bs"`,
		`"` + strings.Repeat(`\u007b\u007bs\u007d\u007d`, 33) + `"`,
		`"` + strings.Repeat(`\u007b\u007bs\u007d\u007d`, 32) + `x"`, // rendered output exceeds 64 KiB
		`[` + strings.Repeat(`0,`, 16383) + `0]`,                     // no markers still obey traversal bound
		`"` + strings.Repeat("x", 65535) + `"`,
		strings.Repeat(`[`, 33) + `0` + strings.Repeat(`]`, 33),
	} {
		if _, _, err := renderTemplateJSON([]byte(body), vars); err == nil {
			t.Fatal("invalid or oversized decoded template accepted")
		}
	}
}

func TestEnvironmentLiteralJSONOutgoingBytes(t *testing.T) {
	const body = " \n{ \"z\":1E-01, \"a\":\"\\u0061\", \"slash\":\"\\/\" }\t"
	p := environmentPlan(t, body, []FrozenVariable{})
	called := make(chan bool, 1)
	prepared := fixturePrepared(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		out, err := io.ReadAll(r.Body)
		called <- err == nil && string(out) == body
		w.WriteHeader(http.StatusNoContent)
	}), p, map[string]string{})
	prepared.Run(context.Background(), NewEvidence(NewID(), NewID(), stampTime(), p))
	select {
	case ok := <-called:
		if !ok {
			t.Fatal("literal JSON body bytes changed before dispatch")
		}
	default:
		t.Fatal("outgoing request was not sent")
	}
}

func TestEnvironmentDecodedJSONOutgoingBody(t *testing.T) {
	secretID := NewID()
	body := `{"values":["\u007b\u007bn\u007d\u007d","\u007b{inert}\u007d","\u007b\u007bsecret\u007d\u007d"],"untouched":1E-01}`
	p := environmentPlan(t, body, []FrozenVariable{
		{VariableID: NewID(), Name: "n", ValueType: "number", Value: testPrimitive("number", "0.10000000000000001")},
		{VariableID: NewID(), Name: "inert", ValueType: "string", Value: testPrimitive("string", `"{{missing}}"`)},
		{VariableID: NewID(), Name: "secret", ValueType: "string", Sensitive: true, SecretRef: &Ref{SecretID: secretID}},
	})
	const want = `{"untouched":1E-01,"values":[0.10000000000000001,"{{missing}}","synthetic-json-canary"]}`
	called := make(chan bool, 1)
	prepared := fixturePrepared(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		out, err := io.ReadAll(r.Body)
		called <- err == nil && string(out) == want
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"echo":"synthetic-json-canary"}`))
	}), p, map[string]string{secretID: "synthetic-json-canary"})
	evidence := NewEvidence(NewID(), NewID(), stampTime(), p)
	prepared.Run(context.Background(), evidence)
	select {
	case ok := <-called:
		if !ok {
			t.Fatal("actual outgoing body lost decoded substitution, numeric lexeme or single-pass insertion")
		}
	default:
		t.Fatal("outgoing request was not sent")
	}
	raw, err := json.Marshal(evidence)
	if err != nil || bytes.Contains(raw, []byte("synthetic-json-canary")) || !bytes.Contains(raw, []byte("[REDACTED]")) {
		t.Fatal("protected request/response body escaped evidence sanitization")
	}
	if p.Configuration.Body.Text != body {
		t.Fatal("execution mutated frozen source body")
	}
}
