package execution

import (
	"context"
	runner "github.com/VioletProto/Kurier/services/request-runtime"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func fixturePlan() Plan {
	return Plan{Configuration: Configuration{Name: "fixture", Method: "GET", URL: "http://fixture.example/", Headers: []Field{}, QueryParameters: []Field{}}, Options: Options{TimeoutSeconds: 30, ResponseRedaction: RedactionPolicy{Headers: []string{}, JSONPointers: []string{}}}, RequestID: id(), Revision: 1}
}
func stampTime() string { return "2026-10-08T00:00:00.000000000Z" }
func fixturePrepared(t *testing.T, h http.Handler, p Plan, v map[string]string) *Prepared {
	t.Helper()
	server := httptest.NewServer(h)
	t.Cleanup(server.Close)
	original, _ := url.Parse(p.Configuration.URL)
	local, _ := url.Parse(server.URL)
	original.Host = local.Host
	original.Scheme = local.Scheme
	p.Configuration.URL = original.String()
	dest, e := runner.NormalizeLocalDestination(runner.Destination{Origin: server.URL, AddressClass: "loopback"})
	if e != nil {
		t.Fatal(e)
	}
	prepared, e := runner.PrepareLocal(context.Background(), p, v, dest)
	if e != nil {
		t.Fatal(e)
	}
	return prepared
}

type auditSanitizer struct{ values map[string]string }

func newSanitizer(v map[string]string) *auditSanitizer { return &auditSanitizer{v} }
func (s *auditSanitizer) text(v string) string         { return runner.Scrub(v, s.values) }
