//go:build integration

package execution

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dt "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Test-only interleaving at actual read/validation boundaries. DynamoDB Local,
// not an authored error, rejects the stale ConditionChecks. Worker processing
// uses the existing loopback target/object/KMS/queue fixtures, never AWS.
type runDetailInterleaveDB struct {
	Database
	runKey          string
	afterRunRead    func()
	beforeValidate  func()
	readValidations int
	mutations       int
	failedKeys      map[string]bool
}

func TestIntegrationCoordinatorRunDetailGETBackoff(t *testing.T) {
	f, wf := workflowProtocolFixture(t)
	ctx := context.Background()
	run := workflowRun(t, f, wf, id())
	db := &runDetailInterleaveDB{Database: f.db, runKey: "RUN#" + run.WorkflowRunID, failedKeys: map[string]bool{}}
	f.s.DB = db
	db.beforeValidate = func() {
		for pos := 0; pos < 2; pos++ {
			if e := f.s.Process(ctx, workflowMessage(f, run, pos)); e != nil {
				t.Error("controlled worker transition failed", e)
			}
		}
	}
	done := make(chan struct{}, 4)
	statuses := []int{}
	starts := []time.Time{}
	var retryAfterSentAt time.Time
	var afterTransitionQuota row
	var afterTransitionSends int32
	var afterTransitionQueue, afterTransitionPuts, afterTransitionMutations int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() { done <- struct{}{} }()
		if r.Method != "GET" || r.URL.Path != "/api/v1/projects/"+f.project+"/workflow-runs/"+run.WorkflowRunID {
			t.Error("polling client attempted a write or changed scope")
			w.WriteHeader(405)
			return
		}
		starts = append(starts, time.Now())
		v, e := f.s.RunDetail(r.Context(), f.owner, f.project, run.WorkflowRunID)
		w.Header().Set("Content-Type", "application/json")
		if e != nil {
			var problem *Error
			if !errors.As(e, &problem) || problem.Status != 503 || problem.Code != "service_unavailable" {
				t.Error("unexpected detail error")
				w.WriteHeader(500)
				return
			}
			statuses = append(statuses, 503)
			afterTransitionQuota, _ = f.s.get(ctx, f.s.Table, "STAGE#dev-api", "QUOTA")
			afterTransitionSends, afterTransitionQueue, afterTransitionPuts, afterTransitionMutations = f.calls.Load(), f.queue.calls, f.objects.puts, db.mutations
			w.Header().Set("Retry-After", "2")
			retryAfterSentAt = time.Now()
			w.WriteHeader(503)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": "service_unavailable"}})
			return
		}
		statuses = append(statuses, 200)
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"run": v}})
	}))
	defer server.Close()
	input, _ := json.Marshal(map[string]string{"origin": server.URL, "projectId": f.project, "runId": run.WorkflowRunID})
	command := exec.CommandContext(ctx, "node", "../../../scripts/fixtures/workflow-run-poll.mjs")
	command.Stdin = bytes.NewReader(input)
	if output, e := command.CombinedOutput(); e != nil {
		t.Fatalf("local coordinator bridge failed:%v %s", e, output)
	}
	// Synchronize with both HTTP handlers before reading their observations.
	<-done
	<-done
	if len(statuses) != 2 || statuses[0] != 503 || statuses[1] != 200 || db.readValidations != 2 || len(db.failedKeys) == 0 {
		t.Fatal("did not observe the actual stale-guard503 followed by fresh200")
	}
	if starts[1].Sub(retryAfterSentAt) < 2*time.Second {
		t.Fatal("actual HTTP retry ignored Retry-After/minimum poll interval")
	}
	q, _ := f.s.get(ctx, f.s.Table, "STAGE#dev-api", "QUOTA")
	if q.Count != 2 || q.Count != afterTransitionQuota.Count || q.Version != afterTransitionQuota.Version || afterTransitionSends != 2 || f.calls.Load() != afterTransitionSends || f.queue.calls != afterTransitionQueue || f.objects.puts != afterTransitionPuts || db.mutations != afterTransitionMutations {
		t.Fatal("HTTP GET retry admitted, dispatched, republished or mutated work")
	}
	t.Log("actual local HTTP503→GET backoff→200;2 existing admissions;retry adds0 target sends/queue notifications/object writes/mutations")
}

func (d *runDetailInterleaveDB) GetItem(ctx context.Context, in *dynamodb.GetItemInput, options ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	out, e := d.Database.GetItem(ctx, in, options...)
	if e == nil && d.afterRunRead != nil && in.Key["SK"].(*dt.AttributeValueMemberS).Value == d.runKey {
		transition := d.afterRunRead
		d.afterRunRead = nil
		transition() // Return the already-read run with newly committed steps.
	}
	return out, e
}

