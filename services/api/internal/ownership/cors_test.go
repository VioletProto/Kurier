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
		{"pin preflight", "http://localhost:5173", "OPTIONS", "PUT", "Authorization", 204, 0},
		{"preflight", "http://localhost:5173", "OPTIONS", "PATCH", "Authorization, Content-Type, If-Match", 204, 0},
		{"foreign", "http://localhost:5174", "POST", "", "", 403, 0},
		{"null", "null", "GET", "", "", 403, 0},
		{"bad header", "http://localhost:5173", "OPTIONS", "POST", "X-User-ID", 403, 0},
		{"bad method", "http://localhost:5173", "OPTIONS", "TRACE", "Authorization", 404, 0},
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

func TestHostedFrontendCORS(t *testing.T) {
	called := 0
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		w.Header().Set("ETag", `"3"`)
		w.Header().Set("Location", "/api/v1/projects/opaque")
		w.WriteHeader(412)
	})
	h, err := FrontendCORS(next, "http://localhost:5173", "https://dexample.cloudfront.net")
	if err != nil {
		t.Fatal(err)
	}
	for _, origin := range []string{"http://localhost:5173", "https://dexample.cloudfront.net"} {
		for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE"} {
			r := httptest.NewRequest("OPTIONS", "/api/v1/projects/opaque", nil)
			r.Header.Set("Origin", origin)
			r.Header.Set("Access-Control-Request-Method", method)
			r.Header.Set("Access-Control-Request-Headers", "authorization, content-type, if-match, idempotency-key")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != 204 || w.Header().Get("Access-Control-Allow-Origin") != origin || called != 0 {
				t.Fatal("approved preflight failed")
			}
		}
		r := httptest.NewRequest("PATCH", "/api/v1/projects/opaque", nil)
		r.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 412 || w.Header().Get("ETag") != `"3"` || w.Header().Get("Access-Control-Expose-Headers") != "ETag, Location, Retry-After" || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("application error headers lost")
		}
		called = 0
	}
	for _, origins := range [][]string{{"null"}, {"https://dother.cloudfront.net"}, {"https://dexample.cloudfront.net.evil.example"}, {"http://127.0.0.1:5173"}, {"http://localhost:5174"}, {"https://dexample.cloudfront.net", "https://dexample.cloudfront.net"}} {
		r := httptest.NewRequest("GET", "/api/v1/users/me", nil)
		r.Header["Origin"] = origins
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 403 || called != 0 || w.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Fatal("unapproved origin accepted")
		}
	}
	for _, hosted := range []string{"*", "null", "https://example.com", "https://dexample.cloudfront.net/", "https://dexample.cloudfront.net:443", "https://user@dexample.cloudfront.net", "https://dexample.cloudfront.net?x=1", "http://dexample.cloudfront.net"} {
		if _, err := FrontendCORS(next, "http://localhost:5173", hosted); err == nil {
			t.Fatal("unsafe hosted configuration accepted")
		}
	}
}
