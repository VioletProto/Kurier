package execution

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kt "github.com/aws/aws-sdk-go-v2/service/kms/types"
	"strconv"
)

type KMS interface {
	GenerateDataKey(context.Context, *kms.GenerateDataKeyInput, ...func(*kms.Options)) (*kms.GenerateDataKeyOutput, error)
	Decrypt(context.Context, *kms.DecryptInput, ...func(*kms.Options)) (*kms.DecryptOutput, error)
}
type Envelope struct {
	Version    int    `json:"version"`
	Algorithm  string `json:"algorithm"`
	KeyARN     string `json:"keyArn"`
	WrappedKey []byte `json:"wrappedKey"`
	Nonce      []byte `json:"nonce"`
	Ciphertext []byte `json:"ciphertext"`
	Tag        []byte `json:"tag"`
}

func (s *Service) cryptoScope(project, binding, purpose string, version int64) map[string]string {
	return map[string]string{"app": "kurier", "stage": s.Stage, "project": project, "binding": binding, "purpose": purpose, "version": strconv.FormatInt(version, 10)}
}
func (s *Service) decrypt(ctx context.Context, project, binding, purpose string, version int64, e *Envelope) ([]byte, error) {
	if s.KMS == nil || e == nil || e.Version != 1 || e.Algorithm != "AES-256-GCM" || e.KeyARN != s.KeyARN || len(e.Nonce) != 12 || len(e.Tag) != 16 || len(e.Ciphertext) > 32768 {
		return nil, safeError()
	}
	scope := s.cryptoScope(project, binding, purpose, version)
	key, err := s.KMS.Decrypt(ctx, &kms.DecryptInput{KeyId: aws.String(s.KeyARN), CiphertextBlob: e.WrappedKey, EncryptionContext: scope})
	if key != nil {
		defer clear(key.Plaintext)
	}
	if err != nil || key == nil || len(key.Plaintext) != 32 || aws.ToString(key.KeyId) != s.KeyARN {
		return nil, safeError()
	}
	block, err := aes.NewCipher(key.Plaintext)
	if err != nil {
		return nil, safeError()
	}
	gcm, _ := cipher.NewGCM(block)
	aad, _ := json.Marshal(scope)
	sealed := append(append([]byte(nil), e.Ciphertext...), e.Tag...)
	raw, err := gcm.Open(nil, e.Nonce, sealed, aad)
	if err != nil {
		return nil, safeError()
	}
	return raw, nil
}
func (s *Service) encrypt(ctx context.Context, project, job string, values map[string]string) (*Envelope, error) {
	raw, err := json.Marshal(values)
	if err != nil {
		return nil, safeError()
	}
	defer clear(raw)
	if len(raw) > 24*1024 {
		return nil, failure(413, "payload_too_large")
	}
	scope := s.cryptoScope(project, job, "job-bindings", 1)
	key, err := s.KMS.GenerateDataKey(ctx, &kms.GenerateDataKeyInput{KeyId: aws.String(s.KeyARN), KeySpec: kt.DataKeySpecAes256, EncryptionContext: scope})
	if key != nil {
		defer clear(key.Plaintext)
	}
	if err != nil || key == nil || len(key.Plaintext) != 32 || aws.ToString(key.KeyId) != s.KeyARN {
		return nil, safeError()
	}
	block, _ := aes.NewCipher(key.Plaintext)
	gcm, _ := cipher.NewGCM(block)
	nonce := make([]byte, 12)
	if _, err = rand.Read(nonce); err != nil {
		return nil, safeError()
	}
	aad, _ := json.Marshal(scope)
	sealed := gcm.Seal(nil, nonce, raw, aad)
	e := &Envelope{1, "AES-256-GCM", s.KeyARN, key.CiphertextBlob, nonce, sealed[:len(sealed)-16], sealed[len(sealed)-16:]}
	encoded, _ := json.Marshal(e)
	if len(encoded) > 32768 {
		return nil, failure(413, "payload_too_large")
	}
	return e, nil
}
