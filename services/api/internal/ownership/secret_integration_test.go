//go:build integration

package ownership

import (
	"context"
	"encoding/json"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"strings"
	"sync"
	"testing"
)

func enableProtectedLocal(t *testing.T, i *integration) {
	t.Helper()
	ctx := context.Background()
	table := i.store.table + "-protected"
	_, err := i.client.CreateTable(ctx, &dynamodb.CreateTableInput{TableName: aws.String(table), BillingMode: types.BillingModePayPerRequest, AttributeDefinitions: []types.AttributeDefinition{{AttributeName: aws.String("PK"), AttributeType: types.ScalarAttributeTypeS}, {AttributeName: aws.String("SK"), AttributeType: types.ScalarAttributeTypeS}}, KeySchema: []types.KeySchemaElement{{AttributeName: aws.String("PK"), KeyType: types.KeyTypeHash}, {AttributeName: aws.String("SK"), KeyType: types.KeyTypeRange}}})
	if err != nil {
		t.Fatal("protected fixture table creation failed")
	}
	cipher, _ := newFixtureCipher("local")
	i.store.ConfigureProtected(table, cipher)
	stage, err := i.store.get(ctx, "STAGE#local", "META")
	if err != nil {
		t.Fatal(err)
	}
	stage.SavedRequestsSchemaVersion = 2
	stage.ProtectedSecretsSchemaVersion = 1
	item, _ := encode(stage)
	if _, err = i.client.PutItem(ctx, &dynamodb.PutItemInput{TableName: aws.String(i.store.table), Item: item}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		i.client.DeleteTable(context.Background(), &dynamodb.DeleteTableInput{TableName: aws.String(table)})
	})
}
func protectedConfig(value string) RequestConfiguration {
	c := publicConfiguration()
	c.Headers = []RequestField{{Name: "Authorization", Enabled: true, Sensitive: true, BindingID: newID(), SecretWrite: &SecretWrite{Action: "set", Value: &value}}}
	return c
}
func wirePatch(c RequestConfiguration) []byte {
	c.Headers = append([]RequestField{}, c.Headers...)
	c.QueryParameters = append([]RequestField{}, c.QueryParameters...)
	if c.Body != nil {
		b := *c.Body
		b.SecretFields = append([]SecretField{}, b.SecretFields...)
		c.Body = &b
	}
	for _, f := range slots(&c) {
		if *f.write == nil {
			*f.write = &SecretWrite{Action: "preserve", SecretRef: *f.ref}
			*f.ref = nil
			*f.masked = false
		}
	}
	raw, _ := configurationJSON(c)
	return raw
}
func assertSafe(t *testing.T, value string, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil || strings.Contains(string(b), value) {
		t.Fatal("protected value detected in a forbidden surface")
	}
}
func TestIntegrationProtectedBindingsAndCleanup(t *testing.T) {
	i := localIntegration(t)
	enableProtectedLocal(t, i)
	exerciseProtected(t, i)
}
func exerciseProtected(t *testing.T, i *integration) {
	t.Helper()
	ctx := context.Background()
	p, err := i.store.CreateProject(ctx, i.alice.UserID, "secret fixture")
	if err != nil {
		t.Fatal(err)
	}
	value := "Bearer " + newID()
	a, err := i.store.CreateRequest(ctx, i.alice.UserID, p.ProjectID, protectedConfig(value))
	if err != nil {
		t.Fatal("protected creation failed", err)
	}
	id := a.Headers[0].SecretRef.SecretID
	assertSafe(t, value, a)
	control, err := i.client.GetItem(ctx, &dynamodb.GetItemInput{TableName: aws.String(i.store.table), Key: key("P#"+p.ProjectID, "REQ#"+a.RequestID), ConsistentRead: aws.Bool(true)})
	if err != nil {
		t.Fatal(err)
	}
	assertSafe(t, value, control.Item)
	r, err := i.store.secret(ctx, p.ProjectID, id)
	if err != nil || r.Envelope == nil {
		t.Fatal("encrypted persistence missing", err)
	}
	assertSafe(t, value, r)
	if _, err = i.store.GetRequest(ctx, i.bob.UserID, p.ProjectID, a.RequestID); err == nil {
		t.Fatal("foreign owner read allowed")
	}
	foreign, err := i.store.CreateProject(ctx, i.alice.UserID, "other fixture")
	if err != nil {
		t.Fatal(err)
	}
	borrow := publicConfiguration()
	borrow.Headers = []RequestField{{Name: "Authorization", Enabled: true, Sensitive: true, BindingID: newID(), SecretWrite: &SecretWrite{Action: "secretRef", SecretRef: &SecretReference{id}}}}
	if _, err = i.store.CreateRequest(ctx, i.alice.UserID, foreign.ProjectID, borrow); err == nil {
		t.Fatal("cross-project ref allowed")
	}
	borrow.Headers[0].SecretWrite.Action = "preserve"
	if _, err = i.store.CreateRequest(ctx, i.alice.UserID, p.ProjectID, borrow); err == nil {
		t.Fatal("POST preserve introduced binding")
	}
	borrow.Headers[0].SecretWrite.Action = "secretRef"
	b, err := i.store.CreateRequest(ctx, i.alice.UserID, p.ProjectID, borrow)
	if err != nil {
		t.Fatal("same-project reuse failed", err)
	}
	c := a.RequestConfiguration
	c.Headers = append(append([]RequestField{}, c.Headers...), RequestField{Name: "Authorization", Enabled: false, Sensitive: true, BindingID: newID(), SecretWrite: &SecretWrite{Action: "secretRef", SecretRef: &SecretReference{id}}})
	a, err = i.store.PatchRequest(ctx, i.alice.UserID, p.ProjectID, a.RequestID, wirePatch(c), a.Revision)
	if err != nil {
		t.Fatal(err)
	}
	c = a.RequestConfiguration
	c.Headers = []RequestField{c.Headers[1], c.Headers[0]}
	a, err = i.store.PatchRequest(ctx, i.alice.UserID, p.ProjectID, a.RequestID, wirePatch(c), a.Revision)
	if err != nil {
		t.Fatal("reorder preserve failed", err)
	}
	c = a.RequestConfiguration
	c.Headers = append([]RequestField{}, c.Headers...)
	c.Headers[0].Name = "X-Api-Key"
	if _, err = i.store.PatchRequest(ctx, i.alice.UserID, p.ProjectID, a.RequestID, wirePatch(c), a.Revision); err == nil {
		t.Fatal("renamed preserve introduced binding")
	}
	sharedValue := "Bearer " + newID()
	meta, err := i.store.ReplaceSecret(ctx, i.alice.UserID, p.ProjectID, id, sharedValue, 0)
	if err != nil || meta.Revision != 1 {
		t.Fatal("shared replace failed", err)
	}
	b2, err := i.store.GetRequest(ctx, i.alice.UserID, p.ProjectID, b.RequestID)
	if err != nil || b2.Revision != b.Revision || b2.Headers[0].SecretRef.SecretID != id {
		t.Fatal("shared replacement rewrote binding or request revision")
	}
	if _, err = i.store.ReplaceSecret(ctx, i.alice.UserID, p.ProjectID, id, sharedValue, 0); err == nil {
		t.Fatal("stale shared replacement accepted")
	}
	c = a.RequestConfiguration
	c.Headers = append([]RequestField{}, c.Headers...)
	localValue := "Bearer " + newID()
	c.Headers[0].SecretRef = nil
	c.Headers[0].Masked = false
	c.Headers[0].SecretWrite = &SecretWrite{Action: "set", Value: &localValue}
	a, err = i.store.PatchRequest(ctx, i.alice.UserID, p.ProjectID, a.RequestID, wirePatch(c), a.Revision)
	if err != nil || a.Headers[0].SecretRef.SecretID == id {
		t.Fatal("local replacement was shared", err)
	}
	if err = i.store.RevokeSecret(ctx, i.alice.UserID, p.ProjectID, id, 1); err != nil {
		t.Fatal(err)
	}
	r, err = i.store.secret(ctx, p.ProjectID, id)
	if err != nil || r.Envelope != nil || r.State != "revoked" {
		t.Fatal("revocation retained ciphertext")
	}
	b, err = i.store.PatchRequest(ctx, i.alice.UserID, p.ProjectID, b.RequestID, []byte("{\"name\":\"unrelated\"}"), b.Revision)
	if err != nil {
		t.Fatal("unrelated edit failed on unavailable ref", err)
	}
	borrow.Headers[0].SecretWrite = &SecretWrite{Action: "secretRef", SecretRef: &SecretReference{id}}
	if _, err = i.store.CreateRequest(ctx, i.alice.UserID, p.ProjectID, borrow); err == nil {
		t.Fatal("revoked reuse allowed")
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := i.store.PatchRequest(ctx, i.alice.UserID, p.ProjectID, b.RequestID, []byte("{\"name\":\"concurrent\"}"), b.Revision)
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	wins := 0
	for err := range results {
		if err == nil {
			wins++
		}
	}
	if wins != 1 {
		t.Fatal("concurrent edit did not have one winner")
	}
	unknown := map[string]types.AttributeValue{"PK": s("P#" + p.ProjectID), "SK": s("JOB#unknown#BINDINGS"), "kind": s("futureBundle")}
	if _, err = i.client.PutItem(ctx, &dynamodb.PutItemInput{TableName: aws.String(i.store.protectedTable), Item: unknown}); err != nil {
		t.Fatal(err)
	}
	project, err := i.store.GetProject(ctx, i.alice.UserID, p.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = i.store.DeleteProject(ctx, i.alice.UserID, p.ProjectID, project.Version); err != nil {
		t.Fatal(err)
	}
	for range 5 {
		i.store.CleanupProject(ctx, p.ProjectID)
	}
	gate, err := i.store.get(ctx, "P#"+p.ProjectID, "META")
	if err != nil || gate.State == "deleted" {
		t.Fatal("unknown protected child ignored")
	}
	out, err := i.client.GetItem(ctx, &dynamodb.GetItemInput{TableName: aws.String(i.store.protectedTable), Key: key("P#"+p.ProjectID, "JOB#unknown#BINDINGS"), ConsistentRead: aws.Bool(true)})
	if err != nil || len(out.Item) == 0 {
		t.Fatal("unknown protected child deleted")
	}
	i.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{TableName: aws.String(i.store.protectedTable), Key: key("P#"+p.ProjectID, "JOB#unknown#BINDINGS")})
	var op DeletionOperation
	for range 10 {
		op, err = i.store.CleanupProject(ctx, p.ProjectID)
		if err == nil && op.State == "completed" {
			break
		}
	}
	if err != nil || op.State != "completed" {
		t.Fatal("protected cleanup did not resume", err)
	}
	remaining, err := i.client.Query(ctx, &dynamodb.QueryInput{TableName: aws.String(i.store.protectedTable), KeyConditionExpression: aws.String("PK = :pk"), ExpressionAttributeValues: map[string]types.AttributeValue{":pk": s("P#" + p.ProjectID)}, ConsistentRead: aws.Bool(true)})
	if err != nil || len(remaining.Items) != 0 {
		t.Fatal("protected records remain")
	}
	foreignGate, _ := i.store.GetProject(ctx, i.alice.UserID, foreign.ProjectID)
	i.store.DeleteProject(ctx, i.alice.UserID, foreign.ProjectID, foreignGate.Version)
	i.store.CleanupProject(ctx, foreign.ProjectID)
}

func TestIntegrationProtectedFailuresAndPointers(t *testing.T) {
	i := localIntegration(t)
	enableProtectedLocal(t, i)
	ctx := context.Background()
	p, err := i.store.CreateProject(ctx, i.alice.UserID, "failure fixture")
	if err != nil {
		t.Fatal(err)
	}
	before, _ := i.store.GetProject(ctx, i.alice.UserID, p.ProjectID)
	fixture := i.store.cipher.kms.(*fixtureKMS)
	fixture.fail = true
	if _, err = i.store.CreateRequest(ctx, i.alice.UserID, p.ProjectID, protectedConfig("Bearer "+newID())); err == nil {
		t.Fatal("KMS failure accepted")
	}
	after, _ := i.store.GetProject(ctx, i.alice.UserID, p.ProjectID)
	if before.Version != after.Version {
		t.Fatal("KMS failure advanced gate")
	}
	fixture.fail = false
	lost := &uncertainDB{database: i.client}
	i.store.db = lost
	if _, err = i.store.CreateRequest(ctx, i.alice.UserID, p.ProjectID, protectedConfig("Bearer "+newID())); err == nil {
		t.Fatal("lost ACK invented success")
	}
	if lost.calls != 1 {
		t.Fatal("uncertain encrypted write retried")
	}
	i.store.db = i.client
	rows, err := i.client.Query(ctx, &dynamodb.QueryInput{TableName: aws.String(i.store.protectedTable), KeyConditionExpression: aws.String("PK = :pk"), ExpressionAttributeValues: map[string]types.AttributeValue{":pk": s("P#" + p.ProjectID)}, ConsistentRead: aws.Bool(true)})
	if err != nil || len(rows.Items) != 1 {
		t.Fatal("lost ACK created orphan/duplicate secrets")
	}
	c := publicConfiguration()
	v := "\"" + newID() + "\""
	c.Body = &RequestBody{Type: "json", Text: "{\"password\":null,\"other\":null}", SecretFields: []SecretField{{Pointer: "/password", BindingID: newID(), SecretWrite: &SecretWrite{Action: "set", Value: &v}}}}
	a, err := i.store.CreateRequest(ctx, i.alice.UserID, p.ProjectID, c)
	if err != nil {
		t.Fatal("JSON pointer create failed", err)
	}
	c = a.RequestConfiguration
	b := *c.Body
	b.SecretFields = append([]SecretField{}, b.SecretFields...)
	c.Body = &b
	c.Body.SecretFields[0].Pointer = "/other"
	if _, err = i.store.PatchRequest(ctx, i.alice.UserID, p.ProjectID, a.RequestID, wirePatch(c), a.Revision); err == nil {
		t.Fatal("pointer move preserve allowed")
	}
	c = a.RequestConfiguration
	a, err = i.store.PatchRequest(ctx, i.alice.UserID, p.ProjectID, a.RequestID, wirePatch(c), a.Revision)
	if err != nil {
		t.Fatal("same pointer preserve rejected", err)
	}
	// Test atomic rollback after encryption: advance the gate immediately before transaction.
	gate := &advanceGateDB{database: i.client, client: i.client, table: i.store.table, pk: "P#" + p.ProjectID}
	i.store.db = gate
	if _, err = i.store.CreateRequest(ctx, i.alice.UserID, p.ProjectID, protectedConfig("Bearer "+newID())); err == nil {
		t.Fatal("changed gate allowed encrypted write")
	}
	i.store.db = i.client
	rows2, err := i.client.Query(ctx, &dynamodb.QueryInput{TableName: aws.String(i.store.protectedTable), KeyConditionExpression: aws.String("PK = :pk"), ExpressionAttributeValues: map[string]types.AttributeValue{":pk": s("P#" + p.ProjectID)}, ConsistentRead: aws.Bool(true)})
	if err != nil || len(rows2.Items) != len(rows.Items)+1 {
		t.Fatal("failed transaction persisted an orphan envelope")
	}
	pg, _ := i.store.GetProject(ctx, i.alice.UserID, p.ProjectID)
	i.store.DeleteProject(ctx, i.alice.UserID, p.ProjectID, pg.Version)
	lost = &uncertainDB{database: i.client}
	i.store.db = lost
	i.store.CleanupProject(ctx, p.ProjectID)
	i.store.db = i.client
	for range 10 {
		op, e := i.store.CleanupProject(ctx, p.ProjectID)
		if e == nil && op.State == "completed" {
			return
		}
	}
	t.Fatal("cleanup did not recover lost acknowledgement")
}

type advanceGateDB struct {
	database
	client    *dynamodb.Client
	table, pk string
	done      bool
}

func (d *advanceGateDB) TransactWriteItems(ctx context.Context, in *dynamodb.TransactWriteItemsInput, opts ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error) {
	if !d.done {
		d.done = true
		_, err := d.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{TableName: aws.String(d.table), Key: key(d.pk, "META"), UpdateExpression: aws.String("SET #v = #v + :one"), ExpressionAttributeNames: map[string]string{"#v": "version"}, ExpressionAttributeValues: map[string]types.AttributeValue{":one": n(1)}})
		if err != nil {
			return nil, err
		}
	}
	return d.database.TransactWriteItems(ctx, in, opts...)
}

