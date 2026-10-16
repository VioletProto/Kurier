package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	wire "github.com/VioletProto/Kurier/services/request-runtime/mcpwire"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func fixtureBridge(t *testing.T) (*bridge, *atomic.Bool) {
	t.Helper()
	revoked := &atomic.Bool{}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/mcp/invocations" || r.Method != "POST" {
			t.Error("authority expanded")
			w.WriteHeader(404)
			return
		}
		var in wire.Invocation
		raw, _ := io.ReadAll(io.LimitReader(r.Body, wire.InputLimit+1))
		if strict(raw, &in) != nil {
			t.Error("invalid bridge input")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if revoked.Load() {
			w.WriteHeader(401)
			_ = json.NewEncoder(w).Encode(safeError("unauthenticated"))
			return
		}
		now := time.Now().UTC()
		w.Header().Set("X-Kurier-Release-Until", now.Add(5*time.Second).Format(time.RFC3339Nano))
		_ = json.NewEncoder(w).Encode(wire.Envelope{Data: &wire.Data{SchemaVersion: 1, ServerTime: now.Format(time.RFC3339Nano), Items: []json.RawMessage{}}})
	}))
	t.Cleanup(srv.Close)
	b, e := newBridge(storedCredential{APIOrigin: srv.URL, Token: strings.Repeat("a", 43)})
	if e != nil {
		t.Fatal(e)
	}
	b.client = srv.Client()
	return b, revoked
}
func TestOfficialSDKProtocolAndRevocation(t *testing.T) {
	b, revoked := fixtureBridge(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	input, writer, e := os.Pipe()
	if e != nil {
		t.Fatal(e)
	}
	reader, output, e := os.Pipe()
	if e != nil {
		t.Fatal(e)
	}
	defer input.Close()
	defer writer.Close()
	defer reader.Close()
	defer output.Close()
	done := make(chan error, 1)
	go func() { done <- serve(ctx, b, input, output) }()
	client := mcp.NewClient(&mcp.Implementation{Name: "protocol-fixture", Version: "1"}, nil)
	session, e := client.Connect(ctx, &mcp.IOTransport{Reader: reader, Writer: writer}, nil)
	if e != nil {
		t.Fatal("SDK protocol connection failed", e)
	}
	if session.InitializeResult().ProtocolVersion != protocolVersion {
		t.Fatal("unexpected protocol revision")
	}
	tools, e := session.ListTools(ctx, nil)
	if e != nil || len(tools.Tools) != 7 {
		t.Fatal("tool discovery failed")
	}
	for _, tool := range tools.Tools {
		if tool.OutputSchema == nil || !tool.Annotations.ReadOnlyHint {
			t.Fatal("missing schema or read-only declaration")
		}
	}
	r, e := session.CallTool(ctx, &mcp.CallToolParams{Name: "list_projects", Arguments: map[string]any{}})
	if e != nil || r.IsError {
		t.Fatal("read failed", e)
	}
	r, e = session.CallTool(ctx, &mcp.CallToolParams{Name: "list_projects"})
	if e != nil || r.IsError {
		t.Fatal("omitted optional arguments rejected")
	}
	revoked.Store(true)
	r, e = session.CallTool(ctx, &mcp.CallToolParams{Name: "list_projects", Arguments: map[string]any{}})
	if e != nil || !r.IsError {
		t.Fatal("revocation did not deny next call")
	}
	r, e = session.CallTool(ctx, &mcp.CallToolParams{Name: "list_projects", Arguments: map[string]any{"token": "canary"}})
	if e != nil {
		t.Fatal("unexpected protocol error")
	}
	raw, _ := json.Marshal(r)
	if !r.IsError || strings.Contains(string(raw), "canary") {
		t.Fatal("invalid input leaked or accepted")
	}
	session.Close()
	writer.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("EOF did not stop foreground process")
	}
}
func TestCredentialFilesRejectSymlinksModesAndReplacement(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "private", "credential.json")
	value := storedCredential{APIOrigin: "https://api.example.test", Token: strings.Repeat("a", 43)}
	if e := writeCredential(path, value); e != nil {
		t.Fatal(e)
	}
	got, e := readCredential(path)
	if e != nil || got != value {
		t.Fatal("private credential roundtrip failed")
	}
	if e = os.Chmod(path, 0644); e != nil {
		t.Fatal(e)
	}
	if _, e = readCredential(path); e == nil {
		t.Fatal("public credential accepted")
	}
	_ = os.Chmod(path, 0600)
	link := filepath.Join(dir, "private", "link")
	if e = os.Symlink(path, link); e != nil {
		t.Fatal(e)
	}
	if _, e = readCredential(link); e == nil {
		t.Fatal("symlink followed")
	}
	if e = os.Link(path, filepath.Join(dir, "private", "hardlink")); e != nil {
		t.Fatal(e)
	}
	if _, e = readCredential(path); e == nil {
		t.Fatal("hardlinked capability accepted")
	}
	if e = writeCredential(path, value); e != nil {
		t.Fatal(e)
	}
	if _, e = readCredential(path); e != nil {
		t.Fatal("atomic replacement invalid")
	}
}
func TestBridgeBoundsRedirectsAndOrigin(t *testing.T) {
	for _, origin := range []string{"http://localhost:3000", "https://user:secret@api.test", "https://api.test/?token=secret", "https://api.test/#frag", "https://api.test/other"} {
		if validOrigin(origin) {
			t.Fatal("unsafe API origin accepted")
		}
	}
	b, _ := fixtureBridge(t)
	if _, _, e := b.invoke(context.Background(), "list_projects", json.RawMessage(`{"url":"https://target.test"}`)); e == nil {
		t.Fatal("arbitrary URL accepted")
	}
	if newBridgeConfig := b.credential; newBridgeConfig.Token == "" {
		t.Fatal("invalid fixture")
	}
}

