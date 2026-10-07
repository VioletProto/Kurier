//go:build integration && aws

package ownership

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// Actual AWS persistence with signed test identities. This deliberately does
// not claim Cognito authentication or API Gateway/browser validation.
func TestAWSControlPropagationAndConditionalWrites(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	table := os.Getenv("KURIER_AWS_TEST_TABLE")
	if !strings.HasPrefix(table, "kurier-dev-api-ControlTable-") || os.Getenv("KURIER_AWS_ACCOUNT_ID") != "747336059622" {
		t.Fatal("scoped development table/account required")
	}
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-east-2"), config.WithRetryer(func() aws.Retryer { return aws.NopRetryer{} }))
	if err != nil {
		t.Fatal("AWS configuration failed")
	}
	identity, err := sts.NewFromConfig(cfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil || aws.ToString(identity.Account) != "747336059622" || strings.HasSuffix(aws.ToString(identity.Arn), ":root") {
		t.Fatal("wrong account/root")
	}
	db := dynamodb.NewFromConfig(cfg)
	desc, err := db.DescribeTable(ctx, &dynamodb.DescribeTableInput{TableName: aws.String(table)})
	if err != nil || desc.Table.BillingModeSummary.BillingMode != types.BillingModePayPerRequest || len(desc.Table.GlobalSecondaryIndexes) != 3 {
		t.Fatal("unexpected table/index configuration")
	}
	for _, index := range desc.Table.GlobalSecondaryIndexes {
		if index.IndexStatus != types.IndexStatusActive || index.Projection.ProjectionType != types.ProjectionTypeKeysOnly {
			t.Fatal("index not active/keys-only")
		}
	}
	st := NewStore(db, table, "dev-api")
	auth := newAuthFixture(t)
	alice, err := st.ResolveUser(ctx, verified(t, auth, "alice"))
	if err != nil {
		t.Fatal(err)
	}
	bob, err := st.ResolveUser(ctx, verified(t, auth, "bob"))
	if err != nil {
		t.Fatal(err)
	}
	var projects []Project
	// Fixture user and identity rows are removed; project tombstones intentionally
	// remain as evidence of the actual cleanup protocol, without project names.
	t.Cleanup(func() {
		cleanupCtx, done := context.WithTimeout(context.Background(), 30*time.Second)
		defer done()
		for _, p := range projects {
			r, e := st.get(cleanupCtx, "P#"+p.ProjectID, "META")
			if e != nil || r.State == "deleted" {
				continue
			}
			if r.State == "active" {
				if _, e = st.DeleteProject(cleanupCtx, alice.UserID, p.ProjectID, r.Version); e != nil {
					t.Error("fixture project deletion failed")
					continue
				}
			}
			if _, e = st.CleanupProject(cleanupCtx, p.ProjectID); e != nil {
				t.Error("fixture project cleanup pending")
			}
		}
		for _, u := range []User{alice, bob} {
			r, e := st.get(cleanupCtx, "U#"+u.UserID, "META")
			if e != nil {
				continue
			}
			for _, k := range []map[string]types.AttributeValue{key(r.PK, "META"), key(identityKey(Identity{issuer: r.Issuer, subject: r.Subject}), "META")} {
				if _, e = db.DeleteItem(cleanupCtx, &dynamodb.DeleteItemInput{TableName: aws.String(table), Key: k}); e != nil {
					t.Error("fixture row cleanup failed")
				}
			}
		}
	})
	fixture := &integration{client: db, store: st, auth: auth, alice: alice, bob: bob}
	fixture.server, err = NewServer(st, auth.verifier, []byte(strings.Repeat("fixture-only", 3)))
	if err != nil {
		t.Fatal(err)
	}
	// exercise uses alice/bob token subjects, matching the fixture users here.
	projects = append(projects, exerciseSavedRequests(t, fixture)...)
	projects = nil // shared exercise completed its projects
	for n := 0; n < 3; n++ {
		p, e := st.CreateProject(ctx, alice.UserID, "AWS fixture")
		if e != nil {
			t.Fatal(e)
		}
		projects = append(projects, p)
	}
	waitIndex := func(index, field, partition string, wanted int) {
		t.Helper()
		start := time.Now()
		for {
			out, e := db.Query(ctx, &dynamodb.QueryInput{TableName: aws.String(table), IndexName: aws.String(index), KeyConditionExpression: aws.String(field + " = :pk"), ExpressionAttributeValues: map[string]types.AttributeValue{":pk": s(partition)}})
			if e != nil {
				t.Fatal("AWS index query failed")
			}
			if len(out.Items) == wanted {
				t.Logf("%s propagated in %s (%d candidates)", index, time.Since(start), wanted)
				return
			}
			if time.Since(start) > 30*time.Second {
				t.Fatal("AWS index did not converge")
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	waitIndex("GSI1", "LPK", "U#"+alice.UserID+"#PROJECT", 3)
	secret := []byte(strings.Repeat("fixture-only-key", 3))
	_, err = NewServer(st, auth.verifier, secret)
	if err != nil {
		t.Fatal(err)
	}
	page, err := st.ListProjects(ctx, alice.UserID, 1, "", secret)
	if err != nil || len(page.Items) != 1 || page.NextCursor == nil {
		t.Fatal("AWS first pagination page failed")
	}
	next, err := st.ListProjects(ctx, alice.UserID, 1, *page.NextCursor, secret)
	if err != nil || len(next.Items) != 1 || next.Items[0].ProjectID == page.Items[0].ProjectID {
		t.Fatal("AWS cursor continuation failed")
	}
	_, err = st.GetProject(ctx, bob.UserID, projects[0].ProjectID)
	var ae *APIError
	if !errors.As(err, &ae) || ae.Status != 404 {
		t.Fatal("cross-user read exposed project")
	}
	_, err = st.RenameProject(ctx, bob.UserID, projects[0].ProjectID, "denied", 0)
	if !errors.As(err, &ae) || ae.Status != 404 {
		t.Fatal("cross-user mutation exposed project")
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for n := 0; n < 2; n++ {
		wg.Go(func() { _, e := st.RenameProject(ctx, alice.UserID, projects[0].ProjectID, "race", 0); results <- e })
	}
	wg.Wait()
	close(results)
	wins := 0
	for e := range results {
		if e == nil {
			wins++
		} else if !errors.As(e, &ae) || (ae.Status != 412 && ae.Status != 409) {
			t.Fatal("unexpected concurrent result")
		}
	}
	if wins != 1 {
		t.Fatal("AWS CAS did not yield one winner")
	}
	// Submit a stale condition to AWS itself, with another write in the same
	// transaction, to establish cancellation and atomic rollback independently
	// of application pre-read rejection.
	rollbackKey := "AWSFIXTURE#" + newID()
	marker, _ := st.put(record{PK: rollbackKey, SK: "META", Kind: "fixture", SchemaVersion: 1}, "attribute_not_exists(PK)", nil)
	condition := types.TransactWriteItem{ConditionCheck: &types.ConditionCheck{TableName: aws.String(table), Key: key("P#"+projects[0].ProjectID, "META"), ConditionExpression: aws.String("#v = :old"), ExpressionAttributeNames: map[string]string{"#v": "version"}, ExpressionAttributeValues: map[string]types.AttributeValue{":old": n(0)}}}
	if e := st.transact(ctx, []types.TransactWriteItem{condition, marker}); !isConditional(e) {
		t.Fatal("AWS failed-condition cancellation not observed")
	}
	if _, e := st.get(ctx, rollbackKey, "META"); !errors.As(e, &ae) || ae.Status != 404 {
		t.Fatal("AWS transaction failed rollback")
	}
	p := projects[0]
	op, err := st.DeleteProject(ctx, alice.UserID, p.ProjectID, 1)
	if err != nil {
		t.Fatal(err)
	}
	waitIndex("GSI1", "LPK", "U#"+alice.UserID+"#PROJECT", 2)
	// Probe exactly the committed work, not a count shared with other users.
	start := time.Now()
	for {
		out, e := db.Query(ctx, &dynamodb.QueryInput{TableName: aws.String(table), IndexName: aws.String("GSI2"), KeyConditionExpression: aws.String("DPK = :pk"), ExpressionAttributeValues: map[string]types.AttributeValue{":pk": s(cleanupShard(p.ProjectID))}})
		if e != nil {
			t.Fatal(e)
		}
		found := false
		for _, item := range out.Items {
			if v, ok := item["PK"].(*types.AttributeValueMemberS); ok && v.Value == "P#"+p.ProjectID {
				found = true
			}
		}
		if found {
			t.Logf("GSI2 work propagated in %s", time.Since(start))
			break
		}
		if time.Since(start) > 30*time.Second {
			t.Fatal("GSI2 work did not propagate")
		}
		time.Sleep(100 * time.Millisecond)
	}
	// Unknown children must survive both manual and scheduled cloud cleanup.
	child := record{PK: "P#" + p.ProjectID, SK: "UNKNOWN#fixture", Kind: "unknown", SchemaVersion: 1}
	item, _ := encode(child)
	if _, err = db.PutItem(ctx, &dynamodb.PutItemInput{TableName: aws.String(table), Item: item}); err != nil {
		t.Fatal(err)
	}
	if _, err = st.CleanupProject(ctx, p.ProjectID); err == nil {
		t.Fatal("unknown child accepted")
	}
	if _, err = st.CleanupPending(ctx); err != nil {
		t.Fatal(err)
	}
	gate, err := st.get(ctx, child.PK, "META")
	if err != nil || gate.State != "deleting" {
		t.Fatal("unknown child completed")
	}
	if _, err = st.get(ctx, child.PK, child.SK); err != nil {
		t.Fatal("unknown child erased")
	}
	if _, err = db.DeleteItem(ctx, &dynamodb.DeleteItemInput{TableName: aws.String(table), Key: key(child.PK, child.SK)}); err != nil {
		t.Fatal(err)
	}
	// Resume using a new store, then repeat safely.
	resumed := NewStore(db, table, "dev-api")
	completed, err := resumed.CleanupProject(ctx, p.ProjectID)
	if err != nil || completed.State != "completed" {
		t.Fatal("AWS resume failed")
	}
	repeated, err := resumed.DeleteProject(ctx, alice.UserID, p.ProjectID, 1)
	if err != nil || repeated.OperationID != op.OperationID {
		t.Fatal("repeat delete changed operation")
	}
	tombstone, err := st.get(ctx, child.PK, "META")
	if err != nil || tombstone.Name != "" || tombstone.LPK != "" {
		t.Fatal("unsafe AWS tombstone")
	}
	// Leave durable work for the actual minute scheduler/Lambda role.
	// No manual invocation/cleanup can satisfy this check.
	scheduled := projects[1]
	live, err := st.CreateRequest(ctx, alice.UserID, scheduled.ProjectID, publicConfiguration())
	if err != nil {
		t.Fatal(err)
	}
	removed, err := st.CreateRequest(ctx, alice.UserID, scheduled.ProjectID, publicConfiguration())
	if err != nil {
		t.Fatal(err)
	}
	if err = st.DeleteRequest(ctx, alice.UserID, scheduled.ProjectID, removed.RequestID, 0); err != nil {
		t.Fatal(err)
	}
	if _, err = st.DeleteProject(ctx, alice.UserID, scheduled.ProjectID, 3); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	for {
		gate, err := st.get(ctx, "P#"+scheduled.ProjectID, "META")
		if err != nil {
			t.Fatal(err)
		}
		if gate.State == "deleted" {
			if gate.Operation == nil || gate.Operation.State != "completed" || gate.Name != "" {
				t.Fatal("invalid scheduled tombstone")
			}
			if _, e := st.get(ctx, "P#"+scheduled.ProjectID, "REQ#"+live.RequestID); !errors.As(e, &ae) || ae.Status != 404 {
				t.Fatal("scheduled Lambda did not drain live request")
			}
			if _, e := st.get(ctx, "P#"+scheduled.ProjectID, "REQ#"+removed.RequestID); !errors.As(e, &ae) || ae.Status != 404 {
				t.Fatal("scheduled Lambda did not drain request tombstone")
			}
			t.Logf("actual scheduled Lambda drained live request and request tombstone in %s", time.Since(started))
			break
		}
		if time.Since(started) > 90*time.Second {
			t.Fatal("scheduled cloud cleanup did not complete")
		}
		time.Sleep(2 * time.Second)
	}
	t.Log("actual AWS conditional cancellation/rollback, one-winner CAS, pagination, isolation, unknown-child preservation and resumed cleanup passed")
}
