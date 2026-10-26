//go:build integration

package execution

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	runner "github.com/VioletProto/Kurier/services/request-runtime"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type localFixture struct {
	*protocolFixture
	token  string
	agent  Agent
	target string
}

func newLocalFixture(t *testing.T) *localFixture {
	return newLocalFixtureWithTarget(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Set-Cookie", "disposable-cookie")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "reflection": r.Header.Get("Authorization")})
	})
}

func newLocalFixtureWithTarget(t *testing.T, target http.HandlerFunc) *localFixture {
	t.Helper()
	f := newProtocolFixture(t)
	ctx := context.Background()
	stage, _ := f.s.stage(ctx)
	stage.SavedRequestsSchemaVersion = 5
	stage.EvidencePinningSchemaVersion = 1
	stage.LocalAgentExecutionsSchemaVersion = 1
	stage.Version++
	f.write(t, f.s.Table, stage)
	p, _ := f.s.project(ctx, f.owner, f.project)
	p.PinnedCount = pointer(int64(0))
	p.PinnedBytes = pointer(int64(0))
	f.write(t, f.s.Table, p)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.calls.Add(1)
		target(w, r)
	}))
	t.Cleanup(server.Close)
	req, _ := f.s.get(ctx, f.s.Table, p.PK, "REQ#"+f.request)
	plan := fixturePlan()
	plan.Configuration.URL = server.URL
	raw, _ := json.Marshal(plan.Configuration)
	req.ConfigurationJSON = string(raw)
	f.write(t, f.s.Table, req)
	pair, _, e := f.s.CreateAgent(ctx, f.owner, id(), PairInput{Name: "Fixture device", Scopes: []Scope{{ProjectID: f.project, Destinations: []Destination{{Origin: server.URL, AddressClass: "loopback"}}}}})
	if e != nil {
		t.Fatal(e)
	}
	return &localFixture{f, *pair.Token, pair.Agent, server.URL}
}

