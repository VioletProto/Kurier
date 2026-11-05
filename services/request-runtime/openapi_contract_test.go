package execution

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	contract "github.com/VioletProto/Kurier/services/request-runtime/openapi"
)

func contractFixturePlan(t *testing.T) Plan {
	t.Helper()
	doc, err := contract.Parse([]byte(`{"openapi":"3.0.3","info":{"title":"Fixture","version":"1"},"servers":[{"url":"https://fixture.example"}],"paths":{"/":{"get":{"responses":{"200":{"description":"ok","headers":{"X-Request-Id":{"required":true,"schema":{"type":"string"}}},"content":{"application/json":{"schema":{"type":"object","required":["value"],"properties":{"value":{"type":"number"}}}}}}}}}}}`), "json", contract.Policy{})
	if err != nil {
		t.Fatal(err)
	}
	p := fixturePlan()
	p.Configuration.OperationRef = &OperationRef{ImportID: NewID(), OperationID: doc.Operations[0].OperationID}
	p.FrozenContract = &FrozenContract{SchemaVersion: 1, ImportID: p.Configuration.OperationRef.ImportID, OperationID: p.Configuration.OperationRef.OperationID, Revision: 1, Operation: doc.Operations[0]}
	return p
}

func TestContractRuntimeFourChecksFailurePrecedenceAndSafeResults(t *testing.T) {
	for _, tc := range []struct {
		name, body, code string
		status           int
		header           bool
		secret           bool
		encoding         string
	}{
		{"pass", `{"value":0.10000000000000001}`, "", 200, true, false, ""},
		{"status", `{"value":1}`, "undocumented_status", 201, true, false, ""},
		{"required", `{}`, "missing_required_property", 200, true, false, ""},
		{"type", `{"value":"wrong"}`, "primitive_type_mismatch", 200, true, false, ""},
		{"header", `{"value":1}`, "missing_required_header", 200, false, false, ""},
		{"sensitive", `{"value":1,"password":"private-canary"}`, "sensitive_response", 200, true, false, ""},
		{"protected-input", `{"value":1}`, "sensitive_response", 200, true, true, ""},
		{"encoding", `{"value":1}`, "evaluation_unavailable", 200, true, false, "br"},
		{"http-error", `{}`, "undocumented_status", 500, true, false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan := contractFixturePlan(t)
			var values map[string]string
			if tc.secret {
				values = map[string]string{"fixture": "private-canary"}
			}
			prepared := fixturePrepared(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if tc.header {
					w.Header().Set("X-Request-Id", "public-fixture")
				}
				if tc.encoding != "" {
					w.Header().Set("Content-Encoding", tc.encoding)
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}), plan, values)
			e := NewEvidence(NewID(), NewID(), stampTime(), plan)
			prepared.Run(context.Background(), e)
			if e.SchemaVersion != 4 || e.Contract == nil || ValidateEvidenceVersion(e) != nil {
				t.Fatalf("invalid contract evidence: %#v", e.Contract)
			}
			if tc.code == "" {
				if e.Status != "completed" || e.Contract.Result.Status != "passed" {
					t.Fatal("successful contract failed")
				}
			} else {
				if e.Status != "failed" {
					t.Fatal("failed/unavailable contract completed")
				}
				found := false
				for _, check := range e.Contract.Result.Checks {
					found = found || check.Code == tc.code
				}
				if !found {
					t.Fatalf("missing fixed reason %s: %#v", tc.code, e.Contract.Result)
				}
				want := "contract_validation_failed"
				if tc.name == "http-error" {
					want = "upstream_http_error"
				}
				if e.Outcome.Code == nil || *e.Outcome.Code != want {
					t.Fatal("failure precedence changed")
				}
			}
			raw, err := json.Marshal(e)
			if err != nil || strings.Contains(string(raw), "private-canary") {
				t.Fatal("unsafe evidence")
			}
			var decoded Evidence
			if StrictJSON(raw, &decoded) != nil {
				t.Fatal("evidence4 roundtrip failed")
			}
			projection, _ := json.Marshal(e.LegacyProjection())
			if strings.Contains(string(projection), `"contract":`) || strings.Contains(string(projection), `"importId":`) {
				t.Fatal("legacy projection expanded")
			}
		})
	}
}

func TestContractEvidenceRejectsUnknownResultAndUnfrozenOrLocalAssociation(t *testing.T) {
	plan := contractFixturePlan(t)
	e := NewEvidence(NewID(), NewID(), stampTime(), plan)
	uncertain := NewEvidence(NewID(), NewID(), stampTime(), plan)
	uncertain.Fail("execution_outcome_unknown", "unknown")
	if uncertain.Contract.Result.Checks[0].Code != "evaluation_unavailable" || ValidateEvidenceVersion(uncertain) != nil {
		t.Fatal("uncertain recovery claimed contract checks were never dispatched")
	}
	raw, _ := json.Marshal(e)
	for _, bad := range []string{
		strings.TrimSuffix(string(raw), "}") + `,"Contract":null}`,
		strings.Replace(string(raw), `"importId":`, `"ImportID":`, 1),
		strings.Replace(string(raw), `"contract":{`, `"contract":null,"discarded":{`, 1),
		strings.Replace(string(raw), `"not_dispatched"`, `"private-parser-text"`, 1),
		strings.Replace(string(raw), `"schemaVersion":4`, `"schemaVersion":3`, 1),
	} {
		var v Evidence
		if StrictJSON([]byte(bad), &v) == nil {
			t.Fatal("malformed/legacy-expanded contract accepted")
		}
	}
	plan.FrozenContract = nil
	if _, err := preparePolicy(context.Background(), plan, nil, nil, nil, false); err == nil {
		t.Fatal("unfrozen contract prepared")
	}
	plan = contractFixturePlan(t)
	plan.Target = "local"
	if _, err := preparePolicy(context.Background(), plan, nil, nil, nil, false); err == nil {
		t.Fatal("local contract prepared")
	}
}
