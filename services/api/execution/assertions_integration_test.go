//go:build integration

package execution

import (
	"context"
	"encoding/json"
	"errors"
	runner "github.com/VioletProto/Kurier/services/request-runtime"
	wire "github.com/VioletProto/Kurier/services/request-runtime/mcpwire"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"net/http"
	"strings"
	"testing"
	"time"
)

func enableAssertionFixture(t *testing.T, f *protocolFixture) {
	t.Helper()
	stage, e := f.s.stage(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	stage.SavedRequestsSchemaVersion = 8
	stage.WorkflowAssertionsSchemaVersion = 1
	stage.MCPReadOnlySchemaVersion = 1
	stage.Version++
	f.write(t, f.s.Table, stage)
}
func assertionRule(kind, path, expected string) runner.Assertion {
	a := runner.Assertion{AssertionID: id(), Kind: kind, Path: path}
	if expected != "" {
		a.Expected = json.RawMessage(expected)
	}
	return a
}
func assertionWorkflowFixture(t *testing.T, rules []runner.Assertion, body string, status int) (*protocolFixture, Workflow) {
	f, wf := workflowProtocolFixture(t)
	enableAssertionFixture(t, f)
	wf.Steps[0].Extract = []runner.Extraction{}
	wf.Steps[1].Bindings = []runner.Binding{}
	wf.Steps[0].Assertions = rules
	updated, e := f.s.SaveWorkflow(context.Background(), f.owner, f.project, wf.WorkflowID, wf.Revision, wf.WorkflowDefinition, false)
	if e != nil {
		t.Fatal(e)
	}
	f.s.prepare = func(ctx context.Context, plan Plan, values map[string]string) (*Prepared, error) {
		return fixturePrepared(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			f.calls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
		}), plan, values), nil
	}
	return f, updated
}
func readAssertionEvidence(t *testing.T, f *protocolFixture, execution string) ([]byte, Evidence) {
	t.Helper()
	raw, e := f.s.Evidence(context.Background(), f.owner, f.project, execution)
	if e != nil {
		t.Fatal(e)
	}
	var response struct {
		Data struct {
			Evidence Evidence `json:"evidence"`
		} `json:"data"`
	}
	if StrictJSON(raw, &response) != nil {
		t.Fatal("invalid versioned evidence read")
	}
	return raw, response.Data.Evidence
}

func TestIntegrationAssertionsAtomicStopNoReplayAndMCPProjection(t *testing.T) {
	for _, test := range []struct {
		name, body, expected, code string
		status                     int
	}{
		{"wrong body", `{"ok":false}`, `true`, "assertion_failed", 200},
		{"http failure", `{"ok":true}`, `true`, "upstream_http_error", 404},
	} {
		t.Run(test.name, func(t *testing.T) {
			f, wf := assertionWorkflowFixture(t, []runner.Assertion{assertionRule("statusEquals", "", jsonNumber(test.status)), assertionRule("jsonEquals", "$.ok", test.expected)}, test.body, test.status)
			ctx := context.Background()
			run := workflowRun(t, f, wf, id())
			msg := workflowMessage(f, run, 0)
			if e := f.s.Process(ctx, msg); e != nil {
				t.Fatal(e)
			}
			terminal, e := f.s.RunDetail(ctx, f.owner, f.project, run.WorkflowRunID)
			if e != nil || terminal.State != "failed" || terminal.Steps[1].State != "skipped" || terminal.Steps[1].ExecutionID != nil {
				t.Fatalf("not atomically stopped: %v", e)
			}
			exec := *terminal.Steps[0].ExecutionID
			before, evidence := readAssertionEvidence(t, f, exec)
			if evidence.SchemaVersion != 2 || evidence.Assertions.Results[0].Outcome != "passed" || evidence.Outcome.Code == nil || *evidence.Outcome.Code != test.code || *evidence.Response.HTTPStatus != test.status {
				t.Fatal("incorrect HTTP/assertion precedence")
			}
			for i := 0; i < 3; i++ {
				if e := f.s.Process(ctx, msg); e != nil {
					t.Fatal(e)
				}
			}
			if f.calls.Load() != 1 {
				t.Fatal("duplicate send or successor dispatch")
			}
			if _, e := f.s.get(ctx, f.s.Table, "P#"+f.project, "JOB#"+workflowIdentity(run.WorkflowRunID, 1, "job")); !missingRow(e) {
				t.Fatal("successor job created")
			}
			for _, key := range []string{"RUN#" + run.WorkflowRunID + "#INPUTS", outputKey(run.WorkflowRunID, 0)} {
				if _, e := f.s.get(ctx, f.s.Protected, "P#"+f.project, key); !missingRow(e) {
					t.Fatal("failed producer runtime retained")
				}
			}
			p, _, e := f.s.CreateMCP(ctx, f.owner, id(), MCPPair{ProjectIDs: []string{f.project}})
			if e != nil {
				t.Fatal(e)
			}
			for _, view := range []string{"complete", "summary"} {
				env := invokeMCP(t, f, *p.Token, "get_execution_evidence", map[string]any{"projectId": f.project, "executionId": exec, "view": view})
				if env.Error != nil || env.Data.Evidence == nil {
					t.Fatal("MCP new evidence unreadable")
				}
				delivery := env.Data.Evidence
				if !strings.Contains(string(mcpRaw(env)), `"assertions"`) {
					t.Fatal("projection omission unmarked")
				}
				if view == "complete" && (delivery.Capture.SchemaVersion != 1 || delivery.OmissionReason == nil || *delivery.OmissionReason != "assertions_not_exposed" || !delivery.Complete) {
					t.Fatal("legacy projection contract violated")
				}
				b, _ := json.Marshal(delivery.Capture)
				if strings.Contains(string(b), "assertionId") || strings.Contains(string(b), "expected") || strings.Contains(string(b), "$.ok") {
					t.Fatal("MCP assertion disclosure")
				}
			}
			if _, e := f.s.Pin(ctx, f.owner, f.project, exec, true); e != nil {
				t.Fatal(e)
			}
			after, _ := readAssertionEvidence(t, f, exec)
			if string(before) != string(after) {
				t.Fatal("pin mutated assertion evidence")
			}
		})
	}
}
func jsonNumber(n int) string { b, _ := json.Marshal(n); return string(b) }

