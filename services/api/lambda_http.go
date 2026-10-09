package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/VioletProto/Kurier/services/api/execution"
	"github.com/aws/aws-lambda-go/events"
)

// Adapt only HTTP API payload v2. Conversion errors never log request URLs,
// headers or body data. The shared handler retains all auth/ownership checks.
func proxyHTTPV2(handler http.Handler) func(context.Context, events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	return func(ctx context.Context, event events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
		bad := func(status int, code, message string) (events.APIGatewayV2HTTPResponse, error) {
			body, _ := json.Marshal(map[string]any{"error": map[string]any{"code": code, "message": message, "details": []any{}, "requestId": event.RequestContext.RequestID}})
			return events.APIGatewayV2HTTPResponse{StatusCode: status, Headers: map[string]string{"Content-Type": "application/json", "Cache-Control": "no-store"}, Body: string(body)}, nil
		}
		if event.Version != "2.0" || !strings.HasPrefix(event.RawPath, "/") {
			return bad(400, "invalid_request", "Invalid request.")
		}
		path, err := url.PathUnescape(event.RawPath)
		if err != nil {
			return bad(400, "invalid_request", "Invalid request.")
		}
		body := event.Body
		if len(body) > ((128*1024+2)/3)*4 {
			return bad(413, "payload_too_large", "Request body is too large.")
		}
		if event.IsBase64Encoded {
			decoded, err := base64.StdEncoding.DecodeString(body)
			if err != nil {
				return bad(400, "invalid_request", "Invalid request.")
			}
			body = string(decoded)
		}
		if len(body) > 128*1024 {
			return bad(413, "payload_too_large", "Request body is too large.")
		}
		target := (&url.URL{Scheme: "https", Host: "api.invalid", Path: path, RawPath: event.RawPath, RawQuery: event.RawQueryString}).String()
		request, err := http.NewRequestWithContext(ctx, event.RequestContext.HTTP.Method, target, strings.NewReader(body))
		if err != nil {
			return bad(400, "invalid_request", "Invalid request.")
		}
		for k, v := range event.Headers {
			request.Header.Set(k, v)
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		result := response.Result()
		defer result.Body.Close()
		payload, err := io.ReadAll(result.Body)
		if err != nil {
			return bad(500, "internal_error", "Request could not be completed.")
		}
		headers := map[string]string{}
		for k, v := range result.Header {
			headers[k] = strings.Join(v, ", ")
		}
		evidenceRoute := strings.HasSuffix(event.RawPath, "/evidence") && strings.Contains(event.RawPath, "/executions/")
		if evidenceRoute && len(payload) > execution.EvidenceLimit {
			return bad(503, "evidence_unavailable", "Evidence unavailable.")
		}
		proxy := events.APIGatewayV2HTTPResponse{StatusCode: result.StatusCode, Headers: headers, Body: string(payload)}
		if evidenceRoute || len(payload) > 64*1024 {
			proxy.Body = base64.StdEncoding.EncodeToString(payload)
			proxy.IsBase64Encoded = true
		}
		metadata := proxy
		metadata.Body = ""
		encodedMetadata, _ := json.Marshal(metadata)
		encoded, _ := json.Marshal(proxy)
		// The Lambda runtime uses json.Marshal for the handler result. Include
		// one byte of framing headroom rather than count only Body/base64.
		if len(encodedMetadata) > execution.MetadataLimit || len(encoded)+1 > execution.LambdaLimit {
			return bad(503, "evidence_unavailable", "Evidence unavailable.")
		}
		return proxy, nil
	}
}