func (d *runDetailInterleaveDB) TransactWriteItems(ctx context.Context, in *dynamodb.TransactWriteItemsInput, options ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error) {
	readOnly, hasRun := len(in.TransactItems) > 0, false
	for _, a := range in.TransactItems {
		if a.ConditionCheck == nil {
			readOnly = false
			continue
		}
		hasRun = hasRun || a.ConditionCheck.Key["SK"].(*dt.AttributeValueMemberS).Value == d.runKey
	}
	if !readOnly || !hasRun {
		if !readOnly {
			d.mutations++
		}
		return d.Database.TransactWriteItems(ctx, in, options...)
	}
	if d.beforeValidate != nil {
		transition := d.beforeValidate
		d.beforeValidate = nil
		transition()
	}
	d.readValidations++
	out, e := d.Database.TransactWriteItems(ctx, in, options...)
	var canceled *dt.TransactionCanceledException
	if errors.As(e, &canceled) {
		for i, reason := range canceled.CancellationReasons {
			if aws.ToString(reason.Code) == "ConditionalCheckFailed" {
				d.failedKeys[in.TransactItems[i].ConditionCheck.Key["SK"].(*dt.AttributeValueMemberS).Value] = true
			}
		}
	}
	return out, e
}

func TestIntegrationRunDetailTransitionThenFreshRead(t *testing.T) {
	for _, scenario := range []string{"between-run-and-step-reads", "before-transaction-validation", "active-project-change", "project-deletion"} {
		t.Run(scenario, func(t *testing.T) {
			f, wf := workflowProtocolFixture(t)
			ctx := context.Background()
			stage, _ := f.s.stage(ctx)
			stage.SavedRequestsSchemaVersion, stage.MCPReadOnlySchemaVersion = 7, 1
			f.write(t, f.s.Table, stage)
			run := workflowRun(t, f, wf, id()) // Exactly one two-step admission.
			db := &runDetailInterleaveDB{Database: f.db, runKey: "RUN#" + run.WorkflowRunID, failedKeys: map[string]bool{}}
			f.s.DB = db
			worker := scenario == "between-run-and-step-reads" || scenario == "before-transaction-validation"
			transition := func() {
				if worker {
					// Actual publication/advance/finalization changes run, steps and
					// project versions while the detail request is suspended.
					for pos := 0; pos < 2; pos++ {
						if e := f.s.Process(ctx, workflowMessage(f, run, pos)); e != nil {
							t.Fatal(e)
						}
					}
					return
				}
				p, e := f.s.get(ctx, f.s.Table, "P#"+f.project, "META")
				if e != nil {
					t.Fatal(e)
				}
				p.Version++
				if scenario == "project-deletion" {
					p.State = "deleting"
					p.DeletionEpoch++
				}
				// Local fixture models a concurrent project mutation, not a GET
				// effect. It changes no admissions and dispatches no work.
				f.write(t, f.s.Table, p)
			}
			if scenario == "between-run-and-step-reads" {
				db.afterRunRead = transition
			} else {
				db.beforeValidate = transition
			}
			stale, e := f.s.RunDetail(ctx, f.owner, f.project, run.WorkflowRunID)
			var unavailable *Error
			if !errors.As(e, &unavailable) || unavailable.Status != 503 || unavailable.Code != "service_unavailable" || stale.WorkflowRunID != "" {
				t.Fatal("stale/torn detail was disclosed instead of fixed503")
			}
			if db.readValidations != 1 || !db.failedKeys["META"] {
				t.Fatal("stale project guard did not reject the first validation")
			}
			if worker && !db.failedKeys[db.runKey] {
				t.Fatal("stale run guard did not reject the first validation")
			}
			if scenario == "before-transaction-validation" && (!db.failedKeys[stepKey(run.WorkflowRunID, 0)] || !db.failedKeys[stepKey(run.WorkflowRunID, 1)]) {
				t.Fatal("stale step guards were not validated")
			}
			quotaBefore, _ := f.s.get(ctx, f.s.Table, "STAGE#dev-api", "QUOTA")
			projectBefore, _ := f.s.get(ctx, f.s.Table, "P#"+f.project, "META")
			sendsBefore, queueBefore, putsBefore := f.calls.Load(), f.queue.calls, f.objects.puts
			mutationsBefore := db.mutations
			for n := 0; n < 3; n++ {
				fresh, e := f.s.RunDetail(ctx, f.owner, f.project, run.WorkflowRunID)
				if scenario == "project-deletion" {
					if !missingRow(e) || fresh.WorkflowRunID != "" {
						t.Fatal("fresh read bypassed project deletion")
					}
				} else if e != nil || fresh.WorkflowRunID != run.WorkflowRunID || worker && (fresh.State != "completed" || fresh.Steps[0].State != "completed" || fresh.Steps[1].State != "completed") || !worker && fresh.State != "queued" {
					t.Fatal("fresh GET did not return current coherent detail", e)
				}
			}
			quotaAfter, _ := f.s.get(ctx, f.s.Table, "STAGE#dev-api", "QUOTA")
			projectAfter, _ := f.s.get(ctx, f.s.Table, "P#"+f.project, "META")
			if quotaBefore.Count != 2 || quotaBefore.Count != quotaAfter.Count || quotaBefore.Version != quotaAfter.Version || projectBefore.Version != projectAfter.Version || sendsBefore != f.calls.Load() || queueBefore != f.queue.calls || putsBefore != f.objects.puts || mutationsBefore != db.mutations {
				t.Fatal("fresh detail reads mutated accounting, published work or sent HTTP")
			}
			if worker && sendsBefore != 2 || !worker && sendsBefore != 0 {
				t.Fatal("unexpected interleaving fixture send count")
			}
			t.Logf("actual stale validation rejected;3 fresh reads;admissions remain2;read-send/queue/object deltas0;worker transition fixture sends=%d", sendsBefore)
		})
	}
}
