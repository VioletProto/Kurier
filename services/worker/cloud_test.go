package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"log"
	"strings"
	"testing"

	runner "github.com/VioletProto/Kurier/services/request-runtime"
	"github.com/aws/aws-lambda-go/events"
)

func TestCredentialNamesFixtureUsesOnlyFixedMarkers(t *testing.T) {
	response, err := controlledEndpoint(context.Background(), events.APIGatewayV2HTTPRequest{
		PathParameters: map[string]string{"mode": "credential-names"},
		Headers:        map[string]string{"key": "caller-value-must-not-be-echoed"},
	})
	if err != nil || response.StatusCode != 200 || !response.IsBase64Encoded {
		t.Fatal("fixture response unavailable")
	}
	raw, err := base64.StdEncoding.DecodeString(response.Body)
	var body map[string]map[string]string
	if err != nil || json.Unmarshal(raw, &body) != nil {
		t.Fatal("fixture encoding invalid")
	}
	for _, name := range []string{"key", "auth", "pwd"} {
		want := "unmapped-fixture-" + name
		if response.Headers[name] != want || body["fields"][name] != want || body["query"][name] != want {
			t.Fatal("fixture did not emit independent fixed markers")
		}
	}
}

func TestAssertionFixturesLiteralResponsesAndSafeAudit(t *testing.T) {
	var audit bytes.Buffer
	old := log.Writer()
	log.SetOutput(&audit)
	defer log.SetOutput(old)
	for _, tc := range []struct {
		mode   string
		status int
		body   string
	}{
		{"assertions-values", 200, `{"precise":0.10000000000000001,"short":0.1,"exponent":1E-01,"min":1e-308,"max":9007199254740991,"zero":-0.0,"exists":null,"flag":true,"text":"public"}`},
		{"assertions-not-found", 404, `{"unavailable":true}`},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			for _, nonce := range []string{"927dd093-2ec1-4577-85c9-29e27648f4ee", "9dfd2d02-99c7-4e08-8e35-acd7b2b05084"} {
				audit.Reset()
				in := events.APIGatewayV2HTTPRequest{
					PathParameters:        map[string]string{"mode": tc.mode},
					QueryStringParameters: map[string]string{"fixture": nonce, "precise": "caller-private-query"},
					Headers:               map[string]string{"Authorization": "Bearer caller-private-auth", "Cookie": "caller-private-cookie"},
					Body:                  "caller-private-body",
					IsBase64Encoded:       true,
					RequestContext:        events.APIGatewayV2HTTPRequestContext{RequestID: "gateway-owned-request-id"},
				}
				r, err := controlledEndpoint(context.Background(), in)
				b, decode := base64.StdEncoding.DecodeString(r.Body)
				if err != nil || decode != nil || !r.IsBase64Encoded || r.StatusCode != tc.status || string(b) != tc.body || r.Headers["Content-Type"] != "application/json" || r.Headers["Cache-Control"] != "no-store" || len(r.Headers) != 2 {
					t.Fatal("fixture changed fixed response/headers")
				}
				line := strings.TrimSpace(audit.String())
				start := strings.IndexByte(line, '{')
				var record map[string]json.RawMessage
				if start < 0 || strings.Count(line, "\n") != 0 || json.Unmarshal([]byte(line[start:]), &record) != nil || len(record) != 5 ||
					string(record["kurierWorkflowFixture"]) != "true" || string(record["fixtureId"]) != `"`+nonce+`"` || string(record["mode"]) != `"`+tc.mode+`"` ||
					string(record["authorized"]) != "false" || string(record["requestId"]) != `"gateway-owned-request-id"` || strings.Contains(line, "caller-private") || strings.Contains(string(b), nonce) {
					t.Fatal("audit must contain only the existing fixed metadata envelope, one record per valid invocation")
				}
			}
			for _, invalid := range []string{"", "invalid\r\ncaller-private", "927dd093-2ec1-4577-85c9-29e27648f4ee\n", strings.Repeat("a", 1000)} {
				audit.Reset()
				r, err := controlledEndpoint(context.Background(), events.APIGatewayV2HTTPRequest{PathParameters: map[string]string{"mode": tc.mode}, QueryStringParameters: map[string]string{"fixture": invalid}})
				b, decode := base64.StdEncoding.DecodeString(r.Body)
				if err != nil || decode != nil || r.StatusCode != 400 || string(b) != `{"invalidFixture":true}` || audit.Len() != 0 {
					t.Fatal("invalid UUID was accepted, reflected or audited")
				}
			}
		})
	}
}

