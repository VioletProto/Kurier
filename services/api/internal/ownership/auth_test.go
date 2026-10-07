package ownership

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// All fixture signing keys, trust roots and fixture issuer/client configuration
// live exclusively in _test.go. No runtime fixture option or token mint route.
type authFixture struct {
	verifier *Verifier
	signer   *rsa.PrivateKey
	calls    atomic.Int32
	status   atomic.Int32
}

func newAuthFixture(t *testing.T) *authFixture {
	t.Helper()
	private, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	f := &authFixture{signer: private}
	f.status.Store(200)
	jwks := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.calls.Add(1)
		if r.URL.Path != "/.well-known/jwks.json" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(int(f.status.Load()))
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]any{"kid": "fixture-key", "kty": "RSA", "alg": "RS256", "use": "sig", "n": base64.RawURLEncoding.EncodeToString(private.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(private.E)).Bytes())}}})
	}))
	t.Cleanup(jwks.Close)
	client := jwks.Client()
	client.Timeout = 2 * time.Second
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return errors.New("redirect denied") }
	f.verifier = &Verifier{issuer: jwks.URL, clientID: "fixture-client", client: client, now: time.Now}
	return f
}
func (f *authFixture) token(t *testing.T, sub string, change func(jwt.MapClaims)) string {
	t.Helper()
	now := f.verifier.now()
	claims := jwt.MapClaims{"iss": f.verifier.issuer, "sub": sub, "client_id": "fixture-client", "token_use": "access", "iat": now.Unix(), "exp": now.Add(15 * time.Minute).Unix()}
	if change != nil {
		change(claims)
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = "fixture-key"
	raw, err := token.SignedString(f.signer)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func verified(t *testing.T, f *authFixture, sub string) Identity {
	t.Helper()
	id, err := f.verifier.Verify(context.Background(), f.token(t, sub, nil))
	if err != nil {
		t.Fatal(err)
	}
	return id
}
func assertStatus(t *testing.T, err error, status int) {
	t.Helper()
	var ae *APIError
	if !errors.As(err, &ae) || ae.Status != status {
		t.Fatalf("wanted status %d, got %v", status, err)
	}
}
func TestVerifierSignedFixturesAndRejections(t *testing.T) {
	f := newAuthFixture(t)
	id := verified(t, f, "alice")
	if id.subject != "alice" || id.issuer != f.verifier.issuer {
		t.Fatal("identity mismatch")
	}
	for name, modify := range map[string]func(jwt.MapClaims){
		"wrong issuer":     func(c jwt.MapClaims) { c["iss"] = "https://wrong.invalid" },
		"wrong client":     func(c jwt.MapClaims) { c["client_id"] = "wrong" },
		"ID token":         func(c jwt.MapClaims) { c["token_use"] = "id" },
		"expired":          func(c jwt.MapClaims) { c["exp"] = time.Now().Add(-2 * time.Minute).Unix() },
		"future issuance":  func(c jwt.MapClaims) { c["iat"] = time.Now().Add(2 * time.Minute).Unix() },
		"missing expiry":   func(c jwt.MapClaims) { delete(c, "exp") },
		"missing issuance": func(c jwt.MapClaims) { delete(c, "iat") },
		"missing subject":  func(c jwt.MapClaims) { delete(c, "sub") },
	} {
		t.Run(name, func(t *testing.T) {
			_, err := f.verifier.Verify(context.Background(), f.token(t, "alice", modify))
			assertStatus(t, err, 401)
		})
	}
	for _, raw := range []string{"", "not-a-token", strings.Repeat("x", 16385)} {
		_, err := f.verifier.Verify(context.Background(), raw)
		assertStatus(t, err, 401)
	}
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	bad := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{"iss": f.verifier.issuer, "sub": "alice", "client_id": "fixture-client", "token_use": "access", "iat": time.Now().Unix(), "exp": time.Now().Add(time.Minute).Unix()})
	bad.Header["kid"] = "fixture-key"
	raw, err := bad.SignedString(other)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.verifier.Verify(context.Background(), raw)
	assertStatus(t, err, 401)
	bad = jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{})
	bad.Header["kid"] = "fixture-key"
	raw, err = bad.SignedString([]byte("test-only"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.verifier.Verify(context.Background(), raw)
	assertStatus(t, err, 401)
	if f.calls.Load() != 1 {
		t.Fatalf("JWKS fetches: %d", f.calls.Load())
	}
	if _, err = NewCognitoVerifier(f.verifier.issuer, "fixture-client"); err == nil {
		t.Fatal("runtime accepted fixture issuer")
	}
}
func TestJWKSBoundedRefreshAndExpiry(t *testing.T) {
	f := newAuthFixture(t)
	_ = verified(t, f, "alice")
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{})
	token.Header["kid"] = "unknown"
	raw, err := token.SignedString(f.signer)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		_, err = f.verifier.Verify(context.Background(), raw)
		assertStatus(t, err, 401)
	}
	if f.calls.Load() != 1 {
		t.Fatal("unknown kid caused repeated fetches")
	}
	f.verifier.lastAttempt = time.Now().Add(-time.Minute)
	_, err = f.verifier.Verify(context.Background(), raw)
	assertStatus(t, err, 401)
	if f.calls.Load() != 2 {
		t.Fatal("bounded refresh did not occur")
	}
	f.verifier.expires = time.Now().Add(-time.Second)
	f.verifier.lastAttempt = time.Now().Add(-time.Minute)
	f.status.Store(500)
	_, err = f.verifier.Verify(context.Background(), f.token(t, "alice", nil))
	assertStatus(t, err, 503)
	_, err = f.verifier.Verify(context.Background(), raw)
	assertStatus(t, err, 503)
	if f.calls.Load() != 3 {
		t.Fatal("failed refresh ignored cooldown")
	}
}
func TestLocalClientRejectsCloudAndNonLoopback(t *testing.T) {
	for _, endpoint := range []string{"", "https://dynamodb.us-east-2.amazonaws.com", "http://192.168.1.2:8000", "http://localhost:8000/path", "http://user@localhost:8000", "http://localhost:8000?x=1"} {
		if _, err := LocalClient(endpoint); err == nil {
			t.Fatalf("accepted %q", endpoint)
		}
	}
	if _, err := LocalClient("http://127.0.0.1:8000"); err != nil {
		t.Fatal(err)
	}
}
