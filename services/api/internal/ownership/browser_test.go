//go:build integration && browser

package ownership

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// This harness exists only in test builds. No fixture trust, token endpoint or
// cleanup HTTP route is compiled into the runnable API.
func TestIntegrationBrowserProjects(t *testing.T) {
	i, pinProject, pinExecution, _ := pinLocal(t)
	var pinTime atomic.Int64
	pinTime.Store(time.Now().UnixNano())
	i.store.executions.Now = func() time.Time { return time.Unix(0, pinTime.Load()).UTC() }
	for n := 0; n < 27; n++ {
		if _, err := i.store.CreateProject(context.Background(), i.alice.UserID, "Seed "+twoDigits(n)); err != nil {
			t.Fatal(err)
		}
	}
	h, err := LocalCORS(i.server, "http://localhost:5175")
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/", h)
	localTarget := configureBrowserLocalAgent(t, i, mux, &pinTime)
	configureBrowserWorkflows(t, i)
	mux.HandleFunc("POST /test-only/pin-expire", func(w http.ResponseWriter, r *http.Request) {
		pinTime.Store(time.Now().Add(31 * 24 * time.Hour).UnixNano())
		w.WriteHeader(204)
	})
	mux.HandleFunc("POST /test-only/cleanup", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ProjectID string `json:"projectId"`
		}
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			w.WriteHeader(400)
			return
		}
		if _, err := i.store.CleanupProject(r.Context(), body.ProjectID); err != nil {
			w.WriteHeader(503)
			return
		}
		w.WriteHeader(200)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "npm", "run", "test:browser", "--workspace", "@kurier/web")
	cmd.Dir = "../../../.."
	cmd.Env = append(os.Environ(), "PLAYWRIGHT_NO_COPY_PROMPT=1", "KURIER_BROWSER_LOCAL_TARGET="+localTarget, "KURIER_BROWSER_PIN_PROJECT="+pinProject.ProjectID, "KURIER_BROWSER_PIN_EXECUTION="+pinExecution, "KURIER_BROWSER_BOB_TOKEN="+i.auth.token(t, "bob", nil), "VITE_API_URL="+server.URL, "VITE_COGNITO_USER_POOL_ID=us-east-2_fixture", "VITE_COGNITO_CLIENT_ID=fixture-client", "KURIER_BROWSER_FIXTURE_TOKEN="+i.auth.token(t, "alice", nil), "KURIER_BROWSER_FIXTURE_ID_TOKEN="+i.auth.token(t, "alice", func(c jwt.MapClaims) {
		c["token_use"] = "id"
		c["aud"] = "fixture-client"
		c["cognito:username"] = "alice"
	}))
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("browser fixture suite failed: %s", output)
	}
	t.Log("Browser-to-Go/DynamoDB fixture flow passed; Cognito/email are simulated, not AWS validated.")
}
func twoDigits(n int) string { return string([]byte{'0' + byte(n/10), '0' + byte(n%10)}) }