func TestIntegrationProtectedHTTPMetadataAndReferenceMisuse(t *testing.T) {
	i := localIntegration(t)
	enableProtectedLocal(t, i)
	ctx := context.Background()
	p, err := i.store.CreateProject(ctx, i.alice.UserID, "HTTP fixture")
	if err != nil {
		t.Fatal(err)
	}
	value := "Bearer " + newID()
	raw, _ := configurationJSON(protectedConfig(value))
	root := "/api/v1/projects/" + p.ProjectID
	w := i.request(t, "alice", "POST", root+"/requests", string(raw), "")
	if w.Code != 201 {
		t.Fatal("protected HTTP creation failed")
	}
	assertSafe(t, value, w.Body.String())
	a := requestFrom(t, w)
	id := a.Headers[0].SecretRef.SecretID
	for _, route := range []struct{ method, path, body string }{{"GET", root + "/secrets", ""}, {"PATCH", root + "/secrets/" + id, "{}"}, {"DELETE", root + "/secrets/" + id, ""}} {
		if i.request(t, "bob", route.method, route.path, route.body, "").Code != 404 {
			t.Fatal("foreign secret route disclosed resource")
		}
	}
	w = i.request(t, "alice", "GET", root+"/secrets?limit=1", "", "")
	if w.Code != 200 {
		t.Fatal("secret metadata list failed")
	}
	assertSafe(t, value, w.Body.String())
	w = i.request(t, "alice", "PATCH", root+"/secrets/"+id, "{\"value\":\"Bearer "+newID()+"\"}", "")
	if w.Code != 428 {
		t.Fatal("missing shared revision allowed")
	}
	w = i.request(t, "alice", "PATCH", root+"/secrets/"+id, "{\"value\":\"Bearer "+newID()+"\"}", "\"0\"")
	if w.Code != 200 || w.Header().Get("ETag") != "\"1\"" {
		t.Fatal("shared HTTP replacement failed")
	}
	assertSafe(t, value, w.Body.String())
	if i.request(t, "alice", "DELETE", root+"/secrets/"+id, "", "\"0\"").Code != 412 {
		t.Fatal("stale shared revocation allowed")
	}
	if i.request(t, "alice", "DELETE", root+"/secrets/"+id, "", "\"1\"").Code != 204 {
		t.Fatal("shared HTTP revocation failed")
	}
	c := a.RequestConfiguration
	c.Headers = append([]RequestField{}, c.Headers...)
	c.Headers[0].BindingID = newID()
	if _, err = i.store.PatchRequest(ctx, i.alice.UserID, p.ProjectID, a.RequestID, wirePatch(c), a.Revision); err == nil {
		t.Fatal("preserve introduced new binding ID")
	}
	fake := newID()
	_, err = i.client.PutItem(ctx, &dynamodb.PutItemInput{TableName: aws.String(i.store.protectedTable), Item: map[string]types.AttributeValue{"PK": s("P#" + p.ProjectID), "SK": s("SECRET#" + fake), "kind": s("runtimeOutput"), "schemaVersion": n(1)}})
	if err != nil {
		t.Fatal(err)
	}
	c = protectedConfig(value)
	c.Headers[0].SecretWrite = &SecretWrite{Action: "secretRef", SecretRef: &SecretReference{fake}}
	if _, err = i.store.CreateRequest(ctx, i.alice.UserID, p.ProjectID, c); err == nil {
		t.Fatal("runtime record eligible as saved secret")
	}
	i.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{TableName: aws.String(i.store.protectedTable), Key: key("P#"+p.ProjectID, "SECRET#"+fake)})
	project, _ := i.store.GetProject(ctx, i.alice.UserID, p.ProjectID)
	i.store.DeleteProject(ctx, i.alice.UserID, p.ProjectID, project.Version)
	if _, err = i.store.CleanupProject(ctx, p.ProjectID); err != nil {
		t.Fatal("HTTP fixture cleanup failed", err)
	}
}
