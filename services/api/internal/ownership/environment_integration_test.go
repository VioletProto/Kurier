//go:build integration

package ownership

import (
	"context"
	"encoding/json"
	runner "github.com/VioletProto/Kurier/services/request-runtime"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"strconv"
	"strings"
	"testing"
)

func environmentLocal(t *testing.T) (*integration, Project) {
	t.Helper()
	i, p, _ := workflowLocal(t)
	stage, e := i.store.get(context.Background(), "STAGE#local", "META")
	if e != nil {
		t.Fatal(e)
	}
	stage.SavedRequestsSchemaVersion = 9
	stage.ProjectEnvironmentsSchemaVersion = 1
	stage.ProtectedSecretsSchemaVersion = 2
	stage.LocalAgentExecutionsSchemaVersion = 2
	stage.MCPReadOnlySchemaVersion = 1
	stage.WorkflowAssertionsSchemaVersion = 1
	stage.Version++
	i.store.cipher.kms = workflowFixtureKMS{}
	item, _ := encode(stage)
	if _, e = i.client.PutItem(context.Background(), &dynamodb.PutItemInput{TableName: &i.store.table, Item: item}); e != nil {
		t.Fatal(e)
	}
	return i, p
}
func environmentResponse(t *testing.T, raw []byte) runner.Environment {
	t.Helper()
	var v struct {
		Data struct {
			Environment runner.Environment `json:"environment"`
		} `json:"data"`
	}
	if runner.StrictJSON(raw, &v) != nil {
		t.Fatal("invalid environment response")
	}
	return v.Data.Environment
}
func TestIntegrationEnvironmentCRUDTypedSecretsFreezingAndCleanup(t *testing.T) {
	i, p := environmentLocal(t)
	ctx := context.Background()
	path := "/api/v1/projects/" + p.ProjectID + "/environments"
	body := `{"name":"Development","variables":[{"name":"n","enabled":true,"sensitive":false,"value":{"type":"number","decimal":"0.10000000000000001"}},{"name":"token","enabled":true,"sensitive":true,"valueType":"string","secretWrite":{"action":"set","value":{"type":"string","value":"environment-synthetic-private-canary"}}},{"name":"quota","enabled":true,"sensitive":true,"valueType":"number","secretWrite":{"action":"set","value":{"type":"number","decimal":"1.0000000000000001"}}}]}`
	response := i.request(t, "alice", "POST", path, body, "")
	if response.Code != 201 {
		t.Fatalf("create=%d", response.Code)
	}
	env := environmentResponse(t, response.Body.Bytes())
	path += "/" + env.EnvironmentID
	if env.Revision != 0 || len(env.Variables) != 3 || response.Header().Get("ETag") != `"0"` || strings.Contains(response.Body.String(), "environment-synthetic-private-canary") {
		t.Fatal("unsafe or invalid environment response")
	}
	for _, who := range []string{"bob", "missing-user"} {
		if r := i.request(t, who, "GET", path, "", ""); r.Code != 404 {
			t.Fatalf("foreign read=%d", r.Code)
		}
	}
	if r := i.request(t, "alice", "PATCH", path, `{"name":"Renamed"}`, ""); r.Code != 428 {
		t.Fatal("missing If-Match accepted")
	}
	if r := i.request(t, "alice", "PATCH", path, `{"name":"Renamed"}`, `"9"`); r.Code != 412 {
		t.Fatal("stale If-Match accepted")
	}
	secret := env.Variables[2].SecretRef.SecretID
	secretPath := "/api/v1/projects/" + p.ProjectID + "/secrets/" + secret
	if r := i.request(t, "alice", "PATCH", secretPath, `{"value":"true"}`, `"0"`); r.Code != 400 {
		t.Fatalf("typed number changed to boolean=%d", r.Code)
	}
	if r := i.request(t, "alice", "PATCH", secretPath, `{"value":"1.0000000000000002"}`, `"0"`); r.Code != 200 {
		t.Fatalf("typed PATCH=%d", r.Code)
	}
	requestBody := `{"name":"Template","method":"POST","url":"https://example.test/","templateSemanticsVersion":1,"headers":[{"name":"Authorization","value":"Bearer {{token}}","enabled":true,"sensitive":false}],"body":{"type":"json","text":"{\"n\":\"{{n}}\"}","sensitive":false}}`
	created := i.request(t, "alice", "POST", "/api/v1/projects/"+p.ProjectID+"/requests", requestBody, "")
	if created.Code != 201 {
		t.Fatalf("template request=%d", created.Code)
	}
	var saved struct {
		Data struct {
			Request SavedRequest `json:"request"`
		} `json:"data"`
	}
	if json.Unmarshal(created.Body.Bytes(), &saved) != nil {
		t.Fatal("invalid request response")
	}
	options, _ := runner.ParseOptions([]byte(`{"environment":{"environmentId":"` + env.EnvironmentID + `","revision":0}}`))
	key := newID()
	execution, e := i.store.executions.Submit(ctx, i.alice.UserID, p.ProjectID, saved.Data.Request.RequestID, key, 0, options, false)
	if e != nil {
		t.Fatal(e)
	}
	renamed := i.request(t, "alice", "PATCH", path, `{"name":"Renamed"}`, `"0"`)
	if renamed.Code != 200 {
		t.Fatalf("rename=%d", renamed.Code)
	}
	retry, e := i.store.executions.Submit(ctx, i.alice.UserID, p.ProjectID, saved.Data.Request.RequestID, key, 0, options, false)
	if e != nil || retry.ExecutionID != execution.ExecutionID {
		t.Fatal("identical receipt rechecked newer environment revision", e)
	}
	if _, e = i.store.executions.Submit(ctx, i.alice.UserID, p.ProjectID, saved.Data.Request.RequestID, newID(), 0, options, false); e == nil {
		t.Fatal("fresh stale selection accepted")
	}
	if r := i.request(t, "alice", "GET", "/api/v1/projects/"+p.ProjectID+"/environments", "", ""); r.Code != 200 || strings.Contains(r.Body.String(), "variables") || strings.Contains(r.Body.String(), "secretRef") {
		t.Fatal("list disclosed variable details")
	}
	// Existing variable identity cannot be renamed or reused after removal.
	replacement := `{"variables":[{"variableId":"` + env.Variables[0].VariableID + `","name":"other","enabled":true,"sensitive":false,"value":{"type":"number","decimal":"2"}}]}`
	if r := i.request(t, "alice", "PATCH", path, replacement, `"1"`); r.Code != 400 {
		t.Fatal("renamed variable identity accepted")
	}
	if r := i.request(t, "alice", "DELETE", path, "", `"1"`); r.Code != 204 {
		t.Fatalf("delete=%d", r.Code)
	}
	if r := i.request(t, "alice", "DELETE", path, "", `"1"`); r.Code != 204 {
		t.Fatal("identical delete retry rejected")
	}
	if r := i.request(t, "alice", "GET", path, "", ""); r.Code != 404 {
		t.Fatal("deleted environment visible")
	}
	metadata, e := i.store.secret(ctx, p.ProjectID, secret)
	if e != nil || metadata.State != "active" {
		t.Fatal("environment deletion revoked shared secret")
	}
	project, e := i.store.GetProject(ctx, i.alice.UserID, p.ProjectID)
	if e != nil {
		t.Fatal(e)
	}
	if r := i.request(t, "alice", "DELETE", "/api/v1/projects/"+p.ProjectID, "", strconv.Quote(strconv.FormatInt(project.Version, 10))); r.Code != 202 {
		t.Fatal("project deletion not accepted")
	}
	for n := 0; n < 40; n++ {
		_, _ = i.store.CleanupProject(ctx, p.ProjectID)
	}
	tomb, e := i.store.get(ctx, "P#"+p.ProjectID, "META")
	if e != nil || tomb.Kind != "projectTombstone" {
		t.Fatal("environment cleanup incomplete", e)
	}
}

