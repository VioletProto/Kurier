//go:build integration

package ownership

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

// API Gateway NONE must never bypass the same application guards used locally.
func TestIntegrationEnvironmentGatewayNoneNegativeGuards(t *testing.T) {
	i, p := environmentLocal(t)
	collection := "/api/v1/projects/" + p.ProjectID + "/environments"
	detail := collection + "/" + newID()
	for _, route := range []struct{ method, path string }{{"GET", collection}, {"POST", collection}, {"GET", detail}, {"PATCH", detail}, {"DELETE", detail}} {
		for _, headers := range [][]string{nil, {"Bearer invalid"}, {"Basic invalid"}, {"Bearer invalid", "Bearer invalid"}, {"Bearer " + i.auth.token(t, "alice", func(c jwt.MapClaims) { c["token_use"] = "id" })}} {
			req := httptest.NewRequest(route.method, route.path, strings.NewReader(`{}`))
			req.Header.Set("Content-Type", "application/json")
			for _, h := range headers {
				req.Header.Add("Authorization", h)
			}
			w := httptest.NewRecorder()
			i.server.ServeHTTP(w, req)
			if w.Code != 401 {
				t.Fatalf("environment authentication gate %s: %d", route.method, w.Code)
			}
		}
		if w := i.request(t, "bob", route.method, route.path, `{}`, ""); w.Code != 404 {
			t.Fatalf("foreign owner gate: %d", w.Code)
		}
	}
	for _, headers := range [][]string{nil, {"Bearer invalid"}, {"Bearer " + i.auth.token(t, "alice", nil)}, {"Bearer invalid", "Bearer invalid"}} {
		req := httptest.NewRequest("POST", "/api/v1/local-agent/capabilities", strings.NewReader(`{"schemaVersion":1}`))
		req.Header.Set("Content-Type", "application/json")
		for _, h := range headers {
			req.Header.Add("Authorization", h)
		}
		w := httptest.NewRecorder()
		i.server.ServeHTTP(w, req)
		if w.Code != 401 {
			t.Fatalf("agent credential separation: %d", w.Code)
		}
	}
}