func TestIntegrationLocalSanitizedJSONExpansionPublication(t *testing.T) {
	for _, tc := range []struct {
		name       string
		characters int
		omit       bool
	}{{"expanded_body_fits", 360 * 1024, false}, {"expanded_envelope_overflow", 700 * 1024, true}} {
		t.Run(tc.name, func(t *testing.T) {
			// Literal '<' is valid JSON. Sanitization marshals it as six-byte
			// HTML escapes, then the result envelope escapes the backslashes.
			body := `{"value":"` + strings.Repeat("<", tc.characters) + `","authorization":"disposable-response-secret"}`
			if !json.Valid([]byte(body)) || len(body) > BodyLimit {
				t.Fatal("fixture must be valid JSON within the raw response budget")
			}
			f := newLocalFixtureWithTarget(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(body))
			})
			ctx := context.Background()
			v, job := f.submit(t)
			work := f.grant(t)
			prepared, err := runner.PrepareLocal(ctx, work.Plan, work.ProtectedInputs.Values, work.Destination)
			if err != nil {
				t.Fatal(err)
			}
			capture := NewEvidence(work.ProjectID, work.ExecutionID, work.DispatchIntentAt, work.Plan)
			prepared.Run(ctx, capture)
			if capture.Status != "completed" || capture.Response.Body.Text == nil || len(*capture.Response.Body.Text) <= BodyLimit || capture.Response.WireBytesRead != int64(len(body)) || capture.Response.DecodedBytesRead != int64(len(body)) {
				t.Fatal("agent did not produce an expanded capture within response-read limits")
			}
			result, err := runner.ResultFromEvidence(*work, capture)
			if err != nil {
				t.Fatal(err)
			}
			packet, err := json.Marshal(result)
			if err != nil || len(packet) > EvidenceLimit {
				t.Fatal("result envelope exceeds budget", err)
			}
			quoted, _ := json.Marshal(string(packet))
			if len(quoted)+32*1024+1 > LambdaLimit || 4*((len(packet)+2)/3)+32*1024+1 > LambdaLimit {
				t.Fatal("result exceeds Lambda ingress budget")
			}
			var uploaded runner.Result
			if err = runner.StrictJSON(packet, &uploaded); err != nil {
				t.Fatal(err)
			}
			ack, err := f.s.UploadAgentResult(ctx, f.token, f.project, job, uploaded)
			if err != nil || ack.ExecutionID != v.ExecutionID {
				t.Fatal("server validation/publication failed", err)
			}
			stored, err := f.s.Evidence(ctx, f.owner, f.project, v.ExecutionID)
			if err != nil || len(stored) > EvidenceLimit || bytes.Contains(stored, []byte("disposable-response-secret")) {
				t.Fatal("invalid immutable evidence", err)
			}
			var envelope struct {
				Data struct {
					Evidence Evidence `json:"evidence"`
				} `json:"data"`
			}
			if err = StrictJSON(stored, &envelope); err != nil {
				t.Fatal(err)
			}
			evidence := envelope.Data.Evidence
			for _, b := range []Body{uploaded.Capture.Response.Body, evidence.Response.Body} {
				if tc.omit {
					if b.Kind != "omitted" || b.Text != nil || b.OmissionReason == nil || *b.OmissionReason != "encoded_evidence_limit" {
						t.Fatal("overflow omission was not frozen/published")
					}
				} else if b.Kind != "json" || b.Text == nil || *b.Text != *capture.Response.Body.Text {
					t.Fatal("expanded sanitized JSON did not survive validation/publication")
				}
			}
			object, _ := json.Marshal(evidence)
			if len(object) > EvidenceLimit || !publicationTransportFits(stored) {
				t.Fatal("published evidence exceeds API/proxy budgets")
			}
			repeat, err := f.s.UploadAgentResult(ctx, f.token, f.project, job, uploaded)
			again, readErr := f.s.Evidence(ctx, f.owner, f.project, v.ExecutionID)
			if err != nil || readErr != nil || repeat != ack || !bytes.Equal(stored, again) || f.objects.puts != 1 || f.calls.Load() != 1 {
				t.Fatal("receipt retry changed publication or replayed HTTP", err, readErr)
			}
		})
	}
}
func (f *localFixture) submit(t *testing.T) (View, string) {
	t.Helper()
	options, _ := ParseOptions([]byte(`{}`))
	options.Target = "local"
	options.Local = &LocalOptions{LocalAgentID: f.agent.LocalAgentID, AllowPrivateNetwork: true}
	options.AllowInsecureSecrets = true
	v, e := f.s.Submit(context.Background(), f.owner, f.project, f.request, id(), 1, options, false)
	if e != nil {
		t.Fatal(e)
	}
	j, _ := f.s.get(context.Background(), f.s.Table, "P#"+f.project, "EXEC#"+v.ExecutionID)
	return v, j.JobID
}
func (f *localFixture) grant(t *testing.T) *runner.Work {
	t.Helper()
	for tries := 0; tries < 30; tries++ {
		f.now = f.now.Add(time.Second)
		r, e := f.s.PollAgent(context.Background(), f.token, f.project)
		if e != nil {
			t.Fatal(e)
		}
		if r.Work != nil {
			return r.Work
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("local discovery timed out")
	return nil
}
func resultFor(t *testing.T, w *runner.Work) runner.Result {
	t.Helper()
	p, e := runner.PrepareLocal(context.Background(), w.Plan, w.ProtectedInputs.Values, w.Destination)
	if e != nil {
		t.Fatal(e)
	}
	evidence := NewEvidence(w.ProjectID, w.ExecutionID, w.DispatchIntentAt, w.Plan)
	p.Run(context.Background(), evidence)
	r, e := runner.ResultFromEvidence(*w, evidence)
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func requireStatus(t *testing.T, e error, status int) {
	t.Helper()
	var fixed *Error
	if !errors.As(e, &fixed) || fixed.Status != status {
		t.Fatalf("expected fixed status %d, got %v", status, e)
	}
}
func TestIntegrationLocalPairingScopeAndCloudIsolation(t *testing.T) {
	f := newLocalFixture(t)
	ctx := context.Background()
	input := PairInput{Name: "other", Scopes: []Scope{{ProjectID: f.project, Destinations: []Destination{{Origin: f.target, AddressClass: "loopback"}}}}}
	foreign := id()
	f.write(t, f.s.Table, row{PK: "U#" + foreign, SK: "META", Kind: "user", SchemaVersion: 1, UserID: foreign})
	_, _, e := f.s.CreateAgent(ctx, foreign, id(), input)
	requireStatus(t, e, 404)
	_, _, e = f.s.CreateAgent(ctx, f.owner, id(), input)
	requireStatus(t, e, 409)
	_, _, e = f.s.AgentSession(ctx, "invalid")
	requireStatus(t, e, 401)
	_, e = f.s.PollAgent(ctx, f.token, id())
	requireStatus(t, e, 404)
	options, _ := ParseOptions([]byte(`{}`))
	_, e = f.s.Submit(ctx, f.owner, f.project, f.request, id(), 1, options, false)
	requireStatus(t, e, 400)
	v, job := f.submit(t)
	if f.queue.calls != 0 {
		t.Fatal("local job sent to cloud queue")
	}
	if e = f.s.Process(ctx, Message{f.project, job, "OUT#" + job}); e != nil || f.calls.Load() != 0 {
		t.Fatal("cloud consumer touched local HTTP")
	}
	view, _ := f.s.Detail(ctx, f.owner, f.project, v.ExecutionID)
	if view.Target != "local" || view.LocalAgentID == nil {
		t.Fatal("local metadata missing")
	}
}

func TestIntegrationLocalIssuanceAndAcceptedAckLoss(t *testing.T) {
	f := newLocalFixture(t)
	ctx := context.Background()
	if e := f.s.RevokeAgent(ctx, f.owner, f.agent.LocalAgentID); e != nil {
		t.Fatal(e)
	}
	key := id()
	input := PairInput{Name: "replacement", Scopes: []Scope{{ProjectID: f.project, Destinations: []Destination{{Origin: f.target, AddressClass: "loopback"}}}}}
	pair, created, e := f.s.CreateAgent(ctx, f.owner, key, input)
	if e != nil || !created || pair.Token == nil {
		t.Fatal("replacement pairing", e)
	}
	retry, created, e := f.s.CreateAgent(ctx, f.owner, key, input)
	if e != nil || created || retry.Token != nil || retry.Agent.LocalAgentID != pair.Agent.LocalAgentID {
		t.Fatal("issuance retry revealed token or made another credential", e)
	}
	input.Name = "different"
	_, _, e = f.s.CreateAgent(ctx, f.owner, key, input)
	requireStatus(t, e, 409)
	f.token = *pair.Token
	f.agent = pair.Agent
	v, job := f.submit(t)
	w := f.grant(t)
	result := resultFor(t, w)
	f.db.after = func(in *dynamodb.TransactWriteItemsInput) error {
		for _, action := range in.TransactItems {
			if action.Put != nil {
				var r row
				_ = attributevalue.UnmarshalMap(action.Put.Item, &r)
				if r.Kind == "executionLocalReceipt" {
					return errors.New("simulated lost finalization ACK")
				}
			}
		}
		return nil
	}
	ack, e := f.s.UploadAgentResult(ctx, f.token, f.project, job, result)
	if e != nil || ack.ExecutionID != v.ExecutionID {
		t.Fatal("lost accepted ACK did not reconcile", e)
	}
	f.db.after = nil
	f.now = f.now.Add(121 * time.Second)
	again, e := f.s.UploadAgentResult(ctx, f.token, f.project, job, result)
	if e != nil || again != ack || f.calls.Load() != 1 || f.objects.puts != 1 {
		t.Fatal("expired lease ACK rewrote evidence or replayed HTTP", e)
	}
}

func TestIntegrationLocalPendingCandidateResumeAndConflicts(t *testing.T) {
	f := newLocalFixture(t)
	ctx := context.Background()
	_, job := f.submit(t)
	w := f.grant(t)
	result := resultFor(t, w)
	failed := false
	f.db.before = func(in *dynamodb.TransactWriteItemsInput) error {
		for _, action := range in.TransactItems {
			if action.Put != nil {
				var r row
				_ = attributevalue.UnmarshalMap(action.Put.Item, &r)
				if r.Kind == "executionUpload" && r.State == "writing" && !failed {
					failed = true
					return errors.New("simulated crash after pending candidate")
				}
			}
		}
		return nil
	}
	_, e := f.s.UploadAgentResult(ctx, f.token, f.project, job, result)
	requireStatus(t, e, 503)
	f.db.before = nil
	conflict := result
	conflict.Capture.Response.WireBytesRead++
	_, e = f.s.UploadAgentResult(ctx, f.token, f.project, job, conflict)
	requireStatus(t, e, 409)
	f.now = f.now.Add(time.Second)
	_, e = f.s.UploadAgentResult(ctx, f.token, f.project, job, result)
	if e != nil || f.objects.puts != 1 || f.calls.Load() != 1 {
		t.Fatal("identical pending resume failed or replayed", e)
	}
}

func TestIntegrationLocalRecoveryResumesOnlySafeRecoveryCapture(t *testing.T) {
	f := newLocalFixture(t)
	ctx := context.Background()
	v, job := f.submit(t)
	_ = f.grant(t)
	f.now = f.now.Add(121 * time.Second)
	failed := false
	f.db.before = func(in *dynamodb.TransactWriteItemsInput) error {
		for _, a := range in.TransactItems {
			if a.Put != nil {
				var r row
				_ = attributevalue.UnmarshalMap(a.Put.Item, &r)
				if r.Kind == "executionSnapshot" && !failed {
					failed = true
					return errors.New("simulated recovery crash after acknowledged PUT")
				}
			}
		}
		return nil
	}
	if e := f.s.reconcileLocal(ctx, f.project, job); e == nil {
		t.Fatal("injected recovery crash missed")
	}
	if f.objects.puts != 1 {
		t.Fatal("missing safe recovery object")
	}
	f.db.before = nil
	f.now = f.now.Add(121 * time.Second)
	if e := f.s.reconcileLocal(ctx, f.project, job); e != nil {
		t.Fatal(e)
	}
	view, e := f.s.Detail(ctx, f.owner, f.project, v.ExecutionID)
	if e != nil || view.Summary.Outcome.Certainty != "unknown" || f.calls.Load() != 0 || f.objects.puts != 1 {
		t.Fatal("safe recovery did not reuse immutable capture", e)
	}
	_, e = f.s.get(ctx, f.s.Table, "P#"+f.project, "JOB#"+job+"#RECEIPT")
	requireStatus(t, e, 404)
}
func TestIntegrationLocalReceiptSecretContainmentAndPinLifetime(t *testing.T) {
	f := newLocalFixture(t)
	ctx := context.Background()
	secret := id()
	value := "disposable-protected-local-credential"
	req, _ := f.s.get(ctx, f.s.Table, "P#"+f.project, "REQ#"+f.request)
	var config Configuration
	_ = StrictJSON([]byte(req.ConfigurationJSON), &config)
	config.Headers = []Field{{Name: "Authorization", Sensitive: true, Enabled: true, SecretRef: &Ref{SecretID: secret}}}
	raw, _ := json.Marshal(config)
	req.ConfigurationJSON = string(raw)
	f.write(t, f.s.Table, req)
	f.write(t, f.s.Protected, row{PK: req.PK, SK: "SECRET#" + secret, Kind: "savedSecret", SchemaVersion: 1, ProjectID: f.project, SecretID: secret, Revision: 1, State: "active", ValueKind: "string", Envelope: fixtureSecretEnvelope(t, f.s, f.project, secret, value, 1)})
	v, job := f.submit(t)
	w := f.grant(t)
	if w.ProtectedInputs.Values[secret] != value {
		t.Fatal("authorized frozen secret missing")
	}
	result := resultFor(t, w)
	ack, e := f.s.UploadAgentResult(ctx, f.token, f.project, job, result)
	if e != nil || ack.ExecutionID != v.ExecutionID {
		t.Fatal("publication failed", e)
	}
	stored, e := f.s.Evidence(ctx, f.owner, f.project, v.ExecutionID)
	if e != nil || strings.Contains(string(stored), value) || strings.Contains(string(stored), "disposable-cookie") {
		t.Fatal("secret leaked into evidence")
	}
	if f.calls.Load() != 1 || f.objects.puts != 1 {
		t.Fatal("unexpected HTTP or object duplicate")
	}
	if _, e = f.s.Pin(ctx, f.owner, f.project, v.ExecutionID, true); e != nil {
		t.Fatal(e)
	}
	f.now = f.now.Add(8 * 24 * time.Hour)
	repeat, e := f.s.UploadAgentResult(ctx, f.token, f.project, job, result)
	if e != nil || repeat != ack || f.objects.puts != 1 || f.calls.Load() != 1 {
		t.Fatal("retained receipt ACK failed", e)
	}
	changed := result
	changed.Capture.Response.WireBytesRead++
	_, e = f.s.UploadAgentResult(ctx, f.token, f.project, job, changed)
	requireStatus(t, e, 409)
	a, _ := f.s.get(ctx, f.s.Table, "U#"+f.owner, "CRED#"+f.agent.LocalAgentID)
	if a.ActiveJob != "" {
		t.Fatal("terminal slot not cleared")
	}
	if e = f.s.RevokeAgent(ctx, f.owner, f.agent.LocalAgentID); e != nil {
		t.Fatal(e)
	}
	_, e = f.s.UploadAgentResult(ctx, f.token, f.project, job, result)
	requireStatus(t, e, 401)
	// Complete sanitized persistence inspection returns only pass/fail, no values.
	for _, table := range []string{f.s.Table, f.s.Protected} {
		page, e := f.db.Client.Scan(ctx, &dynamodb.ScanInput{TableName: &table})
		if e != nil {
			t.Fatal(e)
		}
		encoded, _ := json.Marshal(page.Items)
		if bytes.Contains(encoded, []byte(value)) || bytes.Contains(encoded, []byte(f.token)) {
			t.Fatal("plaintext persisted")
		}
	}
}
func TestIntegrationLocalLostPollAndCrashesNeverRegrant(t *testing.T) {
	for _, mode := range []string{"lost-response", "lost-commit-ack", "crash-before-http", "crash-after-http"} {
		t.Run(mode, func(t *testing.T) {
			f := newLocalFixture(t)
			ctx := context.Background()
			v, job := f.submit(t)
			if mode == "lost-commit-ack" {
				f.db.after = func(in *dynamodb.TransactWriteItemsInput) error {
					for _, a := range in.TransactItems {
						if a.Put != nil {
							var r row
							_ = attributevalue.UnmarshalMap(a.Put.Item, &r)
							if r.Kind == "executionJob" && r.Target == "local" && r.State == "running" {
								return errors.New("simulated lost grant commit ACK")
							}
						}
					}
					return nil
				}
				f.now = f.now.Add(time.Second)
				_, e := f.s.PollAgent(ctx, f.token, f.project)
				requireStatus(t, e, 503)
				f.db.after = nil
			} else {
				w := f.grant(t)
				if mode == "crash-after-http" {
					_ = resultFor(t, w)
				}
			}
			f.now = f.now.Add(time.Second)
			poll, e := f.s.PollAgent(ctx, f.token, f.project)
			if e != nil || poll.Work != nil || !poll.Busy {
				t.Fatal("ambiguous grant replayed", e)
			}
			f.now = f.now.Add(121 * time.Second)
			if e = f.s.reconcileLocal(ctx, f.project, job); e != nil {
				t.Fatal(e)
			}
			poll, e = f.s.PollAgent(ctx, f.token, f.project)
			if e != nil || poll.Work != nil {
				t.Fatal("expired grant replayed", e)
			}
			view, e := f.s.Detail(ctx, f.owner, f.project, v.ExecutionID)
			if e != nil || view.Summary.Outcome.Certainty != "unknown" {
				t.Fatal("unknown outcome lost", e)
			}
			expected := int32(0)
			if mode == "crash-after-http" {
				expected = 1
			}
			if f.calls.Load() != expected {
				t.Fatal("automatic HTTP replay")
			}
		})
	}
}
func TestIntegrationLocalConcurrentGrantAndRevocationPublication(t *testing.T) {
	f := newLocalFixture(t)
	ctx := context.Background()
	_, job := f.submit(t)
	time.Sleep(50 * time.Millisecond)
	var group sync.WaitGroup
	responses := make(chan runner.PollResponse, 2)
	for range 2 {
		group.Add(1)
		go func() {
			defer group.Done()
			r, e := f.s.PollAgent(ctx, f.token, f.project)
			if e == nil {
				responses <- r
			}
		}()
	}
	group.Wait()
	close(responses)
	var w *runner.Work
	for r := range responses {
		if r.Work != nil {
			if w != nil {
				t.Fatal("two grants")
			}
			w = r.Work
		}
	}
	if w == nil {
		w = f.grant(t)
	}
	result := resultFor(t, w)
	f.objects.putFault = func() {
		if e := f.s.RevokeAgent(ctx, f.owner, f.agent.LocalAgentID); e != nil {
			t.Fatal(e)
		}
	}
	_, e := f.s.UploadAgentResult(ctx, f.token, f.project, job, result)
	if e == nil {
		t.Fatal("revoked publication succeeded")
	}
	receipt, e := f.s.get(ctx, f.s.Table, "P#"+f.project, "JOB#"+job+"#RECEIPT")
	if e == nil || receipt.Kind != "" {
		t.Fatal("receipt created after revocation")
	}
	if e = f.s.reconcileLocal(ctx, f.project, job); e != nil {
		t.Fatal(e)
	}
	if f.calls.Load() != 1 {
		t.Fatal("revocation replayed target")
	}
}
func TestIntegrationLocalPendingUploadExpiryOfflineAndDeletion(t *testing.T) {
	for _, mode := range []string{"offline", "pending-put", "delete-during-put", "generation"} {
		t.Run(mode, func(t *testing.T) {
			f := newLocalFixture(t)
			ctx := context.Background()
			v, job := f.submit(t)
			switch mode {
			case "offline":
				f.now = f.now.Add(11 * time.Minute)
				if e := f.s.reconcileLocal(ctx, f.project, job); e != nil {
					t.Fatal(e)
				}
				view, e := f.s.Detail(ctx, f.owner, f.project, v.ExecutionID)
				if e != nil || view.Summary.Outcome.Certainty != "not_dispatched" {
					t.Fatal("offline expiry", e)
				}
			case "generation":
				w := f.grant(t)
				result := resultFor(t, w)
				stage, _ := f.s.stage(ctx)
				stage.RecoveryGeneration = "fixture-new-generation"
				stage.Version++
				f.write(t, f.s.Table, stage)
				_, e := f.s.UploadAgentResult(ctx, f.token, f.project, job, result)
				requireStatus(t, e, 401)
				if e = f.s.reconcileLocal(ctx, f.project, job); e != nil {
					t.Fatal(e)
				}
			default:
				w := f.grant(t)
				result := resultFor(t, w)
				if mode == "pending-put" {
					f.objects.lostACK = true
				} else {
					f.objects.putFault = func() {
						p, _ := f.s.project(ctx, f.owner, f.project)
						p.State = "deleting"
						p.DeletionEpoch++
						p.Version++
						f.write(t, f.s.Table, p)
					}
				}
				_, e := f.s.UploadAgentResult(ctx, f.token, f.project, job, result)
				if e == nil {
					t.Fatal("raced upload accepted")
				}
				_, e = f.s.get(ctx, f.s.Table, "P#"+f.project, "JOB#"+job+"#RECEIPT")
				requireStatus(t, e, 404)
				if mode == "pending-put" {
					f.objects.lostACK = false
					f.now = f.now.Add(121 * time.Second)
					if e = f.s.reconcileLocal(ctx, f.project, job); e != nil {
						t.Fatal(e)
					}
					_, e = f.s.UploadAgentResult(ctx, f.token, f.project, job, result)
					requireStatus(t, e, 409)
					p, _ := f.s.project(ctx, f.owner, f.project)
					p.State = "deleting"
					p.DeletionEpoch++
					p.Version++
					f.write(t, f.s.Table, p)
					if e = f.s.DrainProject(ctx, f.project); e == nil {
						t.Fatal("uncertain late PUT obligation erased")
					}
				}
			}
			if mode == "offline" && f.calls.Load() != 0 || mode != "offline" && f.calls.Load() != 1 {
				t.Fatal("HTTP count violated")
			}
		})
	}
}
