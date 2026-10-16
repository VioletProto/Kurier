//go:build integration

package execution

import (
	"context"
	"encoding/json"
	"errors"
	wire "github.com/VioletProto/Kurier/services/request-runtime/mcpwire"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dt "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func newMCPFixture(t *testing.T) (*protocolFixture, MCPPairResponse) {
	f := newProtocolFixture(t)
	stage, _ := f.s.stage(context.Background())
	stage.SavedRequestsSchemaVersion = 7
	stage.EvidencePinningSchemaVersion = 1
	stage.LocalAgentExecutionsSchemaVersion = 1
	stage.SequentialWorkflowsSchemaVersion = 1
	stage.MCPReadOnlySchemaVersion = 1
	stage.Version++
	f.write(t, f.s.Table, stage)
	p, _ := f.s.project(context.Background(), f.owner, f.project)
	p.PinnedCount = pointer(int64(0))
	p.PinnedBytes = pointer(int64(0))
	f.write(t, f.s.Table, p)
	r, _ := f.s.get(context.Background(), f.s.Table, p.PK, "REQ#"+f.request)
	r.LPK = p.PK + "#REQUEST"
	r.LSK = stamp(f.now) + "#" + f.request
	f.write(t, f.s.Table, r)
	paired, created, e := f.s.CreateMCP(context.Background(), f.owner, id(), MCPPair{ProjectIDs: []string{f.project}})
	if e != nil || !created || paired.Token == nil {
		t.Fatal("MCP fixture issuance failed", e)
	}
	return f, paired
}
func invokeMCP(t *testing.T, f *protocolFixture, token, tool string, in any) wire.Envelope {
	t.Helper()
	raw, _ := json.Marshal(in)
	env, deadline, e := f.s.MCPInvoke(context.Background(), token, tool, raw)
	if e != nil {
		t.Fatal("MCP invocation failed", e)
	}
	if deadline == "" {
		t.Fatal("missing release deadline")
	}
	if _, e = wire.Decode(tool, mcpRaw(env)); e != nil {
		t.Fatal("response schema violation")
	}
	return env
}
func TestIntegrationMCPSevenToolsAndContainment(t *testing.T) {
	f, p := newMCPFixture(t)
	ctx := context.Background()
	v, msg := f.admit(t, id())
	if e := f.s.Process(ctx, msg); e != nil {
		t.Fatal(e)
	}
	for _, tool := range []string{"list_projects", "list_saved_requests", "list_executions", "get_execution", "get_execution_evidence"} {
		in := map[string]any{}
		if tool != "list_projects" {
			in["projectId"] = f.project
		}
		if strings.HasPrefix(tool, "get_") {
			in["executionId"] = v.ExecutionID
		}
		if tool == "get_execution_evidence" {
			in["view"] = "complete"
		}
		env := invokeMCP(t, f, *p.Token, tool, in)
		if env.Error != nil {
			t.Fatal("expected successful read", tool, env.Error.Code)
		}
		raw := string(mcpRaw(env))
		for _, bad := range []string{"configuration", "secretRef", "objectKey", "checksum", "ciphertext", "wrappedKey", "localAgentId", "fixture-key"} {
			if strings.Contains(raw, bad) {
				t.Fatal("containment failure", tool)
			}
		}
	}
	for _, tool := range []string{"list_workflow_runs", "get_workflow_run"} {
		in := map[string]any{"projectId": f.project}
		if tool == "get_workflow_run" {
			in["workflowRunId"] = id()
		}
		env := invokeMCP(t, f, *p.Token, tool, in)
		if tool == "get_workflow_run" && env.Error == nil {
			t.Fatal("missing run disclosed")
		}
		f.now = f.now.Add(11 * time.Second)
	}
	env := invokeMCP(t, f, *p.Token, "get_execution", map[string]any{"projectId": id(), "executionId": v.ExecutionID})
	if env.Error == nil || env.Error.Code != "not_found" {
		t.Fatal("scope bypass")
	}
	if f.calls.Load() != 1 {
		t.Fatal("read caused HTTP dispatch")
	}
	page, e := f.db.Client.Query(ctx, &dynamodb.QueryInput{TableName: &f.s.Table, KeyConditionExpression: aws.String("PK = :p AND begins_with(SK,:a)"), ExpressionAttributeValues: map[string]dt.AttributeValue{":p": str("P#" + f.project), ":a": str("AUDIT#")}, ConsistentRead: aws.Bool(true)})
	if e != nil || len(page.Items) < 5 {
		t.Fatal("audit missing", e)
	}
}
func TestIntegrationMCPRunProjectionAndExpiredSibling(t *testing.T) {
	f, wf := workflowProtocolFixture(t)
	ctx := context.Background()
	stage, _ := f.s.stage(ctx)
	stage.SavedRequestsSchemaVersion = 7
	stage.MCPReadOnlySchemaVersion = 1
	stage.Version++
	f.write(t, f.s.Table, stage)
	p, _, e := f.s.CreateMCP(ctx, f.owner, id(), MCPPair{ProjectIDs: []string{f.project}})
	if e != nil {
		t.Fatal(e)
	}
	run := workflowRun(t, f, wf, id())
	for i := 0; i < 2; i++ {
		if e = f.s.Process(ctx, workflowMessage(f, run, i)); e != nil {
			t.Fatal(e)
		}
	}
	run, e = f.s.RunDetail(ctx, f.owner, f.project, run.WorkflowRunID)
	if e != nil {
		t.Fatal(e)
	}
	env := invokeMCP(t, f, *p.Token, "get_workflow_run", map[string]any{"projectId": f.project, "workflowRunId": run.WorkflowRunID})
	if env.Error != nil || len(env.Data.Run.Steps) != 2 || strings.Contains(string(mcpRaw(env)), "fixture-runtime-credential") || strings.Contains(string(mcpRaw(env)), "bindings") {
		t.Fatal("run projection violated containment")
	}
	list := invokeMCP(t, f, *p.Token, "list_workflow_runs", map[string]any{"projectId": f.project})
	if list.Error != nil || len(list.Data.Items) != 1 {
		t.Fatal("run discovery failed")
	}
	if _, e = f.s.Pin(ctx, f.owner, f.project, *run.Steps[0].ExecutionID, true); e != nil {
		t.Fatal(e)
	}
	f.now = f.now.Add(31 * 24 * time.Hour)
	if e = f.s.RevokeMCP(ctx, f.owner, p.Credential.TokenID); e != nil {
		t.Fatal(e)
	}
	fresh, _, e := f.s.CreateMCP(ctx, f.owner, id(), MCPPair{ProjectIDs: []string{f.project}})
	if e != nil {
		t.Fatal(e)
	}
	env = invokeMCP(t, f, *fresh.Token, "get_workflow_run", map[string]any{"projectId": f.project, "workflowRunId": run.WorkflowRunID})
	if env.Error != nil || !env.Data.Run.Steps[1].EvidenceExpired {
		t.Fatal("expired sibling revived")
	}
	env = invokeMCP(t, f, *fresh.Token, "get_execution_evidence", map[string]any{"projectId": f.project, "executionId": *run.Steps[1].ExecutionID})
	if env.Error == nil || env.Error.Code != "not_found" {
		t.Fatal("expired sibling disclosed")
	}
}
func TestIntegrationMCPIssuanceRevocationAndGeneration(t *testing.T) {
	f, p := newMCPFixture(t)
	ctx := context.Background()
	key := id()
	_, _, e := f.s.CreateMCP(ctx, f.owner, key, MCPPair{ProjectIDs: []string{f.project}})
	requireStatus(t, e, 409)
	if e = f.s.RevokeMCP(ctx, f.owner, p.Credential.TokenID); e != nil {
		t.Fatal(e)
	}
	if _, _, e = f.s.MCPInvoke(ctx, *p.Token, "list_projects", []byte(`{}`)); e == nil {
		t.Fatal("revoked token accepted")
	}
	replacement, created, e := f.s.CreateMCP(ctx, f.owner, key, MCPPair{ProjectIDs: []string{f.project}})
	if e != nil || !created || replacement.Token == nil {
		t.Fatal("replacement failed", e)
	}
	retry, created, e := f.s.CreateMCP(ctx, f.owner, key, MCPPair{ProjectIDs: []string{f.project}})
	if e != nil || created || retry.Token != nil || retry.Credential.TokenID != replacement.Credential.TokenID {
		t.Fatal("issuance retry revealed secret or changed identity")
	}
	stage, _ := f.s.stage(ctx)
	stage.RecoveryGeneration = "fixture-restored"
	stage.Version++
	f.write(t, f.s.Table, stage)
	if _, _, e = f.s.MCPInvoke(ctx, *replacement.Token, "list_projects", []byte(`{}`)); e == nil {
		t.Fatal("old generation token accepted")
	}
}
func TestIntegrationMCPReadAuditRaces(t *testing.T) {
	for _, change := range []string{"revoke", "project_delete", "disable", "unpin_expired", "audit_failure"} {
		t.Run(change, func(t *testing.T) {
			f, p := newMCPFixture(t)
			ctx := context.Background()
			v, msg := f.admit(t, id())
			if e := f.s.Process(ctx, msg); e != nil {
				t.Fatal(e)
			}
			var once sync.Once
			f.db.before = func(in *dynamodb.TransactWriteItemsInput) error {
				audit := false
				for _, a := range in.TransactItems {
					if a.Put != nil {
						if k, ok := a.Put.Item["kind"].(*dt.AttributeValueMemberS); ok && (k.Value == "executionMCPAudit" || k.Value == "mcpAudit") {
							audit = true
						}
					}
				}
				if !audit {
					return nil
				}
				if change == "audit_failure" {
					return errors.New("injected safe audit failure")
				}
				once.Do(func() {
					switch change {
					case "revoke":
						c, _ := f.s.get(ctx, f.s.Table, "U#"+f.owner, "MCPCRED#"+p.Credential.TokenID)
						c.Version++
						c.State = "revoked"
						c.RevokedAt = pointer(stamp(f.now))
						f.write(t, f.s.Table, c)
					case "project_delete":
						r, _ := f.s.get(ctx, f.s.Table, "P#"+f.project, "META")
						r.Version++
						r.State = "deleting"
						r.DeletionEpoch++
						f.write(t, f.s.Table, r)
					case "disable":
						r, _ := f.s.get(ctx, f.s.Table, "U#"+f.owner, "META")
						r.Version++
						r.Disabled = true
						f.write(t, f.s.Table, r)
					case "unpin_expired":
						r, _ := f.s.get(ctx, f.s.Table, "P#"+f.project, "EXEC#"+v.ExecutionID+"#RET")
						r.Version++
						r.NormalExpiresAt = stamp(f.now.Add(-time.Second))
						r.Pinned = false
						f.write(t, f.s.Table, r)
					}
				})
				return nil
			}
			raw := mcpRaw(map[string]any{"projectId": f.project, "executionId": v.ExecutionID, "view": "complete"})
			env, _, e := f.s.MCPInvoke(ctx, *p.Token, "get_execution_evidence", raw)
			if e == nil && env.Error == nil {
				t.Fatal("race released data")
			}
			if f.calls.Load() != 1 {
				t.Fatal("read replayed HTTP")
			}
		})
	}
}
func TestIntegrationMCPPaginationBoundsExpiryAndCleanup(t *testing.T) {
	f, p := newMCPFixture(t)
	ctx := context.Background()
	v, msg := f.admit(t, id())
	if e := f.s.Process(ctx, msg); e != nil {
		t.Fatal(e)
	}
	env := invokeMCP(t, f, *p.Token, "list_saved_requests", map[string]any{"projectId": f.project, "limit": 1})
	if env.Error != nil || len(env.Data.Items) != 1 {
		t.Fatal("request discovery failed")
	}
	f.now = f.now.Add(31 * 24 * time.Hour)
	_, _, e := f.s.MCPInvoke(ctx, *p.Token, "get_execution", mcpRaw(map[string]any{"projectId": f.project, "executionId": v.ExecutionID}))
	requireStatus(t, e, 401)
	f.now = f.now.Add(8 * 24 * time.Hour)
	c, _ := f.s.get(ctx, f.s.Table, "U#"+f.owner, "MCPCRED#"+p.Credential.TokenID)
	if e = f.s.cleanMCP(ctx, c); e != nil {
		t.Fatal(e)
	}
	c, _ = f.s.get(ctx, f.s.Table, c.PK, c.SK)
	if e = f.s.cleanMCP(ctx, c); e != nil {
		t.Fatal(e)
	}
	if _, e = f.s.get(ctx, f.s.Table, "P#"+f.project, "MCP#"+p.Credential.TokenID); !missingRow(e) {
		t.Fatal("scope link survived cleanup")
	}
	if _, e = f.s.get(ctx, f.s.Table, "MCPTOKEN#"+tokenHash(*p.Token), "META"); !missingRow(e) {
		t.Fatal("lookup survived cleanup")
	}
}

