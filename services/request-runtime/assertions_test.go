package execution

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func assertionFixture(kind, path, expected string) Assertion {
	a := Assertion{AssertionID: NewID(), Kind: kind, Path: path}
	if expected != "" {
		a.Expected = json.RawMessage(expected)
	}
	return a
}
func numberExpected(token string) string {
	b, _ := json.Marshal(map[string]string{"type": "number", "decimal": token})
	return string(b)
}

func TestAssertionDecimalExactDomainAndDTO(t *testing.T) {
	for _, token := range []string{"0.10000000000000001", "0.1", "1", "1.0", "1e0", "1E+00", "10e-1", "1e-000", "0", "-0", "-0.0", "0E+308", "9007199254740991", "-9007199254740991", "9007199254740990.999999999999999999", "1e-308", "-1E-308", "10e-308", "1." + strings.Repeat("0", 126)} {
		a := assertionFixture("jsonEquals", "$.ratio", numberExpected(token))
		if a.validate() != nil {
			t.Fatal("valid bounded token rejected", token)
		}
		b, _ := json.Marshal(a)
		var reopened Assertion
		if StrictJSON(b, &reopened) != nil {
			t.Fatal("tagged token lost during strict round trip")
		}
		var tag struct {
			Type    string `json:"type"`
			Decimal string `json:"decimal"`
		}
		_ = json.Unmarshal(reopened.Expected, &tag)
		if tag.Decimal != token {
			t.Fatal("authored decimal changed")
		}
	}
	for _, token := range []string{"", " 1", "1 ", "+1", "01", ".1", "1.", "NaN", "Infinity", "0,1", "١", "9007199254740992", "-9007199254740992", "9007199254740991.1", "1e-309", "1e309", "1e308", "0e309", "1e-999999999999999999", "1." + strings.Repeat("0", 127)} {
		if _, e := ParseAssertionDecimal(token); e == nil {
			t.Fatal("invalid token accepted", token)
		}
	}
	for _, expected := range []string{`0.1`, `1`, `[]`, `{}`, `{"type":"number","decimal":0.1}`, `{"type":"number","decimal":null}`, `{"type":"number","decimal":"1","extra":true}`, `{"Type":"number","decimal":"1"}`, `{"type":"number","decimal":"1","decimal":"2"}`} {
		var a Assertion
		raw := `{"assertionId":"` + NewID() + `","kind":"jsonEquals","path":"$","expected":` + expected + `}`
		if StrictJSON([]byte(raw), &a) == nil {
			t.Fatal("invalid expected DTO accepted")
		}
	}
	for _, extra := range []string{`,"path":""`, `,"Kind":"statusEquals"`, `,"expected":200`} {
		var a Assertion
		if StrictJSON([]byte(`{"assertionId":"`+NewID()+`","kind":"statusEquals","expected":200`+extra+`}`), &a) == nil {
			t.Fatal("invalid union fields accepted")
		}
	}
}

