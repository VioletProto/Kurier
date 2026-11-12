// Package browsersession contains the isolated, cookie-based Cognito refresh broker.
// Credential records are never application evidence or public response DTOs.
package browsersession

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"
)

const Path = "/api/v1/browser-session"
const CookieName = "__Host-kurier-session"

var ErrConflict = errors.New("session condition changed")
var ErrBusy = errors.New("session refresh in progress")
var ErrForbidden = errors.New("session request forbidden")
var ErrEnded = errors.New("session ended")
var ErrUnavailable = errors.New("session temporarily unavailable")
var ErrRate = errors.New("session rate limited")

type Claims struct {
	Subject                       string
	AuthTime, IssuedAt, ExpiresAt int64
}
type Tokens struct{ Access, Refresh string }
type Verifier interface {
	Verify(context.Context, string) (Claims, error)
}
type Provider interface {
	Refresh(context.Context, string) (Tokens, error)
	Revoke(context.Context, string) error
}
type Cipher interface {
	Seal(context.Context, string, []byte) ([]byte, error)
	Open(context.Context, string, []byte) ([]byte, error)
}
type Record struct {
	PK          string `dynamodbav:"PK"`
	Version     int64  `dynamodbav:"version"`
	Kind        string `dynamodbav:"kind"`
	Status      string `dynamodbav:"status"`
	Generation  string `dynamodbav:"generation"`
	Subject     string `dynamodbav:"subject,omitempty"`
	AuthTime    int64  `dynamodbav:"authTime,omitempty"`
	IssuedAt    int64  `dynamodbav:"issuedAt,omitempty"`
	Expires     int64  `dynamodbav:"expires"`
	TTL         int64  `dynamodbav:"ttl"`
	SessionKey  string `dynamodbav:"sessionKey,omitempty"`
	AttemptKey  string `dynamodbav:"attemptKey,omitempty"`
	CancelHash  string `dynamodbav:"cancelHash,omitempty"`
	RequestHash string `dynamodbav:"requestHash,omitempty"`
	Binding     string `dynamodbav:"binding,omitempty"`
	CSRF        string `dynamodbav:"csrf,omitempty"`
	LeaseUntil  int64  `dynamodbav:"leaseUntil,omitempty"`
	Bundle      []byte `dynamodbav:"bundle,omitempty"`
}
type Store interface {
	Generation(context.Context) (string, error)
	Get(context.Context, string) (*Record, error)
	// Commit atomically checks every old version (nil means absent), the active
	// Control generation, and writes every successor. Never automatically retry.
	Commit(context.Context, string, map[string]*Record, map[string]*Record) error
	Rate(context.Context, string, int64, int) error
}
type bundle struct {
	Access       string `json:"access"`
	Refresh      string `json:"refresh"`
	AccessExpiry int64  `json:"accessExpiry"`
}
type Service struct {
	Store                Store
	Cipher               Cipher
	Provider             Provider
	Verifier             Verifier
	Origin, OriginSecret string
	Local                bool
	Now                  func() time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}
func hash(v string) string { b := sha256.Sum256([]byte(v)); return hex.EncodeToString(b[:]) }
func validProof(v string) bool {
	b, e := base64.RawURLEncoding.Strict().DecodeString(v)
	return e == nil && len(b) == 32
}
func same(a, b string) bool {
	return len(a) == len(b) && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
func randomProof() (string, error) {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		return "", e
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func clone(r *Record) *Record {
	if r == nil {
		return nil
	}
	v := *r
	v.Bundle = append([]byte(nil), r.Bundle...)
	return &v
}
