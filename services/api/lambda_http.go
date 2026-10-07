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
		if len(payload) > 64*1024 {
			// Gateway decodes this transparently. Base64 prevents Lambda's outer JSON
			// string escaping from amplifying a bounded public JSON list past its limit.
			return events.APIGatewayV2HTTPResponse{StatusCode: result.StatusCode, Headers: headers, Body: base64.StdEncoding.EncodeToString(payload), IsBase64Encoded: true}, nil
		}
		return events.APIGatewayV2HTTPResponse{StatusCode: result.StatusCode, Headers: headers, Body: string(payload)}, nil
	}
}
