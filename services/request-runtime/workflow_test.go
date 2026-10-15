package execution

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestExtractionGrammar(t *testing.T) {
	for _, path := range []string{"$", "$.token", "$[0]", `$["a.b"]["quote\"key"][""]`, "$[12].a_1"} {
		if _, e := ParseExtractionPath(path); e != nil {
			t.Errorf("valid grammar rejected")
		}
	}
	for _, path := range []string{"", "a", "$..x", "$.*", "$[-1]", "$[01]", "$[1:2]", "$[?(@.x)]", "$.0", `$['a']`, "$.", "$" + strings.Repeat(".a", 33), "$[" + strings.Repeat("1", 513) + "]"} {
		if _, e := ParseExtractionPath(path); e == nil {
			t.Errorf("invalid grammar accepted")
		}
	}
}
func TestHeaderRenderingBounds(t *testing.T) {
	prefix, suffix := strings.Repeat("é", 128), strings.Repeat("x", 256)
	target := BindingTarget{Kind: "header", Prefix: &prefix, Suffix: &suffix}
	if _, e := RenderHeader(strings.Repeat("t", 1536), target); e != nil {
		t.Fatal(e)
	}
	if _, e := RenderHeader(strings.Repeat("t", 1537), target); e == nil {
		t.Fatal("render over cap accepted")
	}
	for _, v := range []string{strings.Repeat("x", 257), "a\rb", "a\nb", "a\x00b", "a\x7fb", "a\x01b"} {
		if _, e := RenderHeader("token", BindingTarget{Prefix: &v}); e == nil {
			t.Fatal("invalid prefix accepted")
		}
		if _, e := RenderHeader("token", BindingTarget{Suffix: &v}); e == nil {
			t.Fatal("invalid suffix accepted")
		}
	}
	for _, v := range []string{"a\rb", "a\nb", "a\x00b", "a\x7fb"} {
		if _, e := RenderHeader(v, BindingTarget{}); e == nil {
			t.Fatal("invalid source accepted")
		}
	}
	inert := "{{token}}"
	v, e := RenderHeader("raw", BindingTarget{Prefix: &inert})
	if e != nil || v != "{{token}}raw" {
		t.Fatal("literal evaluated")
	}
	if _, e := RenderHeader("\t", BindingTarget{}); e != nil {
		t.Fatal(e)
	}
}
func workflowFixture() ([]WorkflowStep, []Plan) {
	first, second := NewID(), NewID()
	p := fixturePlan()
	p.Configuration.Headers = []Field{{Name: "Authorization", Value: "placeholder", Enabled: true}}
	return []WorkflowStep{{StepID: first, RequestID: p.RequestID, RequestRevision: 1, Extract: []Extraction{{Name: "token", Path: "$.value", Type: "string"}}, Bindings: []Binding{}}, {StepID: second, RequestID: p.RequestID, RequestRevision: 1, Extract: []Extraction{}, Bindings: []Binding{{From: Producer{first, "token"}, To: BindingTarget{Kind: "header", Index: pointer(0), Prefix: pointer("Bearer ")}}}}}, []Plan{p, p}
}
func TestWorkflowTypedBindingsAndConsent(t *testing.T) {
	steps, plans := workflowFixture()
	if e := ValidateWorkflow(steps, plans); e != nil {
		t.Fatal(e)
	}
	outputs := map[string]RuntimeValue{RuntimeKey(steps[0].StepID, "token"): {Value: json.RawMessage(`"raw-token"`)}}
	if _, e := ApplyWorkflowBindings(plans[1], steps[1], outputs, map[string]string{}); e == nil {
		t.Fatal("HTTP credential consent bypass")
	}
	plans[1].Options.AllowInsecureSecrets = true
	resolved, e := ApplyWorkflowBindings(plans[1], steps[1], outputs, map[string]string{})
	if e != nil || resolved.Configuration.Headers[0].Value != "Bearer raw-token" {
		t.Fatal("header binding failed")
	}
	if plans[1].Configuration.Headers[0].Value != "placeholder" {
		t.Fatal("frozen plan mutated")
	}
	steps[1].Bindings[0].To.Kind = "query"
	plans[1].Configuration.QueryParameters = []Field{{Name: "id", Enabled: true}}
	if e := ValidateWorkflow(steps, plans); e == nil {
		t.Fatal("query affix accepted")
	}
	steps[1].Bindings[0].To.Prefix = nil
	steps[1].Bindings[0].To.Suffix = nil
	if e := ValidateWorkflow(steps, plans); e != nil {
		t.Fatal(e)
	}
	steps[0].Extract[0].Type = "number"
	steps[1].Bindings[0].To.Kind = "header"
	steps[1].Bindings[0].To.Prefix = pointer("")
	if e := ValidateWorkflow(steps, plans); e == nil {
		t.Fatal("numeric affix accepted")
	}
}
func TestWorkflowExtractionBeforeDisposalContainment(t *testing.T) {
	steps, plans := workflowFixture()
	secret := "disposable-runtime-value"
	plans[0].Options.AllowInsecureSecrets = true
	prepared := fixturePrepared(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Echo", "Bearer "+secret)
		_, _ = w.Write([]byte(`{"value":"` + secret + `","echo":"Bearer ` + secret + `"}`))
	}), plans[0], map[string]string{})
	capture := &ExtractionCapture{Step: steps[0], Steps: steps, Plans: plans, Outputs: map[string]RuntimeValue{}, Values: map[string]string{}}
	prepared.WorkflowCapture(capture)
	evidence := NewEvidence("p", "e", stampTime(), plans[0])
	prepared.Run(context.Background(), evidence)
	if evidence.Status != "completed" {
		t.Fatal("capture failed")
	}
	value := capture.Outputs[RuntimeKey(steps[0].StepID, "token")]
	if !value.Sensitive || string(value.Value) != `"`+secret+`"` {
		t.Fatal("provenance or extraction lost")
	}
	raw, _ := json.Marshal(evidence)
	if strings.Contains(string(raw), secret) || strings.Contains(string(raw), "Bearer "+secret) {
		t.Fatal("runtime secret escaped sanitized capture")
	}
}
func TestWorkflowExtractionFailureBounds(t *testing.T) {
	steps, plans := workflowFixture()
	for _, raw := range []string{`{}`, `{"value":null}`, `{"value":{}}`, `{"value":1}`, `{"value":"` + strings.Repeat("a", 2047) + `"}`, `{"value":"a","value":"b"}`} {
		c := ExtractionCapture{Step: steps[0], Steps: steps, Plans: plans, Outputs: map[string]RuntimeValue{}, Values: map[string]string{}}
		if _, e := c.capture([]byte(raw), "application/json"); e == nil {
			t.Fatal("invalid extraction accepted")
		}
	}
	for _, raw := range []string{`9007199254740992`, `1e999`} {
		var v any
		_ = StrictJSON([]byte(raw), &v)
		if _, e := scalar(v); e == nil {
			t.Fatal("unsafe numeric accepted")
		}
	}
}
func TestWorkflowJSONAndQueryBindingsRemainTyped(t *testing.T) {
	steps, plans := workflowFixture()
	steps[0].Extract[0].Type = "number"
	steps[1].Bindings[0].To = BindingTarget{Kind: "jsonBody", Pointer: pointer("/id")}
	plans[1].Configuration.Body = &RequestBody{Type: "json", Text: `{"id":0}`}
	if e := ValidateWorkflow(steps, plans); e != nil {
		t.Fatal(e)
	}
	values := map[string]RuntimeValue{RuntimeKey(steps[0].StepID, "token"): {Value: json.RawMessage(`123`)}}
	p, e := ApplyWorkflowBindings(plans[1], steps[1], values, map[string]string{})
	if e != nil || p.Configuration.Body.Text != `{"id":123}` {
		t.Fatal("JSON primitive type lost")
	}
	steps[1].Bindings[0].To = BindingTarget{Kind: "query", Index: pointer(0)}
	plans[1].Configuration.QueryParameters = []Field{{Name: "id", Enabled: true}}
	if e := ValidateWorkflow(steps, plans); e != nil {
		t.Fatal(e)
	}
	p, e = ApplyWorkflowBindings(plans[1], steps[1], values, map[string]string{})
	if e != nil || p.Configuration.QueryParameters[0].Value != "123" {
		t.Fatal("query numeric conversion changed")
	}
}
func TestWorkflowExtractionExactScalarLimitAndQuotedProperties(t *testing.T) {
	steps, plans := workflowFixture()
	steps[0].Extract[0].Path = `$["a.b"][""]`
	c := ExtractionCapture{Step: steps[0], Steps: steps, Plans: plans, Outputs: map[string]RuntimeValue{}, Values: map[string]string{}}
	raw, _ := json.Marshal(map[string]any{"a.b": map[string]any{"": strings.Repeat("a", 2046)}})
	paths, e := c.capture(raw, "application/problem+json")
	if e != nil || len(paths) != 1 || paths[0] != "/a.b/" {
		t.Fatal("exact scalar bound or quoted lookup failed")
	}
	if len(c.Outputs[RuntimeKey(steps[0].StepID, "token")].Value) != 2048 {
		t.Fatal("serialized scalar accounting")
	}
	steps[0].Extract[0].Path = "$"
	steps[0].Extract[0].Type = "null"
	c.Step = steps[0]
	if _, e = c.capture([]byte(`null`), "application/json"); e != nil {
		t.Fatal("null root treated as missing")
	}
}
func TestWorkflowSealsOutputsBeforeRawDisposalAndFailsClosed(t *testing.T) {
	steps, plans := workflowFixture()
	capture := &ExtractionCapture{Step: steps[0], Steps: steps, Plans: plans, Outputs: map[string]RuntimeValue{}, Values: map[string]string{}}
	called := false
	capture.Seal = func(values map[string]RuntimeValue) error {
		called = true
		if len(values) != 1 {
			t.Error("seal missed required extraction")
		}
		return errors.New("simulated encryption outage")
	}
	prepared := fixturePrepared(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"value":"ephemeral-seal-fixture"}`))
	}), plans[0], map[string]string{})
	prepared.WorkflowCapture(capture)
	e := NewEvidence("p", "e", stampTime(), plans[0])
	prepared.Run(context.Background(), e)
	raw, _ := json.Marshal(e)
	if !called || e.Status != "failed" || e.Outcome.Code == nil || *e.Outcome.Code != "runtime_storage_failed" || strings.Contains(string(raw), "ephemeral-seal-fixture") {
		t.Fatal("encryption failure bypassed sealed-before-disposal containment")
	}
}