func TestIntegrationAssertionsFrozenDecimalRerunAndExpiry(t *testing.T) {
	for _, token := range []string{"0.10000000000000001", "0.1", "1E+00", "-0.0", "1e-308", "9007199254740991"} {
		t.Run(token, func(t *testing.T) {
			expected, _ := json.Marshal(map[string]string{"type": "number", "decimal": token})
			f, wf := assertionWorkflowFixture(t, []runner.Assertion{assertionRule("jsonEquals", "$.n", string(expected))}, `{"n":`+token+`}`, 200)
			ctx := context.Background()
			run := workflowRun(t, f, wf, id())
			wf.Name = "Unrelated name edit"
			updated, e := f.s.SaveWorkflow(ctx, f.owner, f.project, wf.WorkflowID, wf.Revision, wf.WorkflowDefinition, false)
			if e != nil {
				t.Fatal(e)
			}
			if string(updated.Steps[0].Assertions[0].Expected) != string(expected) {
				t.Fatal("unrelated edit changed decimal")
			}
			for i := 0; i < 2; i++ {
				if e = f.s.Process(ctx, workflowMessage(f, run, i)); e != nil {
					t.Fatal(e)
				}
			}
			finished, e := f.s.RunDetail(ctx, f.owner, f.project, run.WorkflowRunID)
			if e != nil || finished.State != "completed" {
				t.Fatal("valid numeric workflow failed", e)
			}
			_, evidence := readAssertionEvidence(t, f, *finished.Steps[0].ExecutionID)
			if !evidence.Assertions.Passed() {
				t.Fatal("numeric equality failed")
			}
			// Deliberately change the live expectation; reruns must use the old token.
			updated.Steps[0].Assertions[0].Expected = json.RawMessage(`{"type":"number","decimal":"2"}`)
			if _, e = f.s.SaveWorkflow(ctx, f.owner, f.project, wf.WorkflowID, updated.Revision, updated.WorkflowDefinition, false); e != nil {
				t.Fatal(e)
			}
			rerun, e := f.s.SubmitWorkflow(ctx, f.owner, f.project, run.WorkflowRunID, id(), 0, RunOptions{Target: "cloud", AllowInsecureSecrets: true}, true)
			if e != nil || string(rerun.Steps[0].Assertions[0].Expected) != string(expected) || rerun.WorkflowRunID == run.WorkflowRunID {
				t.Fatal("frozen numeric rerun lost token", e)
			}
			for i := 0; i < 2; i++ {
				if e = f.s.Process(ctx, workflowMessage(f, rerun, i)); e != nil {
					t.Fatal(e)
				}
			}
			// Expiry removes the same result objects/context through existing cleanup.
			f.now = f.now.Add(31 * 24 * time.Hour)
			for i := 0; i < 4; i++ {
				if e = f.s.Maintain(ctx); e != nil {
					t.Fatal(e)
				}
			}
			if _, e = f.s.Evidence(ctx, f.owner, f.project, *finished.Steps[0].ExecutionID); e == nil {
				t.Fatal("expired assertion resurrected")
			}
		})
	}
}

