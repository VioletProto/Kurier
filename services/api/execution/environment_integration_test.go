//go:build integration

package execution

import (
	"bytes"
	"context"
	"encoding/json"
	runner "github.com/VioletProto/Kurier/services/request-runtime"
	wire "github.com/VioletProto/Kurier/services/request-runtime/mcpwire"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dt "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"io"
	"net/http"
	"strings"
	"testing"
)

func enableEnvironmentFixture(t *testing.T, f *protocolFixture) {
	t.Helper()
	ctx := context.Background()
	stage, e := f.s.stage(ctx)
	if e != nil {
		t.Fatal(e)
	}
	stage.SavedRequestsSchemaVersion = 9
	stage.ProjectEnvironmentsSchemaVersion = 1
	stage.ProtectedSecretsSchemaVersion = 2
	stage.LocalAgentExecutionsSchemaVersion = 2
	stage.EvidencePinningSchemaVersion = 1
	stage.SequentialWorkflowsSchemaVersion = 1
	stage.MCPReadOnlySchemaVersion = 1
	stage.WorkflowAssertionsSchemaVersion = 1
	stage.Version++
	f.write(t, f.s.Table, stage)
	p, _ := f.s.project(ctx, f.owner, f.project)
	p.PinnedCount = pointer(int64(0))
	p.PinnedBytes = pointer(int64(0))
	f.write(t, f.s.Table, p)
}
func fixtureEnvironment(t *testing.T, f *protocolFixture, url string) (runner.Environment, row, string) {
	t.Helper()
	secret := id()
	env := runner.Environment{EnvironmentID: id(), ProjectID: f.project, Revision: 1, Name: "fixture", CreatedAt: stamp(f.now), UpdatedAt: stamp(f.now), Variables: []runner.EnvironmentVariable{
		{VariableID: id(), Name: "n", Enabled: true, Value: &runner.Primitive{Type: "number", Decimal: "0.10000000000000001"}},
		{VariableID: id(), Name: "token", Enabled: true, Sensitive: true, ValueType: "string", Masked: true, SecretRef: &Ref{SecretID: secret}},
	}}
	raw, _ := json.Marshal(env)
	record := row{PK: "P#" + f.project, SK: "ENV#" + env.EnvironmentID, Kind: "environment", SchemaVersion: 1, EnvironmentID: env.EnvironmentID, ProjectID: f.project, State: "active", Revision: 1, ConfigurationJSON: string(raw)}
	f.write(t, f.s.Table, record)
	saved := row{PK: record.PK, SK: "SECRET#" + secret, Kind: "savedSecret", SchemaVersion: 2, ScalarType: "string", ValueKind: "string", ProjectID: f.project, SecretID: secret, Revision: 1, State: "active", Envelope: fixtureSecretEnvelope(t, f.s, f.project, secret, "environment-private-canary-before", 1)}
	f.write(t, f.s.Protected, saved)
	request, e := f.s.get(context.Background(), f.s.Table, record.PK, "REQ#"+f.request)
	if e != nil {
		t.Fatal(e)
	}
	c := fixturePlan().Configuration
	c.TemplateSemanticsVersion = 1
	c.Method = "POST"
	c.URL = url
	c.Headers = []Field{{Name: "Authorization", Value: "Bearer {{token}}", Enabled: true}}
	c.Body = &RequestBody{Type: "json", Text: `{"n":"\u007b\u007bn\u007d\u007d"}`}
	raw, _ = json.Marshal(c)
	request.ConfigurationJSON = string(raw)
	request.SchemaVersion = 3
	f.write(t, f.s.Table, request)
	return env, record, secret
}
func writeEnvironmentFixture(t *testing.T, f *protocolFixture, env runner.Environment, r row) {
	t.Helper()
	raw, _ := json.Marshal(env)
	r.ConfigurationJSON = string(raw)
	r.Revision = env.Revision
	f.write(t, f.s.Table, r)
}
func environmentOptions(env runner.Environment) Options {
	o, _ := ParseOptions([]byte(`{}`))
	o.Environment = &runner.EnvironmentContext{EnvironmentID: env.EnvironmentID, Revision: env.Revision}
	o.AllowInsecureSecrets = true
	return o
}
func TestIntegrationEnvironmentFrozenCurrentSecretRerunAndMCP(t *testing.T) {
	f := newProtocolFixture(t)
	enableEnvironmentFixture(t, f)
	env, envRow, secret := fixtureEnvironment(t, f, "http://fixture.example/")
	ctx := context.Background()
	options := environmentOptions(env)
	wantNumber, wantSecret := "0.10000000000000001", "environment-private-canary-before"
	f.s.prepare = func(ctx context.Context, plan Plan, values map[string]string) (*Prepared, error) {
		return fixturePrepared(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			f.calls.Add(1)
			body, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(body), wantNumber) || r.Header.Get("Authorization") != "Bearer "+wantSecret {
				t.Error("frozen public/current exact secret mismatch")
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"reflection":"` + wantSecret + `"}`))
		}), plan, values), nil
	}
	key := id()
	admitted, e := f.s.Submit(ctx, f.owner, f.project, f.request, key, 1, options, false)
	if e != nil {
		t.Fatal(e)
	}
	anchor, _ := f.s.get(ctx, f.s.Table, envRow.PK, "EXEC#"+admitted.ExecutionID)
	job := anchor.JobID
	env.Revision++
	env.Variables[0].Value.Decimal = "2"
	writeEnvironmentFixture(t, f, env, envRow)
	saved, _ := f.s.get(ctx, f.s.Protected, envRow.PK, "SECRET#"+secret)
	saved.Revision++
	saved.Envelope = fixtureSecretEnvelope(t, f.s, f.project, secret, "environment-private-canary-after", saved.Revision)
	f.write(t, f.s.Protected, saved)
	retry, e := f.s.Submit(ctx, f.owner, f.project, f.request, key, 1, options, false)
	if e != nil || retry.ExecutionID != admitted.ExecutionID {
		t.Fatal("receipt refroze after edit", e)
	}
	if e = f.s.Process(ctx, Message{f.project, job, "OUT#" + job}); e != nil {
		t.Fatal(e)
	}
	raw, evidence := readAssertionEvidence(t, f, admitted.ExecutionID)
	if evidence.SchemaVersion != 3 || evidence.EnvironmentContext.Revision != 1 || bytes.Contains(raw, []byte("environment-private-canary")) {
		t.Fatal("unsafe evidence or environment drift")
	}
	if _, e = f.s.get(ctx, f.s.Protected, envRow.PK, "JOB#"+job+"#BINDINGS"); !missingRow(e) {
		t.Fatal("terminal inputs retained")
	}
	wantSecret = "environment-private-canary-after"
	again, e := f.s.Submit(ctx, f.owner, f.project, admitted.ExecutionID, id(), 0, Options{AllowInsecureSecrets: true}, true)
	if e != nil {
		t.Fatal(e)
	}
	anchor, _ = f.s.get(ctx, f.s.Table, envRow.PK, "EXEC#"+again.ExecutionID)
	if e = f.s.Process(ctx, Message{f.project, anchor.JobID, "OUT#" + anchor.JobID}); e != nil {
		t.Fatal(e)
	}
	if f.calls.Load() != 2 {
		t.Fatal("rerun target count incorrect")
	}
	pair, _, e := f.s.CreateMCP(ctx, f.owner, id(), MCPPair{ProjectIDs: []string{f.project}})
	if e != nil {
		t.Fatal(e)
	}
	for _, view := range []string{"complete", "summary"} {
		delivery := invokeMCP(t, f, *pair.Token, "get_execution_evidence", map[string]any{"projectId": f.project, "executionId": admitted.ExecutionID, "view": view})
		if delivery.Error != nil {
			t.Fatal("MCP rejected environment evidence", delivery.Error.Code)
		}
		if _, e = wire.Decode("get_execution_evidence", mcpRaw(delivery)); e != nil {
			t.Fatal(e)
		}
		projection := delivery.Data.Evidence
		if !strings.Contains(string(mcpRaw(projection.OmittedFields)), "environmentContext") {
			t.Fatal("context omission unmarked")
		}
		if view == "complete" && (!projection.Complete || projection.View != "complete" || projection.Capture.SchemaVersion != 1 || *projection.OmissionReason != "environment_context_not_exposed") {
			t.Fatal("wrong legacy projection")
		}
	}
	after, _ := f.s.Evidence(ctx, f.owner, f.project, admitted.ExecutionID)
	if !bytes.Equal(raw, after) {
		t.Fatal("MCP changed immutable evidence")
	}
}
func TestIntegrationEnvironmentRevocationStopsFrozenDispatch(t *testing.T) {
	for _, mode := range []string{"disable", "remove", "delete", "secret-revoke"} {
		t.Run(mode, func(t *testing.T) {
			f := newProtocolFixture(t)
			enableEnvironmentFixture(t, f)
			env, r, secret := fixtureEnvironment(t, f, "http://fixture.example/")
			ctx := context.Background()
			options := environmentOptions(env)
			v, e := f.s.Submit(ctx, f.owner, f.project, f.request, id(), 1, options, false)
			if e != nil {
				t.Fatal(e)
			}
			anchor, _ := f.s.get(ctx, f.s.Table, r.PK, "EXEC#"+v.ExecutionID)
			switch mode {
			case "disable":
				env.Variables[0].Enabled = false
				env.Revision++
				writeEnvironmentFixture(t, f, env, r)
			case "remove":
				env.Variables[0].VariableID = id()
				env.Revision++
				writeEnvironmentFixture(t, f, env, r)
			case "delete":
				r.State = "deleted"
				r.Kind = "environmentTombstone"
				r.ConfigurationJSON = ""
				r.Revision++
				f.write(t, f.s.Table, r)
			case "secret-revoke":
				s, _ := f.s.get(ctx, f.s.Protected, r.PK, "SECRET#"+secret)
				s.State = "revoked"
				s.Revision++
				s.Envelope = nil
				f.write(t, f.s.Protected, s)
			}
			message := Message{f.project, anchor.JobID, "OUT#" + anchor.JobID}
			if e = f.s.Process(ctx, message); e != nil {
				t.Fatal(e)
			}
			_, evidence := readAssertionEvidence(t, f, v.ExecutionID)
			if f.calls.Load() != 0 || evidence.Outcome.Certainty != "not_dispatched" || evidence.Outcome.Code == nil || *evidence.Outcome.Code != "source_unavailable" {
				t.Fatal("revoked source dispatched")
			}
			if mode == "disable" {
				env.Variables[0].Enabled = true
				env.Revision++
				writeEnvironmentFixture(t, f, env, r)
				if e = f.s.Process(ctx, message); e != nil || f.calls.Load() != 0 {
					t.Fatal("reenabling replayed terminal execution")
				}
			}
		})
	}
}
func TestIntegrationEnvironmentLocalNegotiationIdentityReceipt(t *testing.T) {
	f := newLocalFixture(t)
	enableEnvironmentFixture(t, f.protocolFixture)
	env, _, _ := fixtureEnvironment(t, f.protocolFixture, f.target)
	ctx := context.Background()
	options := environmentOptions(env)
	options.Target = "local"
	options.Local = &LocalOptions{LocalAgentID: f.agent.LocalAgentID, AllowPrivateNetwork: true}
	before, err := f.db.Query(ctx, &dynamodb.QueryInput{TableName: &f.s.Table, KeyConditionExpression: aws.String("PK = :pk"), ExpressionAttributeValues: map[string]dt.AttributeValue{":pk": str("P#" + f.project)}, ConsistentRead: aws.Bool(true)})
	if err != nil {
		t.Fatal(err)
	}
	if _, e := f.s.Submit(ctx, f.owner, f.project, f.request, id(), 1, options, false); e == nil {
		t.Fatal("older agent admitted environment job")
	}
	after, err := f.db.Query(ctx, &dynamodb.QueryInput{TableName: &f.s.Table, KeyConditionExpression: aws.String("PK = :pk"), ExpressionAttributeValues: map[string]dt.AttributeValue{":pk": str("P#" + f.project)}, ConsistentRead: aws.Bool(true)})
	if err != nil {
		t.Fatal(err)
	}
	beforeBytes, _ := json.Marshal(before.Items)
	afterBytes, _ := json.Marshal(after.Items)
	if !bytes.Equal(beforeBytes, afterBytes) {
		t.Fatal("incompatible admission created project/job/receipt side effects")
	}

	if f.calls.Load() != 0 {
		t.Fatal("refused admission called target")
	}
	if _, e := f.s.DeclareAgentCapabilities(ctx, f.token, *runner.EnvironmentAgentCapabilities()); e != nil {
		t.Fatal(e)
	}
	admitted, e := f.s.Submit(ctx, f.owner, f.project, f.request, id(), 1, options, false)
	if e != nil {
		t.Fatal(e)
	}
	work := f.grant(t)
	if work.SchemaVersion != 2 || work.TemplateSemanticsVersion != 1 || work.EnvironmentContext.EnvironmentID != env.EnvironmentID {
		t.Fatal("grant identity missing")
	}
	prepared, e := runner.PrepareLocal(ctx, work.Plan, work.ProtectedInputs.Values, work.Destination)
	if e != nil {
		t.Fatal(e)
	}
	evidence := NewEvidence(f.project, admitted.ExecutionID, work.DispatchIntentAt, work.Plan)
	prepared.Run(ctx, evidence)
	result, e := runner.ResultFromEvidence(*work, evidence)
	if e != nil {
		t.Fatal(e)
	}
	bad := result
	changed := *result.EnvironmentContext
	changed.Revision++
	bad.EnvironmentContext = &changed
	if _, e = f.s.UploadAgentResult(ctx, f.token, f.project, work.JobID, bad); e == nil {
		t.Fatal("mismatched environment result accepted")
	}
	receipt, e := f.s.UploadAgentResult(ctx, f.token, f.project, work.JobID, result)
	if e != nil {
		t.Fatal(e)
	}
	repeat, e := f.s.UploadAgentResult(ctx, f.token, f.project, work.JobID, result)
	if e != nil || repeat != receipt || f.calls.Load() != 1 {
		t.Fatal("receipt retry after input cleanup failed", e)
	}
	raw, stored := readAssertionEvidence(t, f.protocolFixture, admitted.ExecutionID)
	if stored.SchemaVersion != 3 || stored.EnvironmentContext.EnvironmentID != env.EnvironmentID || bytes.Contains(raw, []byte("environment-private-canary")) {
		t.Fatal("local environment identity/redaction lost")
	}
}

func TestIntegrationEnvironmentWorkflowSelectionFreezeAssertionsAndRerun(t *testing.T) {
	f, wf := workflowProtocolFixture(t)
	enableEnvironmentFixture(t, f)
	env, envRow, _ := fixtureEnvironment(t, f, "http://fixture.example/")
	ctx := context.Background()
	for i := range wf.Steps {
		wf.Steps[i].Extract = []runner.Extraction{}
		wf.Steps[i].Bindings = []runner.Binding{}
		wf.Steps[i].Assertions = []runner.Assertion{{AssertionID: id(), Kind: "statusEquals", Expected: json.RawMessage(`200`)}}
	}
	saved, e := f.s.SaveWorkflow(ctx, f.owner, f.project, wf.WorkflowID, wf.Revision, wf.WorkflowDefinition, false)
	if e != nil {
		t.Fatal(e)
	}
	seenNumber := "0.10000000000000001"
	f.s.prepare = func(ctx context.Context, p Plan, values map[string]string) (*Prepared, error) {
		return fixturePrepared(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			f.calls.Add(1)
			body, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(body), seenNumber) {
				t.Error("workflow public values changed after freeze")
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true}`))
		}), p, values), nil
	}
	options := RunOptions{Target: "cloud", AllowInsecureSecrets: true, Environment: &runner.EnvironmentContext{EnvironmentID: env.EnvironmentID, Revision: env.Revision}}
	key := id()
	run, e := f.s.SubmitWorkflow(ctx, f.owner, f.project, saved.WorkflowID, key, saved.Revision, options, false)
	if e != nil {
		t.Fatal(e)
	}
	env.Variables[0].Value.Decimal = "3"
	env.Revision++
	writeEnvironmentFixture(t, f, env, envRow)
	retry, e := f.s.SubmitWorkflow(ctx, f.owner, f.project, saved.WorkflowID, key, saved.Revision, options, false)
	if e != nil || retry.WorkflowRunID != run.WorkflowRunID {
		t.Fatal("workflow receipt refroze inputs", e)
	}
	for pos := 0; pos < 2; pos++ {
		if e = f.s.Process(ctx, workflowMessage(f, run, pos)); e != nil {
			t.Fatal(e)
		}
	}
	detail, e := f.s.RunDetail(ctx, f.owner, f.project, run.WorkflowRunID)
	if e != nil || detail.State != "completed" || f.calls.Load() != 2 {
		t.Fatal("workflow did not complete", e)
	}
	raw, evidence := readAssertionEvidence(t, f, *detail.Steps[0].ExecutionID)
	if evidence.SchemaVersion != 3 || evidence.Assertions == nil || !evidence.Assertions.Passed() || evidence.EnvironmentContext.Revision != 1 {
		t.Fatal("combined environment/assertion evidence invalid")
	}
	pair, _, e := f.s.CreateMCP(ctx, f.owner, id(), MCPPair{ProjectIDs: []string{f.project}})
	if e != nil {
		t.Fatal(e)
	}
	projection := invokeMCP(t, f, *pair.Token, "get_execution_evidence", map[string]any{"projectId": f.project, "executionId": *detail.Steps[0].ExecutionID, "view": "complete"})
	if projection.Error != nil || projection.Data.Evidence.OmissionReason == nil || *projection.Data.Evidence.OmissionReason != "unsupported_context_not_exposed" || len(projection.Data.Evidence.OmittedFields) != 3 {
		t.Fatal("combined context omission invalid")
	}
	after, _ := f.s.Evidence(ctx, f.owner, f.project, *detail.Steps[0].ExecutionID)
	if !bytes.Equal(raw, after) {
		t.Fatal("projection rewrote owner evidence")
	}
	rerun, e := f.s.SubmitWorkflow(ctx, f.owner, f.project, run.WorkflowRunID, id(), 0, RunOptions{Target: "cloud", AllowInsecureSecrets: true}, true)
	if e != nil {
		t.Fatal(e)
	}
	for pos := 0; pos < 2; pos++ {
		if e = f.s.Process(ctx, workflowMessage(f, rerun, pos)); e != nil {
			t.Fatal(e)
		}
	}
	if f.calls.Load() != 4 {
		t.Fatal("whole-run rerun did not issue exactly two new sends")
	}
	// A template target cannot also accept an explicit workflow output binding.
	bad := saved.WorkflowDefinition
	bad.Steps[0].Extract = []runner.Extraction{{Name: "value", Path: "$.value", Type: "string"}}
	bad.Steps[1].Bindings = []runner.Binding{{From: runner.Producer{StepID: bad.Steps[0].StepID, Name: "value"}, To: runner.BindingTarget{Kind: "header", Index: pointer(0)}}}
	if _, e = f.s.SaveWorkflow(ctx, f.owner, f.project, saved.WorkflowID, saved.Revision, bad, false); e == nil {
		t.Fatal("overlapping workflow/template destination accepted")
	}
}
