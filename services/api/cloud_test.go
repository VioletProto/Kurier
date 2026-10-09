package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/VioletProto/Kurier/services/api/execution"
	"github.com/VioletProto/Kurier/services/api/internal/ownership"
	"github.com/aws/aws-lambda-go/events"
)

func TestEvidenceProxyEncodingAndSerializedBounds(t *testing.T) {
	for _, size := range []int{1, 2, 3, execution.EvidenceLimit - 1, execution.EvidenceLimit, execution.EvidenceLimit + 1} {
		payload := strings.Repeat("x", size)
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
			_, _ = w.Write([]byte(payload))
		})
		event := events.APIGatewayV2HTTPRequest{Version: "2.0", RawPath: "/api/v1/projects/p/executions/e/evidence", RequestContext: events.APIGatewayV2HTTPRequestContext{HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{Method: "GET"}}}
		proxy, err := proxyHTTPV2(h)(context.Background(), event)
		if err != nil {
			t.Fatal(err)
		}
		if size > execution.EvidenceLimit {
			if proxy.StatusCode != 503 {
				t.Fatal("oversized API envelope returned")
			}
			continue
		}
		if !proxy.IsBase64Encoded {
			t.Fatal("evidence transport was not always base64")
		}
		decoded, err := base64.StdEncoding.DecodeString(proxy.Body)
		if err != nil || string(decoded) != payload {
			t.Fatal("transport changed evidence bytes")
		}
		serialized, _ := json.Marshal(proxy)
		if len(serialized)+1 > execution.LambdaLimit {
			t.Fatal("serialized Lambda limit exceeded")
		}
	}
	// A raw body with worst-case escaping exceeds Lambda; base64 avoids it.
	text := strings.Repeat("\"", execution.BodyLimit)
	raw, _ := json.Marshal(text)
	plain, _ := json.Marshal(events.APIGatewayV2HTTPResponse{StatusCode: 200, Body: string(raw)})
	binary, _ := json.Marshal(events.APIGatewayV2HTTPResponse{StatusCode: 200, Body: base64.StdEncoding.EncodeToString(raw), IsBase64Encoded: true})
	if len(plain) <= execution.LambdaLimit || len(binary) > execution.LambdaLimit {
		t.Fatal("base64 must fit where the double-escaped raw proxy exceeds Lambda")
	}
	metadata := strings.Repeat("<", execution.MetadataLimit)
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Fixture", metadata)
		_, _ = w.Write([]byte(`{}`))
	})
	event := events.APIGatewayV2HTTPRequest{Version: "2.0", RawPath: "/api/v1/projects/p/executions/e/evidence", RequestContext: events.APIGatewayV2HTTPRequestContext{HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{Method: "GET"}}}
	proxy, _ := proxyHTTPV2(h)(context.Background(), event)
	if proxy.StatusCode != 503 {
		t.Fatal("unbounded metadata returned")
	}
}

func TestHTTPAPIV2AdapterPreservesRouteAndCORS(t *testing.T) {
	h, err := runtimeHandler(ownership.NewStore(nil, "unused", "unused"), testRuntimeConfig)
	if err != nil {
		t.Fatal(err)
	}
	adapter := proxyHTTPV2(h)
	event := events.APIGatewayV2HTTPRequest{Version: "2.0", RawPath: "/api/v1/users/me", Headers: map[string]string{"origin": "http://localhost:5173"}, RequestContext: events.APIGatewayV2HTTPRequestContext{HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{Method: "GET", Path: "/api/v1/users/me"}}}
	response, err := adapter(context.Background(), event)
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct{ Error struct{ Code string } }
	if json.Unmarshal([]byte(response.Body), &envelope) != nil || response.StatusCode != 401 || envelope.Error.Code != "unauthenticated" {
		t.Fatal("adapter lost users/me error contract")
	}
	if response.Headers["Access-Control-Allow-Origin"] != "http://localhost:5173" || response.Headers["Access-Control-Expose-Headers"] != "ETag, Location, Retry-After" {
		t.Fatal("adapter lost CORS headers")
	}
	event.RequestContext.HTTP.Method = "OPTIONS"
	event.Headers["access-control-request-method"] = "PATCH"
	event.Headers["access-control-request-headers"] = "authorization,content-type,if-match,idempotency-key"
	response, err = adapter(context.Background(), event)
	if err != nil || response.StatusCode != 204 {
		t.Fatal("cloud preflight failed")
	}
	event.Headers["origin"] = "https://evil.example"
	response, err = adapter(context.Background(), event)
	if err != nil || response.StatusCode != 403 {
		t.Fatal("foreign origin accepted")
	}
}
func TestCloudConfigRejectsUnsafeConfiguration(t *testing.T) {
	t.Setenv("KURIER_STAGE", "local")
	t.Setenv("AWS_REGION", "us-east-2")
	t.Setenv("KURIER_CONTROL_TABLE", "test")
	if _, err := cloudConfig(context.Background()); err == nil {
		t.Fatal("local stage accepted by cloud adapter")
	}
	t.Setenv("KURIER_STAGE", "dev-api")
	t.Setenv("KURIER_DYNAMODB_ENDPOINT", "http://127.0.0.1:8000")
	if _, err := cloudConfig(context.Background()); err == nil {
		t.Fatal("endpoint override accepted")
	}
}

