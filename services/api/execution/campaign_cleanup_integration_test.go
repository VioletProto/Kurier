//go:build integration

package execution

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	runner "github.com/VioletProto/Kurier/services/request-runtime"
)

// Exercise the actual cleanup component once per project per simulated scheduler
// tick. No clock/retention aging, direct cloud invocation or production change.
func TestIntegrationReplacementCampaignCleanupTicks(t *testing.T) {
	f, _ := workflowProtocolFixture(t)
	ctx := context.Background()
	stage, _ := f.s.stage(ctx)
	stage.SavedRequestsSchemaVersion = 7
	stage.MCPReadOnlySchemaVersion = 1
	f.write(t, f.s.Table, stage)
	type fixtureProject struct {
		owner, project, request string
		runs                    int
		fail                    map[int]bool
	}
	projects := []fixtureProject{{f.owner, f.project, f.request, 7, map[int]bool{3: true, 5: true}}, {id(), id(), id(), 6, map[int]bool{2: true, 4: true}}}
	f.s.prepare = func(_ context.Context, plan Plan, values map[string]string) (*Prepared, error) {
		return fixturePrepared(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			f.calls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			if strings.Contains(plan.Configuration.Name, "failure") {
				w.WriteHeader(500)
			}
			_, _ = w.Write([]byte(`{"ok":true}`))
		}), plan, values), nil
	}
	var firstExecution string
	for index, p := range projects {
		if index == 1 {
			f.write(t, f.s.Table, row{PK: "U#" + p.owner, SK: "META", Kind: "user", SchemaVersion: 1, UserID: p.owner})
			f.write(t, f.s.Table, row{PK: "P#" + p.project, SK: "META", Kind: "project", SchemaVersion: 1, ProjectID: p.project, OwnerID: p.owner, State: "active", PinnedCount: pointer(int64(0)), PinnedBytes: pointer(int64(0))})
		}
		request := row{PK: "P#" + p.project, SK: "REQ#" + p.request, Kind: "request", SchemaVersion: 2, ProjectID: p.project, RequestID: p.request, Revision: 1, State: "active"}
		config := fixturePlan().Configuration
		config.Name = "cleanup success"
		encoded, _ := json.Marshal(config)
		request.ConfigurationJSON = string(encoded)
		f.write(t, f.s.Table, request)
		failedRequest := request
		failedRequest.RequestID = id()
		failedRequest.SK = "REQ#" + failedRequest.RequestID
		config.Name = "cleanup failure"
		encoded, _ = json.Marshal(config)
		failedRequest.ConfigurationJSON = string(encoded)
		f.write(t, f.s.Table, failedRequest)
		for n := 0; n < p.runs; n++ {
			first := p.request
			if p.fail[n] {
				first = failedRequest.RequestID
			}
			wf, e := f.s.SaveWorkflow(ctx, p.owner, p.project, "", 0, WorkflowDefinition{Name: "cleanup shape", Steps: []runner.WorkflowStep{{StepID: id(), RequestID: first, RequestRevision: 1, Extract: []runner.Extraction{}, Bindings: []runner.Binding{}}, {StepID: id(), RequestID: p.request, RequestRevision: 1, Extract: []runner.Extraction{}, Bindings: []runner.Binding{}}}}, true)
			if e != nil {
				t.Fatal(e)
			}
			run, e := f.s.SubmitWorkflow(ctx, p.owner, p.project, wf.WorkflowID, id(), wf.Revision, RunOptions{Target: "cloud", AllowInsecureSecrets: true}, false)
			if e != nil {
				t.Fatal(e)
			}
			for pos := 0; pos < 2; pos++ {
				if p.fail[n] && pos == 1 {
					break
				}
				job := workflowIdentity(run.WorkflowRunID, pos, "job")
				if e = f.s.Process(ctx, Message{p.project, job, "OUT#" + job}); e != nil {
					t.Fatal(e)
				}
			}
			if firstExecution == "" {
				firstExecution = workflowIdentity(run.WorkflowRunID, 0, "execution")
			}
		}
	}
	if f.calls.Load() != 22 {
		t.Fatal("unexpected local campaign send count", f.calls.Load())
	}
	pair, _, e := f.s.CreateMCP(ctx, f.owner, id(), MCPPair{ProjectIDs: []string{f.project}})
	if e != nil {
		t.Fatal(e)
	}
	for n := 0; n < 4; n++ {
		view := "complete"
		if n%2 == 1 {
			view = "summary"
		}
		invokeMCP(t, f, *pair.Token, "get_execution_evidence", map[string]any{"projectId": f.project, "executionId": firstExecution, "view": view})
	}
	if e = f.s.RevokeMCP(ctx, f.owner, pair.Credential.TokenID); e != nil {
		t.Fatal(e)
	}
	for _, p := range projects {
		r, e := f.s.get(ctx, f.s.Table, "P#"+p.project, "META")
		if e != nil {
			t.Fatal(e)
		}
		r.State = "deleting"
		r.DeletionEpoch++
		r.Version++
		f.write(t, f.s.Table, r)
	}
	ticks := []int{0, 0}
	for tick := 1; tick <= 20; tick++ {
		for index, p := range projects {
			if ticks[index] > 0 {
				continue
			}
			e := f.s.DrainProject(ctx, p.project)
			if e == nil {
				ticks[index] = tick
				continue
			}
			var problem *Error
			if !errors.As(e, &problem) || problem.Status != 409 || problem.Code != "cleanup_pending" {
				t.Fatal("normal execution drain failed", e)
			}
		}
	}
	// Request drain (<=20 public requests), empty Protected and final tombstone
	// checks finish in the same outer coordinator pass. Leave one minute for
	// scheduling/verification overhead within the reviewed20-minute window.
	for _, tick := range ticks {
		if tick == 0 || tick > 19 {
			t.Fatalf("execution drain ticks %v leave insufficient room in20-minute normal cleanup window", ticks)
		}
	}
	q, e := f.s.get(ctx, f.s.Table, "STAGE#dev-api", "QUOTA")
	if e != nil || q.Count != 26 || q.StorageReservedBytes != 0 {
		t.Fatal("cleanup altered usage or left storage", e)
	}
	if len(f.objects.data) != 0 || f.calls.Load() != 22 {
		t.Fatal("cleanup retained objects or replayed HTTP")
	}
	t.Logf("two owned projects:26 admitted/22 fixture sends including reserve; execution cleanup ticks=%v; one-minute headroom;usage26 retained", ticks)
}