func TestIntegrationEnvironmentStrictUnionsAndImmutableVariableIdentity(t *testing.T) {
	i, p := environmentLocal(t)
	path := "/api/v1/projects/" + p.ProjectID + "/environments"
	invalid := []string{
		`{"Name":"bad","variables":[]}`,
		`{"name":"bad","variables":[{"name":"x","enabled":true,"sensitive":false,"value":{"type":"string","value":"a"},"valueType":null}]}`,
		`{"name":"bad","variables":[{"name":"x","enabled":true,"sensitive":false,"value":{"type":"string","value":"a"},"secretWrite":null}]}`,
		`{"name":"bad","variables":[{"name":"x","enabled":null,"sensitive":false,"value":{"type":"null"}}]}`,
		`{"name":"bad","variables":[{"name":"x","enabled":true,"sensitive":true,"valueType":"string","secretWrite":{"action":"set","value":{"type":"string","value":"fixture"},"secretRef":null}}]}`,
		`{"name":"bad","variables":[{"name":"x","enabled":true,"sensitive":false,"value":{"type":"null"}},{"name":"x","enabled":true,"sensitive":false,"value":{"type":"null"}}]}`,
	}
	for _, body := range invalid {
		if r := i.request(t, "alice", "POST", path, body, ""); r.Code != 400 {
			t.Fatalf("invalid union accepted=%d", r.Code)
		}
	}
	r := i.request(t, "alice", "POST", path, `{"name":"Valid","variables":[{"name":"x","enabled":true,"sensitive":false,"value":{"type":"string","value":""}},{"name":"X","enabled":true,"sensitive":false,"value":{"type":"null"}}]}`, "")
	if r.Code != 201 {
		t.Fatalf("case-sensitive names or present empty string denied=%d", r.Code)
	}
	env := environmentResponse(t, r.Body.Bytes())
	path += "/" + env.EnvironmentID
	if r := i.request(t, "alice", "PATCH", path, `{"variables":[]}`, `"0"`); r.Code != 200 {
		t.Fatal("collection clear failed")
	}
	old := env.Variables[0]
	raw, _ := json.Marshal(map[string]any{"variables": []runner.EnvironmentVariable{old}})
	if r := i.request(t, "alice", "PATCH", path, string(raw), `"1"`); r.Code != 400 {
		t.Fatal("removed variable ID resurrected")
	}
	oversize := `{"name":"large","variables":[{"name":"x","enabled":true,"sensitive":false,"value":{"type":"string","value":"` + strings.Repeat("x", 2047) + `"}}]}`
	if r := i.request(t, "alice", "POST", "/api/v1/projects/"+p.ProjectID+"/environments", oversize, ""); r.Code != 413 && r.Code != 400 {
		t.Fatal("oversized scalar accepted")
	}
}

