package ownership

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLocalCORS(t *testing.T) {
	called := 0
	h, err := LocalCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		w.Header().Set("ETag", `"0"`)
		w.Header().Set("Location", "/api/v1/projects/opaque")
		w.WriteHeader(401)
	}), "http://localhost:5173")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, origin, method, requested, headers string
		status, calls                            int
	}{
		{"preflight", "http://localhost:5173", "OPTIONS", "PATCH", "Authorization, Content-Type, If-Match", 204, 0},
		{"foreign", "http://localhost:5174", "POST", "", "", 403, 0},
		{"null", "null", "GET", "", "", 403, 0},
		{"bad header", "http://localhost:5173", "OPTIONS", "POST", "X-User-ID", 403, 0},
		{"bad method", "http://localhost:5173", "OPTIONS", "PUT", "Authorization", 404, 0},
		{"auth still required", "http://localhost:5173", "GET", "", "", 401, 1},
		{"cli", "", "GET", "", "", 401, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			before := called
			r := httptest.NewRequest(test.method, "/api/v1/users/me", nil)
			if test.origin != "" {
				r.Header.Set("Origin", test.origin)
			}
			r.Header.Set("Access-Control-Request-Method", test.requested)
			r.Header.Set("Access-Control-Request-Headers", test.headers)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != test.status || called-before != test.calls {
				t.Fatalf("status %d, calls %d", w.Code, called-before)
			}
			if w.Header().Get("Access-Control-Allow-Credentials") != "" {
				t.Fatal("cookie credentials enabled")
			}
			if test.origin == "http://localhost:5173" && !strings.Contains(w.Header().Get("Access-Control-Expose-Headers"), "ETag, Location") {
				t.Fatal("missing exposed response headers")
			}
			if test.origin != "http://localhost:5173" && w.Header().Get("Access-Control-Allow-Origin") != "" {
				t.Fatal("foreign origin allowed")
			}
		})
	}
	for _, origin := range []string{"*", "https://example.com", "http://localhost", "http://localhost:5173/path", "http://user@localhost:5173", "http://localhost:5173?x=1"} {
		if _, err := LocalCORS(h, origin); err == nil {
			t.Fatalf("accepted %q", origin)
		}
	}
}
