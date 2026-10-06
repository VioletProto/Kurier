//go:build integration

package ownership

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type integration struct {
	store      *Store
	client     *dynamodb.Client
	auth       *authFixture
	server     *Server
	alice, bob User
}

func localIntegration(t *testing.T) *integration {
	t.Helper()
	endpoint := os.Getenv("KURIER_DYNAMODB_TEST_ENDPOINT")
	if endpoint == "" {
		t.Fatal("integration tag requires KURIER_DYNAMODB_TEST_ENDPOINT pointing to actual DynamoDB Local")
	}
	client, err := LocalClient(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, err = client.ListTables(ctx, &dynamodb.ListTablesInput{Limit: aws.Int32(1)})
		cancel()
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("DynamoDB Local did not become ready")
		}
		time.Sleep(100 * time.Millisecond)
	}
	table := "kurier-test-" + newID()
	ctx := context.Background()
	if err = InitLocal(ctx, client, table, "local"); err != nil {
		t.Fatal(err)
	}
	i := &integration{client: client, store: NewStore(client, table, "local"), auth: newAuthFixture(t)}
	i.alice, err = i.store.ResolveUser(ctx, verified(t, i.auth, "alice"))
	if err != nil {
		t.Fatal(err)
	}
	i.bob, err = i.store.ResolveUser(ctx, verified(t, i.auth, "bob"))
	if err != nil {
		t.Fatal(err)
	}
	i.server, err = NewServer(i.store, i.auth.verifier, []byte(strings.Repeat("test-only-cursor-key", 2)))
	if err != nil {
		t.Fatal(err)
	}
	return i
}
func (i *integration) request(t *testing.T, sub, method, path, body, tag string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if sub != "" {
		r.Header.Set("Authorization", "Bearer "+i.auth.token(t, sub, nil))
	}
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	if tag != "" {
		r.Header.Set("If-Match", tag)
	}
	w := httptest.NewRecorder()
	i.server.ServeHTTP(w, r)
	return w
}
func expectHTTP(t *testing.T, w *httptest.ResponseRecorder, status int) {
	t.Helper()
	if w.Code != status {
		t.Fatalf("wanted %d got %d: %s", status, w.Code, w.Body.String())
	}
	if status >= 400 {
		var envelope struct{ Error APIError }
		if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		codes := map[int]string{401: "unauthenticated", 403: "forbidden", 404: "not_found", 412: "precondition_failed", 428: "precondition_required", 503: "service_unavailable"}
		if code := codes[status]; code != "" && envelope.Error.Code != code {
			t.Fatal("wrong error code", envelope.Error.Code)
		}
		if envelope.Error.RequestID == "" || envelope.Error.Details == nil {
			t.Fatal("missing safe error envelope fields")
		}
	}
}
func projectFrom(t *testing.T, w *httptest.ResponseRecorder) Project {
	t.Helper()
	var envelope struct{ Data struct{ Project Project } }
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope.Data.Project
}
func opFrom(t *testing.T, w *httptest.ResponseRecorder) DeletionOperation {
	t.Helper()
	var envelope struct {
		Data struct{ DeletionOperation DeletionOperation }
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope.Data.DeletionOperation
}
func (i *integration) project(t *testing.T) Project {
	t.Helper()
	p, err := i.store.CreateProject(context.Background(), i.alice.UserID, "Test")
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func (i *integration) write(t *testing.T, r record) {
	t.Helper()
	item, err := encode(r)
	if err != nil {
		t.Fatal(err)
	}
	_, err = i.client.PutItem(context.Background(), &dynamodb.PutItemInput{TableName: aws.String(i.store.table), Item: item})
	if err != nil {
		t.Fatal(err)
	}
}

func TestIntegrationAllSevenRoutesAndOwnership(t *testing.T) {
	i := localIntegration(t)
	me := i.request(t, "alice", "GET", "/api/v1/users/me", "", "")
	expectHTTP(t, me, 200)
	if strings.Contains(me.Body.String(), "issuer") || strings.Contains(me.Body.String(), "alice") || !strings.Contains(me.Body.String(), `"displayName":""`) {
		t.Fatal(me.Body.String())
	}
	w := i.request(t, "alice", "POST", "/api/v1/projects", `{"name":"  My API  "}`, "")
	expectHTTP(t, w, 201)
	p := projectFrom(t, w)
	path := "/api/v1/projects/" + p.ProjectID
	if w.Header().Get("ETag") != `"0"` || w.Header().Get("Location") != path || p.Name != "My API" {
		t.Fatal("creation headers/name")
	}
	w = i.request(t, "alice", "GET", path, "", "")
	expectHTTP(t, w, 200)
	w = i.request(t, "alice", "GET", "/api/v1/projects?limit=25", "", "")
	expectHTTP(t, w, 200)
	for _, method := range []string{"GET", "PATCH", "DELETE"} {
		w = i.request(t, "bob", method, path, `{"name":"evil"}`, "")
		expectHTTP(t, w, 404)
		if strings.Contains(w.Body.String(), "My API") {
			t.Fatal("foreign data leaked")
		}
	}
	w = i.request(t, "alice", "PATCH", path, `{"name":"Renamed"}`, "")
	expectHTTP(t, w, 428)
	w = i.request(t, "alice", "PATCH", path, `{"name":"Renamed"}`, `W/"0"`)
	expectHTTP(t, w, 400)
	w = i.request(t, "alice", "PATCH", path, `{"name":"Renamed"}`, `"0"`)
	expectHTTP(t, w, 200)
	if projectFrom(t, w).Version != 1 || w.Header().Get("ETag") != `"1"` {
		t.Fatal("rename version")
	}
	w = i.request(t, "alice", "PATCH", path, `{"name":"Stale"}`, `"0"`)
	expectHTTP(t, w, 412)
	w = i.request(t, "alice", "DELETE", path, "", `"1"`)
	expectHTTP(t, w, 202)
	op := opFrom(t, w)
	opPath := w.Header().Get("Location")
	if op.State != "accepted" || op.CompletedAt != nil || op.RetryAfterSeconds != 2 {
		t.Fatal(op)
	}
	w = i.request(t, "alice", "GET", path, "", "")
	expectHTTP(t, w, 404)
	w = i.request(t, "alice", "PATCH", path, `{"name":"Resurrect"}`, `"1"`)
	expectHTTP(t, w, 404)
	w = i.request(t, "alice", "GET", opPath, "", "")
	expectHTTP(t, w, 200)
	w = i.request(t, "bob", "GET", opPath, "", "")
	expectHTTP(t, w, 404)
	w = i.request(t, "alice", "DELETE", path, "", `"1"`)
	expectHTTP(t, w, 202)
	if opFrom(t, w).OperationID != op.OperationID {
		t.Fatal("duplicate delete")
	}
	completed, err := i.store.CleanupProject(context.Background(), p.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	if completed.State != "completed" || completed.CompletedAt == nil || completed.RetryAfterSeconds != 0 {
		t.Fatal(completed)
	}
	w = i.request(t, "alice", "DELETE", path, "", `"1"`)
	expectHTTP(t, w, 202)
	if opFrom(t, w).OperationID != op.OperationID {
		t.Fatal("post-completion replay")
	}
	w = i.request(t, "alice", "DELETE", path, "", `"0"`)
	expectHTTP(t, w, 412)
	w = i.request(t, "alice", "GET", opPath, "", "")
	expectHTTP(t, w, 200)
	raw, err := i.client.GetItem(context.Background(), &dynamodb.GetItemInput{TableName: aws.String(i.store.table), Key: key("P#"+p.ProjectID, "META"), ConsistentRead: aws.Bool(true)})
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"name", "createdAt", "updatedAt", "LPK", "LSK", "issuer", "sub"} {
		if _, ok := raw.Item[field]; ok {
			t.Fatalf("tombstone retained %s", field)
		}
	}
	_, err = i.store.get(context.Background(), "P#"+p.ProjectID, "WORK#delete#"+op.OperationID)
	assertStatus(t, err, 404)
	w = i.request(t, "", "GET", "/healthz", "", "")
	expectHTTP(t, w, 200)
	w = i.request(t, "", "GET", "/api/v1/users/me", "", "")
	expectHTTP(t, w, 401)
	// Local admin update, not a product endpoint.
	u, err := i.store.get(context.Background(), "U#"+i.alice.UserID, "META")
	if err != nil {
		t.Fatal(err)
	}
	u.Disabled = true
	u.Version++
	i.write(t, u)
	w = i.request(t, "alice", "GET", "/api/v1/users/me", "", "")
	expectHTTP(t, w, 403)
	w = i.request(t, "alice", "POST", "/api/v1/projects", `{"name":"Blocked"}`, "")
	expectHTTP(t, w, 403)
}

func TestIntegrationConcurrentIdentityProvisioning(t *testing.T) {
	i := localIntegration(t)
	identity := verified(t, i.auth, "same-new-subject")
	var wg sync.WaitGroup
	users := make(chan User, 8)
	errs := make(chan error, 8)
	for n := 0; n < 8; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			u, err := i.store.ResolveUser(context.Background(), identity)
			users <- u
			errs <- err
		}()
	}
	wg.Wait()
	close(users)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	id := ""
	for u := range users {
		if id == "" {
			id = u.UserID
		} else if u.UserID != id {
			t.Fatal("multiple application users")
		}
	}
	result, err := i.client.Scan(context.Background(), &dynamodb.ScanInput{TableName: aws.String(i.store.table), FilterExpression: aws.String("issuer = :issuer AND #sub = :sub"), ExpressionAttributeNames: map[string]string{"#sub": "sub"}, ExpressionAttributeValues: map[string]types.AttributeValue{":issuer": s(identity.issuer), ":sub": s(identity.subject)}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("identity/user transaction left %d items", len(result.Items))
	}
}
func TestIntegrationConditionalConflictAndDeleteRace(t *testing.T) {
	i := localIntegration(t)
	p := i.project(t)
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, name := range []string{"A", "B"} {
		go func(name string) {
			<-start
			_, err := i.store.RenameProject(context.Background(), i.alice.UserID, p.ProjectID, name, 0)
			results <- err
		}(name)
	}
	close(start)
	wins := 0
	for n := 0; n < 2; n++ {
		err := <-results
		if err == nil {
			wins++
		} else {
			assertStatus(t, err, 412)
		}
	}
	if wins != 1 {
		t.Fatal("rename did not have one winner")
	}
	for attempt := 0; attempt < 5; attempt++ {
		p = i.project(t)
		start = make(chan struct{})
		results = make(chan error, 2)
		go func() {
			<-start
			_, err := i.store.RenameProject(context.Background(), i.alice.UserID, p.ProjectID, "Race", 0)
			results <- err
		}()
		go func() {
			<-start
			_, err := i.store.DeleteProject(context.Background(), i.alice.UserID, p.ProjectID, 0)
			results <- err
		}()
		close(start)
		wins = 0
		for n := 0; n < 2; n++ {
			err := <-results
			if err == nil {
				wins++
			} else {
				var ae *APIError
				if !errors.As(err, &ae) || (ae.Status != 412 && ae.Status != 404) {
					t.Fatal(err)
				}
			}
		}
		if wins != 1 {
			t.Fatal("delete/rename both won")
		}
	}
}

// hookDB injects a real DynamoDB write at a transaction boundary. It does not
// simulate DynamoDB results: the SDK still sends every request to Local.
type hookDB struct {
	database
	beforeTx   func(*dynamodb.TransactWriteItemsInput)
	afterQuery func(*dynamodb.QueryInput)
}

func (h *hookDB) TransactWriteItems(ctx context.Context, in *dynamodb.TransactWriteItemsInput, opts ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error) {
	if h.beforeTx != nil {
		h.beforeTx(in)
	}
	return h.database.TransactWriteItems(ctx, in, opts...)
}
func (h *hookDB) Query(ctx context.Context, in *dynamodb.QueryInput, opts ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
	result, err := h.database.Query(ctx, in, opts...)
	if h.afterQuery != nil {
		h.afterQuery(in)
	}
	return result, err
}
func TestIntegrationTransactionRollbackOnDisabledUser(t *testing.T) {
	i := localIntegration(t)
	ctx := context.Background()
	ran := false
	i.store.db = &hookDB{database: i.client, beforeTx: func(_ *dynamodb.TransactWriteItemsInput) {
		if ran {
			return
		}
		ran = true
		u, err := i.store.get(ctx, "U#"+i.alice.UserID, "META")
		if err != nil {
			t.Fatal(err)
		}
		u.Disabled = true
		u.Version++
		i.write(t, u)
	}}
	_, err := i.store.CreateProject(ctx, i.alice.UserID, "Must not persist")
	assertStatus(t, err, 403)
	result, err := i.client.Scan(ctx, &dynamodb.ScanInput{TableName: aws.String(i.store.table), FilterExpression: aws.String("#kind = :kind"), ExpressionAttributeNames: map[string]string{"#kind": "kind"}, ExpressionAttributeValues: map[string]types.AttributeValue{":kind": s("project")}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 0 {
		t.Fatal("part of rejected transaction persisted")
	}
}
func TestIntegrationInterruptedCleanupUnexpectedChildrenAndFence(t *testing.T) {
	i := localIntegration(t)
	ctx := context.Background()
	p := i.project(t)
	op, err := i.store.DeleteProject(ctx, i.alice.UserID, p.ProjectID, 0)
	if err != nil {
		t.Fatal(err)
	}
	r, err := i.store.get(ctx, "P#"+p.ProjectID, "META")
	if err != nil {
		t.Fatal(err)
	}
	stage, err := i.store.activeStage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = i.store.markDeleting(ctx, r, stage); err != nil {
		t.Fatal(err)
	}
	// New process/store resumes after a crash between durable phases.
	restarted := NewStore(i.client, i.store.table, "local")
	completed, err := restarted.CleanupProject(ctx, p.ProjectID)
	if err != nil || completed.State != "completed" || completed.OperationID != op.OperationID {
		t.Fatal(completed, err)
	}
	stored, err := i.client.GetItem(ctx, &dynamodb.GetItemInput{TableName: aws.String(i.store.table), Key: key("P#"+p.ProjectID, "META"), ConsistentRead: aws.Bool(true)})
	if err != nil {
		t.Fatal(err)
	}
	initial, ok := stored.Item["initiatingVersion"].(*types.AttributeValueMemberN)
	if !ok || initial.Value != "0" {
		t.Fatal("minimal tombstone lost explicit initiating version zero")
	}
	p = i.project(t)
	_, err = i.store.DeleteProject(ctx, i.alice.UserID, p.ProjectID, 0)
	if err != nil {
		t.Fatal(err)
	}
	child := record{PK: "P#" + p.ProjectID, SK: "UNEXPECTED#child", Kind: "unexpected", SchemaVersion: 1}
	i.write(t, child)
	_, err = i.store.CleanupProject(ctx, p.ProjectID)
	assertStatus(t, err, 409)
	r, err = i.store.get(ctx, child.PK, "META")
	if err != nil || r.State != "deleting" {
		t.Fatal("unexpected child falsely completed")
	}
	if _, err = i.store.get(ctx, child.PK, child.SK); err != nil {
		t.Fatal("unexpected child erased")
	}
	p = i.project(t)
	_, err = i.store.DeleteProject(ctx, i.alice.UserID, p.ProjectID, 0)
	if err != nil {
		t.Fatal(err)
	}
	ran := false
	i.store.db = &hookDB{database: i.client, afterQuery: func(in *dynamodb.QueryInput) {
		if ran || in.IndexName != nil {
			return
		}
		ran = true
		gate, e := i.store.get(ctx, "P#"+p.ProjectID, "META")
		if e != nil {
			t.Fatal(e)
		}
		child = record{PK: gate.PK, SK: "UNEXPECTED#racer", Kind: "unexpected", SchemaVersion: 1}
		childItem, e := encode(child)
		if e != nil {
			t.Fatal(e)
		}
		_, e = i.client.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{TransactItems: []types.TransactWriteItem{
			{Update: &types.Update{TableName: aws.String(i.store.table), Key: key(gate.PK, "META"), UpdateExpression: aws.String("SET #version = :next"), ConditionExpression: aws.String("#version = :v"), ExpressionAttributeNames: map[string]string{"#version": "version"}, ExpressionAttributeValues: map[string]types.AttributeValue{":next": n(gate.Version + 1), ":v": n(gate.Version)}}},
			{Put: &types.Put{TableName: aws.String(i.store.table), Item: childItem}},
		}})
		if e != nil {
			t.Fatal(e)
		}
	}}
	_, err = i.store.CleanupProject(ctx, p.ProjectID)
	assertStatus(t, err, 409)
	r, err = i.store.get(ctx, "P#"+p.ProjectID, "META")
	if err != nil || r.State != "deleting" {
		t.Fatal("racing writer bypassed cleanup fence")
	}
	_, err = i.store.CleanupProject(ctx, p.ProjectID)
	assertStatus(t, err, 409)
}

func TestIntegrationPaginationCursorAndStaleCandidates(t *testing.T) {
	i := localIntegration(t)
	ctx := context.Background()
	secret := []byte(strings.Repeat("test-only", 4))
	base := time.Now().Add(-time.Hour)
	for n := 0; n < 7; n++ {
		i.store.now = func() time.Time { return base.Add(time.Duration(n) * time.Second) }
		_ = i.project(t)
	}
	i.store.now = time.Now
	first, err := i.store.ListProjects(ctx, i.alice.UserID, 2, "", secret)
	if err != nil || len(first.Items) != 2 || first.NextCursor == nil {
		t.Fatal(first, err)
	}
	if first.Items[0].CreatedAt < first.Items[1].CreatedAt {
		t.Fatal("not newest first")
	}
	_, err = i.store.ListProjects(ctx, i.bob.UserID, 2, *first.NextCursor, secret)
	assertStatus(t, err, 400)
	_, err = i.store.ListProjects(ctx, i.alice.UserID, 2, *first.NextCursor+"x", secret)
	assertStatus(t, err, 400)
	boundary, err := decodeCursor(*first.NextCursor, secret)
	if err != nil {
		t.Fatal(err)
	}
	firstTime, err := time.Parse("2006-01-02T15:04:05.000000000Z", strings.Split(boundary.HighWater, "#")[0])
	if err != nil {
		t.Fatal(err)
	}
	i.store.now = func() time.Time { return firstTime.Add(time.Second) }
	_ = i.project(t) // New entries above the first page's high-water are excluded.
	i.store.now = time.Now
	seen := map[string]bool{}
	page := first
	for {
		for _, p := range page.Items {
			if seen[p.ProjectID] {
				t.Fatal("duplicate page entry")
			}
			seen[p.ProjectID] = true
		}
		if page.NextCursor == nil {
			break
		}
		page, err = i.store.ListProjects(ctx, i.alice.UserID, 2, *page.NextCursor, secret)
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(seen) != 7 {
		t.Fatal("lost projects", len(seen))
	}
	i.store.now = func() time.Time { return time.Now().Add(25 * time.Hour) }
	_, err = i.store.ListProjects(ctx, i.alice.UserID, 2, *first.NextCursor, secret)
	assertStatus(t, err, 400)
	i.store.now = time.Now
	stage, err := i.store.get(ctx, "STAGE#local", "META")
	if err != nil {
		t.Fatal(err)
	}
	stage.RecoveryGeneration = newID()
	i.write(t, stage)
	_, err = i.store.ListProjects(ctx, i.alice.UserID, 2, *first.NextCursor, secret)
	assertStatus(t, err, 400)
	// Seed stale index candidates explicitly; Local cannot establish AWS GSI lag.
	result, err := i.client.Query(ctx, &dynamodb.QueryInput{TableName: aws.String(i.store.table), IndexName: aws.String("GSI1"), KeyConditionExpression: aws.String("LPK = :owner"), ExpressionAttributeValues: map[string]types.AttributeValue{":owner": s("U#" + i.alice.UserID + "#PROJECT")}})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range result.Items {
		var k cursorKey
		if attributevalue.UnmarshalMap(item, &k) != nil {
			t.Fatal("bad keys")
		}
		r, e := i.store.get(ctx, k.PK, "META")
		if e != nil {
			t.Fatal(e)
		}
		r.State = "deleted"
		r.Version++
		i.write(t, r)
	}
	page, err = i.store.ListProjects(ctx, i.alice.UserID, 1, "", secret)
	if err != nil || len(page.Items) != 0 || page.NextCursor == nil {
		t.Fatal("bounded filtered empty page", page, err)
	}
	for _, query := range []string{"limit=0", "limit=101", "limit=x", "limit=1&limit=2", "cursor=", "offset=1", "cursor=%zz"} {
		w := i.request(t, "alice", "GET", "/api/v1/projects?"+query, "", "")
		expectHTTP(t, w, 400)
	}
}

type uncertainDB struct {
	database
	calls int
}

func (d *uncertainDB) TransactWriteItems(ctx context.Context, in *dynamodb.TransactWriteItemsInput, opts ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error) {
	d.calls++
	out, err := d.database.TransactWriteItems(ctx, in, opts...)
	if err != nil {
		return out, err
	}
	return nil, fmt.Errorf("simulated lost acknowledgment after real Local commit")
}
func TestIntegrationUncertainCreationIsNotRetriedAndDeletionReadsBack(t *testing.T) {
	i := localIntegration(t)
	ctx := context.Background()
	wrapped := &uncertainDB{database: i.client}
	i.store.db = wrapped
	_, err := i.store.CreateProject(ctx, i.alice.UserID, "Uncertain")
	assertStatus(t, err, 503)
	if wrapped.calls != 1 {
		t.Fatal("automatic creation retry")
	}
	result, err := i.client.Query(ctx, &dynamodb.QueryInput{TableName: aws.String(i.store.table), IndexName: aws.String("GSI1"), KeyConditionExpression: aws.String("LPK = :owner"), ExpressionAttributeValues: map[string]types.AttributeValue{":owner": s("U#" + i.alice.UserID + "#PROJECT")}})
	if err != nil || len(result.Items) != 1 {
		t.Fatal("committed creation did not remain singular", err)
	}
	var k cursorKey
	if attributevalue.UnmarshalMap(result.Items[0], &k) != nil {
		t.Fatal("bad key")
	}
	projectID := strings.TrimPrefix(k.PK, "P#")
	_, err = i.store.RenameProject(ctx, i.alice.UserID, projectID, "Acknowledgment lost", 0)
	assertStatus(t, err, 503) // Commit uncertainty is not a failed If-Match.
	if wrapped.calls != 2 {
		t.Fatal("uncertain rename was retried")
	}
	op, err := i.store.DeleteProject(ctx, i.alice.UserID, projectID, 1)
	if err != nil || op.State != "accepted" {
		t.Fatal("lost DELETE acknowledgment not reconciled", err)
	}
	if wrapped.calls != 3 {
		t.Fatal("DELETE retried acceptance")
	}
	repeated, err := i.store.DeleteProject(ctx, i.alice.UserID, projectID, 1)
	if err != nil || repeated.OperationID != op.OperationID || wrapped.calls != 3 {
		t.Fatal("repeat delete changed operation")
	}
	if err = i.store.transact(ctx, make([]types.TransactWriteItem, 5)); err == nil || wrapped.calls != 3 {
		t.Fatal("slice transaction budget not enforced before I/O")
	}
}
func TestIntegrationRecoveringStageDeniesAndDoesNotReactivate(t *testing.T) {
	i := localIntegration(t)
	ctx := context.Background()
	p := i.project(t)
	stage, err := i.store.get(ctx, "STAGE#local", "META")
	if err != nil {
		t.Fatal(err)
	}
	stage.State = "recovering"
	i.write(t, stage)
	if err = InitLocal(ctx, i.client, i.store.table, "local"); err != nil {
		t.Fatal(err)
	}
	w := i.request(t, "alice", "GET", "/api/v1/projects/"+p.ProjectID, "", "")
	expectHTTP(t, w, 503)
	w = i.request(t, "alice", "POST", "/api/v1/projects", `{"name":"Blocked"}`, "")
	expectHTTP(t, w, 503)
	w = i.request(t, "", "GET", "/healthz", "", "")
	expectHTTP(t, w, 200)
}
func TestIntegrationCleanupCLI(t *testing.T) {
	i := localIntegration(t)
	p := i.project(t)
	_, err := i.store.DeleteProject(context.Background(), i.alice.UserID, p.ProjectID, 0)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", "../..", "-cleanup-project", p.ProjectID)
	cmd.Env = append(os.Environ(), "KURIER_DYNAMODB_ENDPOINT="+os.Getenv("KURIER_DYNAMODB_TEST_ENDPOINT"), "KURIER_CONTROL_TABLE="+i.store.table, "KURIER_STAGE=local")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cleanup command: %v: %s", err, output)
	}
	if !strings.Contains(string(output), `"state":"completed"`) {
		t.Fatal(string(output))
	}
	// Repeat the runnable command against the retained completed tombstone.
	cmd = exec.Command("go", "run", "../..", "-cleanup-project", p.ProjectID)
	cmd.Env = append(os.Environ(), "KURIER_DYNAMODB_ENDPOINT="+os.Getenv("KURIER_DYNAMODB_TEST_ENDPOINT"), "KURIER_CONTROL_TABLE="+i.store.table, "KURIER_STAGE=local")
	if output, err = cmd.CombinedOutput(); err != nil {
		t.Fatalf("repeat cleanup: %v %s", err, output)
	}
	p = i.project(t)
	if _, err = i.store.DeleteProject(context.Background(), i.alice.UserID, p.ProjectID, 0); err != nil {
		t.Fatal(err)
	}
	i.write(t, record{PK: "P#" + p.ProjectID, SK: "UNEXPECTED#cli", Kind: "unexpected", SchemaVersion: 1})
	cmd = exec.Command("go", "run", "../..", "-cleanup-project", p.ProjectID)
	cmd.Env = append(os.Environ(), "KURIER_DYNAMODB_ENDPOINT="+os.Getenv("KURIER_DYNAMODB_TEST_ENDPOINT"), "KURIER_CONTROL_TABLE="+i.store.table, "KURIER_STAGE=local")
	output, err = cmd.CombinedOutput()
	if err == nil || strings.Contains(string(output), `"state":"completed"`) {
		t.Fatal("CLI falsely completed unexpected children", string(output))
	}
}