func TestIntegrationAssertionsLegacyRerunAndGate(t *testing.T) {
	f, wf := workflowProtocolFixture(t)
	a := assertionRule("jsonExists", "$", "")
	wf.Steps[0].Assertions = []runner.Assertion{a}
	if _, e := f.s.SaveWorkflow(context.Background(), f.owner, f.project, wf.WorkflowID, wf.Revision, wf.WorkflowDefinition, false); e == nil {
		t.Fatal("assertions wrote before capability")
	}
	wf.Steps[0].Assertions = nil
	run := workflowRun(t, f, wf, id())
	for i := 0; i < 2; i++ {
		if e := f.s.Process(context.Background(), workflowMessage(f, run, i)); e != nil {
			t.Fatal(e)
		}
	}
	enableAssertionFixture(t, f)
	rerun, e := f.s.SubmitWorkflow(context.Background(), f.owner, f.project, run.WorkflowRunID, id(), 0, RunOptions{Target: "cloud", AllowInsecureSecrets: true}, true)
	if e != nil {
		t.Fatal(e)
	}
	if e = f.s.Process(context.Background(), workflowMessage(f, rerun, 0)); e != nil {
		t.Fatal(e)
	}
	_, evidence := readAssertionEvidence(t, f, *rerun.Steps[0].ExecutionID)
	if evidence.SchemaVersion != 1 || evidence.Assertions != nil {
		t.Fatal("legacy run evaluation fabricated")
	}
}

// Compile the old stdio decoder against the dedicated legacy capture type.
var _ *runner.LegacyEvidence = (&wire.Evidence{}).Capture

func TestIntegrationAssertionsPublicationUncertaintyNeverReplays(t *testing.T) {
	for _, phase := range []string{"lost-final-ack", "lost-result-buffer", "lost-put-ack"} {
		t.Run(phase, func(t *testing.T) {
			f, wf := assertionWorkflowFixture(t, []runner.Assertion{assertionRule("jsonEquals", "$.ok", `true`)}, `{"ok":false}`, 200)
			ctx := context.Background()
			run := workflowRun(t, f, wf, id())
			msg := workflowMessage(f, run, 0)
			hit := false
			if phase == "lost-put-ack" {
				f.objects.lostACK = true
			} else {
				fault := func(in *dynamodb.TransactWriteItemsInput) error {
					if writesState(in, "executionWorkflowRun", "failed") {
						hit = true
						return errors.New("fixture publication uncertainty")
					}
					return nil
				}
				if phase == "lost-final-ack" {
					f.db.after = fault
				} else {
					f.db.before = fault
				}
			}
			e := f.s.Process(ctx, msg)
			if phase == "lost-final-ack" && e != nil {
				t.Fatal(e)
			}
			if phase != "lost-final-ack" && e == nil {
				t.Fatal("uncertainty fault missed")
			}
			if phase != "lost-put-ack" && !hit {
				t.Fatal("finalization fault missed")
			}
			f.db.before = nil
			f.db.after = nil
			f.objects.lostACK = false
			f.now = f.now.Add(3 * time.Minute)
			if e = f.s.Process(ctx, msg); e != nil {
				t.Fatal(e)
			}
			state, e := f.s.RunDetail(ctx, f.owner, f.project, run.WorkflowRunID)
			if e != nil || state.State != "failed" || state.Steps[1].State != "skipped" || f.calls.Load() != 1 {
				t.Fatal("uncertainty replayed/advanced", e)
			}
			_, evidence := readAssertionEvidence(t, f, *state.Steps[0].ExecutionID)
			if phase == "lost-final-ack" {
				if evidence.Assertions.Results[0].Reason != "value_mismatch" {
					t.Fatal("terminal assertion mutated")
				}
			} else if evidence.Outcome.Certainty != "unknown" || evidence.Assertions.Results[0].Reason != "evaluation_unavailable" {
				t.Fatal("missing buffer fabricated a result")
			}
		})
	}
}
