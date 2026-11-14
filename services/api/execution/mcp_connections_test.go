package execution

import (
	"encoding/json"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"testing"
)

func TestMCPStrictScopeUnion(t *testing.T) {
	for _, raw := range []string{`null`, `{}`, `{"mode":"all_owned_projects","projectIds":[]}`, `{"mode":"all_owned_projects","projectIds":null}`, `{"mode":"all_owned_projects","extra":true}`, `{"mode":"all_owned_projects","mode":"selected_projects"}`, `{"mode":"selected_projects"}`, `{"mode":"selected_projects","projectIds":null}`, `{"mode":"selected_projects","projectIds":[]}`, `{"mode":"all"}`} {
		var s MCPScope
		if json.Unmarshal([]byte(raw), &s) == nil {
			t.Fatal("invalid scope accepted", raw)
		}
	}
	for _, raw := range []string{`{"mode":"all_owned_projects"}`, `{"mode":"selected_projects","projectIds":["00000000-0000-4000-8000-000000000001"]}`} {
		var s MCPScope
		if json.Unmarshal([]byte(raw), &s) != nil {
			t.Fatal("valid scope rejected")
		}
	}
	ids := []string{}
	for n := 0; n < 11; n++ {
		ids = append(ids, id())
	}
	if !(MCPScope{Mode: "selected_projects", ProjectIDs: ids[:10]}).valid() || (MCPScope{Mode: "selected_projects", ProjectIDs: ids}).valid() || (MCPScope{Mode: "selected_projects", ProjectIDs: []string{ids[0], ids[0]}}).valid() {
		t.Fatal("scope bound or duplicate")
	}
}
func TestMCPCredentialSchemaIsolation(t *testing.T) {
	c := row{PK: "U#" + id(), Kind: "mcpCredential", SchemaVersion: 2, MCPTokenID: id(), MCPScopeMode: "all_owned_projects", TokenHash: tokenHash("fixture"), State: "active", ExpiresAt: "2026-12-01T00:00:00Z", RecoveryGeneration: "fixture"}
	c.OwnerID = c.PK[2:]
	c.SK = "MCPCRED#" + c.MCPTokenID
	check := func(want bool) {
		t.Helper()
		m, e := attributevalue.MarshalMap(c)
		if e != nil || knownMCP(m, c) != want {
			t.Fatal("credential schema isolation")
		}
	}
	check(true)
	c.SchemaVersion = 1
	check(false)
	c.SchemaVersion = 2
	c.MCPProjectIDs = []string{id()}
	check(false)
	c.MCPScopeMode = "selected_projects"
	check(true)
	c.MCPProjectIDs = nil
	check(false)
	c.State = "revoked"
	check(true)
}
