package ownership

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"strings"
	"testing"
)

type fixtureKMS struct {
	key  []byte
	fail bool
}

func (f *fixtureKMS) GenerateDataKey(_ context.Context, in *kms.GenerateDataKeyInput, _ ...func(*kms.Options)) (*kms.GenerateDataKeyOutput, error) {
	if f.fail {
		return nil, errors.New("fixture KMS unavailable")
	}
	f.key = make([]byte, 32)
	rand.Read(f.key)
	return &kms.GenerateDataKeyOutput{Plaintext: append([]byte{}, f.key...), CiphertextBlob: []byte("fixture-wrapped-key"), KeyId: in.KeyId}, nil
}
func newFixtureCipher(stage string) (*EnvelopeCipher, *fixtureKMS) {
	f := &fixtureKMS{}
	return &EnvelopeCipher{f, "arn:aws:kms:us-east-2:747336059622:key/fixture", stage}, f
}
func TestEnvelopeAuthenticatesScopeAndCiphertext(t *testing.T) {
	c, k := newFixtureCipher("local")
	value := make([]byte, 100)
	rand.Read(value)
	expected := append([]byte{}, value...)
	id := newID()
	env, err := c.encrypt(context.Background(), "project", id, 0, value)
	if err != nil {
		t.Fatal("encryption failed")
	}
	for _, v := range value {
		if v != 0 {
			t.Fatal("plaintext input buffer retained")
		}
	}
	block, _ := aes.NewCipher(k.key)
	gcm, _ := cipher.NewGCM(block)
	sealed := append(append([]byte{}, env.Ciphertext...), env.Tag...)
	aad, _ := json.Marshal(cryptoContext("local", "project", id, 0))
	got, err := gcm.Open(nil, env.Nonce, sealed, aad)
	if err != nil || string(got) != string(expected) {
		t.Fatal("round trip failed")
	}
	for _, scope := range []map[string]string{cryptoContext("foreign", "project", id, 0), cryptoContext("local", "other", id, 0), cryptoContext("local", "project", newID(), 0), cryptoContext("local", "project", id, 1)} {
		aad, _ = json.Marshal(scope)
		if _, err = gcm.Open(nil, env.Nonce, sealed, aad); err == nil {
			t.Fatal("wrong scope accepted")
		}
	}
	sealed[0] ^= 1
	aad, _ = json.Marshal(cryptoContext("local", "project", id, 0))
	if _, err = gcm.Open(nil, env.Nonce, sealed, aad); err == nil {
		t.Fatal("tamper accepted")
	}
}
func TestProtectedDescriptorBoundaries(t *testing.T) {
	value := strings.Repeat("a", 8192)
	c := publicConfiguration()
	for range 9 {
		v := value
		c.Headers = append(c.Headers, RequestField{Name: "X-Api-Key", Enabled: true, Sensitive: true, BindingID: newID(), SecretWrite: &SecretWrite{Action: "set", Value: &v}})
	}
	if validateConfiguration(&c) != nil {
		t.Fatal("plaintext incorrectly counted as saved configuration")
	}
	raw, _ := configurationJSON(c)
	if _, err := mergeConfiguration(raw, nil); err != nil {
		t.Fatal("write DTO rejected")
	}
	c.Headers[1].BindingID = c.Headers[0].BindingID
	if validateConfiguration(&c) == nil {
		t.Fatal("duplicate binding accepted")
	}
	c = publicConfiguration()
	v := "\"fixture\""
	c.Body = &RequestBody{Type: "json", Text: "{\"password\":null,\"rows\":[null],\"a/b\":null}", SecretFields: []SecretField{{Pointer: "/password", BindingID: newID(), SecretWrite: &SecretWrite{Action: "set", Value: &v}}, {Pointer: "/rows/0", BindingID: newID(), SecretWrite: &SecretWrite{Action: "set", Value: &v}}, {Pointer: "/a~1b", BindingID: newID(), SecretWrite: &SecretWrite{Action: "set", Value: &v}}}}
	if validateConfiguration(&c) != nil {
		t.Fatal("protected paths rejected")
	}
	for _, p := range []string{"/rows/00", "/a~2b", "/missing"} {
		c.Body.SecretFields[2].Pointer = p
		if validateConfiguration(&c) == nil {
			t.Fatal("invalid pointer accepted")
		}
	}
	c = publicConfiguration()
	c.URL = "https://example.com/path-unmarked-value"
	if validateConfiguration(&c) != nil {
		t.Fatal("unsupported claim of arbitrary URL secret detection")
	}
	c.URL = "https://example.com?api_key=unmarked"
	if validateConfiguration(&c) == nil {
		t.Fatal("defined URL credential pattern accepted")
	}
}
