package browsersession

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/kms"
)

type fixtureKMS struct{ scope string }

func (k *fixtureKMS) GenerateDataKey(_ context.Context, in *kms.GenerateDataKeyInput, _ ...func(*kms.Options)) (*kms.GenerateDataKeyOutput, error) {
	scope, _ := json.Marshal(in.EncryptionContext)
	k.scope = string(scope)
	return &kms.GenerateDataKeyOutput{KeyId: in.KeyId, Plaintext: make([]byte, 32), CiphertextBlob: []byte("fixture-wrapped-key")}, nil
}
func (k *fixtureKMS) Decrypt(_ context.Context, in *kms.DecryptInput, _ ...func(*kms.Options)) (*kms.DecryptOutput, error) {
	scope, _ := json.Marshal(in.EncryptionContext)
	if string(scope) != k.scope {
		return nil, errors.New("context mismatch")
	}
	return &kms.DecryptOutput{KeyId: in.KeyId, Plaintext: make([]byte, 32)}, nil
}
func TestEnvelopeEncryptionBindsKMSContextAndSession(t *testing.T) {
	k := &fixtureKMS{}
	c := &EnvelopeCipher{Client: k, KeyARN: "arn:aws:kms:us-east-2:123456789012:key/fixture", Stage: "dev-api"}
	raw := []byte("disposable-fixture-refresh")
	sealed, e := c.Seal(context.Background(), "S#fixture", raw)
	if e != nil {
		t.Fatal(e)
	}
	opened, e := c.Open(context.Background(), "S#fixture", sealed)
	if e != nil || string(opened) != string(raw) {
		t.Fatal("envelope failed to round trip")
	}
	if _, e = c.Open(context.Background(), "S#foreign", sealed); e == nil {
		t.Fatal("foreign session decrypted")
	}
	if k.scope != `{"app":"kurier","purpose":"browser-session","session":"S#fixture","stage":"dev-api"}` {
		t.Fatal("wrong KMS encryption context")
	}
	c.Stage = "foreign"
	if _, e = c.Open(context.Background(), "S#fixture", sealed); e == nil {
		t.Fatal("foreign stage decrypted")
	}
}
