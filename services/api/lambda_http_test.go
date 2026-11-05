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
	"github.com/aws/aws-lambda-go/events"
)

const importIngressPath = "/api/v1/projects/12345678-1234-4123-8123-123456789abc/openapi-imports"

func importIngressEvent(path, method, body string, encoded bool) events.APIGatewayV2HTTPRequest {
	if encoded {
		body = base64.StdEncoding.EncodeToString([]byte(body))
	}
	return events.APIGatewayV2HTTPRequest{Version: "2.0", RawPath: path, Body: body, IsBase64Encoded: encoded, Headers: map[string]string{"content-type": "application/json"}, RequestContext: events.APIGatewayV2HTTPRequestContext{HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{Method: method}}}
}
func TestOpenAPIImportIngressSupportsBoundedSourceEnvelope(t *testing.T) {
	// An actual JSON envelope containing a supported document plus legal source
	// whitespace reaches the handler intact above the ordinary 128 KiB ceiling.
	source := `{"openapi":"3.0.4","info":{"title":"Fixture","version":"1"},"paths":{"/items":{"get":{"responses":{"200":{"description":"OK"}}}}}}`
	source += strings.Repeat(" ", 2*1024*1024-len(source))
	raw, e := json.Marshal(map[string]any{"format": "json", "source": source, "publicDataConfirmed": true})
	if e != nil {
		t.Fatal(e)
	}
	for _, encoded := range []bool{false, true} {
		calls := 0
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			received, e := io.ReadAll(r.Body)
			if e != nil || string(received) != string(raw) {
				t.Error("import envelope changed")
			}
			var input struct {
				Source string `json:"source"`
			}
			if json.Unmarshal(received, &input) != nil || len(input.Source) != 2*1024*1024 {
				t.Error("decoded source changed")
			}
			w.WriteHeader(201)
		})
		response, e := proxyHTTPV2(handler)(context.Background(), importIngressEvent(importIngressPath, "POST", string(raw), encoded))
		if e != nil || response.StatusCode != 201 || calls != 1 {
			t.Fatalf("encoded=%v status=%d calls=%d err=%v", encoded, response.StatusCode, calls, e)
		}
	}
}
func TestOpenAPIImportIngressDoesNotWidenOtherRoutes(t *testing.T) {
	cases := []struct{ name, path, method, query string }{
		{"get", importIngressPath, "GET", ""},
		{"patch", importIngressPath, "PATCH", ""},
		{"delete", importIngressPath, "DELETE", ""},
		{"options", importIngressPath, "OPTIONS", ""},
		{"detail", importIngressPath + "/12345678-1234-4123-8123-123456789abc", "POST", ""},
		{"publish", importIngressPath + "/12345678-1234-4123-8123-123456789abc/publish", "POST", ""},
		{"trailing-slash", importIngressPath + "/", "POST", ""},
		{"nested", importIngressPath + "/anything/openapi-imports", "POST", ""},
		{"prefix", "/unexpected" + importIngressPath, "POST", ""},
		{"non-uuid", "/api/v1/projects/project/openapi-imports", "POST", ""},
		{"encoded-path", strings.Replace(importIngressPath, "openapi-imports", "%6fpenapi-imports", 1), "POST", ""},
		{"encoded-separator", strings.Replace(importIngressPath, "/openapi-imports", "%2fopenapi-imports", 1), "POST", ""},
		{"other-collection", strings.Replace(importIngressPath, "openapi-imports", "requests", 1), "POST", ""},
		{"query", importIngressPath, "POST", "unexpected=1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, encoded := range []bool{false, true} {
				calls := 0
				handler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls++ })
				event := importIngressEvent(tc.path, tc.method, strings.Repeat("x", 128*1024+1), encoded)
				event.RawQueryString = tc.query
				response, e := proxyHTTPV2(handler)(context.Background(), event)
				if e != nil || response.StatusCode != 413 || calls != 0 {
					t.Fatalf("encoded=%v status=%d calls=%d err=%v", encoded, response.StatusCode, calls, e)
				}
			}
		})
	}
}
func TestOpenAPIImportIngressRetainsBodyAndLambdaEnvelopeBounds(t *testing.T) {
	for _, tc := range []struct {
		name     string
		size     int
		encoded  bool
		char     string
		metadata bool
		status   int
	}{
		{"plain-limit", 5 * 1024 * 1024, false, "x", false, 204},
		{"plain-over", 5*1024*1024 + 1, false, "x", false, 413},
		{"decoded-over", 5*1024*1024 + 1, true, "x", false, 413},
		{"base64-envelope", 5 * 1024 * 1024, true, "x", false, 413},
		{"escaped-envelope", 2 * 1024 * 1024, false, "<", false, 413},
		{"metadata", 128*1024 + 1, false, "x", true, 413},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				raw, e := io.ReadAll(r.Body)
				if e != nil || len(raw) != tc.size {
					t.Error("accepted body changed")
				}
				w.WriteHeader(204)
			})
			event := importIngressEvent(importIngressPath, "POST", strings.Repeat(tc.char, tc.size), tc.encoded)
			if tc.metadata {
				event.Headers["x-metadata"] = strings.Repeat("x", 32*1024)
			}
			response, e := proxyHTTPV2(handler)(context.Background(), event)
			if e != nil || response.StatusCode != tc.status || (tc.status == 204 && calls != 1) || (tc.status != 204 && calls != 0) {
				t.Fatalf("status=%d calls=%d err=%v", response.StatusCode, calls, e)
			}
			if tc.name == "base64-envelope" {
				raw, _ := json.Marshal(event)
				if len(raw)+1 <= execution.LambdaLimit {
					t.Fatal("fixture did not exceed Lambda transport")
				}
			}
		})
	}
}
