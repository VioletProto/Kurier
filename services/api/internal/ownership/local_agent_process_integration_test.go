//go:build integration && linux

package ownership

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/VioletProto/Kurier/services/api/execution"
	runner "github.com/VioletProto/Kurier/services/request-runtime"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

func TestIntegrationLocalAgentProcessKilledAfterDispatch(t *testing.T) {
	directory := t.TempDir()
	binary := filepath.Join(directory, "kurier-agent")
	build := exec.CommandContext(t.Context(), "go", "build", "-o", binary, ".")
	build.Dir = "../../../local-agent"
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("foreground agent build failed: %s", output)
	}

	i, project, _, _ := pinLocal(t)
	t.Cleanup(func() {
		if _, err := i.client.DeleteTable(context.Background(), &dynamodb.DeleteTableInput{TableName: &i.store.table}); err != nil {
			t.Error("control fixture table cleanup failed")
		}
	})
	service := i.store.executions
	stage, err := i.store.get(t.Context(), "STAGE#local", "META")
	if err != nil {
		t.Fatal("stage read failed", err)
	}
	stage.SavedRequestsSchemaVersion = 5
	stage.LocalAgentExecutionsSchemaVersion = 1
	stage.Version++
	i.write(t, stage)
	i.store.cipher = &EnvelopeCipher{workflowFixtureKMS{}, i.store.cipher.keyARN, "local"}
	service.KMS = workflowFixtureKMS{}
	service.KeyARN = i.store.cipher.keyARN
	var clockOffset atomic.Int64
	service.Now = func() time.Time { return time.Now().Add(time.Duration(clockOffset.Load())) }

	secret := "fixture-" + newID()
	targetSeen := make(chan bool, 1)
	var targetSends atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		if targetSends.Add(1) == 1 {
			targetSeen <- r.Header.Get("Authorization") == "Bearer "+secret
		}
		// Hold the response until the client connection closes.
		<-r.Context().Done()
	}))
	t.Cleanup(target.Close)
	config := protectedConfig("Bearer " + secret)
	config.URL = target.URL
	owner := i.alice.UserID
	saved, err := i.store.CreateRequest(t.Context(), owner, project.ProjectID, config)
	if err != nil {
		t.Fatal("protected request creation failed", err)
	}
	paired, _, err := service.CreateAgent(t.Context(), owner, newID(), execution.PairInput{
		Name:         "Process recovery fixture",
		ProjectScope: runner.AllOwnedProjects,
		Destinations: []execution.Destination{{Origin: target.URL, AddressClass: "loopback"}},
	})
	if err != nil || paired.Token == nil {
		t.Fatal("account pairing failed", err)
	}
	options, err := execution.ParseOptions([]byte(`{"timeoutSeconds":30}`))
	if err != nil {
		t.Fatal("fixture options failed", err)
	}
	options.Target = "local"
	options.Local = &execution.LocalOptions{LocalAgentID: paired.Agent.LocalAgentID, AllowPrivateNetwork: true}
	options.AllowInsecureSecrets = true
	run, err := service.Submit(t.Context(), owner, project.ProjectID, saved.RequestID, newID(), saved.Revision, options, false)
	if err != nil {
		t.Fatal("local execution submission failed", err)
	}

	type poll struct {
		status    int
		busy      bool
		granted   bool
		malformed bool
	}
	polls := make(chan poll, 8)
	var resultUploads atomic.Int32
	api := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/result") {
			resultUploads.Add(1)
		}
		if r.URL.Path != "/api/v1/local-agent/poll" {
			i.server.ServeHTTP(w, r)
			return
		}
		response := httptest.NewRecorder()
		defer clear(response.Body.Bytes())
		i.server.ServeHTTP(response, r)
		var envelope struct {
			Data struct {
				Busy bool            `json:"busy"`
				Work json.RawMessage `json:"work"`
			} `json:"data"`
		}
		err := json.Unmarshal(response.Body.Bytes(), &envelope)
		defer clear(envelope.Data.Work)
		for key, values := range response.Header() {
			w.Header()[key] = values
		}
		w.WriteHeader(response.Code)
		_, _ = w.Write(response.Body.Bytes())
		select {
		case polls <- poll{response.Code, envelope.Data.Busy, len(envelope.Data.Work) > 0 && !bytes.Equal(envelope.Data.Work, []byte("null")), err != nil}:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(api.Close)
	certFile := filepath.Join(directory, "fixture-ca.pem")
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: api.Certificate().Raw})
	if err := os.WriteFile(certFile, cert, 0600); err != nil {
		t.Fatal("fixture CA write failed", err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	t.Cleanup(cancel)
	start := func() func() {
		t.Helper()
		command := exec.CommandContext(ctx, binary, "run", "--api", api.URL, "--stdin")
		command.Env = append(os.Environ(), "SSL_CERT_FILE="+certFile, "XDG_CONFIG_HOME="+directory)
		command.Stdin = strings.NewReader(*paired.Token + "\nyes\n")
		var diagnostics bytes.Buffer
		command.Stdout, command.Stderr = &diagnostics, &diagnostics
		if err := command.Start(); err != nil {
			t.Fatal("agent startup failed", err)
		}
		stopped := false
		stop := func() {
			t.Helper()
			if stopped {
				return
			}
			stopped = true
			killErr := command.Process.Kill()
			_ = command.Wait()
			status, ok := command.ProcessState.Sys().(syscall.WaitStatus)
			if killErr != nil || !ok || status.Signal() != syscall.SIGKILL {
				t.Error("agent did not terminate via SIGKILL")
			}
			logs := diagnostics.Bytes()
			if !bytes.Contains(logs, []byte("Agent scope and expiry:")) || bytes.Contains(logs, []byte(secret)) || bytes.Contains(logs, []byte(*paired.Token)) {
				t.Error("agent diagnostics missing or contained a fixture secret")
			}
			clear(logs)
		}
		t.Cleanup(stop)
		return stop
	}
	expectPoll := func(busy, granted bool) {
		t.Helper()
		select {
		case got := <-polls:
			if got != (poll{status: 200, busy: busy, granted: granted}) {
				t.Fatalf("unexpected poll outcome: %+v", got)
			}
		case <-ctx.Done():
			t.Fatal("agent poll timed out")
		}
	}

	stop := start()
	expectPoll(false, true)
	select {
	case authorized := <-targetSeen:
		if !authorized {
			t.Fatal("controlled target did not receive the approved fixture credential")
		}
	case <-ctx.Done():
		t.Fatal("controlled target received no request")
	}
	stop()
	stopRestarted := start()
	expectPoll(true, false)
	clockOffset.Store(int64(121 * time.Second))
	expectPoll(false, false)
	stopRestarted()

	settled, err := service.Detail(t.Context(), owner, project.ProjectID, run.ExecutionID)
	if err != nil || settled.Status != "failed" || settled.Summary == nil || settled.Summary.Outcome.Certainty != "unknown" ||
		settled.Summary.Outcome.Code == nil || *settled.Summary.Outcome.Code != "execution_outcome_unknown" {
		t.Fatal("interrupted execution did not settle as outcome unknown", err)
	}
	evidence, err := service.Evidence(t.Context(), owner, project.ProjectID, run.ExecutionID)
	if err != nil || bytes.Contains(evidence, []byte(secret)) || bytes.Contains(evidence, []byte(*paired.Token)) {
		t.Fatal("failure evidence absent or contained a fixture secret", err)
	}
	if targetSends.Load() != 1 || resultUploads.Load() != 0 {
		t.Fatalf("expected one target request and no agent result uploads, got %d and %d", targetSends.Load(), resultUploads.Load())
	}
	if err := service.RevokeAgent(t.Context(), owner, paired.Agent.LocalAgentID); err != nil {
		t.Fatal("fixture agent revocation failed", err)
	}
}
