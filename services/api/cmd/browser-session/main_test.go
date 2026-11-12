package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/aws/aws-lambda-go/events"
)

func TestLambdaCookieListsAndErrorBodiesSurviveAdapter(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, e := r.Cookie("__Host-kurier-session")
		if e != nil || c.Value != "fixture-cookie" || r.RemoteAddr != "192.0.2.1" {
			t.Fatal("HTTP API cookie/source adapter failed")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		http.SetCookie(w, &http.Cookie{Name: "__Host-kurier-session", Value: "fixture-cookie", Path: "/", Secure: true, HttpOnly: true})
		w.WriteHeader(403)
		_, _ = w.Write([]byte(`{"error":{"code":"forbidden"}}`))
	})
	event := events.APIGatewayV2HTTPRequest{Version: "2.0", RawPath: "/api/v1/browser-session", Cookies: []string{"__Host-kurier-session=fixture-cookie"}, RequestContext: events.APIGatewayV2HTTPRequestContext{HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{Method: "GET", SourceIP: "192.0.2.1"}}}
	response := proxy(handler, context.Background(), event)
	if response.StatusCode != 403 || response.Body != `{"error":{"code":"forbidden"}}` || len(response.Cookies) != 1 || response.Headers["Set-Cookie"] != "" || response.Headers["Cache-Control"] != "no-store" {
		t.Fatal("broker JSON status/body or cookie list changed")
	}
}
func TestMalformedLambdaInputRejectedBeforeHandler(t *testing.T) {
	h := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("malformed event reached broker") })
	for _, e := range []events.APIGatewayV2HTTPRequest{{Version: "1.0", RawPath: "/api/v1/browser-session"}, {Version: "2.0", RawPath: "/api/v1/browser-session", RawQueryString: "credential=fixture"}, {Version: "2.0", RawPath: "/api/v1/browser-session", IsBase64Encoded: true, Body: "not-base64"}} {
		if proxy(h, context.Background(), e).StatusCode != 400 {
			t.Fatal("malformed event accepted")
		}
	}
}
