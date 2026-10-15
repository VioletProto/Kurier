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
			if f.calls.Add(1) == 2 && r.Header.Get("Authorization") != "Bearer fixture-runtime-credential" {
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
