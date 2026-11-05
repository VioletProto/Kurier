//go:build integration

package execution

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	runner "github.com/VioletProto/Kurier/services/request-runtime"
	contract "github.com/VioletProto/Kurier/services/request-runtime/openapi"
)

func enableOpenAPIFixture(t *testing.T, f *protocolFixture) (row, row) {
	t.Helper()
	enableEnvironmentFixture(t, f)
	ctx := context.Background()
	stage, _ := f.s.stage(ctx)
	stage.SavedRequestsSchemaVersion = 10
	stage.OpenAPISchemaVersion = 1
	stage.Version++
	f.write(t, f.s.Table, stage)
	doc, e := contract.Parse([]byte(`{"openapi":"3.0.3","info":{"title":"Fixture","version":"1"},"servers":[{"url":"https://fixture.example"}],"paths":{"/":{"get":{"responses":{"200":{"description":"ok","content":{"application/json":{"schema":{"type":"object","required":["ok"],"properties":{"ok":{"type":"boolean"}}}}}}}}}}}`), "json", contract.Policy{})
	if e != nil {
		t.Fatal(e)
	}
	importID := id()
	op := doc.Operations[0]
	raw, _ := json.Marshal(op)
	root := row{PK: "P#" + f.project, SK: "OPENAPI#" + importID + "#META", Kind: "openapiImport", SchemaVersion: 1, ProjectID: f.project, ImportID: importID, State: "ready", Revision: 1}
	operation := row{PK: root.PK, SK: "OPENAPI#" + importID + "#OP#" + op.OperationID, Kind: "openapiOperation", SchemaVersion: 1, ProjectID: f.project, ImportID: importID, OperationID: op.OperationID, OperationJSON: string(raw)}
	f.write(t, f.s.Table, root)
	f.write(t, f.s.Table, operation)
	r, _ := f.s.get(ctx, f.s.Table, root.PK, "REQ#"+f.request)
	c := fixturePlan().Configuration
	c.OperationRef = &runner.OperationRef{ImportID: importID, OperationID: op.OperationID}
	raw, _ = json.Marshal(c)
	r.ImportID = importID
	r.SchemaVersion = 4
	r.ConfigurationJSON = string(raw)
	f.write(t, f.s.Table, r)
	return root, operation
}