func TestAssertionValuesFixtureUsesExactRuntimeNumbers(t *testing.T) {
	r, err := controlledEndpoint(context.Background(), events.APIGatewayV2HTTPRequest{PathParameters: map[string]string{"mode": "assertions-values"}, QueryStringParameters: map[string]string{"fixture": "927dd093-2ec1-4577-85c9-29e27648f4ee"}})
	raw, decode := base64.StdEncoding.DecodeString(r.Body)
	if err != nil || decode != nil {
		t.Fatal("fixture unavailable")
	}
	var values map[string]any
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if d.Decode(&values) != nil {
		t.Fatal("fixture is invalid JSON")
	}
	for key, token := range map[string]string{"precise": "0.10000000000000001", "short": "0.1", "exponent": "1E-01", "min": "1e-308", "max": "9007199254740991", "zero": "-0.0"} {
		n, ok := values[key].(json.Number)
		if !ok || n.String() != token {
			t.Fatal("numeric token changed before evaluation")
		}
	}
	for _, tc := range []struct{ path, expected, outcome string }{
		{"$.precise", `{"type":"number","decimal":"0.10000000000000001"}`, "passed"},
		{"$.short", `{"type":"number","decimal":"0.1"}`, "passed"},
		{"$.short", `{"type":"number","decimal":"0.10000000000000001"}`, "failed"},
		{"$.precise", `{"type":"number","decimal":"0.1"}`, "failed"},
		{"$.exponent", `{"type":"number","decimal":"1e-1"}`, "passed"},
		{"$.exponent", `{"type":"number","decimal":"1E-01"}`, "passed"},
		{"$.zero", `{"type":"number","decimal":"0"}`, "passed"},
		{"$.min", `{"type":"number","decimal":"1e-308"}`, "passed"},
		{"$.max", `{"type":"number","decimal":"9007199254740991"}`, "passed"},
		{"$.exists", `null`, "passed"},
		{"$.missing", `null`, "failed"},
		{"$.flag", `true`, "passed"},
		{"$.text", `"public"`, "passed"},
	} {
		step := runner.WorkflowStep{Assertions: []runner.Assertion{{AssertionID: runner.NewID(), Kind: "jsonEquals", Path: tc.path, Expected: json.RawMessage(tc.expected)}}}
		result := runner.EvaluateAssertions(step, &r.StatusCode, raw, "application/json", runner.RedactionPolicy{}, nil, "")
		if len(result.Results) != 1 || result.Results[0].Outcome != tc.outcome {
			t.Fatal("fixture/runtime exact assertion mismatch", tc.path)
		}
	}
}

func TestWorkflowFixtureBindingAndSafeAudit(t *testing.T) {
	var audit bytes.Buffer
	old := log.Writer()
	log.SetOutput(&audit)
	defer log.SetOutput(old)
	nonce := "927dd093-2ec1-4577-85c9-29e27648f4ee"
	in := events.APIGatewayV2HTTPRequest{PathParameters: map[string]string{"mode": "workflow-token"}, QueryStringParameters: map[string]string{"fixture": nonce}}
	r, e := controlledEndpoint(context.Background(), in)
	raw, decode := base64.StdEncoding.DecodeString(r.Body)
	var issued map[string]string
	if e != nil || decode != nil || json.Unmarshal(raw, &issued) != nil || len(issued["token"]) != 64 {
		t.Fatal("token fixture unavailable")
	}
	in.PathParameters["mode"] = "workflow-consume"
	in.Headers = map[string]string{"Authorization": "Bearer " + issued["token"], "Cookie": "caller-private-cookie"}
	r, e = controlledEndpoint(context.Background(), in)
	raw, decode = base64.StdEncoding.DecodeString(r.Body)
	var reflected map[string]any
	if e != nil || decode != nil || json.Unmarshal(raw, &reflected) != nil || r.StatusCode != 200 || reflected["authorized"] != true || reflected["rawReflection"] != issued["token"] || reflected["renderedReflection"] != "Bearer "+issued["token"] {
		t.Fatal("bounded-header fixture mismatch")
	}
	if strings.Contains(audit.String(), issued["token"]) || strings.Contains(audit.String(), "caller-private-cookie") || strings.Count(audit.String(), nonce) != 2 {
		t.Fatal("fixture audit containment failure")
	}
	in.Headers = nil
	r, _ = controlledEndpoint(context.Background(), in)
	if r.StatusCode != 401 {
		t.Fatal("fixture accepted missing Bearer binding")
	}
	in.QueryStringParameters["fixture"] = "invalid\r\nvalue"
	before := audit.Len()
	r, _ = controlledEndpoint(context.Background(), in)
	if r.StatusCode != 400 || audit.Len() != before {
		t.Fatal("fixture accepted/logged invalid correlation")
	}
}