func TestIntegrationMCPSizeCursorRateAndNoResurrection(t *testing.T) {
	f, p := newMCPFixture(t)
	ctx := context.Background()
	req, _ := f.s.get(ctx, f.s.Table, "P#"+f.project, "REQ#"+f.request)
	second := req
	second.RequestID = id()
	second.SK = "REQ#" + second.RequestID
	second.LSK = stamp(f.now) + "#" + second.RequestID
	f.write(t, f.s.Table, second)
	page := invokeMCP(t, f, *p.Token, "list_saved_requests", map[string]any{"projectId": f.project, "limit": 1})
	if page.Error != nil || page.Data.NextCursor == nil {
		t.Fatal("bounded cursor missing")
	}
	next := invokeMCP(t, f, *p.Token, "list_saved_requests", map[string]any{"projectId": f.project, "limit": 1, "cursor": *page.Data.NextCursor})
	if next.Error != nil || len(next.Data.Items) != 1 || string(next.Data.Items[0]) == string(page.Data.Items[0]) {
		t.Fatal("cursor repeated or skipped item")
	}
	bad := invokeMCP(t, f, *p.Token, "list_executions", map[string]any{"projectId": f.project, "cursor": *page.Data.NextCursor})
	if bad.Error == nil || bad.Error.Code != "invalid_request" {
		t.Fatal("cross-tool cursor accepted")
	}
	f.s.prepare = func(ctx context.Context, plan Plan, values map[string]string) (*Prepared, error) {
		return fixturePrepared(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			f.calls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"public": strings.Repeat("<", 100*1024), "authorization": "response-credential-canary"})
		}), plan, values), nil
	}
	v, msg := f.admit(t, id())
	if e := f.s.Process(ctx, msg); e != nil {
		t.Fatal(e)
	}
	complete := invokeMCP(t, f, *p.Token, "get_execution_evidence", map[string]any{"projectId": f.project, "executionId": v.ExecutionID, "view": "complete"})
	if complete.Error == nil || complete.Error.Code != "evidence_too_large" {
		t.Fatal("large evidence not explicitly rejected")
	}
	summary := invokeMCP(t, f, *p.Token, "get_execution_evidence", map[string]any{"projectId": f.project, "executionId": v.ExecutionID})
	if summary.Error != nil || summary.Data.Evidence.Complete || summary.Data.Evidence.OmissionReason == nil || strings.Contains(string(mcpRaw(summary)), "response-credential-canary") {
		t.Fatal("summary mislabeled or unsafe")
	}
	before, _ := f.s.Evidence(ctx, f.owner, f.project, v.ExecutionID)
	f.now = f.now.Add(11 * time.Second)
	after, _ := f.s.Evidence(ctx, f.owner, f.project, v.ExecutionID)
	if string(before) != string(after) {
		t.Fatal("MCP changed immutable capture")
	}
	for n := 0; n < 10; n++ {
		env, _, e := f.s.MCPInvoke(ctx, *p.Token, "list_projects", []byte(`{}`))
		if e != nil || env.Error != nil {
			t.Fatal("premature burst limit")
		}
	}
	limited, _, e := f.s.MCPInvoke(ctx, *p.Token, "list_projects", []byte(`{}`))
	if e != nil || limited.Error == nil || limited.Error.Code != "rate_limited" {
		t.Fatal("burst limit not audited or enforced")
	}
	f.now = f.now.Add(time.Minute)
	if _, e = f.s.Pin(ctx, f.owner, f.project, v.ExecutionID, true); e != nil {
		t.Fatal(e)
	}
	ret, _ := f.s.get(ctx, f.s.Table, "P#"+f.project, "EXEC#"+v.ExecutionID+"#RET")
	ret.NormalExpiresAt = stamp(f.now.Add(-time.Second))
	ret.Version++
	f.write(t, f.s.Table, ret)
	if env := invokeMCP(t, f, *p.Token, "get_execution", map[string]any{"projectId": f.project, "executionId": v.ExecutionID}); env.Error != nil {
		t.Fatal("pin lost evidence")
	}
	if _, e = f.s.Pin(ctx, f.owner, f.project, v.ExecutionID, false); e != nil {
		t.Fatal(e)
	}
	if env := invokeMCP(t, f, *p.Token, "get_execution", map[string]any{"projectId": f.project, "executionId": v.ExecutionID}); env.Error == nil || env.Error.Code != "not_found" {
		t.Fatal("last unpin resurrected evidence")
	}
}
func TestIntegrationMCPUnavailableEvidenceCannotBeRecoveredFromObject(t *testing.T) {
	f, p := newMCPFixture(t)
	ctx := context.Background()
	v, msg := f.admit(t, id())
	if e := f.s.Process(ctx, msg); e != nil {
		t.Fatal(e)
	}
	ret, e := f.s.get(ctx, f.s.Table, "P#"+f.project, "EXEC#"+v.ExecutionID+"#RET")
	if e != nil {
		t.Fatal(e)
	}
	ret.EvidenceAvailability = "unavailable"
	ret.Version++
	f.write(t, f.s.Table, ret)
	for _, view := range []string{"summary", "complete"} {
		env := invokeMCP(t, f, *p.Token, "get_execution_evidence", map[string]any{"projectId": f.project, "executionId": v.ExecutionID, "view": view})
		if env.Data != nil || env.Error == nil || env.Error.Code != "evidence_unavailable" {
			t.Fatal("unavailable evidence resurrected from its surviving object")
		}
	}
}
func TestIntegrationMCPConcurrentIssuanceAndExpiryBoundary(t *testing.T) {
	f, p := newMCPFixture(t)
	ctx := context.Background()
	if e := f.s.RevokeMCP(ctx, f.owner, p.Credential.TokenID); e != nil {
		t.Fatal(e)
	}
	var successes atomic.Int32
	var wg sync.WaitGroup
	for n := 0; n < 2; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, created, e := f.s.CreateMCP(ctx, f.owner, id(), MCPPair{ProjectIDs: []string{f.project}})
			if e == nil && created {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatal("one MCP slot allowed concurrent issuances")
	}
	_, _, e := f.s.authenticateAgent(ctx, *p.Token)
	requireStatus(t, e, 401)
	f.now = f.now.Add(30*24*time.Hour - 5*time.Second)
	if _, _, e = f.s.authenticateMCP(ctx, *p.Token); e == nil {
		t.Fatal("expiry margin bypass")
	}
}

