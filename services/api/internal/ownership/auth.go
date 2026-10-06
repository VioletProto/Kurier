package ownership

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Identity fields are private. Runtime handlers can obtain one only from Verify.
type Identity struct{ issuer, subject string }
type accessClaims struct {
	jwt.RegisteredClaims
	ClientID string `json:"client_id"`
	TokenUse string `json:"token_use"`
}

type Verifier struct {
	issuer, clientID     string
	client               *http.Client
	mu                   sync.Mutex
	keys                 map[string]*rsa.PublicKey
	expires, lastAttempt time.Time
	now                  func() time.Time
}

var cognitoIssuer = regexp.MustCompile(`^https://cognito-idp\.[a-z0-9-]+\.amazonaws\.com(?:\.cn)?/[a-z0-9-]+_[A-Za-z0-9]+$`)

// NewCognitoVerifier accepts only an explicitly configured Cognito issuer.
// No fixture issuer/JWKS URL or insecure trust setting is configurable at runtime.
func NewCognitoVerifier(issuer, clientID string) (*Verifier, error) {
	if !cognitoIssuer.MatchString(issuer) || strings.TrimSpace(clientID) == "" || len(clientID) > 256 {
		return nil, errors.New("valid Cognito issuer and client ID are required")
	}
	return &Verifier{issuer: issuer, clientID: clientID, client: &http.Client{
		Timeout:       2 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return errors.New("JWKS redirects disabled") },
	}, now: time.Now}, nil
}

func (v *Verifier) Verify(ctx context.Context, raw string) (Identity, error) {
	deny := func() (Identity, error) {
		return Identity{}, apiError(401, "unauthenticated", "Valid access token required.")
	}
	if len(raw) == 0 || len(raw) > 16384 {
		return deny()
	}
	claims := &accessClaims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (any, error) {
		kid, ok := token.Header["kid"].(string)
		if !ok || kid == "" || len(kid) > 256 {
			return nil, errors.New("invalid signing key")
		}
		return v.key(ctx, kid)
	}, jwt.WithValidMethods([]string{"RS256"}), jwt.WithIssuer(v.issuer), jwt.WithExpirationRequired(), jwt.WithIssuedAt(), jwt.WithLeeway(60*time.Second), jwt.WithTimeFunc(v.now), jwt.WithStrictDecoding())
	if err != nil {
		var serviceError *APIError
		if errors.As(err, &serviceError) && serviceError.Status == 503 {
			return Identity{}, serviceError
		}
		return deny()
	}
	if !token.Valid || claims.ClientID != v.clientID || claims.TokenUse != "access" || claims.Subject == "" || len(claims.Subject) > 2048 || claims.IssuedAt == nil {
		return deny()
	}
	return Identity{v.issuer, claims.Subject}, nil
}

func (v *Verifier) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	now := v.now()
	if now.Before(v.expires) && v.keys[kid] != nil {
		return v.keys[kid], nil
	}
	// Unknown kids cannot cause unbounded network requests. Failed refreshes
	// also observe cooldown; expired cached keys are never used on failure.
	if !v.lastAttempt.IsZero() && now.Sub(v.lastAttempt) < 30*time.Second {
		if !now.Before(v.expires) {
			return nil, unavailable()
		}
		return nil, errors.New("unknown signing key")
	}
	v.lastAttempt = now
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.issuer+"/.well-known/jwks.json", nil)
	if err != nil {
		return nil, unavailable()
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return nil, unavailable()
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, unavailable()
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 65537))
	if err != nil || len(data) > 65536 {
		return nil, unavailable()
	}
	var set struct {
		Keys []struct{ Kid, Kty, Alg, Use, N, E string } `json:"keys"`
	}
	if json.Unmarshal(data, &set) != nil || len(set.Keys) == 0 || len(set.Keys) > 64 {
		return nil, unavailable()
	}
	keys := make(map[string]*rsa.PublicKey)
	for _, k := range set.Keys {
		if k.Kty != "RSA" || k.Alg != "RS256" || k.Use != "sig" || k.Kid == "" || len(k.Kid) > 256 || keys[k.Kid] != nil {
			return nil, unavailable()
		}
		n, e1 := base64.RawURLEncoding.DecodeString(k.N)
		e, e2 := base64.RawURLEncoding.DecodeString(k.E)
		if e1 != nil || e2 != nil || len(e) == 0 || len(e) > 4 {
			return nil, unavailable()
		}
		mod := new(big.Int).SetBytes(n)
		exp := new(big.Int).SetBytes(e).Int64()
		if mod.BitLen() < 2048 || mod.BitLen() > 4096 || exp < 3 || exp > 2147483647 || exp%2 == 0 {
			return nil, unavailable()
		}
		keys[k.Kid] = &rsa.PublicKey{N: mod, E: int(exp)}
	}
	v.keys = keys
	v.expires = now.Add(5 * time.Minute)
	if keys[kid] == nil {
		return nil, errors.New("unknown signing key")
	}
	return keys[kid], nil
}
