package mcpwire

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestStrictToolBoundary(t *testing.T) {
	id := "00000000-0000-4000-8000-000000000001"
	for _, tc := range []struct {
		tool, raw string
		valid     bool
	}{{"get_execution", `{"projectId":"` + id + `","executionId":"` + id + `"}`, true}, {"list_projects", `{}`, true}, {"list_projects", `{"limit":10}`, true}, {"list_projects", `{"limit":11}`, false}, {"list_projects", `{"limit":null}`, false}, {"list_projects", `{"limit":1,"limit":2}`, false}, {"list_projects", `{"ownerId":"secret"}`, false}, {"list_projects", `null`, false}, {"list_projects", `{} {}`, false}, {"get_execution", `{"executionId":"` + id + `"}`, false}, {"list_workflow_runs", `{"projectId":"` + id + `","limit":6}`, false}, {"submit_execution", `{}`, false}, {"get_execution_evidence", `{"projectId":"` + id + `","executionId":"` + id + `","view":"raw"}`, false}} {
		_, e := Parse(tc.tool, []byte(tc.raw))
		if (e == nil) != tc.valid {
			t.Fatal("validation contract mismatch", tc.tool)
		}
	}
}
func TestOutputBoundAndProjection(t *testing.T) {
	v := Envelope{Data: &Data{SchemaVersion: 1, ServerTime: "2026-10-15T00:00:00Z", Items: []json.RawMessage{json.RawMessage(`{"projectId":"id","name":"public","createdAt":"now","updatedAt":"now"}`)}}}
	if _, e := Decode("list_projects", mustJSON(t, v)); e != nil {
		t.Fatal("valid projection rejected")
	}
	v.Data.Items = []json.RawMessage{json.RawMessage(`{"projectId":"id","name":"public","createdAt":"now","updatedAt":"now","ciphertext":"canary"}`)}
	if _, e := Decode("list_projects", mustJSON(t, v)); e == nil {
		t.Fatal("unknown field exposed")
	}
	for _, text := range []string{strings.Repeat("x", OutputLimit), strings.Repeat("\\\"<\n", OutputLimit/4)} {
		if Fits(map[string]string{"body": text}) {
			t.Fatal("encoded output bound bypass")
		}
	}
}
func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