func TestIntegrationMCPScopeCleanupPreservesReplacementAndUnknownRecords(t *testing.T) {
	f, pair := newMCPFixture(t)
	ctx := context.Background()
	p, _ := f.s.get(ctx, f.s.Table, "P#"+f.project, "META")
	p.State = "deleting"
	p.Version++
	p.DeletionEpoch++
	f.write(t, f.s.Table, p)
	link, _ := f.s.get(ctx, f.s.Table, p.PK, "MCP#"+pair.Credential.TokenID)
	if e := f.s.removeMCPScope(ctx, p, link); e != nil {
		t.Fatal(e)
	}
	credential, e := f.s.CurrentMCP(ctx, f.owner)
	if e != nil || credential == nil || len(credential.ProjectIDs) != 0 || credential.RevokedAt == nil {
		t.Fatal("deleted final scope did not revoke")
	}
	other := row{PK: "P#" + id(), SK: "META", Kind: "project", SchemaVersion: 1, OwnerID: f.owner, State: "active", PinnedCount: pointer(int64(0)), PinnedBytes: pointer(int64(0))}
	other.ProjectID = strings.TrimPrefix(other.PK, "P#")
	f.write(t, f.s.Table, other)
	replacement, _, e := f.s.CreateMCP(ctx, f.owner, id(), MCPPair{ProjectIDs: []string{other.ProjectID}})
	if e != nil {
		t.Fatal(e)
	}
	f.now = f.now.Add(8 * 24 * time.Hour)
	old, e := f.s.get(ctx, f.s.Table, "U#"+f.owner, "MCPCRED#"+pair.Credential.TokenID)
	if e != nil {
		t.Fatal(e)
	}
	if e = f.s.cleanMCP(ctx, old); e != nil {
		t.Fatal(e)
	}
	current, e := f.s.CurrentMCP(ctx, f.owner)
	if e != nil || current == nil || current.TokenID != replacement.Credential.TokenID {
		t.Fatal("old cleanup removed replacement")
	}
	_, e = f.db.Client.UpdateItem(ctx, &dynamodb.UpdateItemInput{TableName: &f.s.Table, Key: keys("U#"+f.owner, "MCPCRED#"+replacement.Credential.TokenID), UpdateExpression: aws.String("SET futureField = :yes"), ExpressionAttributeValues: map[string]dt.AttributeValue{":yes": &dt.AttributeValueMemberBOOL{Value: true}}})
	if e != nil {
		t.Fatal(e)
	}
	if e = f.s.RevokeMCP(ctx, f.owner, replacement.Credential.TokenID); e == nil {
		t.Fatal("unknown credential fields overwritten")
	}
	read, e := f.db.Client.GetItem(ctx, &dynamodb.GetItemInput{TableName: &f.s.Table, Key: keys("U#"+f.owner, "MCPCRED#"+replacement.Credential.TokenID), ConsistentRead: aws.Bool(true)})
	if e != nil || read.Item["futureField"] == nil {
		t.Fatal("unknown field lost")
	}
}
