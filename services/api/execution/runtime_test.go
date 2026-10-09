package execution

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type fixtureResolver []netip.Addr

func (r fixtureResolver) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	return append([]netip.Addr{}, r...), nil
}

func TestDestinationProtection(t *testing.T) {
	for _, address := range []string{"127.0.0.1", "10.0.0.1", "169.254.169.254", "100.64.0.1", "192.0.2.1", "224.0.0.1", "::1", "::ffff:127.0.0.1", "64:ff9b::808:808", "2002:808:808::1", "2001:db8::1", "3fff::1"} {
		if PublicIP(netip.MustParseAddr(address)) {
			t.Errorf("special-use address allowed: %s", address)
		}
	}
	for _, address := range []string{"8.8.8.8", "::ffff:8.8.8.8", "2606:4700:4700::1111"} {
		if !PublicIP(netip.MustParseAddr(address)) {
			t.Errorf("public address blocked: %s", address)
		}
	}
	for _, raw := range []string{"http://localhost/", "http://127.1/", "http://2130706433/", "http://0x7f000001/", "http://127.0.0.1/", "http://example.com:8080/", "https://name:pass@example.com/", "file:///tmp/a", "https://example.com/#x"} {
		if _, err := ValidateDestination(raw); err == nil {
			t.Errorf("destination allowed: %s", raw)
		}
	}
	if _, err := validatedIPs(context.Background(), fixtureResolver{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("10.0.0.1")}, "fixture.example"); err == nil {
		t.Fatal("mixed DNS answer allowed")
	}
	// Rebinding is checked again at the dial, even after a safe preparation.
	if _, err := safeDialer(fixtureResolver{netip.MustParseAddr("127.0.0.1")})(context.Background(), "tcp", "fixture.example:80"); err == nil {
		t.Fatal("unsafe dial allowed")
	}
}

func TestPolicyRejectsInvalidUnionAndNull(t *testing.T) {
	for _, raw := range []string{`null`, `[]`, `{"timeoutSeconds":null}`, `{"allowInsecureSecrets":null}`, `{"responseRedaction":null}`, `{"timeoutSeconds":1.5}`, `{"extra":true}`, `{"timeoutSeconds":1,"timeoutSeconds":2}`, `{"responseRedaction":{"jsonPointers":["/a","/a/b"]}}`, `{"responseRedaction":{"headers":null}}`} {
		if _, err := ParseOptions([]byte(raw)); err == nil {
			t.Errorf("invalid policy accepted: %s", raw)
		}
	}
}

func TestReflectedRedactionAndSafeContent(t *testing.T) {
	secret := "disposable-fixture-credential-9+/"
	san := newSanitizer(map[string]string{"one": "Bearer " + secret, "two": `{"leaf":"nested-fixture-secret","scalar":987654321}`})
	for _, variant := range []string{secret, base64.StdEncoding.EncodeToString([]byte(secret)), base64.RawURLEncoding.EncodeToString([]byte(secret)), "nested-fixture-secret", "987654321"} {
		if strings.Contains(san.text("prefix "+variant+" suffix"), variant) {
			t.Fatal("protected reflection survived")
		}
	}
	body := san.body([]byte(`{"authorization":{"nested":"credential"},"x":987654321,"custom":"sensitive"}`), "application/json", RedactionPolicy{JSONPointers: []string{"/custom"}})
	if body.Text == nil || strings.Contains(*body.Text, "credential") || strings.Contains(*body.Text, "sensitive") || strings.Contains(*body.Text, "987654321") {
		t.Fatal("JSON redaction failed")
	}
	for _, pair := range [][2]string{{"<script>alert(1)</script>", "text/html"}, {"x", "application/octet-stream"}, {"x", "text/plain; charset=latin1"}, {"\xff", "text/plain"}, {`{"a":1,"a":2}`, "application/json"}} {
		if got := san.body([]byte(pair[0]), pair[1], RedactionPolicy{}); got.Kind != "omitted" || got.Text != nil {
			t.Fatal("unsafe content returned")
		}
	}
	if got := san.body([]byte(`{"ok":true}`), "application/json", RedactionPolicy{JSONPointers: []string{"/missing"}}); got.Kind != "omitted" {
		t.Fatal("unapplied redaction policy returned")
	}
}

func fixturePrepared(t *testing.T, handler http.Handler, plan Plan, values map[string]string) *Prepared {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	prepared, err := prepare(context.Background(), plan, values, fixtureResolver{netip.MustParseAddr("8.8.8.8")})
	if err != nil {
		t.Fatal(err)
	}
	// Only this test helper connects the validated public fixture name to loopback.
	prepared.transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}
	return prepared
}
func fixturePlan() Plan {
	return Plan{Configuration: Configuration{Name: "fixture", Method: "GET", URL: "http://fixture.example/", Headers: []Field{}, QueryParameters: []Field{}}, Options: Options{TimeoutSeconds: 30, ResponseRedaction: RedactionPolicy{Headers: []string{}, JSONPointers: []string{}}}, RequestID: id(), Revision: 1}
}