func TestAssertionPathTypeAndNumberEvaluation(t *testing.T) {
	tests := []struct{ kind, path, expected, body, outcome, reason string }{
		{"jsonEquals", "$.n", numberExpected("0.10000000000000001"), `{"n":0.10000000000000001}`, "passed", "matched"},
		{"jsonEquals", "$.n", numberExpected("0.10000000000000001"), `{"n":0.1}`, "failed", "value_mismatch"},
		{"jsonEquals", "$.n", numberExpected("0.1"), `{"n":0.10000000000000001}`, "failed", "value_mismatch"},
		{"jsonEquals", "$", numberExpected("1E+00"), `1.0`, "passed", "matched"},
		{"jsonEquals", "$", numberExpected("-0.0"), `0e1`, "passed", "matched"},
		{"jsonEquals", "$", numberExpected("1e-308"), `1e-308`, "passed", "matched"},
		{"jsonEquals", "$", numberExpected("9007199254740991"), `9007199254740991`, "passed", "matched"},
		{"jsonEquals", "$", `"1"`, `1`, "failed", "type_mismatch"},
		{"jsonEquals", "$.n", `null`, `{"n":null}`, "passed", "matched"},
		{"jsonEquals", "$.n", `null`, `{}`, "failed", "path_missing"},
		{"jsonEquals", "$.n", `false`, `{"n":false}`, "passed", "matched"},
		{"jsonEquals", "$.n", `""`, `{"n":""}`, "passed", "matched"},
		{"jsonEquals", "$", `"é"`, `"e\u0301"`, "failed", "value_mismatch"},
		{"jsonEquals", "$", `"é"`, `"\u00e9"`, "passed", "matched"},
		{"jsonEquals", "$.n", `null`, `{"n":[]}`, "failed", "type_mismatch"},
		{"jsonExists", "$.n", "", `{"n":{}}`, "passed", "exists"},
		{"jsonExists", "$.n", "", `{"n":null}`, "passed", "exists"},
		{"jsonExists", "$", "", `null`, "passed", "exists"},
		{"jsonExists", "$[0].a", "", `[{"a":[]}]`, "passed", "exists"},
		{"jsonExists", "$[1]", "", `[0]`, "failed", "path_missing"},
		{"jsonExists", "$.a.b", "", `{"a":false}`, "failed", "path_missing"},
		{"jsonExists", `$["a.b"][""]`, "", `{"a.b":{"":true}}`, "passed", "exists"},
		{"jsonExists", "$", "", `{"a":1,"a":2}`, "not_evaluated", "json_invalid"},
		{"jsonExists", "$", "", `{"a":"\ud800"}`, "not_evaluated", "json_invalid"},
		{"jsonExists", "$", "", `9007199254740992`, "not_evaluated", "json_number_unsupported"},
		{"jsonEquals", "$.n", `true`, `{"n":true,"elsewhere":1e999}`, "not_evaluated", "json_number_unsupported"},
		{"jsonExists", "$", "", ``, "not_evaluated", "body_absent"},
	}
	for _, tt := range tests {
		t.Run(tt.kind+tt.path+tt.reason, func(t *testing.T) {
			a := assertionFixture(tt.kind, tt.path, tt.expected)
			out := EvaluateAssertions(WorkflowStep{Assertions: []Assertion{a}}, pointer(200), []byte(tt.body), "application/json", RedactionPolicy{}, nil, "")
			r := out.Results[0]
			if r.Outcome != tt.outcome || r.Reason != tt.reason {
				t.Fatalf("got %s/%s", r.Outcome, r.Reason)
			}
		})
	}
}

func TestAssertionStaticAndDynamicContainment(t *testing.T) {
	for _, path := range []string{"$.token", "$.user.password", "$[\"api_key\"]", "$.value", "$"} {
		s := WorkflowStep{Extract: []Extraction{{Name: "x", Path: "$.value", Type: "string"}}, Assertions: []Assertion{assertionFixture("jsonEquals", path, `"public"`)}}
		if ValidateAssertions(s, RedactionPolicy{}) == nil {
			t.Fatal("unsafe overlap accepted")
		}
	}
	for _, path := range []string{"$.user", "$.user.id", "$[\"user\"][\"id\"].n"} {
		s := WorkflowStep{Assertions: []Assertion{assertionFixture("jsonEquals", path, `null`)}}
		if ValidateAssertions(s, RedactionPolicy{JSONPointers: []string{"/user/id"}}) == nil {
			t.Fatal("configured overlap accepted")
		}
	}
	secret := "disposable-assertion-secret"
	a := assertionFixture("jsonEquals", "$.public", `"public"`)
	out := EvaluateAssertions(WorkflowStep{Assertions: []Assertion{a}}, pointer(200), []byte(`{"public":"`+secret+`"}`), "application/json", RedactionPolicy{}, map[string]string{"runtime": secret}, "")
	if out.Results[0].Reason != "sensitive_input" {
		t.Fatal("reflected secret comparison disclosed")
	}
	encoded, _ := json.Marshal(out)
	if strings.Contains(string(encoded), secret) || strings.Contains(string(encoded), "$.public") || strings.Contains(string(encoded), "expected") {
		t.Fatal("unsafe result")
	}
	a = assertionFixture("jsonExists", "$.token", "")
	out = EvaluateAssertions(WorkflowStep{Assertions: []Assertion{a}}, pointer(200), []byte(`{"token":"`+secret+`"}`), "application/json", RedactionPolicy{}, nil, "")
	if out.Results[0].Outcome != "passed" {
		t.Fatal("existence presence policy changed")
	}
}

