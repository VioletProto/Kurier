//go:build integration

package ownership

import (
	"context"
	"encoding/json"
	"github.com/VioletProto/Kurier/services/api/execution"
	runner "github.com/VioletProto/Kurier/services/request-runtime"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"strconv"
	"strings"
	"testing"
)

func TestIntegrationAssertionHTTPNullNumericRevisionsAndDeletion(t *testing.T) {
	i, p, request := workflowLocal(t)
	ctx := context.Background()
	stage, _ := i.store.get(ctx, "STAGE#local", "META")
	stage.SavedRequestsSchemaVersion = 8
	stage.WorkflowAssertionsSchemaVersion = 1
	stage.MCPReadOnlySchemaVersion = 1
	stage.Version++
	item, _ := encode(stage)
	if _, e := i.client.PutItem(ctx, &dynamodb.PutItemInput{TableName: &i.store.table, Item: item}); e != nil {
		t.Fatal(e)
	}
	first, second := newID(), newID()
	definition := execution.WorkflowDefinition{Name: "numeric fixture", Steps: []runner.WorkflowStep{
		{StepID: first, RequestID: request, Extract: []runner.Extraction{}, Bindings: []runner.Binding{}, Assertions: []runner.Assertion{{AssertionID: newID(), Kind: "jsonEquals", Path: "$.n", Expected: json.RawMessage(`{"type":"number","decimal":"0.10000000000000001"}`)}, {AssertionID: newID(), Kind: "jsonEquals", Path: "$.nullable", Expected: json.RawMessage(`null`)}}},
		{StepID: second, RequestID: request, Extract: []runner.Extraction{}, Bindings: []runner.Binding{}},
	}}
	raw, _ := json.Marshal(definition)
	base := "/api/v1/projects/" + p.ProjectID + "/workflows"
	response := i.request(t, "alice", "POST", base, string(raw), "")
	if response.Code != 201 {
		t.Fatalf("assertion create=%d", response.Code)
	}
	var data struct {
		Data struct {
			Workflow execution.Workflow `json:"workflow"`
		} `json:"data"`
	}
	if json.Unmarshal(response.Body.Bytes(), &data) != nil {
		t.Fatal("bad owner response")
	}
	path := base + "/" + data.Data.Workflow.WorkflowID
	for _, body := range []string{`{"name":"rename only"}`, string(raw)} {
		r := i.request(t, "alice", "PATCH", path, body, `"`+strconv.FormatInt(data.Data.Workflow.Revision, 10)+`"`)
		if r.Code != 200 || !strings.Contains(r.Body.String(), "0.10000000000000001") {
			t.Fatal("unrelated edit lost exact token")
		}
		_ = json.Unmarshal(r.Body.Bytes(), &data)
	}
	if r := i.request(t, "alice", "PATCH", path, `{"name":"stale"}`, `"0"`); r.Code != 412 {
		t.Fatal("stale assertion edit accepted")
	}
	for _, replacement := range []string{`0.1`, `{"type":"number","decimal":0.1}`, `{"type":"number","decimal":"9007199254740991.1"}`} {
		bad := strings.Replace(string(raw), `{"type":"number","decimal":"0.10000000000000001"}`, replacement, 1)
		if r := i.request(t, "alice", "POST", base, bad, ""); r.Code != 400 {
			t.Fatalf("unsafe numeric DTO accepted=%d", r.Code)
		}
	}
	if r := i.request(t, "bob", "GET", path, "", ""); r.Code != 404 {
		t.Fatal("foreign assertions disclosed")
	}
	// Run existing ownership deletion through the new capability and definition.
	project, e := i.store.GetProject(ctx, i.alice.UserID, p.ProjectID)
	if e != nil {
		t.Fatal(e)
	}
	deleteResponse := i.request(t, "alice", "DELETE", "/api/v1/projects/"+p.ProjectID, "", `"`+strconv.FormatInt(project.Version, 10)+`"`)
	if deleteResponse.Code != 202 {
		t.Fatalf("project delete=%d", deleteResponse.Code)
	}
	for n := 0; n < 30; n++ {
		_, _ = i.store.CleanupProject(ctx, p.ProjectID)
	}
	tombstone, e := i.store.get(ctx, "P#"+p.ProjectID, "META")
	if e != nil || tombstone.Kind != "projectTombstone" || tombstone.Operation == nil || tombstone.Operation.State != "completed" {
		t.Fatal("assertion project cleanup incomplete", e)
	}
	if r := i.request(t, "alice", "GET", path, "", ""); r.Code != 404 {
		t.Fatal("deleted assertion definition visible")
	}
}
