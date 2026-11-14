//go:build integration

package execution

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dt "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"testing"
	"time"
)

func scopesFixture(t *testing.T) *protocolFixture {
	f := newProtocolFixture(t)
	enableEnvironmentFixture(t, f)
	stage, e := f.s.stage(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	stage.SavedRequestsSchemaVersion = 11
	stage.OpenAPISchemaVersion = 1
	stage.MCPReadOnlySchemaVersion = 2
	stage.Version++
	f.write(t, f.s.Table, stage)
	return f
}
func TestIntegrationMCPConnectionDiscoveryAndLiveNarrowing(t *testing.T) {
	f := scopesFixture(t)
	ctx := context.Background()
	pair, created, e := f.s.CreateMCPConnection(ctx, f.owner, id(), MCPScope{Mode: "all_owned_projects"})
	if e != nil || !created || pair.Token == nil {
		t.Fatal("automatic issuance", e)
	}
	token := *pair.Token
	invoke := func(tool string, in any) []byte {
		t.Helper()
		f.now = f.now.Add(11 * time.Second)
		env := invokeMCP(t, f, token, tool, in)
		if env.Error != nil {
			t.Fatal("read rejected", env.Error.Code)
		}
		return mcpRaw(env)
	}
	empty := invoke("list_projects", map[string]any{})
	var page struct {
		Data struct {
			Items []struct {
				ProjectID string `json:"projectId"`
			} `json:"items"`
			NextCursor *string `json:"nextCursor"`
		} `json:"data"`
	}
	json.Unmarshal(empty, &page)
	if len(page.Data.Items) != 0 {
		t.Fatal("empty account")
	}
	ids := []string{}
	for n := 0; n < 12; n++ {
		pid := id()
		ids = append(ids, pid)
		p := row{PK: "P#" + pid, SK: "META", Kind: "project", SchemaVersion: 1, ProjectID: pid, OwnerID: f.owner, Name: "scope fixture", State: "active", CreatedAt: stamp(f.now), UpdatedAt: stamp(f.now), LPK: "U#" + f.owner + "#PROJECT", LSK: stamp(f.now) + "#" + pid}
		f.write(t, f.s.Table, p)
	}
	var stale string
	seen := map[string]bool{}
	cursor := ""
	for n := 0; n < 8; n++ {
		in := map[string]any{"limit": 3}
		if cursor != "" {
			in["cursor"] = cursor
		}
		raw := invoke("list_projects", in)
		json.Unmarshal(raw, &page)
		for _, p := range page.Data.Items {
			if seen[p.ProjectID] {
				t.Fatal("duplicate project")
			}
			seen[p.ProjectID] = true
		}
		if page.Data.NextCursor == nil {
			break
		}
		cursor = *page.Data.NextCursor
		stale = cursor
	}
	if len(seen) != 12 || stale == "" {
		t.Fatal("pagination lost projects", len(seen))
	}
	// Same token, no restart, no expiry change, old cursors fenced.
	updated, e := f.s.UpdateMCPScope(ctx, f.owner, pair.Connection.TokenID, 0, MCPScope{Mode: "selected_projects", ProjectIDs: ids[:1]})
	if e != nil || updated.Version != 1 || updated.ExpiresAt != pair.Connection.ExpiresAt {
		t.Fatal("scope edit", e)
	}
	if _, e = f.s.UpdateMCPScope(ctx, f.owner, pair.Connection.TokenID, 0, MCPScope{Mode: "all_owned_projects"}); e == nil {
		t.Fatal("stale edit accepted")
	}
	f.now = f.now.Add(11 * time.Second)
	env := invokeMCP(t, f, token, "list_projects", map[string]any{"cursor": stale})
	if env.Error == nil || env.Error.Code != "invalid_request" {
		t.Fatal("old cursor survived")
	}
	raw := invoke("list_projects", map[string]any{})
	json.Unmarshal(raw, &page)
	if len(page.Data.Items) != 1 || page.Data.Items[0].ProjectID != ids[0] {
		t.Fatal("narrowing failed")
	}
	f.now = f.now.Add(11 * time.Second)
	env = invokeMCP(t, f, token, "list_saved_requests", map[string]any{"projectId": ids[1]})
	if env.Error == nil || env.Error.Code != "not_found" {
		t.Fatal("out of scope")
	}
	same, e := f.s.UpdateMCPScope(ctx, f.owner, pair.Connection.TokenID, 1, updated.Scope)
	if e != nil || same.Version != 1 {
		t.Fatal("no-op version")
	}
	if _, e = f.s.CurrentMCP(ctx, f.owner); e == nil {
		t.Fatal("legacy status misrepresented scope")
	}
	// Removed explicit link, and future-project consent survives all deletions.
	all, e := f.s.UpdateMCPScope(ctx, f.owner, pair.Connection.TokenID, 1, MCPScope{Mode: "all_owned_projects"})
	if e != nil || all.Version != 2 {
		t.Fatal("automatic switch", e)
	}
	if _, e = f.s.get(ctx, f.s.Table, "P#"+ids[0], "MCP#"+pair.Connection.TokenID); !missingRow(e) {
		t.Fatal("scope link retained")
	}
	for _, pid := range ids {
		p, e := f.s.project(ctx, f.owner, pid)
		if e != nil {
			t.Fatal(e)
		}
		p.State = "deleting"
		p.Version++
		f.write(t, f.s.Table, p)
	}
	raw = invoke("list_projects", map[string]any{})
	json.Unmarshal(raw, &page)
	if len(page.Data.Items) != 0 {
		t.Fatal("deleting projects disclosed")
	}
	current, e := f.s.CurrentMCPConnection(ctx, f.owner)
	if e != nil || current.RevokedAt != nil || current.Version != 2 {
		t.Fatal("automatic consent revoked by deletion")
	}
	if e = f.s.RevokeMCP(ctx, f.owner, pair.Connection.TokenID); e != nil {
		t.Fatal(e)
	}
	f.now = f.now.Add(11 * time.Second)
	_, _, e = f.s.MCPInvoke(ctx, token, "list_projects", []byte(`{}`))
	if e == nil {
		t.Fatal("revoked credential survived")
	}
}
func TestIntegrationMCPConnectionReceiptsAndCleanup(t *testing.T) {
	f := scopesFixture(t)
	ctx := context.Background()
	key := id()
	scope := MCPScope{Mode: "selected_projects", ProjectIDs: []string{f.project}}
	pair, _, e := f.s.CreateMCPConnection(ctx, f.owner, key, scope)
	if e != nil {
		t.Fatal(e)
	}
	retry, created, e := f.s.CreateMCPConnection(ctx, f.owner, key, scope)
	if e != nil || created || retry.Token != nil {
		t.Fatal("receipt leaked token", e)
	}
	if _, _, e = f.s.CreateMCP(ctx, f.owner, id(), MCPPair{ProjectIDs: []string{f.project}}); e == nil {
		t.Fatal("cross-route slot bypass")
	}
	if _, e = f.s.UpdateMCPScope(ctx, f.owner, pair.Connection.TokenID, 0, MCPScope{Mode: "selected_projects", ProjectIDs: []string{id()}}); e == nil {
		t.Fatal("foreign project grant")
	}
	p, _ := f.s.project(ctx, f.owner, f.project)
	p.State = "deleting"
	p.Version++
	f.write(t, f.s.Table, p)
	link, e := f.s.get(ctx, f.s.Table, p.PK, "MCP#"+pair.Connection.TokenID)
	if e != nil {
		t.Fatal(e)
	}
	if e = f.s.removeMCPScope(ctx, p, link); e != nil {
		t.Fatal(e)
	}
	c, e := f.s.CurrentMCPConnection(ctx, f.owner)
	if e != nil || c.RevokedAt == nil || c.Version != 1 {
		t.Fatal("last explicit deletion did not revoke", e)
	}
	// Account-level audits must contain no request values or verifier.
	out, e := f.db.Client.Query(ctx, &dynamodb.QueryInput{TableName: &f.s.Table, KeyConditionExpression: aws.String("PK = :p AND begins_with(SK,:a)"), ExpressionAttributeValues: map[string]dt.AttributeValue{":p": str("U#" + f.owner), ":a": str("AUDIT#")}, ConsistentRead: aws.Bool(true)})
	if e != nil {
		t.Fatal(e)
	}
	_ = out
}

func TestIntegrationMCPConnectionReadRaces(t *testing.T) {
	for _, change := range []string{"narrow", "revoke", "delete", "disable", "generation", "audit_failure"} {
		t.Run(change, func(t *testing.T) {
			f := scopesFixture(t)
			ctx := context.Background()
			pair, _, e := f.s.CreateMCPConnection(ctx, f.owner, id(), MCPScope{Mode: "all_owned_projects"})
			if e != nil {
				t.Fatal(e)
			}
			changed := false
			f.db.before = func(in *dynamodb.TransactWriteItemsInput) error {
				if changed {
					return nil
				}
				audit := false
				for _, a := range in.TransactItems {
					if a.Put != nil {
						if k, ok := a.Put.Item["kind"].(*dt.AttributeValueMemberS); ok && (k.Value == "mcpAudit" || k.Value == "executionMCPAudit") {
							audit = true
						}
					}
				}
				if !audit {
					return nil
				}
				changed = true
				switch change {
				case "narrow":
					pid := id()
					p := row{PK: "P#" + pid, SK: "META", Kind: "project", SchemaVersion: 1, ProjectID: pid, OwnerID: f.owner, State: "active", CreatedAt: stamp(f.now)}
					f.write(t, f.s.Table, p)
					if _, err := f.s.UpdateMCPScope(ctx, f.owner, pair.Connection.TokenID, 0, MCPScope{Mode: "selected_projects", ProjectIDs: []string{pid}}); err != nil {
						t.Fatal(err)
					}
				case "revoke":
					if err := f.s.RevokeMCP(ctx, f.owner, pair.Connection.TokenID); err != nil {
						t.Fatal(err)
					}
				case "delete":
					p, _ := f.s.project(ctx, f.owner, f.project)
					p.State = "deleting"
					p.Version++
					f.write(t, f.s.Table, p)
				case "disable":
					u, _ := f.s.get(ctx, f.s.Table, "U#"+f.owner, "META")
					u.Disabled = true
					u.Version++
					f.write(t, f.s.Table, u)
				case "generation":
					s, _ := f.s.stage(ctx)
					s.RecoveryGeneration = "next"
					s.Version++
					f.write(t, f.s.Table, s)
				case "audit_failure":
					return errors.New("fixture audit failure")
				}
				return nil
			}
			env, _, e := f.s.MCPInvoke(ctx, *pair.Token, "list_saved_requests", mcpRaw(map[string]any{"projectId": f.project}))
			if e == nil && env.Error == nil {
				t.Fatal("authority race released data")
			}
		})
	}
}