func TestAssertionRuntimeFailurePrecedenceAndOmission(t *testing.T) {
	for _, tt := range []struct {
		name, body, media, expected, code string
		status                            int
		extract                           bool
		omit                              bool
	}{
		{"success", `{"ok":true}`, "application/json", `true`, "", 200, false, false},
		{"wrong", `{"ok":false}`, "application/json", `true`, "assertion_failed", 200, false, false},
		{"http", `{"ok":false}`, "application/json", `true`, "upstream_http_error", 404, false, false},
		{"invalid", `invalid`, "application/json", `true`, "assertion_failed", 200, false, false},
		{"html", `<html>`, "text/html", `true`, "assertion_failed", 200, false, false},
		{"empty", ``, "application/json", `true`, "assertion_failed", 200, false, false},
		{"oversize", strings.Repeat("x", BodyLimit+1), "application/json", `true`, "response_limit_exceeded", 200, false, false},
		{"policy omission", `{"ok":true}`, "application/json", `true`, "", 200, false, true},
		{"extraction", `{"ok":false}`, "application/json", `true`, "extraction_failed", 200, true, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := fixturePlan()
			p.Options.ResponseRedaction.OmitBody = tt.omit
			s := WorkflowStep{Assertions: []Assertion{assertionFixture("jsonEquals", "$.ok", tt.expected)}}
			if tt.omit {
				s.Assertions = []Assertion{assertionFixture("jsonExists", "$.ok", "")}
			}
			if tt.extract {
				s.Extract = []Extraction{{Name: "x", Path: "$.missing", Type: "string"}}
			}
			prepared := fixturePrepared(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tt.media)
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}), p, nil)
			prepared.WorkflowCapture(&ExtractionCapture{AssertionSemanticsVersion: 1, Step: s, Values: map[string]string{}, Outputs: map[string]RuntimeValue{}})
			e := NewEvidence("p", "e", stampTime(), p)
			prepared.Run(context.Background(), e)
			if e.SchemaVersion != 2 || e.Assertions == nil || e.Response.HTTPStatus == nil || *e.Response.HTTPStatus != tt.status {
				t.Fatal("exact capture metadata lost")
			}
			if tt.code == "" {
				if e.Status != "completed" || e.Outcome.Code != nil {
					t.Fatal("false failure")
				}
			} else if e.Outcome.Code == nil || *e.Outcome.Code != tt.code {
				t.Fatalf("wrong precedence: %v", e.Outcome)
			}
			if tt.omit && (e.Response.Body.Kind != "omitted" || !e.Assertions.Passed()) {
				t.Fatal("omission invalidated raw evaluation")
			}
			if ValidateFrozenAssertions(e, s, 1) != nil {
				t.Fatal("invalid result publication")
			}
		})
	}
}