func TestRuntimeLimitsRedirectAndHTTPFailure(t *testing.T) {
	for _, mode := range []string{"wire", "gzip", "redirect", "http-error", "html"} {
		t.Run(mode, func(t *testing.T) {
			var calls atomic.Int32
			plan := fixturePlan()
			prepared := fixturePrepared(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				switch mode {
				case "wire":
					w.Header().Set("Content-Type", "text/plain")
					_, _ = w.Write(bytes.Repeat([]byte("x"), BodyLimit+1))
				case "gzip":
					w.Header().Set("Content-Type", "text/plain")
					w.Header().Set("Content-Encoding", "gzip")
					writer := gzip.NewWriter(w)
					_, _ = writer.Write(bytes.Repeat([]byte("x"), BodyLimit+1))
					_ = writer.Close()
				case "redirect":
					w.Header().Set("Location", "http://127.0.0.1/metadata")
					w.WriteHeader(302)
				case "http-error":
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(500)
					_, _ = w.Write([]byte(`{"ok":false}`))
				case "html":
					w.Header().Set("Content-Type", "text/html")
					_, _ = w.Write([]byte("<script>alert(1)</script>"))
				}
			}), plan, nil)
			e := NewEvidence(id(), id(), stampTime(), plan)
			prepared.Run(context.Background(), e)
			if calls.Load() != 1 {
				t.Fatal("outbound request replayed or redirected")
			}
			if mode == "wire" || mode == "gzip" {
				if e.Outcome.Code == nil || *e.Outcome.Code != "response_limit_exceeded" || e.Response.Body.Text != nil {
					t.Fatal("body limit not enforced")
				}
			}
			if mode == "redirect" && (*e.Response.HTTPStatus != 302 || e.Status != "completed") {
				t.Fatal("redirect capture lost")
			}
			if mode == "http-error" && (e.Status != "failed" || e.Outcome.Certainty != "known") {
				t.Fatal("upstream error outcome incorrect")
			}
			if mode == "html" && e.Response.Body.Kind != "omitted" {
				t.Fatal("HTML persisted")
			}
		})
	}
}
func stampTime() string { return "2026-10-08T00:00:00.000000000Z" }

func TestUnsupportedEncodingStillBoundsWireAndPreservesUnknownReads(t *testing.T) {
	for _, mode := range []string{"complete", "oversize", "interrupted"} {
		t.Run(mode, func(t *testing.T) {
			prepared := fixturePrepared(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/plain")
				w.Header().Set("Content-Encoding", "br")
				if mode == "oversize" {
					_, _ = w.Write(bytes.Repeat([]byte("x"), BodyLimit+1))
					return
				}
				_, _ = w.Write([]byte("opaque unsupported content"))
				if mode == "interrupted" {
					w.(http.Flusher).Flush()
					<-r.Context().Done()
				}
			}), fixturePlan(), nil)
			if mode == "interrupted" {
				prepared.timeout = 100 * time.Millisecond
			}
			e := NewEvidence(id(), id(), stampTime(), fixturePlan())
			prepared.Run(context.Background(), e)
			if e.Response.Body.Kind != "omitted" || e.Response.Body.Text != nil {
				t.Fatal("unsupported encoding exposed content")
			}
			switch mode {
			case "complete":
				if e.Status != "completed" || e.Outcome.Certainty != "known" {
					t.Fatal("complete unsupported response lost its known outcome")
				}
			case "oversize":
				if e.Outcome.Code == nil || *e.Outcome.Code != "response_limit_exceeded" {
					t.Fatal("unsupported encoding bypassed wire limit")
				}
			case "interrupted":
				if e.Outcome.Certainty != "unknown" || e.Outcome.Code == nil || *e.Outcome.Code != "execution_timeout" {
					t.Fatal("incomplete unsupported response fabricated a known outcome")
				}
			}
		})
	}
}

func TestEvidenceWorstCaseEscapingAndEnvelopeBoundary(t *testing.T) {
	for _, text := range []string{strings.Repeat("\"", BodyLimit), strings.Repeat("\\", BodyLimit), strings.Repeat("\x00", BodyLimit), strings.Repeat("<", BodyLimit), strings.Repeat("é", BodyLimit/2), strings.Repeat(`\u0000`, BodyLimit/6)} {
		plan := fixturePlan()
		e := NewEvidence(id(), id(), stampTime(), plan)
		e.Fail("execution_outcome_unknown", "unknown")
		e.Response.Body = Body{Kind: "text", Text: &text}
		raw, api, err := EncodeEvidence(e)
		if err != nil {
			t.Fatal(err)
		}
		if len(raw) > EvidenceLimit || len(api) > EvidenceLimit || base64.StdEncoding.EncodedLen(len(api))+MetadataLimit > LambdaLimit {
			t.Fatal("encoded publication bounds violated")
		}
		var envelope struct {
			Data struct {
				Evidence Evidence `json:"evidence"`
			} `json:"data"`
		}
		decoded, _ := base64.StdEncoding.DecodeString(base64.StdEncoding.EncodeToString(api))
		if json.Unmarshal(decoded, &envelope) != nil {
			t.Fatal("delivery roundtrip failed")
		}
		published, _ := json.Marshal(envelope.Data.Evidence)
		if !bytes.Equal(raw, published) {
			t.Fatal("delivery changed immutable capture")
		}
	}
	// Construct a capture that fits S3 but its wrapper crosses the browser cap.
	e := NewEvidence(id(), id(), stampTime(), fixturePlan())
	e.Fail("execution_outcome_unknown", "unknown")
	text := ""
	e.Response.Body = Body{Kind: "text", Text: &text}
	minimal, _ := json.Marshal(e)
	text = strings.Repeat("x", EvidenceLimit-len(minimal))
	candidate, _ := json.Marshal(e)
	if len(candidate) != EvidenceLimit {
		t.Fatal("boundary fixture incorrect")
	}
	_, _, err := EncodeEvidence(e)
	if err != nil || e.Response.Body.Kind != "omitted" || *e.Response.Body.OmissionReason != "encoded_evidence_limit" {
		t.Fatal("API-envelope overflow was not omitted before publication")
	}
}
