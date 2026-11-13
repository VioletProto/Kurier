package browsersession

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/VioletProto/Kurier/services/api/internal/ownership"
	"github.com/golang-jwt/jwt/v5"
)

type CognitoVerifier struct{ verifier *ownership.Verifier }

func NewVerifier(issuer, client string) (*CognitoVerifier, error) {
	v, e := ownership.NewCognitoVerifier(issuer, client)
	if e != nil {
		return nil, e
	}
	return &CognitoVerifier{v}, nil
}
func (v *CognitoVerifier) Verify(ctx context.Context, raw string) (Claims, error) {
	if _, e := v.verifier.Verify(ctx, raw); e != nil {
		var api *ownership.APIError
		if errors.As(e, &api) && api.Status == 503 {
			return Claims{}, ErrUnavailable
		}
		return Claims{}, ErrEnded
	}
	var c struct {
		jwt.RegisteredClaims
		AuthTime int64 `json:"auth_time"`
	}
	if _, _, e := new(jwt.Parser).ParseUnverified(raw, &c); e != nil || c.IssuedAt == nil || c.ExpiresAt == nil {
		return Claims{}, ErrEnded
	}
	return Claims{c.Subject, c.AuthTime, c.IssuedAt.Unix(), c.ExpiresAt.Unix()}, nil
}

type CognitoProvider struct {
	clientID string
	http     *http.Client
}

func NewProvider(clientID string) *CognitoProvider {
	return &CognitoProvider{clientID: clientID, http: &http.Client{Timeout: 3 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return errors.New("provider redirects denied") }}}
}
func (p *CognitoProvider) call(ctx context.Context, target string, in any, out any) error {
	raw, e := json.Marshal(in)
	if e != nil {
		return ErrUnavailable
	}
	defer clear(raw)
	req, e := http.NewRequestWithContext(ctx, "POST", "https://cognito-idp.us-east-2.amazonaws.com/", bytes.NewReader(raw))
	if e != nil {
		return ErrUnavailable
	}
	req.Header.Set("Content-Type", "application/x-amz-json-1.1")
	req.Header.Set("X-Amz-Target", "AWSCognitoIdentityProviderService."+target)
	resp, e := p.http.Do(req)
	if e != nil {
		return ErrUnavailable
	}
	defer resp.Body.Close()
	b, e := io.ReadAll(io.LimitReader(resp.Body, 32769))
	if e != nil || len(b) > 32768 {
		return ErrUnavailable
	}
	defer clear(b)
	if resp.StatusCode != 200 {
		var remote struct {
			Type string `json:"__type"`
		}
		_ = json.Unmarshal(b, &remote)
		if strings.HasSuffix(remote.Type, "NotAuthorizedException") || strings.HasSuffix(remote.Type, "UserNotFoundException") {
			return ErrEnded
		}
		return ErrUnavailable
	}
	if out != nil && json.Unmarshal(b, out) != nil {
		return ErrUnavailable
	}
	return nil
}
func (p *CognitoProvider) Refresh(ctx context.Context, refresh string) (Tokens, error) {
	var out struct {
		AuthenticationResult struct {
			AccessToken  string
			RefreshToken string
		}
	}
	e := p.call(ctx, "GetTokensFromRefreshToken", map[string]string{"ClientId": p.clientID, "RefreshToken": refresh}, &out)
	return Tokens{out.AuthenticationResult.AccessToken, out.AuthenticationResult.RefreshToken}, e
}
func (p *CognitoProvider) Revoke(ctx context.Context, refresh string) error {
	if refresh == "" {
		return nil
	}
	e := p.call(ctx, "RevokeToken", map[string]string{"ClientId": p.clientID, "Token": refresh}, nil)
	if errors.Is(e, ErrEnded) {
		return nil
	}
	return e
}
