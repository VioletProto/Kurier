//go:build integration

package ownership

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	oapi "github.com/VioletProto/Kurier/services/request-runtime/openapi"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func openapiLocal(t *testing.T) (*integration, Project) {
	t.Helper()
	i, p := environmentLocal(t)
	ctx := context.Background()
	stage, e := i.store.get(ctx, "STAGE#local", "META")
	if e != nil {
		t.Fatal(e)
	}
	stage.SavedRequestsSchemaVersion = 10
	stage.OpenAPISchemaVersion = 1
	stage.Version++
	item, _ := encode(stage)
	if _, e = i.client.PutItem(ctx, &dynamodb.PutItemInput{TableName: &i.store.table, Item: item}); e != nil {
		t.Fatal(e)
	}
	return i, p
}
func openapiDocument(operations int, parameter bool) string {
	paths := map[string]any{}
	for n := 0; n < operations; n++ {
		path := fmt.Sprintf("/items/%d", n)
		op := map[string]any{"responses": map[string]any{"200": map[string]any{"description": "OK", "content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"type": "object", "required": []string{"ok"}, "properties": map[string]any{"ok": map[string]any{"type": "boolean"}}}}}}}}
		if parameter {
			path = "/items/{itemId}"
			op["parameters"] = []any{map[string]any{"name": "itemId", "in": "path", "required": true, "schema": map[string]any{"type": "string"}}, map[string]any{"name": "ratio", "in": "query", "example": json.RawMessage("0.10000000000000001"), "schema": map[string]any{"type": "number"}}, map[string]any{"name": "api_key", "in": "query", "example": "openapi-source-private-canary", "schema": map[string]any{"type": "string"}}}
		}
		paths[path] = map[string]any{"get": op}
	}
	raw, _ := json.Marshal(map[string]any{"openapi": "3.0.4", "info": map[string]any{"title": "Fixture", "version": "1"}, "servers": []any{map[string]any{"url": "https://example.com"}}, "paths": paths})
	return string(raw)
}
func importResponse(t *testing.T, raw []byte) OpenAPIImport {
	t.Helper()
	var v struct {
		Data struct {
			Import OpenAPIImport `json:"import"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &v) != nil {
		t.Fatal("invalid import reply")
	}
	return v.Data.Import
}
func TestIntegrationOpenAPIStagingCompletionExportOwnershipAndCleanup(t *testing.T) {
	i, p := openapiLocal(t)
	base := "/api/v1/projects/" + p.ProjectID + "/openapi-imports"
	source := openapiDocument(1, true)
	body, _ := json.Marshal(map[string]any{"format": "json", "source": source, "publicDataConfirmed": true})
	r := i.request(t, "alice", "POST", base, string(body), "")
	if r.Code != 201 {
		t.Fatalf("create=%d %s", r.Code, r.Body.String())
	}
	created := importResponse(t, r.Body.Bytes())
	if created.State != "preparing" || created.Revision != 1 || len(created.Operations) != 1 || created.Operations[0].Ready {
		t.Fatal("invalid preparing view")
	}
	path := base + "/" + created.ImportID
	if strings.Contains(r.Body.String(), "openapi-source-private-canary") {
		t.Fatal("unsafe source example retained")
	}
	for _, method := range []string{"GET", "PATCH", "DELETE"} {
		if r = i.request(t, "bob", method, path, `{}`, `"1"`); r.Code != 404 {
			t.Fatalf("foreign=%s/%d", method, r.Code)
		}
	}
	for _, suffix := range []string{"?format=json", "?format=yaml"} {
		r = i.request(t, "alice", "GET", path+"/export"+suffix, "", "")
		if r.Code != 200 || strings.Contains(r.Body.String(), "openapi-source-private-canary") {
			t.Fatalf("unsafe export %d", r.Code)
		}
		if _, e := oapi.Parse(r.Body.Bytes(), strings.TrimPrefix(suffix, "?format="), oapi.Policy{}); e != nil {
			t.Fatal("export not parseable")
		}
	}
	r = i.request(t, "alice", "POST", path+"/publish", `{}`, `"1"`)
	if r.Code != 400 {
		t.Fatal("missing inputs published", r.Code)
	}
	if _, e := i.store.GetRequest(context.Background(), i.alice.UserID, p.ProjectID, created.Operations[0].RequestID); e == nil {
		t.Fatal("preparing request visible")
	}
	input := ""
	for _, p := range created.Operations[0].Operation.Parameters {
		if p.Name == "itemId" {
			input = p.InputID
		}
	}
	protected, e := i.store.saveEnvironment(context.Background(), i.alice.UserID, p.ProjectID, newID(), []byte(`{"name":"Imported credential","variables":[{"name":"token","enabled":true,"sensitive":true,"valueType":"string","secretWrite":{"action":"set","value":{"type":"string","value":"authored-protected-private-canary"}}}]}`), 0, true)
	if e != nil {
		t.Fatal(e)
	}
	secretInput := ""
	for _, parameter := range created.Operations[0].Operation.Parameters {
		if parameter.Name == "api_key" {
			secretInput = parameter.InputID
		}
	}
	patch := fmt.Sprintf(`{"operationId":%q,"publicDataConfirmed":true,"choices":{"parameters":{%q:"a/b"},"secretRefs":{%q:%q}}}`, created.Operations[0].Operation.OperationID, input, secretInput, protected.Variables[0].SecretRef.SecretID)
	r = i.request(t, "alice", "PATCH", path, patch, `"0"`)
	if r.Code != 412 {
		t.Fatal("stale accepted")
	}
	r = i.request(t, "alice", "PATCH", path, patch, `"1"`)
	if r.Code != 200 {
		t.Fatalf("patch=%d %s", r.Code, r.Body.String())
	}
	completed := importResponse(t, r.Body.Bytes())
	if !completed.Operations[0].Ready {
		t.Fatal("completion not ready")
	}
	r = i.request(t, "alice", "POST", path+"/publish", `{}`, strconv.Quote(fmt.Sprint(completed.Revision)))
	if r.Code != 200 {
		t.Fatalf("publish=%d %s", r.Code, r.Body.String())
	}
	ready := importResponse(t, r.Body.Bytes())
	if ready.State != "ready" {
		t.Fatal("not ready")
	}
	saved, e := i.store.GetRequest(context.Background(), i.alice.UserID, p.ProjectID, ready.Operations[0].RequestID)
	if e != nil || saved.OperationRef == nil || !strings.Contains(saved.URL, "a%2Fb") {
		t.Fatal("generated request mismatch", e)
	}
	if saved.QueryParameters[0].Value != "0.10000000000000001" {
		t.Fatal("decimal rounded")
	}
	if len(saved.QueryParameters) != 2 || !saved.QueryParameters[1].Sensitive || !saved.QueryParameters[1].Masked || saved.QueryParameters[1].SecretRef == nil {
		t.Fatal("protected generated parameter not bound")
	}
	savedRaw, _ := json.Marshal(saved)
	if strings.Contains(string(savedRaw), "authored-protected-private-canary") {
		t.Fatal("protected value disclosed")
	}

	r = i.request(t, "alice", "DELETE", path, "", strconv.Quote(fmt.Sprint(ready.Revision)))
	if r.Code != 409 {
		t.Fatal("ready import removed")
	}
	// Detachment is explicit and retains the ordinary request for local execution.
	if _, e = i.store.PatchRequest(context.Background(), i.alice.UserID, p.ProjectID, saved.RequestID, []byte(`{"operationRef":null}`), saved.Revision); e != nil {
		t.Fatal("detach failed", e)
	}
	// Normal cleanup recognizes import/source/operation and request schema4 rows.
	project, e := i.store.GetProject(context.Background(), i.alice.UserID, p.ProjectID)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = i.store.DeleteProject(context.Background(), i.alice.UserID, p.ProjectID, project.Version); e != nil {
		t.Fatal(e)
	}
	for n := 0; n < 20; n++ {
		_, _ = i.store.CleanupProject(context.Background(), p.ProjectID)
	}
	row, e := i.store.get(context.Background(), "P#"+p.ProjectID, "META")
	if e != nil || row.Kind != "projectTombstone" {
		t.Fatal("normal cleanup incomplete", e)
	}
}

type failOpenAPIPublishDB struct {
	database
	requestBatches int
	failAt         int
}

func (f *failOpenAPIPublishDB) TransactWriteItems(ctx context.Context, in *dynamodb.TransactWriteItemsInput, opt ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error) {
	hasRequest := false
	for _, a := range in.TransactItems {
		if a.Put != nil {
			if kind, ok := a.Put.Item["kind"].(*types.AttributeValueMemberS); ok && kind.Value == "request" {
				hasRequest = true
			}
		}
	}
	if hasRequest {
		f.requestBatches++
		if f.requestBatches == f.failAt {
			return nil, errors.New("synthetic unavailable")
		}
	}
	return f.database.TransactWriteItems(ctx, in, opt...)
}
func TestIntegrationOpenAPIHundredOperationsAndInterruptedPublication(t *testing.T) {
	i, p := openapiLocal(t)
	ctx := context.Background()
	doc, e := i.store.createOpenAPI(ctx, i.alice.UserID, p.ProjectID, "json", openapiDocument(100, false), oapi.Policy{})
	if e != nil {
		t.Fatal(e)
	}
	if len(doc.Operations) != 100 {
		t.Fatal("operation100 lost")
	}
	base := i.store.db
	fault := &failOpenAPIPublishDB{database: base, failAt: 2}
	i.store.db = fault
	if _, e = i.store.publishOpenAPI(ctx, i.alice.UserID, p.ProjectID, doc.ImportID, doc.Revision); e == nil {
		t.Fatal("injected failure did not stop")
	}
	i.store.db = base
	root, e := i.store.openapiGet(ctx, p.ProjectID, doc.ImportID, openapiKey(doc.ImportID))
	if e != nil || root.State != "publishing" {
		t.Fatal("missing publishing fence")
	}
	for _, c := range doc.Operations {
		if _, e = i.store.GetRequest(ctx, i.alice.UserID, p.ProjectID, c.RequestID); e == nil {
			t.Fatal("partial request visible")
		}
	}
	if e = i.store.deleteOpenAPI(ctx, i.alice.UserID, p.ProjectID, doc.ImportID, root.Revision); e != nil {
		t.Fatal("cancel partial", e)
	}
	for _, c := range doc.Operations {
		if _, e = i.store.get(ctx, "P#"+p.ProjectID, "REQ#"+c.RequestID); e == nil {
			t.Fatal("staged child survived cancellation")
		}
	}
	doc, e = i.store.createOpenAPI(ctx, i.alice.UserID, p.ProjectID, "json", openapiDocument(100, false), oapi.Policy{})
	if e != nil {
		t.Fatal(e)
	}
	ready, e := i.store.publishOpenAPI(ctx, i.alice.UserID, p.ProjectID, doc.ImportID, doc.Revision)
	if e != nil || ready.State != "ready" {
		t.Fatal("hundred-op publish failed", e)
	}
	for _, c := range ready.Operations {
		if _, e = i.store.GetRequest(ctx, i.alice.UserID, p.ProjectID, c.RequestID); e != nil {
			t.Fatal("ready request missing", e)
		}
	}
}
func TestOpenAPIChoicesRejectSensitiveInputEvenWhileOtherInputsMissing(t *testing.T) {
	if publicOpenAPIChoices(oapi.Choices{Body: json.RawMessage(`{"password":"secret-canary"}`)}) {
		t.Fatal("credential body accepted")
	}
	if publicOpenAPIChoices(oapi.Choices{Parameters: map[string]json.RawMessage{"p0": json.RawMessage(`"Bearer secret-canary"`)}}) {
		t.Fatal("credential scalar accepted")
	}
}

func TestIntegrationOpenAPIUnknownMetadataIsNotErased(t *testing.T) {
	i, p := openapiLocal(t)
	ctx := context.Background()
	imported, e := i.store.createOpenAPI(ctx, i.alice.UserID, p.ProjectID, "json", openapiDocument(1, false), oapi.Policy{})
	if e != nil {
		t.Fatal(e)
	}
	target := openapiPrefix(imported.ImportID) + "OP#" + imported.Operations[0].Operation.OperationID
	_, e = i.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{TableName: &i.store.table, Key: key("P#"+p.ProjectID, target), UpdateExpression: aws.String("SET unexpectedFutureField = :v"), ExpressionAttributeValues: map[string]types.AttributeValue{":v": s("future-record")}})
	if e != nil {
		t.Fatal(e)
	}
	if e = i.store.deleteOpenAPI(ctx, i.alice.UserID, p.ProjectID, imported.ImportID, imported.Revision); e == nil {
		t.Fatal("cancel accepted unknown record")
	}
	project, e := i.store.GetProject(ctx, i.alice.UserID, p.ProjectID)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = i.store.DeleteProject(ctx, i.alice.UserID, p.ProjectID, project.Version); e != nil {
		t.Fatal(e)
	}
	for n := 0; n < 8; n++ {
		_, _ = i.store.CleanupProject(ctx, p.ProjectID)
	}
	retained, e := i.client.GetItem(ctx, &dynamodb.GetItemInput{TableName: &i.store.table, Key: key("P#"+p.ProjectID, target), ConsistentRead: aws.Bool(true)})
	if e != nil || len(retained.Item) == 0 {
		t.Fatal("unknown child erased", e)
	}
	meta, e := i.store.get(ctx, "P#"+p.ProjectID, "META")
	if e != nil || meta.Kind == "projectTombstone" {
		t.Fatal("cleanup completed over unknown child", e)
	}
}

func TestIntegrationOpenAPIPartialChoicesNeverStoreSensitiveProvidedValues(t *testing.T) {
	i, p := openapiLocal(t)
	ctx := context.Background()
	imported, e := i.store.createOpenAPI(ctx, i.alice.UserID, p.ProjectID, "json", openapiDocument(1, true), oapi.Policy{})
	if e != nil {
		t.Fatal(e)
	}
	operation := imported.Operations[0].Operation
	secretInput := ""
	for _, parameter := range operation.Parameters {
		if parameter.Sensitive {
			secretInput = parameter.InputID
		}
	}
	choices := oapi.Choices{Parameters: map[string]json.RawMessage{secretInput: json.RawMessage(`"partial-private-canary"`)}}
	if _, e = i.store.updateOpenAPICandidate(ctx, i.alice.UserID, p.ProjectID, imported.ImportID, operation.OperationID, choices, imported.Revision); e == nil {
		t.Fatal("missing required path hid sensitive supplied parameter")
	}
	row, e := i.store.openapiGet(ctx, p.ProjectID, imported.ImportID, openapiPrefix(imported.ImportID)+"OP#"+operation.OperationID)
	if e != nil || row.ChoicesJSON != "{}" {
		t.Fatal("unsafe partial choices persisted", e)
	}
	var document map[string]json.RawMessage
	if json.Unmarshal([]byte(openapiDocument(1, false)), &document) != nil {
		t.Fatal("fixture JSON")
	}
	delete(document, "servers")
	raw, _ := json.Marshal(document)
	imported, e = i.store.createOpenAPI(ctx, i.alice.UserID, p.ProjectID, "json", string(raw), oapi.Policy{})
	if e != nil {
		t.Fatal(e)
	}
	choices = oapi.Choices{Parameters: map[string]json.RawMessage{"password": json.RawMessage(`"partial-private-canary"`)}}
	if _, e = i.store.updateOpenAPICandidate(ctx, i.alice.UserID, p.ProjectID, imported.ImportID, imported.Operations[0].Operation.OperationID, choices, imported.Revision); e == nil {
		t.Fatal("missing server hid unknown supplied parameter")
	}
}
