package browsersession

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kt "github.com/aws/aws-sdk-go-v2/service/kms/types"
)

type KMSAPI interface {
	GenerateDataKey(context.Context, *kms.GenerateDataKeyInput, ...func(*kms.Options)) (*kms.GenerateDataKeyOutput, error)
	Decrypt(context.Context, *kms.DecryptInput, ...func(*kms.Options)) (*kms.DecryptOutput, error)
}
type EnvelopeCipher struct {
	Client        KMSAPI
	KeyARN, Stage string
}
type envelope struct {
	Version int    `json:"version"`
	Key     []byte `json:"key"`
	Nonce   []byte `json:"nonce"`
	Data    []byte `json:"data"`
}

func (c *EnvelopeCipher) scope(id string) map[string]string {
	return map[string]string{"app": "kurier", "stage": c.Stage, "purpose": "browser-session", "session": id}
}
func sealBytes(key, raw, aad []byte) ([]byte, []byte, error) {
	b, e := aes.NewCipher(key)
	if e != nil {
		return nil, nil, ErrUnavailable
	}
	g, e := cipher.NewGCM(b)
	if e != nil {
		return nil, nil, ErrUnavailable
	}
	nonce := make([]byte, g.NonceSize())
	if _, e = rand.Read(nonce); e != nil {
		return nil, nil, ErrUnavailable
	}
	return nonce, g.Seal(nil, nonce, raw, aad), nil
}
func openBytes(key, nonce, data, aad []byte) ([]byte, error) {
	b, e := aes.NewCipher(key)
	if e != nil {
		return nil, ErrUnavailable
	}
	g, e := cipher.NewGCM(b)
	if e != nil || len(nonce) != g.NonceSize() {
		return nil, ErrUnavailable
	}
	raw, e := g.Open(nil, nonce, data, aad)
	if e != nil {
		return nil, ErrUnavailable
	}
	return raw, nil
}
func (c *EnvelopeCipher) Seal(ctx context.Context, id string, raw []byte) ([]byte, error) {
	if len(raw) > 24*1024 {
		return nil, ErrUnavailable
	}
	scope := c.scope(id)
	k, e := c.Client.GenerateDataKey(ctx, &kms.GenerateDataKeyInput{KeyId: aws.String(c.KeyARN), KeySpec: kt.DataKeySpecAes256, EncryptionContext: scope})
	if k != nil {
		defer clear(k.Plaintext)
	}
	if e != nil || k == nil || len(k.Plaintext) != 32 || aws.ToString(k.KeyId) != c.KeyARN {
		return nil, ErrUnavailable
	}
	aad, _ := json.Marshal(scope)
	n, data, e := sealBytes(k.Plaintext, raw, aad)
	if e != nil {
		return nil, e
	}
	return json.Marshal(envelope{1, k.CiphertextBlob, n, data})
}
func (c *EnvelopeCipher) Open(ctx context.Context, id string, raw []byte) ([]byte, error) {
	var v envelope
	if len(raw) > 40*1024 || json.Unmarshal(raw, &v) != nil || v.Version != 1 || len(v.Key) == 0 {
		return nil, ErrUnavailable
	}
	scope := c.scope(id)
	k, e := c.Client.Decrypt(ctx, &kms.DecryptInput{KeyId: aws.String(c.KeyARN), CiphertextBlob: v.Key, EncryptionContext: scope})
	if k != nil {
		defer clear(k.Plaintext)
	}
	if e != nil || k == nil || len(k.Plaintext) != 32 || aws.ToString(k.KeyId) != c.KeyARN {
		return nil, ErrUnavailable
	}
	aad, _ := json.Marshal(scope)
	return openBytes(k.Plaintext, v.Nonce, v.Data, aad)
}

// LocalCipher is selected exclusively by the explicit loopback development
// entrypoint. Production startup never accepts a fixed encryption key.
type LocalCipher struct{ Key []byte }

func (c LocalCipher) Seal(_ context.Context, id string, raw []byte) ([]byte, error) {
	n, data, e := sealBytes(c.Key, raw, []byte(id))
	if e != nil {
		return nil, e
	}
	return append(n, data...), nil
}
func (c LocalCipher) Open(_ context.Context, id string, raw []byte) ([]byte, error) {
	if len(raw) < 12 {
		return nil, ErrUnavailable
	}
	return openBytes(c.Key, raw[:12], raw[12:], []byte(id))
}
