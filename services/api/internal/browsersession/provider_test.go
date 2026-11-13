package browsersession

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type providerTransport func(*http.Request) (*http.Response, error)

func (f providerTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestProviderPinsEndpointAndMapsErrorsWithoutSensitiveDetails(t *testing.T) {
	p := NewProvider("fixture-client")
	calls := 0
	p.http.Transport = providerTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.String() != "https://cognito-idp.us-east-2.amazonaws.com/" || r.Header.Get("X-Amz-Target") != "AWSCognitoIdentityProviderService.GetTokensFromRefreshToken" || r.Header.Get("Content-Type") != "application/x-amz-json-1.1" {
			t.Fatal("provider request target not pinned")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"AuthenticationResult":{"AccessToken":"fixture-access","RefreshToken":"fixture-next"}}`))}, nil
	})
	tokens, e := p.Refresh(context.Background(), "fixture-sensitive-refresh")
	if e != nil || tokens.Access != "fixture-access" || tokens.Refresh != "fixture-next" || calls != 1 {
		t.Fatal("provider response parsing failed")
	}
	for _, entry := range []struct {
		status int
		body   string
		want   error
	}{{400, `{"__type":"NotAuthorizedException","message":"fixture-sensitive-refresh"}`, ErrEnded}, {429, `{"__type":"TooManyRequestsException","message":"fixture-sensitive-refresh"}`, ErrUnavailable}, {503, `fixture-sensitive-refresh`, ErrUnavailable}, {200, strings.Repeat("x", 32769), ErrUnavailable}, {200, `{`, ErrUnavailable}} {
		p.http.Transport = providerTransport(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: entry.status, Body: io.NopCloser(strings.NewReader(entry.body))}, nil
		})
		if _, e = p.Refresh(context.Background(), "fixture-sensitive-refresh"); !errors.Is(e, entry.want) || strings.Contains(e.Error(), "fixture-sensitive-refresh") {
			t.Fatal("provider error exposed data or misclassified failure")
		}
	}
}
func TestProviderRevokeAcceptsEmptySuccessAndInvalidatedCredential(t *testing.T) {
	p := NewProvider("fixture-client")
	p.http.Transport = providerTransport(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("X-Amz-Target") != "AWSCognitoIdentityProviderService.RevokeToken" {
			t.Fatal("wrong revoke target")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(""))}, nil
	})
	if e := p.Revoke(context.Background(), "fixture-refresh"); e != nil {
		t.Fatal(e)
	}
}
