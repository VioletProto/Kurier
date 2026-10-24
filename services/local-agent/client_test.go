package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	runner "github.com/VioletProto/Kurier/services/request-runtime"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testWork(t *testing.T, target string) (runner.Work, runner.Agent) {
	t.Helper()
	d, e := runner.NormalizeLocalDestination(runner.Destination{Origin: target, AddressClass: "loopback"})
	if e != nil {
		t.Fatal(e)
	}
	now := time.Now().UTC()
	w := runner.Work{SchemaVersion: 1, ProjectID: runner.NewID(), JobID: runner.NewID(), ExecutionID: runner.NewID(), LocalAgentID: runner.NewID(), RecoveryGeneration: "fixture", LeaseID: runner.NewID(), Fence: 1, Nonce: base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)), DispatchIntentAt: now.Format(time.RFC3339Nano), ExecuteNotAfter: now.Add(30 * time.Second).Format(time.RFC3339Nano), Deadline: now.Add(120 * time.Second).Format(time.RFC3339Nano), Destination: d}
	w.Plan = runner.Plan{Target: "local", LocalAgentID: w.LocalAgentID, Destination: &d, RequestID: runner.NewID(), Revision: 1, Configuration: runner.Configuration{Name: "fixture", Method: "GET", URL: target, Headers: []runner.Field{}, QueryParameters: []runner.Field{}}, Options: runner.Options{Target: "local", Local: &runner.LocalOptions{LocalAgentID: w.LocalAgentID, AllowPrivateNetwork: true}, TimeoutSeconds: 30, ResponseRedaction: runner.RedactionPolicy{Headers: []string{}, JSONPointers: []string{}}}}
	w.ProtectedInputs.Values = map[string]string{}
	a := runner.Agent{LocalAgentID: w.LocalAgentID, Name: "fixture", RecoveryGeneration: w.RecoveryGeneration, Scopes: []runner.Scope{{ProjectID: w.ProjectID, Destinations: []runner.Destination{d}}}}
	return w, a
}
func TestAgentLostUploadResponseRetriesPacketNeverHTTP(t *testing.T) {
	var sends, uploads atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sends.Add(1)
		if r.Header.Get("Authorization") != "" {
			t.Error("agent bearer leaked to target")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer target.Close()
	work, a := testWork(t, target.URL)
	var previous []byte
	hosted := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-only-agent-token" {
			t.Error("hosted auth missing")
		}
		var result runner.Result
		_ = json.NewDecoder(r.Body).Decode(&result)
		raw, _ := json.Marshal(result)
		if previous != nil && !bytes.Equal(raw, previous) {
			t.Error("retry changed candidate")
		}
		previous = raw
		if uploads.Add(1) == 1 {
			w.WriteHeader(503)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": runner.Receipt{ExecutionID: work.ExecutionID, AcceptedAt: time.Now().Format(time.RFC3339Nano)}})
	}))
	defer hosted.Close()
	c, e := newClient(hosted.URL, "test-only-agent-token")
	if e != nil {
		t.Fatal(e)
	}
	c.http = hosted.Client()
	c.agent = a
	c.pause = func(context.Context, time.Duration) error { return nil }
	if e = c.execute(context.Background(), work, time.Now().Format(time.RFC3339Nano), time.Now()); e != nil {
		t.Fatal(e)
	}
	if e = c.execute(context.Background(), work, time.Now().Format(time.RFC3339Nano), time.Now()); e != nil {
		t.Fatal(e)
	}
	if sends.Load() != 1 || uploads.Load() != 2 {
		t.Fatal("lost ACK caused target replay or failed packet reconciliation")
	}
}
func TestAgentScopeDeadlinesAndHostedBoundary(t *testing.T) {
	var sends atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { sends.Add(1) }))
	defer target.Close()
	for _, mode := range []string{"expired", "wrong-origin", "wrong-generation", "incomplete", "missing-nonce"} {
		work, a := testWork(t, target.URL)
		c, _ := newClient("https://api.invalid", "fixture")
		c.agent = a
		switch mode {
		case "expired":
			work.ExecuteNotAfter = time.Now().Add(-time.Second).Format(time.RFC3339Nano)
		case "wrong-origin":
			work.Destination.Origin = "http://localhost:1"
		case "wrong-generation":
			work.RecoveryGeneration = "other"
		case "incomplete":
			work.SchemaVersion = 0
		case "missing-nonce":
			work.Nonce = ""
		}
		if c.execute(context.Background(), work, time.Now().Format(time.RFC3339Nano), time.Now()) == nil {
			t.Fatal("invalid grant accepted")
		}
	}
	if sends.Load() != 0 {
		t.Fatal("unsafe HTTP attempt")
	}
	for _, origin := range []string{"http://localhost:8080", "https://u:p@api.invalid", "https://api.invalid/path", "https://api.invalid?x=1"} {
		if _, e := newClient(origin, "fixture"); e == nil {
			t.Fatal("unsafe hosted origin accepted")
		}
	}
}
func TestAgentStorageConsentAndContainment(t *testing.T) {
	dir := t.TempDir()
	_ = os.Chmod(dir, 0700)
	path := filepath.Join(dir, "credential.json")
	value := storedCredential{"https://api.invalid", "disposable-capability-fixture"}
	if e := writeCredential(path, value); e != nil {
		t.Fatal(e)
	}
	read, e := readCredential(path)
	if e != nil || read != value {
		t.Fatal("private credential persistence failed")
	}
	_ = os.Chmod(path, 0644)
	if _, e = readCredential(path); e == nil {
		t.Fatal("public token permissions accepted")
	}
	_ = os.Remove(path)
	outside := filepath.Join(t.TempDir(), "outside")
	_ = os.WriteFile(outside, []byte("unchanged"), 0600)
	_ = os.Symlink(outside, path)
	if writeCredential(path, value) == nil {
		t.Fatal("symlink token destination accepted")
	}
	raw, _ := os.ReadFile(outside)
	if string(raw) != "unchanged" {
		t.Fatal("symlink target modified")
	}
}
func TestAgentFixedDiagnosticsDoNotExposeHostedResponse(t *testing.T) {
	secret := "disposable-error-payload"
	hosted := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500); _, _ = w.Write([]byte(secret)) }))
	defer hosted.Close()
	c, _ := newClient(hosted.URL, "fixture")
	c.http = hosted.Client()
	e := c.session(context.Background())
	if e == nil || strings.Contains(e.Error(), secret) || strings.Contains(e.Error(), hosted.URL) {
		t.Fatal("raw diagnostics exposed")
	}
}

