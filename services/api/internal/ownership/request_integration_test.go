//go:build integration

package ownership

import (
	"context"
	"encoding/json"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func requestFrom(t *testing.T, w *httptest.ResponseRecorder) SavedRequest {
	t.Helper()
	var e struct {
		Data struct{ Request SavedRequest }
	}
	if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
		t.Fatal(err)
	}
	return e.Data.Request
}
func TestIntegrationSavedRequests(t *testing.T) { exerciseSavedRequests(t, localIntegration(t)) }

// Shared exercise runs unchanged against actual Local and AWS Control. Signed
// fixture identity is test-only; these HTTP adapters do not establish Cognito.
func exerciseSavedRequests(t *testing.T, i *integration) []Project {
	t.Helper()
	ctx := context.Background()
	p := i.project(t)
	projects := []Project{p}
	path := "/api/v1/projects/" + p.ProjectID + "/requests"
	w := i.request(t, "alice", "POST", path, `{"name":" Public ","method":"POST","url":"https://example.com/public","headers":[{"name":"Accept","value":"application/json","enabled":true,"sensitive":false},{"name":"Accept","value":"text/plain","enabled":false,"sensitive":false}],"body":{"type":"json","text":" {\"public\":true} ","sensitive":false}}`, "")
	expectHTTP(t, w, 201)
	saved := requestFrom(t, w)
	detail := path + "/" + saved.RequestID
	if saved.Revision != 0 || saved.Name != "Public" || saved.Body.Text != ` {"public":true} ` || w.Header().Get("ETag") != `"0"` || w.Header().Get("Location") != detail {
		t.Fatal("request creation semantics")
	}
	for _, route := range []struct{ method, path, body string }{{"GET", path, ""}, {"POST", path, "{}"}, {"GET", detail, ""}, {"PATCH", detail, "{}"}, {"DELETE", detail, ""}} {
		expectHTTP(t, i.request(t, "bob", route.method, route.path, route.body, ""), 404)
	}
	expectHTTP(t, i.request(t, "alice", "GET", detail, "", ""), 200)
	expectHTTP(t, i.request(t, "alice", "PATCH", detail, `{"name":"Changed"}`, ""), 428)
	expectHTTP(t, i.request(t, "alice", "PATCH", detail, `{"name":"Changed"}`, `W/"0"`), 400)
	expectHTTP(t, i.request(t, "alice", "PATCH", detail, `{"name":"Changed"}`, `"0"`), 200)
	expectHTTP(t, i.request(t, "alice", "PATCH", detail, `{"name":"Stale"}`, `"0"`), 412)
	expectHTTP(t, i.request(t, "alice", "DELETE", detail, "", `"0"`), 412)
	expectHTTP(t, i.request(t, "alice", "PATCH", detail, `{"headers":[],"body":null}`, `"1"`), 200)
	got, err := i.store.GetRequest(ctx, i.alice.UserID, p.ProjectID, saved.RequestID)
	if err != nil || got.Body != nil || len(got.Headers) != 0 || got.Method != "POST" || got.Revision != 2 {
		t.Fatal("partial clearing failed", err)
	}
	// Invalid/secret writes do not change request or gate and never echo inputs.
	before, _ := i.store.GetProject(ctx, i.alice.UserID, p.ProjectID)
	for _, body := range []string{`{"headers":[{"name":"Authorization","value":"fixture-secret","enabled":false,"sensitive":false}]}`, `{"body":{"type":"text","text":"fixture-secret","sensitive":true}}`, `{"unknown":"fixture-secret"}`, `{"name":"A","name":"B"}`} {
		w = i.request(t, "alice", "PATCH", detail, body, `"2"`)
		expectHTTP(t, w, 400)
		if strings.Contains(w.Body.String(), "fixture-secret") {
			t.Fatal("validation echoed sensitive input")
		}
	}
	after, _ := i.store.GetProject(ctx, i.alice.UserID, p.ProjectID)
	if before.Version != after.Version {
		t.Fatal("invalid writes advanced gate")
	}
	// Request writes invalidate existing project versions.
	_, err = i.store.DeleteProject(ctx, i.alice.UserID, p.ProjectID, 0)
	assertStatus(t, err, 412)
	expectHTTP(t, i.request(t, "alice", "DELETE", detail, "", `"2"`), 204)
	expectHTTP(t, i.request(t, "alice", "GET", detail, "", ""), 404)
	tomb, _ := i.store.get(ctx, "P#"+p.ProjectID, "REQ#"+saved.RequestID)
	if tomb.Kind != "requestTombstone" || tomb.ConfigurationJSON != "" || tomb.LPK != "" {
		t.Fatal("unsafe request tombstone")
	}
	// Enough records for interrupted two-chunk cleanup and bounded pagination.
	for n := 0; n < 24; n++ {
		if _, err = i.store.CreateRequest(ctx, i.alice.UserID, p.ProjectID, publicConfiguration()); err != nil {
			t.Fatal(err)
		}
	}
	secret := []byte(strings.Repeat("fixture-only", 3))
	var page RequestCollection
	deadline := time.Now().Add(30 * time.Second)
	for {
		page, err = i.store.ListRequests(ctx, i.alice.UserID, p.ProjectID, 5, "", secret)
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Items) == 5 && page.NextCursor != nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("request GSI did not converge")
		}
		time.Sleep(100 * time.Millisecond)
	}
	p2 := i.project(t)
	projects = append(projects, p2)
	_, err = i.store.ListRequests(ctx, i.alice.UserID, p2.ProjectID, 5, *page.NextCursor, secret)
	assertStatus(t, err, 400)
	_, err = i.store.ListRequests(ctx, i.bob.UserID, p.ProjectID, 5, *page.NextCursor, secret)
	assertStatus(t, err, 404)
	_, err = i.store.ListRequests(ctx, i.alice.UserID, p.ProjectID, 5, *page.NextCursor+"x", secret)
	assertStatus(t, err, 400)
	seen := map[string]bool{}
	for {
		for _, r := range page.Items {
			if seen[r.RequestID] {
				t.Fatal("duplicate pagination")
			}
			seen[r.RequestID] = true
		}
		if page.NextCursor == nil {
			break
		}
		page, err = i.store.ListRequests(ctx, i.alice.UserID, p.ProjectID, 5, *page.NextCursor, secret)
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(seen) != 24 {
		t.Fatal("pagination lost requests", len(seen))
	}
	// Real transaction commits followed by lost ACKs: no automatic write replay.
	uncertain := &uncertainDB{database: i.client}
	i.store.db = uncertain
	_, err = i.store.CreateRequest(ctx, i.alice.UserID, p2.ProjectID, publicConfiguration())
	assertStatus(t, err, 503)
	if uncertain.calls != 1 {
		t.Fatal("creation replayed")
	}
	i.store.db = i.client
	raw, err := i.client.Query(ctx, &dynamodb.QueryInput{TableName: aws.String(i.store.table), KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :prefix)"), ExpressionAttributeValues: map[string]types.AttributeValue{":pk": s("P#" + p2.ProjectID), ":prefix": s("REQ#")}, ConsistentRead: aws.Bool(true)})
	if err != nil || len(raw.Items) != 1 {
		t.Fatal("uncertain creation was not singular")
	}
	id := strings.TrimPrefix(raw.Items[0]["SK"].(*types.AttributeValueMemberS).Value, "REQ#")
	i.store.db = uncertain
	_, err = i.store.PatchRequest(ctx, i.alice.UserID, p2.ProjectID, id, []byte(`{"name":"Committed"}`), 0)
	assertStatus(t, err, 503)
	if uncertain.calls != 2 {
		t.Fatal("patch replayed")
	}
	err = i.store.DeleteRequest(ctx, i.alice.UserID, p2.ProjectID, id, 1)
	assertStatus(t, err, 503)
	if uncertain.calls != 3 {
		t.Fatal("delete replayed")
	}
	i.store.db = i.client
	_, err = i.store.GetRequest(ctx, i.alice.UserID, p2.ProjectID, id)
	assertStatus(t, err, 404)
	// Deterministic request create vs project deletion at the commit boundary.
	ran := false
	i.store.db = &hookDB{database: i.client, beforeTx: func(_ *dynamodb.TransactWriteItemsInput) {
		if ran {
			return
		}
		ran = true
		other := NewStore(i.client, i.store.table, i.store.stage)
		other.ConfigureProtected(i.store.protectedTable, i.store.cipher)
		current, e := other.GetProject(ctx, i.alice.UserID, p2.ProjectID)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = other.DeleteProject(ctx, i.alice.UserID, p2.ProjectID, current.Version); e != nil {
			t.Fatal(e)
		}
	}}
	_, err = i.store.CreateRequest(ctx, i.alice.UserID, p2.ProjectID, publicConfiguration())
	assertStatus(t, err, 404)
	i.store.db = i.client
	if _, err = i.store.CleanupProject(ctx, p2.ProjectID); err != nil {
		t.Fatal(err)
	}
	// Unknown REQ kinds must not prevent draining later recognized keys.
	unknown := record{PK: "P#" + p.ProjectID, SK: "REQ#000-unknown", Kind: "future", SchemaVersion: 2, ProjectID: p.ProjectID}
	i.write(t, unknown)
	extra := record{PK: unknown.PK, SK: "UNKNOWN#child", Kind: "future", SchemaVersion: 1}
	i.write(t, extra)
	current, _ := i.store.GetProject(ctx, i.alice.UserID, p.ProjectID)
	op, err := i.store.DeleteProject(ctx, i.alice.UserID, p.ProjectID, current.Version)
	if err != nil {
		t.Fatal(err)
	}
	expectHTTP(t, i.request(t, "alice", "GET", path, "", ""), 404)
	_, err = i.store.CleanupProject(ctx, p.ProjectID)
	assertStatus(t, err, 409)
	// Construct a new store to prove the next chunk uses the durable checkpoint.
	resumed := NewStore(i.client, i.store.table, i.store.stage)
	resumed.ConfigureProtected(i.store.protectedTable, i.store.cipher)
	_, err = resumed.CleanupProject(ctx, p.ProjectID)
	assertStatus(t, err, 409)
	remaining, e := i.client.Query(ctx, &dynamodb.QueryInput{TableName: aws.String(i.store.table), KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :prefix)"), ExpressionAttributeValues: map[string]types.AttributeValue{":pk": s(unknown.PK), ":prefix": s("REQ#")}, ConsistentRead: aws.Bool(true)})
	if e != nil || len(remaining.Items) != 1 {
		t.Fatal("known requests not drained on resume", e, len(remaining.Items))
	}
	for _, child := range []record{unknown, extra} {
		out, e := i.client.GetItem(ctx, &dynamodb.GetItemInput{TableName: aws.String(i.store.table), Key: key(child.PK, child.SK), ConsistentRead: aws.Bool(true)})
		if e != nil || len(out.Item) == 0 {
			t.Fatal("unknown child erased")
		}
		if _, e = i.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{TableName: aws.String(i.store.table), Key: key(child.PK, child.SK)}); e != nil {
			t.Fatal(e)
		}
	}
	done, err := resumed.CleanupProject(ctx, p.ProjectID)
	if err != nil || done.State != "completed" || done.OperationID != op.OperationID {
		t.Fatal("cleanup failed after unknown child removed", err)
	}
	return projects
}

func TestIntegrationRequestRevisionRaceAndCleanupFence(t *testing.T) {
	i := localIntegration(t)
	ctx := context.Background()
	p := i.project(t)
	saved, err := i.store.CreateRequest(ctx, i.alice.UserID, p.ProjectID, publicConfiguration())
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	for n := 0; n < 2; n++ {
		go func() {
			<-start
			_, e := i.store.PatchRequest(ctx, i.alice.UserID, p.ProjectID, saved.RequestID, []byte(`{"name":"race"}`), 0)
			results <- e
		}()
	}
	close(start)
	wins := 0
	for n := 0; n < 2; n++ {
		e := <-results
		if e == nil {
			wins++
		} else {
			ae, ok := e.(*APIError)
			if !ok || (ae.Status != 409 && ae.Status != 412) {
				t.Fatal(e)
			}
		}
	}
	if wins != 1 {
		t.Fatal("request revision race did not have one winner")
	}
	current, _ := i.store.GetProject(ctx, i.alice.UserID, p.ProjectID)
	ran := false
	i.store.db = &hookDB{database: i.client, beforeTx: func(_ *dynamodb.TransactWriteItemsInput) {
		if ran {
			return
		}
		ran = true
		other := NewStore(i.client, i.store.table, i.store.stage)
		other.ConfigureProtected(i.store.protectedTable, i.store.cipher)
		if _, e := other.DeleteProject(ctx, i.alice.UserID, p.ProjectID, current.Version); e != nil {
			t.Fatal(e)
		}
	}}
	_, err = i.store.PatchRequest(ctx, i.alice.UserID, p.ProjectID, saved.RequestID, []byte(`{"name":"must not commit"}`), 1)
	assertStatus(t, err, 404)
	i.store.db = i.client
	r, _ := i.store.get(ctx, "P#"+p.ProjectID, "REQ#"+saved.RequestID)
	if r.Revision != 1 {
		t.Fatal("request patch bypassed deletion gate")
	}
	// Advance gate after cleanup's partition proof; final tombstone CAS must fail.
	ran = false
	i.store.db = &hookDB{database: i.client, afterQuery: func(input *dynamodb.QueryInput) {
		if ran || aws.ToString(input.KeyConditionExpression) != "PK = :pk" {
			return
		}
		ran = true
		gate, e := i.store.get(ctx, "P#"+p.ProjectID, "META")
		if e != nil {
			t.Fatal(e)
		}
		gate.Version++
		i.write(t, gate)
		i.write(t, record{PK: gate.PK, SK: "UNKNOWN#racing", Kind: "unknown", SchemaVersion: 1})
	}}
	_, err = i.store.CleanupProject(ctx, p.ProjectID)
	assertStatus(t, err, 409)
	i.store.db = i.client
	gate, _ := i.store.get(ctx, "P#"+p.ProjectID, "META")
	if gate.State != "deleting" {
		t.Fatal("cleanup weakened completion gate")
	}
}

func TestIntegrationRequestListByteBudgetAndCapabilityGate(t *testing.T) {
	i := localIntegration(t)
	ctx := context.Background()
	p := i.project(t)
	c := publicConfiguration()
	c.Body = &RequestBody{Type: "text", Text: ""}
	encoded, _ := configurationJSON(c)
	c.Body.Text = strings.Repeat("<", savedConfigurationLimit-len(encoded))
	for n := 0; n < 10; n++ {
		if _, e := i.store.CreateRequest(ctx, i.alice.UserID, p.ProjectID, c); e != nil {
			t.Fatal(e)
		}
	}
	secret := []byte(strings.Repeat("fixture", 6))
	first, err := i.store.ListRequests(ctx, i.alice.UserID, p.ProjectID, 100, "", secret)
	if err != nil || len(first.Items) == 0 || len(first.Items) >= 10 || first.NextCursor == nil {
		t.Fatal("large configurations not byte bounded", err, len(first.Items))
	}
	second, err := i.store.ListRequests(ctx, i.alice.UserID, p.ProjectID, 100, *first.NextCursor, secret)
	if err != nil || len(first.Items)+len(second.Items) != 10 {
		t.Fatal("byte continuation lost records", err)
	}
	stage, _ := i.store.activeStage(ctx)
	stage.SavedRequestsSchemaVersion = 0
	stage.LocalEmptyProjectsOnly = true
	i.write(t, stage)
	_, err = i.store.CreateRequest(ctx, i.alice.UserID, p.ProjectID, publicConfiguration())
	assertStatus(t, err, 503)
	if _, err = i.store.GetProject(ctx, i.alice.UserID, p.ProjectID); err != nil {
		t.Fatal("legacy stage lost project compatibility")
	}
	stage.SavedRequestsSchemaVersion = 2
	i.write(t, stage)
	_, err = i.store.GetProject(ctx, i.alice.UserID, p.ProjectID)
	assertStatus(t, err, 503)
}
