//go:build integration

package execution

import (
	"context"
	"encoding/json"
	"errors"
	runner "github.com/VioletProto/Kurier/services/request-runtime"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dt "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

func workflowProtocolFixture(t *testing.T) (*protocolFixture, Workflow) {
	f := newProtocolFixture(t)
	ctx := context.Background()
	stage, _ := f.s.get(ctx, f.s.Table, "STAGE#dev-api", "META")
	stage.SavedRequestsSchemaVersion = 6
	stage.EvidencePinningSchemaVersion = 1
	stage.LocalAgentExecutionsSchemaVersion = 1
	stage.SequentialWorkflowsSchemaVersion = 1
	f.write(t, f.s.Table, stage)
	p, _ := f.s.get(ctx, f.s.Table, "P#"+f.project, "META")
	p.PinnedCount = pointer(int64(0))
	p.PinnedBytes = pointer(int64(0))
	f.write(t, f.s.Table, p)
	c := fixturePlan().Configuration
	c.Headers = []Field{{Name: "Authorization", Value: "", Enabled: true}}
	raw, _ := json.Marshal(c)
	request, _ := f.s.get(ctx, f.s.Table, p.PK, "REQ#"+f.request)
	request.ConfigurationJSON = string(raw)
	f.write(t, f.s.Table, request)
	first, second := id(), id()
	d := WorkflowDefinition{Name: "fixture", Steps: []runner.WorkflowStep{{StepID: first, RequestID: f.request, RequestRevision: 1, Extract: []runner.Extraction{{Name: "token", Path: "$.value", Type: "string"}}, Bindings: []runner.Binding{}}, {StepID: second, RequestID: f.request, RequestRevision: 1, Extract: []runner.Extraction{}, Bindings: []runner.Binding{{From: runner.Producer{StepID: first, Name: "token"}, To: runner.BindingTarget{Kind: "header", Index: pointer(0), Prefix: pointer("Bearer ")}}}}}}
	wf, e := f.s.SaveWorkflow(ctx, f.owner, f.project, "", 0, d, true)
	if e != nil {
		t.Fatal(e)
	}
	f.s.prepare = func(ctx context.Context, plan Plan, values map[string]string) (*Prepared, error) {
		return fixturePrepared(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if f.calls.Add(1)%2 == 0 && r.Header.Get("Authorization") != "Bearer fixture-runtime-credential" {
				t.Error("consumer did not receive rendered header")
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"value":"fixture-runtime-credential","echo":"Bearer fixture-runtime-credential"}`))
		}), plan, values), nil
	}
	return f, wf
}
func workflowRun(t *testing.T, f *protocolFixture, wf Workflow, key string) WorkflowRun {
	t.Helper()
	v, e := f.s.SubmitWorkflow(context.Background(), f.owner, f.project, wf.WorkflowID, key, wf.Revision, RunOptions{Target: "cloud", AllowInsecureSecrets: true}, false)
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func workflowMessage(f *protocolFixture, run WorkflowRun, pos int) Message {
	job := workflowIdentity(run.WorkflowRunID, pos, "job")
	return Message{f.project, job, "OUT#" + job}
}
func TestIntegrationWorkflowAtomicAdvanceContainmentAccounting(t *testing.T) {
	f, wf := workflowProtocolFixture(t)
	ctx := context.Background()
	key := id()
	run := workflowRun(t, f, wf, key)
	again := workflowRun(t, f, wf, key)
	if again.WorkflowRunID != run.WorkflowRunID {
		t.Fatal("retry duplicated run")
	}
	p, _ := f.s.get(ctx, f.s.Table, "P#"+f.project, "META")
	if p.InflightCount != 1 {
		t.Fatal("inflight")
	}
	q, _ := f.s.get(ctx, f.s.Table, "STAGE#dev-api", "QUOTA")
	if q.Count != 2 {
		t.Fatal("N-step admission")
	}
	if e := f.s.Process(ctx, workflowMessage(f, run, 0)); e != nil {
		t.Fatal(e)
	}
	run, e := f.s.RunDetail(ctx, f.owner, f.project, run.WorkflowRunID)
	if e != nil || run.Steps[0].State != "completed" || run.Steps[1].State != "queued" {
		t.Fatalf("atomic successor unavailable: %v", e)
	}
	output, e := f.s.get(ctx, f.s.Protected, "P#"+f.project, outputKey(run.WorkflowRunID, 0))
	if e != nil || output.Envelope == nil {
		t.Fatal("output absent")
	}
	raw, _ := json.Marshal(output)
	if strings.Contains(string(raw), "fixture-runtime-credential") {
		t.Fatal("unencrypted output")
	}
	for i := 0; i < 3; i++ {
		if e = f.s.Process(ctx, workflowMessage(f, run, 0)); e != nil {
			t.Fatal(e)
		}
	}
	if e = f.s.Process(ctx, workflowMessage(f, run, 1)); e != nil {
		t.Fatal(e)
	}
	if f.calls.Load() != 2 {
		t.Fatal("duplicate HTTP")
	}
	run, e = f.s.RunDetail(ctx, f.owner, f.project, run.WorkflowRunID)
	if e != nil || run.State != "completed" {
		t.Fatalf("run completion: %v", e)
	}
	for i := 0; i < 2; i++ {
		raw, e = f.s.Evidence(ctx, f.owner, f.project, *run.Steps[i].ExecutionID)
		if e != nil || strings.Contains(string(raw), "fixture-runtime-credential") {
			t.Fatal("unsafe evidence")
		}
	}
	p, _ = f.s.get(ctx, f.s.Table, "P#"+f.project, "META")
	if p.InflightCount != 0 {
		t.Fatal("inflight not released")
	}
	for _, sk := range []string{"RUN#" + run.WorkflowRunID + "#INPUTS", outputKey(run.WorkflowRunID, 0)} {
		if _, e = f.s.get(ctx, f.s.Protected, p.PK, sk); !missingRow(e) {
			t.Fatal("terminal ciphertext retained")
		}
	}
	if _, e = f.s.Submit(ctx, f.owner, f.project, *run.Steps[0].ExecutionID, id(), 0, Options{}, true); e == nil {
		t.Fatal("step rerun allowed")
	}
	if _, e = f.s.SubmitWorkflow(ctx, f.owner, f.project, wf.WorkflowID, key, wf.Revision, RunOptions{Target: "cloud"}, false); e == nil {
		t.Fatal("different consent key reuse")
	}
}
func TestIntegrationWorkflowCrashBeforeFinalizationNoReplay(t *testing.T) {
	f, wf := workflowProtocolFixture(t)
	ctx := context.Background()
	run := workflowRun(t, f, wf, id())
	f.db.before = func(in *dynamodb.TransactWriteItemsInput) error {
		for _, a := range in.TransactItems {
			if a.Put != nil && a.Put.Item["kind"] != nil {
				if k, ok := a.Put.Item["kind"].(*dt.AttributeValueMemberS); ok && k.Value == "executionWorkflowOutput" {
					return errors.New("simulated crash before commit")
				}
			}
		}
		return nil
	}
	if e := f.s.Process(ctx, workflowMessage(f, run, 0)); e == nil {
		t.Fatal("fault not hit")
	}
	state, _ := f.s.RunDetail(ctx, f.owner, f.project, run.WorkflowRunID)
	if state.Steps[1].State != "pending" {
		t.Fatal("partial successor")
	}
	if _, e := f.s.get(ctx, f.s.Protected, "P#"+f.project, outputKey(run.WorkflowRunID, 0)); !missingRow(e) {
		t.Fatal("partial output")
	}
	f.db.before = nil
	f.now = f.now.Add(3 * time.Minute)
	if e := f.s.Process(ctx, workflowMessage(f, run, 0)); e != nil {
		t.Fatal(e)
	}
	state, e := f.s.RunDetail(ctx, f.owner, f.project, run.WorkflowRunID)
	if e != nil || state.State != "failed" || state.Steps[1].State != "skipped" || state.Steps[0].Summary.Outcome.Certainty != "unknown" || f.calls.Load() != 1 {
		t.Fatal("unknown outcome replayed or advanced")
	}
}
func TestIntegrationWorkflowLostFinalACKAndScheduling(t *testing.T) {
	f, wf := workflowProtocolFixture(t)
	ctx := context.Background()
	run := workflowRun(t, f, wf, id())
	faults := 0
	f.db.after = func(in *dynamodb.TransactWriteItemsInput) error {
		for _, a := range in.TransactItems {
			if a.Put != nil {
				if k, ok := a.Put.Item["kind"].(*dt.AttributeValueMemberS); ok && k.Value == "executionWorkflowOutput" {
					faults++
					return errors.New("simulated lost final ACK")
				}
			}
		}
		return nil
	}
	if e := f.s.Process(ctx, workflowMessage(f, run, 0)); e != nil {
		t.Fatal(e)
	}
	f.db.after = nil
	if faults == 0 {
		t.Fatal("fault not reached")
	}
	out, e := f.s.get(ctx, f.s.Table, "P#"+f.project, "OUT#"+workflowIdentity(run.WorkflowRunID, 1, "job"))
	if e != nil {
		t.Fatal("no durable schedule")
	}
	if e = f.s.publishOutbox(ctx, out); e != nil {
		t.Fatal(e)
	}
	if e = f.s.Process(ctx, workflowMessage(f, run, 1)); e != nil {
		t.Fatal(e)
	}
	if f.calls.Load() != 2 {
		t.Fatal("lost ACK duplicated HTTP")
	}
}
func TestIntegrationWorkflowPinsExpiryAndDelete(t *testing.T) {
	f, wf := workflowProtocolFixture(t)
	ctx := context.Background()
	run := workflowRun(t, f, wf, id())
	for i := 0; i < 2; i++ {
		if e := f.s.Process(ctx, workflowMessage(f, run, i)); e != nil {
			t.Fatal(e)
		}
	}
	run, _ = f.s.RunDetail(ctx, f.owner, f.project, run.WorkflowRunID)
	exec := *run.Steps[0].ExecutionID
	if _, e := f.s.Pin(ctx, f.owner, f.project, exec, true); e != nil {
		t.Fatal(e)
	}
	f.now = f.now.Add(31 * 24 * time.Hour)
	for i := 0; i < 100; i++ {
		_ = f.s.Maintain(ctx)
	}
	if _, e := f.s.RunDetail(ctx, f.owner, f.project, run.WorkflowRunID); e != nil {
		t.Fatal("pin lost run context")
	}
	if _, e := f.s.Evidence(ctx, f.owner, f.project, *run.Steps[1].ExecutionID); !missingRow(e) {
		t.Fatal("expired sibling protected")
	}
	if _, e := f.s.Pin(ctx, f.owner, f.project, exec, false); e != nil {
		t.Fatal(e)
	}
	if _, e := f.s.RunDetail(ctx, f.owner, f.project, run.WorkflowRunID); !missingRow(e) {
		t.Fatal("expired last unpin retained logical context")
	}
	for i := 0; i < 100; i++ {
		_ = f.s.Maintain(ctx)
	}
	if _, e := f.s.get(ctx, f.s.Table, "P#"+f.project, "RUN#"+run.WorkflowRunID); !missingRow(e) {
		t.Fatal("run context not cleaned")
	}
	p, _ := f.s.get(ctx, f.s.Table, "P#"+f.project, "META")
	p.State = "deleting"
	p.Version++
	p.DeletionEpoch++
	f.write(t, f.s.Table, p)
	for i := 0; i < 100; i++ {
		_ = f.s.DrainProject(ctx, f.project)
	}
	for _, table := range []string{f.s.Table, f.s.Protected} {
		r, e := f.db.Query(ctx, &dynamodb.QueryInput{TableName: &table, ConsistentRead: aws.Bool(true), KeyConditionExpression: aws.String("PK = :pk"), ExpressionAttributeValues: map[string]dt.AttributeValue{":pk": str(p.PK)}})
		if e != nil {
			t.Fatal(e)
		}
		for _, item := range r.Items {
			if item["SK"].(*dt.AttributeValueMemberS).Value != "META" && item["kind"].(*dt.AttributeValueMemberS).Value != "request" {
				t.Fatalf("project child remains kind=%s; cleanup error=%v", item["kind"].(*dt.AttributeValueMemberS).Value, f.s.DrainProject(ctx, f.project))
			}
		}
	}
}
func TestIntegrationWorkflowConcurrentDeliveryAndRevocation(t *testing.T) {
	f, wf := workflowProtocolFixture(t)
	ctx := context.Background()
	run := workflowRun(t, f, wf, id())
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _ = f.s.Process(ctx, workflowMessage(f, run, 0)) }()
	}
	wg.Wait()
	for i := 0; i < 3; i++ {
		_ = f.s.Process(ctx, workflowMessage(f, run, 0))
	}
	if f.calls.Load() != 1 {
		t.Fatal("concurrent deliveries repeated HTTP")
	}
	state, e := f.s.RunDetail(ctx, f.owner, f.project, run.WorkflowRunID)
	if e != nil || state.Steps[1].State != "queued" {
		t.Fatal("successor not atomic")
	}
	request, e := f.s.get(ctx, f.s.Table, "P#"+f.project, "REQ#"+f.request)
	if e != nil {
		t.Fatal(e)
	}
	request.State = "deleted"
	request.Revision++
	f.write(t, f.s.Table, request)
	if e = f.s.Process(ctx, workflowMessage(f, run, 1)); e != nil {
		t.Fatal(e)
	}
	state, e = f.s.RunDetail(ctx, f.owner, f.project, run.WorkflowRunID)
	if e != nil || state.State != "failed" || f.calls.Load() != 1 {
		t.Fatal("revoked request dispatched")
	}
}
func TestIntegrationWorkflowDeleteVersusFinalization(t *testing.T) {
	f, wf := workflowProtocolFixture(t)
	ctx := context.Background()
	run := workflowRun(t, f, wf, id())
	f.objects.putFault = func() {
		p, _ := f.s.get(ctx, f.s.Table, "P#"+f.project, "META")
		p.State = "deleting"
		p.Version++
		p.DeletionEpoch++
		f.write(t, f.s.Table, p)
	}
	if e := f.s.Process(ctx, workflowMessage(f, run, 0)); e == nil {
		t.Fatal("deletion did not fence publication")
	}
	f.objects.putFault = nil
	if _, e := f.s.RunDetail(ctx, f.owner, f.project, run.WorkflowRunID); !missingRow(e) {
		t.Fatal("deleted project still readable")
	}
	if _, e := f.s.get(ctx, f.s.Table, "P#"+f.project, "JOB#"+workflowIdentity(run.WorkflowRunID, 1, "job")); !missingRow(e) {
		t.Fatal("successor survived deletion race")
	}
	for i := 0; i < 30; i++ {
		_ = f.s.DrainProject(ctx, f.project)
	}
	if _, e := f.s.get(ctx, f.s.Protected, "P#"+f.project, "RUN#"+run.WorkflowRunID+"#INPUTS"); !missingRow(e) {
		t.Fatal("input retained after deletion")
	}
}
func TestIntegrationWorkflowRestoredGenerationNoHTTP(t *testing.T) {
	f, wf := workflowProtocolFixture(t)
	ctx := context.Background()
	run := workflowRun(t, f, wf, id())
	stage, _ := f.s.get(ctx, f.s.Table, "STAGE#dev-api", "META")
	stage.RecoveryGeneration = "fixture-restored"
	stage.Version++
	f.write(t, f.s.Table, stage)
	if e := f.s.Process(ctx, workflowMessage(f, run, 0)); e != nil {
		t.Fatal(e)
	}
	v, e := f.s.RunDetail(ctx, f.owner, f.project, run.WorkflowRunID)
	if e != nil || v.State != "failed" || v.Steps[1].State != "skipped" || f.calls.Load() != 0 {
		t.Fatal("restored work replayed or remained active")
	}
	if v.Steps[0].Summary.Outcome.Code == nil || *v.Steps[0].Summary.Outcome.Code != "recovery_interrupted" {
		t.Fatal("restore outcome missing")
	}
}
func TestIntegrationWorkflowAggregateFreezeAndAdmissionBounds(t *testing.T) {
	f, wf := workflowProtocolFixture(t)
	ctx := context.Background()
	p, _ := f.s.get(ctx, f.s.Table, "P#"+f.project, "REQ#"+f.request)
	var config Configuration
	_ = StrictJSON([]byte(p.ConfigurationJSON), &config)
	config.Body = &RequestBody{Type: "text", Text: strings.Repeat("x", 40*1024)}
	raw, _ := json.Marshal(config)
	p.ConfigurationJSON = string(raw)
	f.write(t, f.s.Table, p)
	d := wf.WorkflowDefinition
	for len(d.Steps) < 10 {
		d.Steps = append(d.Steps, runner.WorkflowStep{StepID: id(), RequestID: f.request, RequestRevision: 1, Extract: []runner.Extraction{}, Bindings: []runner.Binding{}})
	}
	wf, e := f.s.SaveWorkflow(ctx, f.owner, f.project, wf.WorkflowID, wf.Revision, d, false)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.s.SubmitWorkflow(ctx, f.owner, f.project, wf.WorkflowID, id(), wf.Revision, RunOptions{Target: "cloud", AllowInsecureSecrets: true}, false); e == nil {
		t.Fatal("aggregate freeze cap ignored")
	}
	if _, e = f.s.get(ctx, f.s.Table, "STAGE#dev-api", "QUOTA"); !missingRow(e) {
		t.Fatal("rejected freeze charged admission")
	}
}
func TestIntegrationWorkflowPinLosesToExpiryClaim(t *testing.T) {
	f, wf := workflowProtocolFixture(t)
	ctx := context.Background()
	run := workflowRun(t, f, wf, id())
	for i := 0; i < 2; i++ {
		if e := f.s.Process(ctx, workflowMessage(f, run, i)); e != nil {
			t.Fatal(e)
		}
	}
	run, _ = f.s.RunDetail(ctx, f.owner, f.project, run.WorkflowRunID)
	ret, e := f.s.get(ctx, f.s.Table, "P#"+f.project, "EXEC#"+*run.Steps[0].ExecutionID+"#RET")
	if e != nil {
		t.Fatal(e)
	}
	ret.NormalExpiresAt = stamp(f.now.Add(6 * time.Second))
	ret.DSK = ret.NormalExpiresAt + "#" + ret.ExecutionID
	f.write(t, f.s.Table, ret)
	injected := false
	f.db.before = func(in *dynamodb.TransactWriteItemsInput) error {
		for _, a := range in.TransactItems {
			if a.Put != nil {
				if pin, ok := a.Put.Item["pinned"].(*dt.AttributeValueMemberBOOL); ok && pin.Value && !injected {
					injected = true
					f.now = f.now.Add(7 * time.Second)
					_ = f.s.expire(ctx, ret)
				}
			}
		}
		return nil
	}
	if _, e = f.s.Pin(ctx, f.owner, f.project, ret.ExecutionID, true); e == nil {
		t.Fatal("pin resurrected expiry-claimed evidence")
	}
	f.db.before = nil
	if !injected {
		t.Fatal("race not exercised")
	}
	parent, e := f.s.get(ctx, f.s.Table, ret.PK, "RUN#"+run.WorkflowRunID)
	if e != nil || parent.RunPinnedCount != 0 || parent.AvailableCount != 1 {
		t.Fatal("run classification race corrupted counters")
	}
}
func TestIntegrationWorkflowTenStepTerminalBudget(t *testing.T) {
	f, wf := workflowProtocolFixture(t)
	d := wf.WorkflowDefinition
	for len(d.Steps) < 10 {
		d.Steps = append(d.Steps, runner.WorkflowStep{StepID: id(), RequestID: f.request, RequestRevision: 1, Extract: []runner.Extraction{}, Bindings: []runner.Binding{}})
	}
	ctx := context.Background()
	wf, e := f.s.SaveWorkflow(ctx, f.owner, f.project, wf.WorkflowID, wf.Revision, d, false)
	if e != nil {
		t.Fatal(e)
	}
	largest := 0
	f.db.before = func(in *dynamodb.TransactWriteItemsInput) error {
		if len(in.TransactItems) > largest {
			largest = len(in.TransactItems)
		}
		raw, _ := json.Marshal(in.TransactItems)
		if len(in.TransactItems) > 60 || len(raw) > 2*1024*1024 {
			t.Error("workflow transaction budget exceeded")
		}
		return nil
	}
	run := workflowRun(t, f, wf, id())
	request, _ := f.s.get(ctx, f.s.Table, "P#"+f.project, "REQ#"+f.request)
	request.State = "deleted"
	request.Revision++
	f.write(t, f.s.Table, request)
	if e = f.s.Process(ctx, workflowMessage(f, run, 0)); e != nil {
		t.Fatal(e)
	}
	v, e := f.s.RunDetail(ctx, f.owner, f.project, run.WorkflowRunID)
	if e != nil || v.State != "failed" || largest < 25 || f.calls.Load() != 0 {
		t.Fatal("bounded terminalization not exercised")
	}
	for i := 1; i < 10; i++ {
		if v.Steps[i].State != "skipped" {
			t.Fatal("successor not skipped atomically")
		}
	}
	quota, _ := f.s.get(ctx, f.s.Table, "STAGE#dev-api", "QUOTA")
	if quota.Count != 10 {
		t.Fatal("skipped steps incorrectly refunded")
	}
}
func TestIntegrationWorkflowPaginationKeepsPayloadBounded(t *testing.T) {
	f, wf := workflowProtocolFixture(t)
	ctx := context.Background()
	for i := 0; i < 25; i++ {
		if _, e := f.s.SaveWorkflow(ctx, f.owner, f.project, "", 0, wf.WorkflowDefinition, true); e != nil {
			t.Fatal(e)
		}
	}
	page, e := f.s.Workflows(ctx, f.owner, f.project, 100, "")
	if e != nil || len(page.Items) != 25 || page.NextCursor == nil {
		t.Fatal("bounded collection page failed")
	}
	next, e := f.s.Workflows(ctx, f.owner, f.project, 100, *page.NextCursor)
	if e != nil || len(next.Items) != 1 || next.NextCursor != nil {
		t.Fatal("cursor dropped a workflow")
	}
	seen := map[string]bool{}
	for _, w := range page.Items {
		seen[w.WorkflowID] = true
	}
	if seen[next.Items[0].WorkflowID] {
		t.Fatal("cursor duplicated a workflow")
	}
	if _, e = f.s.Workflows(ctx, f.owner, f.project, 101, ""); e == nil {
		t.Fatal("over-limit page accepted")
	}
}
func TestIntegrationWorkflowRepeatedUnpinAfterContextCleanup(t *testing.T) {
	f, wf := workflowProtocolFixture(t)
	ctx := context.Background()
	v := workflowRun(t, f, wf, id())
	for i := 0; i < 2; i++ {
		if e := f.s.Process(ctx, workflowMessage(f, v, i)); e != nil {
			t.Fatal(e)
		}
	}
	v, _ = f.s.RunDetail(ctx, f.owner, f.project, v.WorkflowRunID)
	f.now = f.now.Add(31 * 24 * time.Hour)
	for _, step := range v.Steps {
		ret, e := f.s.get(ctx, f.s.Table, "P#"+f.project, "EXEC#"+*step.ExecutionID+"#RET")
		if e != nil {
			t.Fatal(e)
		}
		_ = f.s.expire(ctx, ret)
	}
	run, e := f.s.get(ctx, f.s.Table, "P#"+f.project, "RUN#"+v.WorkflowRunID)
	if e != nil {
		t.Fatal(e)
	}
	if e = f.s.expireRun(ctx, run); e != nil {
		t.Fatal(e)
	}
	result, e := f.s.Pin(ctx, f.owner, f.project, *v.Steps[0].ExecutionID, false)
	if e != nil || result.Retention.Pinned {
		t.Fatal("metadata-only repeated unpin lost after context cleanup")
	}
}
func TestIntegrationWorkflowFullProjectRejectsNewAdmission(t *testing.T) {
	f, wf := workflowProtocolFixture(t)
	ctx := context.Background()
	p, _ := f.s.get(ctx, f.s.Table, "P#"+f.project, "META")
	p.StorageReservedBytes = 1024 * 1024 * 1024
	p.Version++
	f.write(t, f.s.Table, p)
	_, e := f.s.SubmitWorkflow(ctx, f.owner, f.project, wf.WorkflowID, id(), wf.Revision, RunOptions{Target: "cloud", AllowInsecureSecrets: true}, false)
	var code *Error
	if !errors.As(e, &code) || code.Status != 429 {
		t.Fatal("project storage limit bypassed")
	}
	if _, e = f.s.get(ctx, f.s.Table, "STAGE#dev-api", "QUOTA"); !missingRow(e) {
		t.Fatal("rejected storage admission charged quota")
	}
}

// These tests use DynamoDB Local and controlled HTTP/KMS/queue/object fixtures.
func finishWorkflowFixture(t *testing.T, f *protocolFixture, run WorkflowRun) {
	t.Helper()
	for i := range run.Steps {
		if e := f.s.Process(context.Background(), workflowMessage(f, run, i)); e != nil {
			t.Fatal(e)
		}
	}
}
func assertWorkflowAccounting(t *testing.T, f *protocolFixture, run WorkflowRun, state string, sends, admissions int64) {
	t.Helper()
	ctx := context.Background()
	detail, e := f.s.RunDetail(ctx, f.owner, f.project, run.WorkflowRunID)
	if e != nil || detail.State != state || int64(f.calls.Load()) != sends {
		t.Fatalf("unexpected outcome: state=%s sends=%d err=%v", detail.State, f.calls.Load(), e)
	}
	p, e := f.s.get(ctx, f.s.Table, "P#"+f.project, "META")
	if e != nil || p.InflightCount != 0 {
		t.Fatal("inflight accounting not released exactly once", e)
	}
	q, e := f.s.get(ctx, f.s.Table, "STAGE#dev-api", "QUOTA")
	if e != nil || q.Count != admissions {
		t.Fatal("admission accounting changed", e)
	}
	if state == "failed" && (detail.Steps[0].State != "failed" || detail.Steps[1].State != "skipped") {
		t.Fatal("undispatched run did not fail and skip successors")
	}
}
func TestIntegrationWorkflowDeletedDefinitionFrozenRerun(t *testing.T) {
	f, wf := workflowProtocolFixture(t)
	ctx := context.Background()
	original := workflowRun(t, f, wf, id())
	finishWorkflowFixture(t, f, original)
	if e := f.s.DeleteWorkflow(ctx, f.owner, f.project, wf.WorkflowID, wf.Revision); e != nil {
		t.Fatal(e)
	}
	key := id()
	rerun, e := f.s.SubmitWorkflow(ctx, f.owner, f.project, original.WorkflowRunID, key, 0, RunOptions{Target: "cloud", AllowInsecureSecrets: true}, true)
	if e != nil {
		t.Fatal(e)
	}
	stored, e := f.s.get(ctx, f.s.Table, "P#"+f.project, "RUN#"+rerun.WorkflowRunID)
	if e != nil || stored.WorkflowSubmission != workflowFrozenRerun || stored.RerunSourceRunID != original.WorkflowRunID {
		t.Fatal("rerun provenance missing", e)
	}
	again, e := f.s.SubmitWorkflow(ctx, f.owner, f.project, original.WorkflowRunID, key, 0, RunOptions{Target: "cloud", AllowInsecureSecrets: true}, true)
	if e != nil || again.WorkflowRunID != rerun.WorkflowRunID {
		t.Fatal("rerun admission duplicated", e)
	}
	// Change live configuration: the rerun must still extract/bind its frozen sequence.
	req, _ := f.s.get(ctx, f.s.Table, "P#"+f.project, "REQ#"+f.request)
	req.ConfigurationJSON = "{}"
	req.Revision++
	f.write(t, f.s.Table, req)
	finishWorkflowFixture(t, f, rerun)
	for i := 0; i < 2; i++ {
		if e = f.s.Process(ctx, workflowMessage(f, rerun, i)); e != nil {
			t.Fatal(e)
		}
	}
	assertWorkflowAccounting(t, f, rerun, "completed", 4, 4)
	detail, e := f.s.RunDetail(ctx, f.owner, f.project, rerun.WorkflowRunID)
	if e != nil || detail.StartedAt == nil || detail.LiveWorkflowID != nil {
		t.Fatal("rerun intent or deleted-definition inspection incorrect", e)
	}
}
func TestIntegrationWorkflowQueuedDefinitionDeletion(t *testing.T) {
	f, wf := workflowProtocolFixture(t)
	run := workflowRun(t, f, wf, id())
	if e := f.s.DeleteWorkflow(context.Background(), f.owner, f.project, wf.WorkflowID, wf.Revision); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 3; i++ {
		if e := f.s.Process(context.Background(), workflowMessage(f, run, 0)); e != nil {
			t.Fatal(e)
		}
	}
	assertWorkflowAccounting(t, f, run, "failed", 0, 2)
}
func TestIntegrationWorkflowFrozenRerunRequiredSources(t *testing.T) {
	for _, kind := range []string{"future_request", "secret"} {
		for _, timing := range []string{"before_admission", "after_admission", "at_intent"} {
			t.Run(kind+"/"+timing, func(t *testing.T) {
				f, wf := workflowProtocolFixture(t)
				ctx := context.Background()
				// Give step two a distinct required request so step one's source
				// check alone cannot authorize the entire frozen sequence.
				def, e := f.s.WorkflowDetail(ctx, f.owner, f.project, wf.WorkflowID)
				if e != nil {
					t.Fatal(e)
				}
				req, _ := f.s.get(ctx, f.s.Table, "P#"+f.project, "REQ#"+f.request)
				req.RequestID = id()
				req.SK = "REQ#" + req.RequestID
				var source row
				table := f.s.Table
				if kind == "secret" {
					secret := id()
					source = row{PK: req.PK, SK: "SECRET#" + secret, Kind: "savedSecret", SchemaVersion: 1, ProjectID: f.project, SecretID: secret, Revision: 1, State: "active", ValueKind: "string", Envelope: fixtureSecretEnvelope(t, f.s, f.project, secret, "fixture-required-secret", 1)}
					table = f.s.Protected
					f.write(t, table, source)
					var cfg Configuration
					if e := json.Unmarshal([]byte(req.ConfigurationJSON), &cfg); e != nil {
						t.Fatal(e)
					}
					cfg.Headers = append(cfg.Headers, Field{Name: "X-Protected", Enabled: true, Sensitive: true, BindingID: id(), Masked: true, SecretRef: &Ref{SecretID: secret}})
					raw, _ := json.Marshal(cfg)
					req.ConfigurationJSON = string(raw)
				}
				f.write(t, f.s.Table, req)
				if kind == "future_request" {
					source = req
				}
				def.Steps[1].RequestID = req.RequestID
				wf, e = f.s.SaveWorkflow(ctx, f.owner, f.project, wf.WorkflowID, wf.Revision, WorkflowDefinition{Name: def.Name, Steps: def.Steps}, false)
				if e != nil {
					t.Fatal(e)
				}
				original := workflowRun(t, f, wf, id())
				finishWorkflowFixture(t, f, original)
				if e = f.s.DeleteWorkflow(ctx, f.owner, f.project, wf.WorkflowID, wf.Revision); e != nil {
					t.Fatal(e)
				}
				revoke := func() {
					source.State = "deleted"
					source.Revision++
					f.write(t, table, source)
				}
				if timing == "before_admission" {
					revoke()
				}
				rerun, e := f.s.SubmitWorkflow(ctx, f.owner, f.project, original.WorkflowRunID, id(), 0, RunOptions{Target: "cloud", AllowInsecureSecrets: true}, true)
				if timing == "before_admission" {
					if e == nil {
						t.Fatal("revoked source admitted")
					}
					assertWorkflowAccounting(t, f, original, "completed", 2, 2)
					return
				}
				if e != nil {
					t.Fatal(e)
				}
				if timing == "after_admission" {
					revoke()
				} else {
					fired := false
					f.db.before = func(in *dynamodb.TransactWriteItemsInput) error {
						if !fired && workflowIntentTransaction(in) {
							fired = true
							revoke()
						}
						return nil
					}
					_ = f.s.Process(ctx, workflowMessage(f, rerun, 0))
					f.db.before = nil
					if !fired || f.calls.Load() != 2 {
						t.Fatal("revocation did not fence first intent")
					}
					f.now = f.now.Add(121 * time.Second)
				}
				for i := 0; i < 3; i++ {
					if e = f.s.Process(ctx, workflowMessage(f, rerun, 0)); e != nil {
						t.Fatal(e)
					}
				}
				assertWorkflowAccounting(t, f, rerun, "failed", 2, 4)
			})
		}
	}
}
func workflowIntentTransaction(in *dynamodb.TransactWriteItemsInput) bool {
	for _, a := range in.TransactItems {
		if a.Put != nil {
			kind, _ := a.Put.Item["kind"].(*dt.AttributeValueMemberS)
			state, _ := a.Put.Item["state"].(*dt.AttributeValueMemberS)
			if kind != nil && state != nil && kind.Value == "executionJob" && state.Value == "running" {
				return true
			}
		}
	}
	return false
}
func TestIntegrationWorkflowDefinitionDeletionAtFirstIntent(t *testing.T) {
	for _, origin := range []string{"definition", "frozen_rerun"} {
		for _, winner := range []string{"deletion", "intent"} {
			t.Run(origin+"/"+winner, func(t *testing.T) {
				f, wf := workflowProtocolFixture(t)
				ctx := context.Background()
				run := workflowRun(t, f, wf, id())
				sends, admissions := int64(0), int64(2)
				if origin == "frozen_rerun" {
					finishWorkflowFixture(t, f, run)
					var e error
					run, e = f.s.SubmitWorkflow(ctx, f.owner, f.project, run.WorkflowRunID, id(), 0, RunOptions{Target: "cloud", AllowInsecureSecrets: true}, true)
					if e != nil {
						t.Fatal(e)
					}
					sends, admissions = 2, 4
				}
				fired := false
				hook := func(in *dynamodb.TransactWriteItemsInput) error {
					if !fired && workflowIntentTransaction(in) {
						fired = true
						if e := f.s.DeleteWorkflow(ctx, f.owner, f.project, wf.WorkflowID, wf.Revision); e != nil {
							t.Fatal(e)
						}
					}
					return nil
				}
				if winner == "deletion" {
					f.db.before = hook
				} else {
					f.db.after = hook
				}
				_ = f.s.Process(ctx, workflowMessage(f, run, 0))
				f.db.before, f.db.after = nil, nil
				if !fired {
					t.Fatal("first-intent race not exercised")
				}
				f.now = f.now.Add(121 * time.Second)
				// Duplicate deliveries race after either side of the intent boundary.
				var wg sync.WaitGroup
				for i := 0; i < 8; i++ {
					wg.Add(1)
					go func() { defer wg.Done(); _ = f.s.Process(ctx, workflowMessage(f, run, 0)) }()
				}
				wg.Wait()
				if winner == "deletion" && origin == "definition" {
					assertWorkflowAccounting(t, f, run, "failed", sends, admissions)
					return
				}
				if e := f.s.Process(ctx, workflowMessage(f, run, 1)); e != nil {
					t.Fatal(e)
				}
				assertWorkflowAccounting(t, f, run, "completed", sends+2, admissions)
			})
		}
	}
}

func TestIntegrationWorkflowRerunAdmissionContextDeletionRace(t *testing.T) {
	f, wf := workflowProtocolFixture(t)
	ctx := context.Background()
	original := workflowRun(t, f, wf, id())
	finishWorkflowFixture(t, f, original)
	if e := f.s.DeleteWorkflow(ctx, f.owner, f.project, wf.WorkflowID, wf.Revision); e != nil {
		t.Fatal(e)
	}
	fired := false
	f.db.before = func(in *dynamodb.TransactWriteItemsInput) error {
		for _, a := range in.TransactItems {
			if a.Put == nil {
				continue
			}
			kind, _ := a.Put.Item["kind"].(*dt.AttributeValueMemberS)
			if !fired && kind != nil && kind.Value == "executionWorkflowRun" {
				fired = true
				// Model context cleanup winning after the authorized retained
				// read and before admission. No new run/quota may commit.
				_, e := f.db.Client.DeleteItem(ctx, &dynamodb.DeleteItemInput{TableName: &f.s.Table, Key: keys("P#"+f.project, "RUN#"+original.WorkflowRunID)})
				return e
			}
		}
		return nil
	}
	_, e := f.s.SubmitWorkflow(ctx, f.owner, f.project, original.WorkflowRunID, id(), 0, RunOptions{Target: "cloud", AllowInsecureSecrets: true}, true)
	f.db.before = nil
	if !fired || e == nil || f.calls.Load() != 2 {
		t.Fatal("source context deletion did not fence rerun admission")
	}
	p, _ := f.s.get(ctx, f.s.Table, "P#"+f.project, "META")
	q, _ := f.s.get(ctx, f.s.Table, "STAGE#dev-api", "QUOTA")
	if p.InflightCount != 0 || q.Count != 2 {
		t.Fatal("rejected rerun consumed accounting")
	}
}
func TestIntegrationWorkflowLegacyRunRequiresDefinition(t *testing.T) {
	f, wf := workflowProtocolFixture(t)
	ctx := context.Background()
	run := workflowRun(t, f, wf, id())
	stored, _ := f.s.get(ctx, f.s.Table, "P#"+f.project, "RUN#"+run.WorkflowRunID)
	stored.WorkflowSubmission = ""
	f.write(t, f.s.Table, stored)
	if e := f.s.DeleteWorkflow(ctx, f.owner, f.project, wf.WorkflowID, wf.Revision); e != nil {
		t.Fatal(e)
	}
	if e := f.s.Process(ctx, workflowMessage(f, run, 0)); e != nil {
		t.Fatal(e)
	}
	assertWorkflowAccounting(t, f, run, "failed", 0, 2)
}

// Faults model AWS TransactionConflict cancellations around real Local
// transactions. Polling uses the real RunDetail condition checks, not a mock.
func TestIntegrationWorkflowPublicationConflictsWithPolling(t *testing.T) {
	for _, phase := range []string{"pending", "writing", "uploaded", "published"} {
		t.Run(phase, func(t *testing.T) {
			f, wf := workflowProtocolFixture(t)
			run := workflowRun(t, f, wf, id())
			ctx := context.Background()
			conflicts := 0
			f.db.before = func(in *dynamodb.TransactWriteItemsInput) error {
				if conflicts < 2 && writesState(in, "executionUpload", phase) {
					if _, e := f.s.RunDetail(ctx, f.owner, f.project, run.WorkflowRunID); e != nil {
						t.Fatal("normal detail polling failed before injected conflict")
					}
					conflicts++
					return canceledCodes("None", "TransactionConflict")
				}
				return nil
			}
			if e := f.s.Process(ctx, workflowMessage(f, run, 0)); e != nil {
				t.Fatalf("publication failed after transient %s conflict", phase)
			}
			f.db.before = nil
			if conflicts != 2 {
				t.Fatal("publication boundary not exercised")
			}
			for i := 0; i < 2; i++ {
				if e := f.s.Process(ctx, workflowMessage(f, run, i)); e != nil {
					t.Fatal(e)
				}
				if e := f.s.Process(ctx, workflowMessage(f, run, i)); e != nil {
					t.Fatal(e)
				}
			}
			result, e := f.s.RunDetail(ctx, f.owner, f.project, run.WorkflowRunID)
			p, _ := f.s.get(ctx, f.s.Table, "P#"+f.project, "META")
			quota, _ := f.s.get(ctx, f.s.Table, "STAGE#dev-api", "QUOTA")
			if e != nil || result.State != "completed" || f.calls.Load() != 2 || f.objects.puts != 2 || p.InflightCount != 0 || quota.Count != 2 {
				t.Fatal("poll/conflict recovery duplicated effects or accounting")
			}
			for i := 0; i < 2; i++ {
				if _, err := f.s.get(ctx, f.s.Protected, p.PK, outputKey(run.WorkflowRunID, i)); !missingRow(err) {
					t.Fatal("terminal ciphertext retained")
				}
			}
		})
	}
}

func TestIntegrationWorkflowExhaustedPublicationConflictNeverReplays(t *testing.T) {
	f, wf := workflowProtocolFixture(t)
	run := workflowRun(t, f, wf, id())
	ctx := context.Background()
	f.db.before = func(in *dynamodb.TransactWriteItemsInput) error {
		if writesState(in, "executionUpload", "published") {
			return canceledCodes("TransactionConflict")
		}
		return nil
	}
	if f.s.Process(ctx, workflowMessage(f, run, 0)) == nil {
		t.Fatal("exhaustion must surface")
	}
	if f.calls.Load() != 1 {
		t.Fatal("unexpected target count")
	}
	f.db.before = nil
	f.now = f.now.Add(121 * time.Second)
	if e := f.s.Process(ctx, workflowMessage(f, run, 0)); e != nil {
		t.Fatal(e)
	}
	if e := f.s.Process(ctx, workflowMessage(f, run, 0)); e != nil {
		t.Fatal(e)
	}
	result, e := f.s.RunDetail(ctx, f.owner, f.project, run.WorkflowRunID)
	if e != nil || result.State != "failed" || result.Steps[0].Summary.Outcome.Certainty != "unknown" || result.Steps[1].State != "skipped" || f.calls.Load() != 1 {
		t.Fatal("exhausted publication failed to stop conservatively without HTTP replay")
	}
	p, _ := f.s.get(ctx, f.s.Table, "P#"+f.project, "META")
	quota, _ := f.s.get(ctx, f.s.Table, "STAGE#dev-api", "QUOTA")
	if p.InflightCount != 0 || quota.Count != 2 {
		t.Fatal("recovery accounting mismatch")
	}
}

func TestIntegrationWorkflowConflictRetryPreservesFinalizationGuards(t *testing.T) {
	for _, guard := range []string{"project-deletion", "source-revocation", "recovery-generation"} {
		t.Run(guard, func(t *testing.T) {
			f, wf := workflowProtocolFixture(t)
			ctx := context.Background()
			run := workflowRun(t, f, wf, id())
			fired := false
			f.db.before = func(in *dynamodb.TransactWriteItemsInput) error {
				if !fired && writesState(in, "executionUpload", "published") {
					fired = true
					switch guard {
					case "project-deletion":
						p, _ := f.s.get(ctx, f.s.Table, "P#"+f.project, "META")
						p.State = "deleting"
						p.Version++
						p.DeletionEpoch++
						f.write(t, f.s.Table, p)
					case "source-revocation":
						r, _ := f.s.get(ctx, f.s.Table, "P#"+f.project, "REQ#"+f.request)
						r.State = "deleted"
						r.Revision++
						f.write(t, f.s.Table, r)
					case "recovery-generation":
						stage, _ := f.s.get(ctx, f.s.Table, "STAGE#dev-api", "META")
						stage.RecoveryGeneration = "recovered"
						stage.Version++
						f.write(t, f.s.Table, stage)
					}
					return canceledCodes("TransactionConflict")
				}
				return nil
			}
			if f.s.Process(ctx, workflowMessage(f, run, 0)) == nil || !fired || f.calls.Load() != 1 {
				t.Fatal("changed finalization guard bypassed")
			}
			if _, e := f.s.get(ctx, f.s.Table, "P#"+f.project, "JOB#"+workflowIdentity(run.WorkflowRunID, 1, "job")); !missingRow(e) {
				t.Fatal("successor created despite failed guard")
			}
			if _, e := f.s.get(ctx, f.s.Protected, "P#"+f.project, outputKey(run.WorkflowRunID, 0)); !missingRow(e) {
				t.Fatal("output committed despite failed guard")
			}
		})
	}
}
