//go:build integration && aws

package ownership

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/json"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"os"
	"strings"
	"testing"
	"time"
)

func TestAWSProtectedEncryptionAndCrossTableLifecycle(t *testing.T) {
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-east-2"), config.WithRetryer(func() aws.Retryer { return aws.NopRetryer{} }))
	if err != nil {
		t.Fatal("AWS session unavailable")
	}
	identity, err := sts.NewFromConfig(cfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil || aws.ToString(identity.Account) != "747336059622" || strings.HasSuffix(aws.ToString(identity.Arn), ":root") {
		t.Fatal("intended non-root identity required")
	}
	table, protected, keyARN := os.Getenv("KURIER_AWS_TEST_TABLE"), os.Getenv("KURIER_AWS_TEST_PROTECTED"), os.Getenv("KURIER_AWS_TEST_KEY")
	if !strings.HasPrefix(table, "kurier-dev-api-ControlTable-") || !strings.HasPrefix(protected, "kurier-dev-api-ProtectedTable-") || !strings.HasPrefix(keyARN, "arn:aws:kms:us-east-2:747336059622:key/") {
		t.Fatal("scoped resources required")
	}
	db := dynamodb.NewFromConfig(cfg)
	km := kms.NewFromConfig(cfg)
	store := NewStore(db, table, "dev-api")
	configureAWSExecutionFixture(store, cfg)
	store.ConfigureProtected(protected, NewEnvelopeCipher(km, keyARN, "dev-api"))
	auth := newAuthFixture(t)
	alice, err := store.ResolveUser(ctx, verified(t, auth, "protected-alice-"+newID()))
	if err != nil {
		t.Fatal(err)
	}
	bob, err := store.ResolveUser(ctx, verified(t, auth, "protected-bob-"+newID()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, u := range []User{alice, bob} {
			r, e := store.get(ctx, "U#"+u.UserID, "META")
			if e == nil {
				db.DeleteItem(ctx, &dynamodb.DeleteItemInput{TableName: aws.String(table), Key: key(identityKey(Identity{issuer: r.Issuer, subject: r.Subject}), "META")})
				db.DeleteItem(ctx, &dynamodb.DeleteItemInput{TableName: aws.String(table), Key: key(r.PK, r.SK)})
			}
		}
	})
	exerciseProtected(t, &integration{store: store, client: db, alice: alice, bob: bob, auth: auth})
	p, err := store.CreateProject(ctx, alice.UserID, "KMS scheduled fixture")
	if err != nil {
		t.Fatal(err)
	}
	value := "Bearer " + newID()
	saved, err := store.CreateRequest(ctx, alice.UserID, p.ProjectID, protectedConfig(value))
	if err != nil {
		t.Fatal("actual KMS write failed", err)
	}
	id := saved.Headers[0].SecretRef.SecretID
	r, err := store.secret(ctx, p.ProjectID, id)
	if err != nil {
		t.Fatal(err)
	}
	scope := cryptoContext("dev-api", p.ProjectID, id, 0)
	decoded, err := km.Decrypt(ctx, &kms.DecryptInput{KeyId: aws.String(keyARN), CiphertextBlob: r.Envelope.WrappedKey, EncryptionContext: scope})
	if err != nil {
		t.Fatal("operator wrapped key decrypt failed")
	}
	defer clear(decoded.Plaintext)
	block, _ := aes.NewCipher(decoded.Plaintext)
	gcm, _ := cipher.NewGCM(block)
	aad, _ := json.Marshal(scope)
	plain, err := gcm.Open(nil, r.Envelope.Nonce, append(append([]byte{}, r.Envelope.Ciphertext...), r.Envelope.Tag...), aad)
	defer clear(plain)
	if err != nil || string(plain) != value {
		t.Fatal("actual encrypted persistence mismatch")
	}
	scope["project"] = newID()
	if _, err = km.Decrypt(ctx, &kms.DecryptInput{KeyId: aws.String(keyARN), CiphertextBlob: r.Envelope.WrappedKey, EncryptionContext: scope}); err == nil {
		t.Fatal("KMS accepted wrong context")
	}
	for range 24 {
		if _, err = store.CreateRequest(ctx, alice.UserID, p.ProjectID, protectedConfig("Bearer "+newID())); err != nil {
			t.Fatal("multichunk setup failed", err)
		}
	}
	project, _ := store.GetProject(ctx, alice.UserID, p.ProjectID)
	op, err := store.DeleteProject(ctx, alice.UserID, p.ProjectID, project.Version)
	if err != nil {
		t.Fatal(err)
	}
	// The scheduled Lambda must drain both stores; do not invoke cleanup manually.
	start := time.Now()
	for time.Since(start) < 220*time.Second {
		current, e := store.GetDeletion(ctx, alice.UserID, p.ProjectID, op.OperationID)
		if e == nil && current.State == "completed" {
			out, e := db.Query(ctx, &dynamodb.QueryInput{TableName: aws.String(protected), KeyConditionExpression: aws.String("PK = :pk"), ExpressionAttributeValues: map[string]types.AttributeValue{":pk": s("P#" + p.ProjectID)}, ConsistentRead: aws.Bool(true)})
			if e != nil || len(out.Items) != 0 {
				t.Fatal("scheduled cleanup left Protected records")
			}
			t.Logf("Actual KMS/context and scheduled cross-table cleanup passed in %.1fs", time.Since(start).Seconds())
			return
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatal("scheduled cleanup did not complete; durable work remains")
}