func TestIntegrationEnvironmentUnknownRecordIsPreservedByCleanup(t *testing.T) {
	i, p := environmentLocal(t)
	ctx := context.Background()
	path := "/api/v1/projects/" + p.ProjectID + "/environments"
	response := i.request(t, "alice", "POST", path, `{"name":"Future","variables":[]}`, "")
	if response.Code != 201 {
		t.Fatal("create failed")
	}
	env := environmentResponse(t, response.Body.Bytes())
	row, e := i.store.get(ctx, "P#"+p.ProjectID, "ENV#"+env.EnvironmentID)
	if e != nil {
		t.Fatal(e)
	}
	item, _ := encode(row)
	item["futureEnvironmentSemantics"] = &types.AttributeValueMemberBOOL{Value: true}
	if _, e = i.client.PutItem(ctx, &dynamodb.PutItemInput{TableName: &i.store.table, Item: item}); e != nil {
		t.Fatal(e)
	}
	project, e := i.store.GetProject(ctx, i.alice.UserID, p.ProjectID)
	if e != nil {
		t.Fatal(e)
	}
	if r := i.request(t, "alice", "DELETE", "/api/v1/projects/"+p.ProjectID, "", strconv.Quote(strconv.FormatInt(project.Version, 10))); r.Code != 202 {
		t.Fatal("delete failed")
	}
	for n := 0; n < 5; n++ {
		_, _ = i.store.CleanupProject(ctx, p.ProjectID)
	}
	found, e := i.client.GetItem(ctx, &dynamodb.GetItemInput{TableName: &i.store.table, Key: key(row.PK, row.SK), ConsistentRead: aws.Bool(true)})
	if e != nil || found.Item["futureEnvironmentSemantics"] == nil {
		t.Fatal("unknown environment was deleted")
	}
	parent, e := i.store.get(ctx, row.PK, "META")
	if e != nil || parent.State == "deleted" {
		t.Fatal("parent falsely completed cleanup with an unknown child")
	}
	// Restoring the exact recognized fixture demonstrates resumability, not a
	// production migration or permission to rewrite unknown records automatically.
	delete(item, "futureEnvironmentSemantics")
	if _, e = i.client.PutItem(ctx, &dynamodb.PutItemInput{TableName: &i.store.table, Item: item}); e != nil {
		t.Fatal(e)
	}
	for n := 0; n < 30; n++ {
		_, _ = i.store.CleanupProject(ctx, p.ProjectID)
	}
	parent, e = i.store.get(ctx, row.PK, "META")
	if e != nil || parent.State != "deleted" {
		t.Fatal("recognized fixture cleanup did not resume", e)
	}
}
