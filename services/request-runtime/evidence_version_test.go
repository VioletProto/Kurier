package execution

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestVersionedAssertionEvidenceDoesNotChangeLegacyBytes(t *testing.T) {
	e := NewEvidence("p", "e", stampTime(), fixturePlan())
	old, _ := json.Marshal(e.LegacyProjection())
	current, _ := json.Marshal(e)
	if string(old) != string(current) {
		t.Fatal("schema-1 evidence bytes changed")
	}
	for _, raw := range []string{strings.Replace(string(old), `"schemaVersion":1`, `"schemaVersion":99`, 1), strings.TrimSuffix(string(old), "}") + `,"assertions":null}`} {
		var decoded Evidence
		if StrictJSON([]byte(raw), &decoded) == nil {
			t.Fatal("unknown version/expanded v1 accepted")
		}
	}
	s := WorkflowStep{Assertions: []Assertion{assertionFixture("jsonEquals", "$", `true`)}}
	e.SchemaVersion = 2
	e.Assertions = NewAssertionEvaluation(s, "evaluation_unavailable")
	b, _ := json.Marshal(e)
	var decoded Evidence
	if StrictJSON(b, &decoded) != nil || ValidateFrozenAssertions(&decoded, s, 1) != nil {
		t.Fatal("valid unavailable results rejected")
	}
	decoded.Assertions.Results[0].AssertionID = NewID()
	if ValidateFrozenAssertions(&decoded, s, 1) == nil {
		t.Fatal("foreign results accepted")
	}
	decoded.Assertions.Results[0].Reason = "untrusted arbitrary diagnostic"
	if ValidateEvidenceVersion(&decoded) == nil {
		t.Fatal("unbounded reason accepted")
	}
	e.Assertions = NewAssertionEvaluation(WorkflowStep{}, "")
	if ValidateEvidenceVersion(e) != nil || e.Assertions.State != "not_configured" {
		t.Fatal("no rules fabricated pass")
	}
}

func TestWorstAssertionResultWidthFitsCap(t *testing.T) {
	s := WorkflowStep{}
	for i := 0; i < 8; i++ {
		s.Assertions = append(s.Assertions, assertionFixture("jsonEquals", "$", `true`))
	}
	r := NewAssertionEvaluation(s, "json_number_unsupported")
	b, _ := json.Marshal(r)
	if len(b) > AssertionResultLimit {
		t.Fatal("bounded results overflow")
	}
	if ValidateFrozenAssertions(&Evidence{SchemaVersion: 2, Target: "cloud", Assertions: r}, s, 1) != nil {
		t.Fatal("result bound invalid")
	}
}
