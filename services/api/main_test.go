package main

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DrKaelum/Kurier/services/api/internal/ownership"
)

func TestHealth(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()

	handler, err := runtimeHandler(ownership.NewStore(nil, "unused", "unused"), testRuntimeConfig)
	if err != nil {
		t.Fatal(err)
	}
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("expected application/json, got %q", contentType)
	}
}

func testRuntimeConfig(k string) string {
	return map[string]string{"KURIER_COGNITO_ISSUER": "https://cognito-idp.us-east-2.amazonaws.com/us-east-2_Example", "KURIER_COGNITO_CLIENT_ID": "example-client", "KURIER_CURSOR_KEY_BASE64": base64.StdEncoding.EncodeToString(make([]byte, 32))}[k]
}

func TestRuntimeAuthenticationConfigurationFailsClosed(t *testing.T) {
	store := ownership.NewStore(nil, "unused", "unused")
	for _, field := range []string{"KURIER_COGNITO_ISSUER", "KURIER_COGNITO_CLIENT_ID", "KURIER_CURSOR_KEY_BASE64"} {
		t.Run(field, func(t *testing.T) {
			_, err := runtimeHandler(store, func(k string) string {
				if k == field {
					return ""
				}
				return testRuntimeConfig(k)
			})
			if err == nil {
				t.Fatal("missing configuration did not fail closed")
			}
		})
	}
	h, err := runtimeHandler(store, testRuntimeConfig)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/healthz", nil))
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/users/me", nil))
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
}