func TestEnvironmentGrantRequiresNegotiationAndPreservesResultIdentity(t *testing.T) {
	var sends atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sends.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer target.Close()
	work, agent := testWork(t, target.URL)
	work.SchemaVersion = 2
	work.TemplateSemanticsVersion = 1
	work.EnvironmentContext = &runner.EnvironmentContext{EnvironmentID: runner.NewID(), Revision: 7}
	work.Plan.Configuration.TemplateSemanticsVersion = 1
	work.Plan.TemplateSnapshot = &runner.TemplateSnapshot{SchemaVersion: 1, EnvironmentContext: work.EnvironmentContext, Variables: []runner.FrozenVariable{}}
	var result runner.Result
	hosted := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if json.NewDecoder(r.Body).Decode(&result) != nil || runner.ValidateResultContext(result, work.Plan) != nil {
			t.Error("agent lost environment identity")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": runner.Receipt{ExecutionID: work.ExecutionID, AcceptedAt: time.Now().Format(time.RFC3339Nano)}})
	}))
	defer hosted.Close()
	client, e := newClient(hosted.URL, "test-only-agent-token")
	if e != nil {
		t.Fatal(e)
	}
	client.http = hosted.Client()
	client.agent = agent
	if client.workAllowed(work) || sends.Load() != 0 {
		t.Fatal("unnegotiated client admitted Work2")
	}
	client.templateNegotiated = true
	if !client.workAllowed(work) {
		t.Fatal("negotiated Work2 rejected")
	}
	if e = client.execute(context.Background(), work, time.Now().Format(time.RFC3339Nano), time.Now()); e != nil {
		t.Fatal(e)
	}
	if result.SchemaVersion != 2 || result.EnvironmentContext == nil || result.EnvironmentContext.Revision != 7 || sends.Load() != 1 {
		t.Fatal("result version or send count incorrect")
	}
	tampered := work
	other := *work.EnvironmentContext
	other.Revision++
	tampered.EnvironmentContext = &other
	if client.workAllowed(tampered) {
		t.Fatal("grant accepted mismatched frozen environment identity")
	}
}