func TestAssertionLimitsAndNullAuthoring(t *testing.T) {
	a := assertionFixture("jsonEquals", "$.n", `"`+strings.Repeat("x", 2046)+`"`)
	if a.validate() != nil {
		t.Fatal("exact expected cap rejected")
	}
	a.Expected = append(a.Expected[:len(a.Expected)-1], 'x', '"')
	if a.validate() == nil {
		t.Fatal("oversize expected accepted")
	}
	s := WorkflowStep{Assertions: []Assertion{assertionFixture("statusEquals", "", `200`), assertionFixture("statusEquals", "", `201`)}}
	if ValidateAssertions(s, RedactionPolicy{}) == nil {
		t.Fatal("multiple status accepted")
	}
	s.Assertions = []Assertion{}
	for i := 0; i < 8; i++ {
		s.Assertions = append(s.Assertions, assertionFixture("jsonEquals", "$.ok", `"`+strings.Repeat("x", 1000)+`"`))
	}
	if ValidateAssertions(s, RedactionPolicy{}) == nil {
		t.Fatal("array aggregate overflow accepted")
	}
	valid := `{"steps":[{"assertions":[{"kind":"jsonEquals","expected":null}]}]}`
	if ValidateWorkflowJSON([]byte(valid)) != nil {
		t.Fatal("explicit null denied")
	}
	for _, raw := range []string{`null`, `{"name":null}`, `{"steps":null}`, `{"steps":[{"assertions":null}]}`, `{"steps":[{"assertions":[{"expected":{"decimal":null}}]}]}`} {
		if ValidateWorkflowJSON([]byte(raw)) == nil {
			t.Fatal("unrelated null admitted")
		}
	}
}

func TestAssertionArrayExactByteBudget(t *testing.T) {
	encoded := assertionFixture("jsonEquals", "$.ok", `"`+strings.Repeat("<", 341)+`"`)
	if encoded.validate() != nil {
		t.Fatal("exact HTML-escaped primitive cap rejected")
	}
	encoded.Expected = json.RawMessage(`"` + strings.Repeat("<", 342) + `"`)
	if encoded.validate() == nil {
		t.Fatal("escaped expected bytes bypassed cap")
	}
	s := WorkflowStep{}
	for i := 0; i < 3; i++ {
		s.Assertions = append(s.Assertions, assertionFixture("jsonEquals", "$.ok", `"`+strings.Repeat("x", 2046)+`"`))
	}
	s.Assertions = append(s.Assertions, assertionFixture("jsonEquals", "$.ok", `""`))
	b, _ := json.Marshal(s.Assertions)
	padding := AssertionArrayLimit - len(b)
	if padding < 0 || padding > 2046 {
		t.Fatal("fixture does not exercise combined byte limit")
	}
	s.Assertions[3].Expected = json.RawMessage(`"` + strings.Repeat("x", padding) + `"`)
	if ValidateAssertions(s, RedactionPolicy{}) != nil {
		t.Fatal("exact array cap rejected")
	}
	s.Assertions[3].Expected = json.RawMessage(`"` + strings.Repeat("x", padding+1) + `"`)
	if ValidateAssertions(s, RedactionPolicy{}) == nil {
		t.Fatal("one-over array cap accepted")
	}
}

func TestAssertionStorageFailureRetainsBaselinePrecedence(t *testing.T) {
	for _, semantics := range []int{0, 1} {
		p := fixturePlan()
		step := WorkflowStep{Extract: []Extraction{{Name: "x", Path: "$.value", Type: "string"}}, Assertions: []Assertion{assertionFixture("statusEquals", "", `404`)}}
		prepared := fixturePrepared(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(404)
			_, _ = w.Write([]byte(`{"value":"public"}`))
		}), p, nil)
		capture := &ExtractionCapture{AssertionSemanticsVersion: semantics, Step: step, Values: map[string]string{}, Outputs: map[string]RuntimeValue{}, Seal: func(map[string]RuntimeValue) error { return errors.New("fixture storage failure") }}
		prepared.WorkflowCapture(capture)
		e := NewEvidence("p", "e", stampTime(), p)
		prepared.Run(context.Background(), e)
		if e.Outcome.Code == nil || *e.Outcome.Code != "runtime_storage_failed" {
			t.Fatal("HTTP classification overwrote baseline storage failure")
		}
	}
}
