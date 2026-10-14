//go:build integration

package ownership

import (
	"context"
	"encoding/json"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dt "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIntegrationAgentHTTPAuthenticationSeparation(t *testing.T) {
	i, p, _, _ := pinLocal(t)
	ctx := context.Background()
	stage, _ := i.store.get(ctx, "STAGE#local", "META")
	stage.SavedRequestsSchemaVersion = 5
	stage.LocalAgentExecutionsSchemaVersion = 1
	stage.Version++
	item, _ := encode(stage)
	_, e := i.client.PutItem(ctx, &dynamodb.PutItemInput{TableName: &i.store.table, Item: item})
	if e != nil {
		t.Fatal(e)
	}
	body, _ := json.Marshal(map[string]any{"name": "HTTP fixture", "scopes": []any{map[string]any{"projectId": p.ProjectID, "destinations": []any{map[string]string{"origin": "http://localhost:3000", "addressClass": "loopback"}}}}})
	r := httptest.NewRequest("POST", "/api/v1/local-agents", strings.NewReader(string(body)))
	r.Header.Set("Authorization", "Bearer "+i.auth.token(t, "alice", nil))
	r.Header.Set("Idempotency-Key", newID())
	response := httptest.NewRecorder()
	i.server.ServeHTTP(response, r)
	if response.Code != 201 {
		t.Fatal("pairing HTTP status", response.Code)
	}
	var pair struct {
		Data struct {
			Token string `json:"token"`
			Agent struct {
				LocalAgentID string `json:"localAgentId"`
			} `json:"agent"`
		} `json:"data"`
	}
	if json.Unmarshal(response.Body.Bytes(), &pair) != nil || pair.Data.Token == "" {
		t.Fatal("pairing response invalid")
	}
	for _, check := range []struct {
		method, path, token string
		status              int
	}{{"GET", "/api/v1/users/me", pair.Data.Token, 401}, {"GET", "/api/v1/local-agent/session", i.auth.token(t, "alice", nil), 401}, {"GET", "/api/v1/local-agent/session", pair.Data.Token, 200}, {"POST", "/api/v1/projects/" + p.ProjectID + "/local-agent/poll", i.auth.token(t, "alice", nil), 401}} {
		req := httptest.NewRequest(check.method, check.path, nil)
		req.Header.Set("Authorization", "Bearer "+check.token)
		record := httptest.NewRecorder()
		i.server.ServeHTTP(record, req)
		if record.Code != check.status {
			t.Fatal("authentication separation status", record.Code)
		}
	}
	foreign := i.request(t, "bob", "DELETE", "/api/v1/local-agents/"+pair.Data.Agent.LocalAgentID, "", "")
	if foreign.Code != 404 {
		t.Fatal("foreign revoke status", foreign.Code)
	}
	// Unknown metadata remains intact and prevents an old/partial reader's CAS.
	_, e = i.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{TableName: &i.store.table, Key: key("U#"+i.alice.UserID, "CRED#"+pair.Data.Agent.LocalAgentID), UpdateExpression: aws.String("SET futureField = :yes"), ExpressionAttributeValues: map[string]dt.AttributeValue{":yes": &dt.AttributeValueMemberBOOL{Value: true}}})
	if e != nil {
		t.Fatal(e)
	}
	revoked := i.request(t, "alice", "DELETE", "/api/v1/local-agents/"+pair.Data.Agent.LocalAgentID, "", "")
	if revoked.Code != 503 {
		t.Fatal("unknown credential overwritten", revoked.Code)
	}
	read, e := i.client.GetItem(ctx, &dynamodb.GetItemInput{TableName: &i.store.table, Key: key("U#"+i.alice.UserID, "CRED#"+pair.Data.Agent.LocalAgentID), ConsistentRead: aws.Bool(true)})
	if e != nil || read.Item["futureField"] == nil {
		t.Fatal("unknown field erased")
	}
}