func TestProtocolWriterSanitizesAndExpiresAtActualWrite(t *testing.T) {
	reader, output, e := os.Pipe()
	if e != nil {
		t.Fatal(e)
	}
	defer reader.Close()
	defer output.Close()
	pipe, e := deadlineOutput(output)
	if e != nil {
		t.Fatal(e)
	}
	defer pipe.Close()
	w := &protocolWriter{out: pipe, deadlines: map[[32]byte][]time.Time{}}
	diagnostic := []byte("{\"jsonrpc\":\"2.0\",\"id\":1,\"error\":{\"code\":-32602,\"message\":\"credential-canary\",\"data\":{\"value\":\"credential-canary\"}}}\n")
	if n, e := w.Write(diagnostic); e != nil || n != len(diagnostic) {
		t.Fatal("safe protocol error failed")
	}
	buf := make([]byte, 1024)
	n, e := reader.Read(buf)
	if e != nil || strings.Contains(string(buf[:n]), "credential-canary") || !json.Valid(buf[:n]) {
		t.Fatal("SDK protocol error leaked untrusted values")
	}
	env := wire.Envelope{Data: &wire.Data{SchemaVersion: 1, ServerTime: time.Now().Format(time.RFC3339Nano), Items: []json.RawMessage{}}}
	raw, _ := json.Marshal(env)
	key := sha256.Sum256(raw)
	w.deadlines[key] = []time.Time{time.Now().Add(-time.Second)}
	packet, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 2, "result": result(env)})
	if _, e = w.Write(append(packet, '\n')); e == nil {
		t.Fatal("expired result written")
	}
	w.deadlines[key] = []time.Time{time.Now().Add(20 * time.Millisecond)}
	huge := strings.Repeat("x", 200*1024)
	env.Data.Items = []json.RawMessage{json.RawMessage(`{"name":"` + huge + `"}`)}
	raw, _ = json.Marshal(env)
	w.deadlines[sha256.Sum256(raw)] = []time.Time{time.Now().Add(20 * time.Millisecond)}
	packet, _ = json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 3, "result": result(env)})
	if _, e = w.Write(append(packet, '\n')); e == nil {
		t.Fatal("oversized result written")
	}
}
func TestCredentialAncestorAndOversizeRefusal(t *testing.T) {
	dir := t.TempDir()
	private := filepath.Join(dir, "private")
	if e := os.Mkdir(private, 0700); e != nil {
		t.Fatal(e)
	}
	alias := filepath.Join(dir, "alias")
	if e := os.Symlink(private, alias); e != nil {
		t.Fatal(e)
	}
	if e := writeCredential(filepath.Join(alias, "credential.json"), storedCredential{}); e == nil {
		t.Fatal("credential ancestor symlink followed")
	}
	path := filepath.Join(private, "credential.json")
	if e := os.WriteFile(path, []byte(strings.Repeat("x", 4097)), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := readCredential(path); e == nil {
		t.Fatal("oversize capability file accepted")
	}
}
