//go:build integration

package ownership

import (
	"context"
	"encoding/json"
	wire "github.com/VioletProto/Kurier/services/request-runtime/mcpwire"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIntegrationMCPHTTPManagementAndCleanup(t *testing.T) {
	i, p, _ := workflowLocal(t)
	ctx := context.Background()
	stage, _ := i.store.get(ctx, "STAGE#local", "META")
	stage.SavedRequestsSchemaVersion = 7
	stage.MCPReadOnlySchemaVersion = 1
	stage.Version++
	item, _ := encode(stage)
	if _, e := i.client.PutItem(ctx, &dynamodb.PutItemInput{TableName: &i.store.table, Item: item}); e != nil {
		t.Fatal(e)
	}
	call := func(method, path, token, body, key string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer "+token)
		if key != "" {
			r.Header.Set("Idempotency-Key", key)
		}
		w := httptest.NewRecorder()
		i.server.ServeHTTP(w, r)
		return w
	}
	owner := i.auth.token(t, "alice", nil)
	foreign := i.auth.token(t, "bob", nil)
	key := newID()
	body, _ := json.Marshal(map[string]any{"projectIds": []string{p.ProjectID}})
	response := call("POST", "/api/v1/mcp/tokens", owner, string(body), key)
	if response.Code != 201 {
		t.Fatal("issuance failed", response.Code)
	}
	var pair struct {
		Data struct {
			Credential wire.Credential `json:"credential"`
			Token      string          `json:"token"`
		} `json:"data"`
	}
	if json.Unmarshal(response.Body.Bytes(), &pair) != nil || pair.Data.Token == "" {
		t.Fatal("invalid one-time response")
	}
	for _, c := range []struct {
		method, path, token, body string
		want                      int
	}{{"GET", "/api/v1/users/me", pair.Data.Token, "", 401}, {"GET", "/api/v1/local-agent/session", pair.Data.Token, "", 401}, {"POST", "/api/v1/mcp/invocations", owner, `{"tool":"list_projects","input":{}}`, 401}, {"GET", "/api/v1/mcp/tokens/current", owner, "", 200}, {"DELETE", "/api/v1/mcp/tokens/" + pair.Data.Credential.TokenID, foreign, "", 404}, {"POST", "/api/v1/mcp/invocations", pair.Data.Token, `{"tool":"list_projects","input":{}}`, 200}} {
		w := call(c.method, c.path, c.token, c.body, "")
		if w.Code != c.want {
			t.Fatal("auth separation failure", c.path, w.Code)
		}
	}
	if r := call("POST", "/api/v1/mcp/tokens", owner, string(body), key); r.Code != 200 || strings.Contains(r.Body.String(), pair.Data.Token) {
		t.Fatal("issuance retry leaked capability")
	}
	project, e := i.store.GetProject(ctx, i.alice.UserID, p.ProjectID)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = i.store.DeleteProject(ctx, i.alice.UserID, p.ProjectID, project.Version); e != nil {
		t.Fatal(e)
	}
	read := call("POST", "/api/v1/mcp/invocations", pair.Data.Token, `{"tool":"list_saved_requests","input":{"projectId":"`+p.ProjectID+`"}}`, "")
	if read.Code != 404 {
		t.Fatal("project deletion did not deny read")
	}
	for n := 0; n < 30; n++ {
		_, _ = i.store.CleanupProject(ctx, p.ProjectID)
	}
	r, e := i.store.get(ctx, "P#"+p.ProjectID, "META")
	if e != nil || r.State != "deleted" {
		t.Fatal("MCP records blocked cleanup completion", e)
	}
	current := call("GET", "/api/v1/mcp/tokens/current", owner, "", "")
	if current.Code != 200 || !strings.Contains(current.Body.String(), "revokedAt") {
		t.Fatal("scope cleanup did not retain safe metadata")
	}
}