func TestIntegrationOpenAPIFrozenRerunReadyGateLocalRejectionAndMCP(t *testing.T) {
	f := newProtocolFixture(t)
	root, _ := enableOpenAPIFixture(t, f)
	ctx := context.Background()
	f.s.prepare = func(ctx context.Context, p Plan, values map[string]string) (*Prepared, error) {
		return fixturePrepared(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			f.calls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true}`))
		}), p, values), nil
	}
	options, _ := ParseOptions([]byte(`{}`))
	key := id()
	v, e := f.s.Submit(ctx, f.owner, f.project, f.request, key, 1, options, false)
	if e != nil {
		t.Fatal(e)
	}
	anchor, _ := f.s.get(ctx, f.s.Table, root.PK, "EXEC#"+v.ExecutionID)
	if anchor.Plan.FrozenContract == nil || anchor.Plan.FrozenContract.Revision != 1 {
		t.Fatal("bundle not frozen")
	}
	if e = f.s.Process(ctx, Message{f.project, anchor.JobID, "OUT#" + anchor.JobID}); e != nil {
		t.Fatal(e)
	}
	before, evidence := readAssertionEvidence(t, f, v.ExecutionID)
	if evidence.SchemaVersion != 4 || evidence.Contract == nil || evidence.Contract.Result.Status != "passed" {
		t.Fatal("contract result not persisted")
	}
	// An unrelated edit/detachment of the live request does not alter frozen reruns.
	request, _ := f.s.get(ctx, f.s.Table, root.PK, "REQ#"+f.request)
	var c Configuration
	_ = StrictJSON([]byte(request.ConfigurationJSON), &c)
	c.OperationRef = nil
	raw, _ := json.Marshal(c)
	request.ConfigurationJSON = string(raw)
	request.Revision++
	f.write(t, f.s.Table, request)
	rerun, e := f.s.Submit(ctx, f.owner, f.project, v.ExecutionID, id(), 0, options, true)
	if e != nil {
		t.Fatal(e)
	}
	replay, _ := f.s.get(ctx, f.s.Table, root.PK, "EXEC#"+rerun.ExecutionID)
	if replay.Plan.FrozenContract == nil || replay.Plan.FrozenContract.OperationID != anchor.Plan.FrozenContract.OperationID {
		t.Fatal("rerun rebound contract")
	}
	if e = f.s.Process(ctx, Message{f.project, replay.JobID, "OUT#" + replay.JobID}); e != nil {
		t.Fatal(e)
	}
	after, _ := readAssertionEvidence(t, f, v.ExecutionID)
	if string(before) != string(after) || f.calls.Load() != 2 {
		t.Fatal("evidence changed or sends replayed")
	}
	pair, _, e := f.s.CreateMCP(ctx, f.owner, id(), MCPPair{ProjectIDs: []string{f.project}})
	if e != nil {
		t.Fatal(e)
	}
	for _, view := range []string{"complete", "summary"} {
		out := invokeMCP(t, f, *pair.Token, "get_execution_evidence", map[string]any{"projectId": f.project, "executionId": v.ExecutionID, "view": view})
		if out.Error != nil || out.Data.Evidence == nil {
			t.Fatal("MCP projection failed")
		}
		encoded := string(mcpRaw(out))
		if !strings.Contains(encoded, `"contract"`) || strings.Contains(encoded, `"semanticsVersion"`) || strings.Contains(encoded, root.ImportID) {
			t.Fatal("MCP result leaked or omission absent")
		}
	}
	root.State = "preparing"
	f.write(t, f.s.Table, root)
	if _, e = f.s.Submit(ctx, f.owner, f.project, f.request, id(), request.Revision, options, false); e == nil {
		t.Fatal("detaching validation bypassed import publication")
	}
	root.State = "ready"
	f.write(t, f.s.Table, root)
	// Restore the public association, then reject local admission before any agent/target work.
	c.OperationRef = anchor.Plan.Configuration.OperationRef
	raw, _ = json.Marshal(c)
	request.ConfigurationJSON = string(raw)
	f.write(t, f.s.Table, request)
	local := options
	local.Target = "local"
	if _, e = f.s.Submit(ctx, f.owner, f.project, f.request, id(), request.Revision, local, false); e == nil {
		t.Fatal("local validation admitted")
	}
	root.State = "preparing"
	f.write(t, f.s.Table, root)
	if _, e = f.s.Submit(ctx, f.owner, f.project, f.request, id(), request.Revision, options, false); e == nil {
		t.Fatal("unready source admitted")
	}
	if f.calls.Load() != 2 {
		t.Fatal("refusal dispatched target")
	}
}

func TestIntegrationOpenAPIContractFailureAtomicallySkipsWorkflowSuccessor(t *testing.T) {
	f, wf := workflowProtocolFixture(t)
	enableOpenAPIFixture(t, f)
	ctx := context.Background()
	wf.Steps[0].Extract = []runner.Extraction{}
	wf.Steps[1].Bindings = []runner.Binding{}
	updated, e := f.s.SaveWorkflow(ctx, f.owner, f.project, wf.WorkflowID, wf.Revision, wf.WorkflowDefinition, false)
	if e != nil {
		t.Fatal(e)
	}
	f.s.prepare = func(ctx context.Context, p Plan, values map[string]string) (*Prepared, error) {
		return fixturePrepared(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			f.calls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		}), p, values), nil
	}
	run := workflowRun(t, f, updated, id())
	message := workflowMessage(f, run, 0)
	if e = f.s.Process(ctx, message); e != nil {
		t.Fatal(e)
	}
	final, e := f.s.RunDetail(ctx, f.owner, f.project, run.WorkflowRunID)
	if e != nil || final.State != "failed" || final.Steps[1].State != "skipped" || final.Steps[1].ExecutionID != nil {
		t.Fatalf("successor not skipped: %v", e)
	}
	_, evidence := readAssertionEvidence(t, f, *final.Steps[0].ExecutionID)
	if evidence.SchemaVersion != 4 || evidence.Contract.Result.Status != "failed" || evidence.Outcome.Code == nil || *evidence.Outcome.Code != "contract_validation_failed" {
		t.Fatal("contract failure not recorded")
	}
	if e = f.s.Process(ctx, message); e != nil || f.calls.Load() != 1 {
		t.Fatal("workflow replay dispatched")
	}
}

func TestIntegrationOpenAPISharedImportAndDetachedWorkflowDependency(t *testing.T) {
	f, wf := workflowProtocolFixture(t)
	root, _ := enableOpenAPIFixture(t, f)
	ctx := context.Background()
	first, _ := f.s.get(ctx, f.s.Table, root.PK, "REQ#"+f.request)
	second := first
	second.RequestID = id()
	second.SK = "REQ#" + second.RequestID
	var c Configuration
	if StrictJSON([]byte(second.ConfigurationJSON), &c) != nil {
		t.Fatal("invalid fixture")
	}
	c.OperationRef = nil
	raw, _ := json.Marshal(c)
	second.ConfigurationJSON = string(raw)
	f.write(t, f.s.Table, second)
	wf.Steps[0].Extract = []runner.Extraction{}
	wf.Steps[1].Bindings = []runner.Binding{}
	wf.Steps[1].RequestID = second.RequestID
	updated, e := f.s.SaveWorkflow(ctx, f.owner, f.project, wf.WorkflowID, wf.Revision, wf.WorkflowDefinition, false)
	if e != nil {
		t.Fatal(e)
	}
	f.s.prepare = func(ctx context.Context, p Plan, values map[string]string) (*Prepared, error) {
		return fixturePrepared(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			f.calls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true}`))
		}), p, values), nil
	}
	run := workflowRun(t, f, updated, id())
	for pos := 0; pos < 2; pos++ {
		if e = f.s.Process(ctx, workflowMessage(f, run, pos)); e != nil {
			t.Fatal(e)
		}
	}
	finished, e := f.s.RunDetail(ctx, f.owner, f.project, run.WorkflowRunID)
	if e != nil || finished.State != "completed" || f.calls.Load() != 2 {
		t.Fatalf("shared import guards blocked normal workflow: %v", e)
	}
	retained, e := f.s.get(ctx, f.s.Table, root.PK, "RUN#"+run.WorkflowRunID)
	if e != nil {
		t.Fatal(e)
	}
	// This dependency read is used at durable workflow intent and finalization.
	// Even an association-free imported request must retain the publication gate.
	root.State = "preparing"
	f.write(t, f.s.Table, root)
	if _, e = f.s.workflowRequestGuards(ctx, retained, f.request); e == nil {
		t.Fatal("future detached import escaped workflow dependency fence")
	}
}
