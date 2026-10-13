//go:build integration && browser

package ownership

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/pem"
	"errors"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// Test-only KMS context double, memory-only S3 and a compiled foreground CLI.
// None of these resources or trust roots are part of the runnable API.
type browserLocalKMS struct{}

func (browserLocalKMS) GenerateDataKey(_ context.Context, in *kms.GenerateDataKeyInput, _ ...func(*kms.Options)) (*kms.GenerateDataKeyOutput, error) {
	scope, _ := json.Marshal(in.EncryptionContext)
	return &kms.GenerateDataKeyOutput{KeyId: in.KeyId, Plaintext: bytes.Repeat([]byte{7}, 32), CiphertextBlob: scope}, nil
}
func (browserLocalKMS) Decrypt(_ context.Context, in *kms.DecryptInput, _ ...func(*kms.Options)) (*kms.DecryptOutput, error) {
	scope, _ := json.Marshal(in.EncryptionContext)
	if !bytes.Equal(scope, in.CiphertextBlob) {
		return nil, errors.New("fixture scope mismatch")
	}
	return &kms.DecryptOutput{KeyId: in.KeyId, Plaintext: bytes.Repeat([]byte{7}, 32)}, nil
}
func configureBrowserLocalAgent(t *testing.T, i *integration, mux *http.ServeMux, clock *atomic.Int64) string {
	t.Helper()
	ctx := context.Background()
	stage, e := i.store.get(ctx, "STAGE#local", "META")
	if e != nil {
		t.Fatal(e)
	}
	stage.SavedRequestsSchemaVersion = 5
	stage.LocalAgentExecutionsSchemaVersion = 1
	stage.Version++
	item, _ := encode(stage)
	_, e = i.client.PutItem(ctx, &dynamodb.PutItemInput{TableName: &i.store.table, Item: item})
	if e != nil {
		t.Fatal(e)
	}
	keyARN := "arn:aws:kms:us-east-2:747336059622:key/fixture"
	i.store.cipher = &EnvelopeCipher{browserLocalKMS{}, keyARN, "local"}
	i.store.executions.KMS = browserLocalKMS{}
	i.store.executions.KeyARN = keyARN
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "reflected": r.Header.Get("Authorization")})
	}))
	t.Cleanup(target.Close)
	apiTLS := httptest.NewTLSServer(i.server)
	t.Cleanup(apiTLS.Close)
	directory := t.TempDir()
	certFile := filepath.Join(directory, "fixture-ca.pem")
	_ = os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: apiTLS.Certificate().Raw}), 0600)
	binary := filepath.Join(directory, "kurier-agent")
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Dir = "../../../local-agent"
	if _, e = build.CombinedOutput(); e != nil {
		t.Fatal("foreground CLI build failed")
	}
	mux.HandleFunc("POST /test-only/local-agent-reset", func(w http.ResponseWriter, r *http.Request) { clock.Store(time.Now().UnixNano()); w.WriteHeader(204) })
	mux.HandleFunc("POST /test-only/local-agent-run", func(w http.ResponseWriter, r *http.Request) {
		raw, e := io.ReadAll(io.LimitReader(r.Body, 4097))
		if e != nil {
			w.WriteHeader(400)
			return
		}
		defer clear(raw)
		var input struct{ Token, ProjectID, ExecutionID string }
		if json.Unmarshal(raw, &input) != nil {
			w.WriteHeader(400)
			return
		}
		clientCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		command := exec.CommandContext(clientCtx, binary, "run", "--api", apiTLS.URL, "--stdin")
		command.Env = append(os.Environ(), "SSL_CERT_FILE="+certFile)
		command.Stdin = bytes.NewBufferString(input.Token + "\nyes\n")
		var logs bytes.Buffer
		command.Stdout = &logs
		command.Stderr = &logs
		if e = command.Start(); e != nil {
			w.WriteHeader(503)
			return
		}
		done := make(chan error, 1)
		go func() { done <- command.Wait() }()
		success := false
		for attempt := 0; attempt < 150; attempt++ {
			v, e := i.store.executions.Detail(clientCtx, i.alice.UserID, input.ProjectID, input.ExecutionID)
			if e == nil && (v.Status == "completed" || v.Status == "failed") {
				success = v.Status == "completed"
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		_ = command.Process.Signal(os.Interrupt)
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			_ = command.Process.Kill()
			<-done
		}
		if !success || bytes.Contains(logs.Bytes(), []byte(input.Token)) {
			w.WriteHeader(503)
			return
		}
		w.WriteHeader(204)
	})
	return target.URL
}
