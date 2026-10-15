//go:build integration

package ownership

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/VioletProto/Kurier/services/api/execution"
	runner "github.com/VioletProto/Kurier/services/request-runtime"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"net/http/httptest"
	"strings"
	"testing"
)

func workflowLocal(t *testing.T) (*integration, Project, string) {
	i, _, _, _ := pinLocal(t)
	ctx := context.Background()
	stage, _ := i.store.get(ctx, "STAGE#local", "META")
	stage.SavedRequestsSchemaVersion = 6
	stage.LocalAgentExecutionsSchemaVersion = 1
	stage.SequentialWorkflowsSchemaVersion = 1
	stage.Version++
	item, _ := encode(stage)
	if _, e := i.client.PutItem(ctx, &dynamodb.PutItemInput{TableName: &i.store.table, Item: item}); e != nil {
		t.Fatal(e)
	}
	i.store.executions.KMS = workflowFixtureKMS{}
	i.store.executions.KeyARN = i.store.cipher.keyARN
	p, e := i.store.CreateProject(ctx, i.alice.UserID, "Workflow browser fixture")
	if e != nil {
		t.Fatal(e)
	}
	c := publicConfiguration()
	c.Headers = []RequestField{{Name: "Authorization", Value: "", Enabled: true}}
	r, e := i.store.CreateRequest(ctx, i.alice.UserID, p.ProjectID, c)
	if e != nil {
		t.Fatal(e)
	}
	return i, p, r.RequestID
}
func TestIntegrationWorkflowHTTPRevisionsOwnershipAndCleanup(t *testing.T) {
	i, p, request := workflowLocal(t)
	first, second := newID(), newID()
	d := execution.WorkflowDefinition{Name: "owned workflow", Steps: []runner.WorkflowStep{{StepID: first, RequestID: request, Extract: []runner.Extraction{{Name: "token", Path: "$.token", Type: "string"}}, Bindings: []runner.Binding{}}, {StepID: second, RequestID: request, Extract: []runner.Extraction{}, Bindings: []runner.Binding{{From: runner.Producer{StepID: first, Name: "token"}, To: runner.BindingTarget{Kind: "header", Index: func() *int { n := 0; return &n }(), Prefix: func() *string { s := "Bearer "; return &s }()}}}}}}
	raw, _ := json.Marshal(d)
	base := "/api/v1/projects/" + p.ProjectID
	response := i.request(t, "alice", "POST", base+"/workflows", string(raw), "")
	if response.Code != 201 {
		t.Fatalf("create=%d %s", response.Code, response.Body.String())
	}
	var data struct {
		Data struct {
			Workflow execution.Workflow `json:"workflow"`
		} `json:"data"`
	}
	_ = json.Unmarshal(response.Body.Bytes(), &data)
	path := base + "/workflows/" + data.Data.Workflow.WorkflowID
	for _, method := range []string{"GET", "PATCH", "DELETE"} {
		r := i.request(t, "bob", method, path, `{"name":"foreign"}`, `"0"`)
		if r.Code != 404 {
			t.Fatalf("foreign %s=%d", method, r.Code)
		}
	}
	if r := i.request(t, "alice", "PATCH", path, `{"name":"changed"}`, ""); r.Code != 428 {
		t.Fatal("missing precondition")
	}
	if r := i.request(t, "alice", "PATCH", path, `{"name":"changed"}`, `"9"`); r.Code != 412 {
		t.Fatal("stale precondition")
	}
	if r := i.request(t, "alice", "PATCH", path, `{"name":"changed"}`, `"0"`); r.Code != 200 {
		t.Fatalf("patch=%d", r.Code)
	}
	key := newID()
	submit := func(allow bool) *httptest.ResponseRecorder {
		body := `{"target":"cloud","allowInsecureSecrets":false}`
		if allow {
			body = `{"target":"cloud","allowInsecureSecrets":true}`
		}
		r := httptest.NewRequest("POST", path+"/runs", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer "+i.auth.token(t, "alice", nil))
		r.Header.Set("If-Match", `"1"`)
		r.Header.Set("Idempotency-Key", key)
		w := httptest.NewRecorder()
		i.server.ServeHTTP(w, r)
		return w
	}
	// Public configuration fixture uses HTTPS; admission freezes outputs without executing HTTP.
	r := submit(true)
	if r.Code != 202 {
		t.Fatalf("submit=%d %s", r.Code, r.Body.String())
	}
	again := submit(true)
	if again.Code != 202 {
		t.Fatal("retry")
	}
	if r := submit(false); r.Code != 409 {
		t.Fatal("key consent mismatch")
	}
	detail, e := i.store.GetProject(context.Background(), i.alice.UserID, p.ProjectID)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = i.store.DeleteProject(context.Background(), i.alice.UserID, p.ProjectID, detail.Version); e != nil {
		t.Fatal(e)
	}
	for n := 0; n < 100; n++ {
		_, _ = i.store.CleanupProject(context.Background(), p.ProjectID)
	}
	if r := i.request(t, "alice", "GET", path, "", ""); r.Code != 404 {
		t.Fatal("delete did not deny workflows")
	}
}

type workflowFixtureKMS struct{}

func (workflowFixtureKMS) GenerateDataKey(_ context.Context, in *kms.GenerateDataKeyInput, _ ...func(*kms.Options)) (*kms.GenerateDataKeyOutput, error) {
	scope, _ := json.Marshal(in.EncryptionContext)
	return &kms.GenerateDataKeyOutput{KeyId: in.KeyId, Plaintext: bytes.Repeat([]byte{7}, 32), CiphertextBlob: scope}, nil
}
func (workflowFixtureKMS) Decrypt(_ context.Context, in *kms.DecryptInput, _ ...func(*kms.Options)) (*kms.DecryptOutput, error) {
	scope, _ := json.Marshal(in.EncryptionContext)
	if !bytes.Equal(scope, in.CiphertextBlob) {
		return nil, errors.New("fixture scope")
	}
	return &kms.DecryptOutput{KeyId: in.KeyId, Plaintext: bytes.Repeat([]byte{7}, 32)}, nil
}