func TestHTTPAPIV2AdapterRetainsBodyQueryETagAndRejectsMalformedInput(t *testing.T) {
	seen := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = true
		body, _ := io.ReadAll(r.Body)
		if string(body) != "{\"name\":\"cloud\"}" || r.URL.Query().Get("cursor") != "a+b/c=" || r.Header.Get("If-Match") != "\"4\"" {
			t.Error("request semantics lost")
		}
		w.Header().Set("ETag", "\"5\"")
		w.Header().Set("Location", "/api/v1/projects/opaque")
		w.WriteHeader(200)
		_, _ = w.Write([]byte("{\"data\":{}}"))
	})
	adapter := proxyHTTPV2(handler)
	event := events.APIGatewayV2HTTPRequest{Version: "2.0", RawPath: "/api/v1/projects/opaque", RawQueryString: "cursor=a%2Bb%2Fc%3D", IsBase64Encoded: true, Body: base64.StdEncoding.EncodeToString([]byte("{\"name\":\"cloud\"}")), Headers: map[string]string{"if-match": "\"4\""}, RequestContext: events.APIGatewayV2HTTPRequestContext{HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{Method: "PATCH"}}}
	response, err := adapter(context.Background(), event)
	if err != nil || !seen || response.Headers[http.CanonicalHeaderKey("ETag")] != "\"5\"" || response.Headers["Location"] != "/api/v1/projects/opaque" {
		t.Fatal("response semantics lost")
	}
	seen = false
	event.RawPath = "/%ZZ-secret"
	response, err = adapter(context.Background(), event)
	if err != nil || seen || response.StatusCode != 400 || strings.Contains(response.Body, "secret") {
		t.Fatal("invalid URL did not fail safely")
	}
}

func TestProxySavedConfigurationTransportAndResponseBounds(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(payload)
	})
	proxy := proxyHTTPV2(handler)
	event := events.APIGatewayV2HTTPRequest{Version: "2.0", RawPath: "/api/v1/projects/project/requests", Body: strings.Repeat("<", 128*1024)}
	event.RequestContext.HTTP.Method = "POST"
	response, err := proxy(context.Background(), event)
	if err != nil || response.StatusCode != 200 || !response.IsBase64Encoded {
		t.Fatal("accepted transport not supported")
	}
	decoded, err := base64.StdEncoding.DecodeString(response.Body)
	if err != nil || string(decoded) != event.Body {
		t.Fatal("response changed")
	}
	event.Body = base64.StdEncoding.EncodeToString([]byte(event.Body))
	event.IsBase64Encoded = true
	response, err = proxy(context.Background(), event)
	if err != nil || response.StatusCode != 200 {
		t.Fatal("base64 transport rejected")
	}
	event.Body = base64.StdEncoding.EncodeToString([]byte(strings.Repeat("x", 128*1024+1)))
	response, err = proxy(context.Background(), event)
	if err != nil || response.StatusCode != 413 {
		t.Fatal("decoded transport bound not enforced")
	}
}
