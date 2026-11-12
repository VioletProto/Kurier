//go:build integration

package browsersession

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dt "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func TestIntegrationDynamoBootstrapCancellationAndGenerationCAS(t *testing.T) {
	endpoint := os.Getenv("KURIER_DYNAMODB_TEST_ENDPOINT")
	if endpoint == "" {
		t.Skip("DynamoDB Local endpoint not configured")
	}
	if !strings.HasPrefix(endpoint, "http://127.0.0.1:") {
		t.Fatal("test requires explicit loopback DynamoDB Local")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cfg, e := config.LoadDefaultConfig(ctx, config.WithRegion("us-east-2"), config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("local", "local", "")), config.WithRetryer(func() aws.Retryer { return aws.NopRetryer{} }))
	if e != nil {
		t.Fatal(e)
	}
	db := dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) { o.BaseEndpoint = aws.String(endpoint) })
	suffix, _ := randomProof()
	table := "kurier-session-test-" + suffix[:16]
	control := table + "-control"
	for _, name := range []string{table, control} {
		schema := []dt.KeySchemaElement{{AttributeName: aws.String("PK"), KeyType: dt.KeyTypeHash}}
		defs := []dt.AttributeDefinition{{AttributeName: aws.String("PK"), AttributeType: dt.ScalarAttributeTypeS}}
		if name == control {
			schema = append(schema, dt.KeySchemaElement{AttributeName: aws.String("SK"), KeyType: dt.KeyTypeRange})
			defs = append(defs, dt.AttributeDefinition{AttributeName: aws.String("SK"), AttributeType: dt.ScalarAttributeTypeS})
		}
		if _, e = db.CreateTable(ctx, &dynamodb.CreateTableInput{TableName: aws.String(name), BillingMode: dt.BillingModePayPerRequest, KeySchema: schema, AttributeDefinitions: defs}); e != nil {
			t.Fatal(e)
		}
		t.Cleanup(func() {
			_, _ = db.DeleteTable(context.Background(), &dynamodb.DeleteTableInput{TableName: aws.String(name)})
		})
	}
	putStage := func(gen, state string) {
		t.Helper()
		_, e := db.PutItem(ctx, &dynamodb.PutItemInput{TableName: aws.String(control), Item: map[string]dt.AttributeValue{"PK": avs("STAGE#fixture"), "SK": avs("META"), "state": avs(state), "recoveryGeneration": avs(gen)}})
		if e != nil {
			t.Fatal(e)
		}
	}
	putStage("fixture-generation", "active")
	store := &DynamoStore{Client: db, Table: table, Control: control, Stage: "fixture"}
	f := newFixture(t)
	f.s.Store = store
	first := f.bootstrap(t)
	again := f.bootstrap(t)
	if first != again || f.p.rotations.Load() != 1 {
		t.Fatal("Dynamo retry rotated twice")
	}
	// Real DynamoDB version checks serialize rotation, and revocation wins over
	// an in-flight provider result without permitting stale credential writes.
	sk := "S#" + hash(f.in.SessionProof)
	r, _ := store.Get(ctx, sk)
	b, _ := f.s.loadBundle(ctx, r)
	b.AccessExpiry = f.clock.Load()
	nextR := clone(r)
	nextR.Version++
	nextR.Bundle, _ = f.s.seal(ctx, sk, b)
	if e = store.Commit(ctx, "fixture-generation", map[string]*Record{sk: r}, map[string]*Record{sk: nextR}); e != nil {
		t.Fatal(e)
	}
	started, release := make(chan struct{}), make(chan struct{})
	f.p.refreshHook = func() { close(started); <-release }
	done := make(chan error, 1)
	go func() { _, e := f.s.Token(ctx, f.in.SessionProof, first.CSRFToken, first.SessionBinding); done <- e }()
	<-started
	if _, e = f.s.Token(ctx, f.in.SessionProof, first.CSRFToken, first.SessionBinding); !errors.Is(e, ErrBusy) {
		t.Fatal("real Dynamo waiter not busy")
	}
	if ok, e := f.s.Logout(ctx, f.in.SessionProof, first.CSRFToken, first.SessionBinding); e != nil || !ok {
		t.Fatal("real Dynamo revoke failed")
	}
	close(release)
	if e = <-done; !errors.Is(e, ErrEnded) {
		t.Fatal("real Dynamo stale rotate committed after signout")
	}
	// Multi-item CAS is atomic: an attempted write to an existing attempt must not
	// create an unrelated session record when its condition fails.
	ak := "A#" + hash(f.in.AttemptID)
	ghost := &Record{PK: "S#ghost", Kind: "session", Version: 1}
	if e := store.Commit(ctx, "fixture-generation", map[string]*Record{ak: nil, ghost.PK: nil}, map[string]*Record{ak: {PK: ak, Version: 1}, ghost.PK: ghost}); !errors.Is(e, ErrConflict) {
		t.Fatal("CAS did not reject existing attempt")
	}
	if row, e := store.Get(ctx, ghost.PK); e != nil || row != nil {
		t.Fatal("failed transaction partially wrote session")
	}
	if e = f.s.Cancel(ctx, f.in.AttemptID, f.in.CancellationProof); e != nil {
		t.Fatal(e)
	}
	if _, e = f.s.Metadata(ctx, f.in.SessionProof); !errors.Is(e, ErrEnded) {
		t.Fatal("Dynamo canceled session remained active")
	}
	// Cancellation-first tombstones are checked by the same absent-row CAS used
	// by creation, even though the session proof was not known at cancellation.
	f.in.AttemptID, _ = randomProof()
	f.in.SessionProof, _ = randomProof()
	if e = f.s.Cancel(ctx, f.in.AttemptID, f.in.CancellationProof); e != nil {
		t.Fatal(e)
	}
	if _, e = f.s.Bootstrap(ctx, "original", f.in); !errors.Is(e, ErrEnded) {
		t.Fatal("Dynamo precreation cancel lost")
	}
	putStage("next-generation", "active")
	if e = store.Commit(ctx, "fixture-generation", map[string]*Record{ghost.PK: nil}, map[string]*Record{ghost.PK: ghost}); !errors.Is(e, ErrUnavailable) {
		t.Fatal("old generation transaction committed")
	}
	if row, _ := store.Get(ctx, ghost.PK); row != nil {
		t.Fatal("generation fence partially wrote")
	}
	putStage("next-generation", "closed")
	if _, e = store.Generation(ctx); !errors.Is(e, ErrUnavailable) {
		t.Fatal("closed stage authorized sessions")
	}
}
