//go:build integration

package ownership

import (
	"context"
	"encoding/json"
	"github.com/VioletProto/Kurier/services/api/execution"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIntegrationMCPConnectionHTTP(t *testing.T) {
	i, p := environmentLocal(t)
	ctx := context.Background()
	stage, _ := i.store.get(ctx, "STAGE#local", "META")
	stage.SavedRequestsSchemaVersion = 11
	stage.MCPReadOnlySchemaVersion = 2
	stage.OpenAPISchemaVersion = 1
	stage.Version++
	item, _ := encode(stage)
	if _, e := i.client.PutItem(ctx, &dynamodb.PutItemInput{TableName: &i.store.table, Item: item}); e != nil {
		t.Fatal(e)
	}
	owner := i.auth.token(t, "alice", nil)
	call := func(method, path, body, key, tag string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+owner)
		r.Header.Set("Content-Type", "application/json")
		if key != "" {
			r.Header.Set("Idempotency-Key", key)
		}
		if tag != "" {
			r.Header.Set("If-Match", tag)
		}
		w := httptest.NewRecorder()
		i.server.ServeHTTP(w, r)
		return w
	}
	for _, body := range []string{`null`, `{}`, `{"scope":null}`, `{"scope":{"mode":"all_owned_projects","projectIds":[]}}`, `{"scope":{"mode":"all_owned_projects"},"extra":true}`} {
		if w := call("POST", "/api/v1/mcp/connections", body, newID(), ""); w.Code != 400 {
			t.Fatal("invalid union", w.Code)
		}
	}
	response := call("POST", "/api/v1/mcp/connections", `{"scope":{"mode":"all_owned_projects"}}`, newID(), "")
	if response.Code != 201 {
		t.Fatal("create", response.Code)
	}
	var pair struct {
		Data execution.MCPConnectionPair `json:"data"`
	}
	if json.Unmarshal(response.Body.Bytes(), &pair) != nil || pair.Data.Token == nil {
		t.Fatal("one-time response")
	}
	path := "/api/v1/mcp/connections/" + pair.Data.Connection.TokenID + "/scope"
	body := `{"scope":{"mode":"selected_projects","projectIds":["` + p.ProjectID + `"]}}`
	for _, tc := range []struct {
		tag    string
		status int
	}{{"", 428}, {`W/"0"`, 400}, {`"1"`, 412}, {`"0"`, 200}} {
		if w := call("PATCH", path, body, "", tc.tag); w.Code != tc.status {
			t.Fatal("CAS", tc.tag, w.Code)
		}
	}
	current := call("GET", "/api/v1/mcp/connections/current", "", "", "")
	if current.Code != 200 || current.Header().Get("ETag") != `"1"` || strings.Contains(current.Body.String(), *pair.Data.Token) {
		t.Fatal("safe current metadata")
	}
	if w := call("GET", "/api/v1/mcp/tokens/current", "", "", ""); w.Code != 409 {
		t.Fatal("legacy misrepresentation")
	}
	if w := call("DELETE", "/api/v1/mcp/tokens/"+pair.Data.Connection.TokenID, "", "", ""); w.Code != 204 {
		t.Fatal("revoke compatibility")
	}
}
