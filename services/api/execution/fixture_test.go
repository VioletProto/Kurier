package execution

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	runner "github.com/VioletProto/Kurier/services/request-runtime"
)

func fixturePlan() Plan {
	return Plan{Configuration: Configuration{Name: "fixture", Method: "GET", URL: "http://fixture.example/", Headers: []Field{}, QueryParameters: []Field{}}, Options: Options{TimeoutSeconds: 30, ResponseRedaction: RedactionPolicy{Headers: []string{}, JSONPointers: []string{}}}, RequestID: id(), Revision: 1}
}
func stampTime() string { return "2026-10-08T00:00:00.000000000Z" }
func fixturePrepared(t *testing.T, h http.Handler, p Plan, v map[string]string) *Prepared {
	t.Helper()
	server := httptest.NewServer(h)
	t.Cleanup(server.Close)
	prepared, e := prepareAtFixtureURL(context.Background(), server.URL, p, v)
	if e != nil {
		t.Fatal(e)
	}
	return prepared
}

func prepareAtFixtureURL(ctx context.Context, address string, p Plan, v map[string]string) (*Prepared, error) {
	dest, e := runner.NormalizeLocalDestination(runner.Destination{Origin: address, AddressClass: "loopback"})
	if e != nil {
		return nil, e
	}
	original, e := url.Parse(p.Configuration.URL)
	if e != nil {
		return nil, e
	}
	local, e := url.Parse(address)
	if e != nil {
		return nil, e
	}
	original.Host, original.Scheme = local.Host, local.Scheme
	p.Configuration.URL = original.String()
	return runner.PrepareLocal(ctx, p, v, dest)
}

type auditSanitizer struct{ values map[string]string }

func newSanitizer(v map[string]string) *auditSanitizer { return &auditSanitizer{v} }
func (s *auditSanitizer) text(v string) string         { return runner.Scrub(v, s.values) }
