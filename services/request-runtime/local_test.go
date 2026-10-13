package execution

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/cyberphone/json-canonicalization/go/src/webpki.org/jsoncanonicalizer"
	"net/netip"
	"strings"
	"testing"
)

func TestLocalDestinationConsentAndRebinding(t *testing.T) {
	for _, entry := range []Destination{{"http://localhost:3000", "loopback"}, {"http://127.0.0.1:1", "loopback"}, {"https://[fd00::1]:65535", "private"}, {"http://192.168.1.5:8080", "private"}} {
		d, e := NormalizeLocalDestination(entry)
		if e != nil {
			t.Fatal("valid local destination denied")
		}
		if _, e = ValidateLocalURL(entry.Origin+"/path", d); e != nil {
			t.Fatal(e)
		}
		if _, e = ValidateLocalURL(entry.Origin+":99", d); e == nil {
			t.Fatal("wrong origin allowed")
		}
	}
	for _, origin := range []string{"http://169.254.169.254:80", "http://100.64.0.1:80", "http://8.8.8.8:80", "http://[64:ff9b::a00:1]:80", "http://[fe80::1%25eth0]:80", "http://0x7f000001:80", "http://127.1:80", "http://localhost.:80", "http://u:p@localhost:80", "http://localhost:03000", "http://*:3000", "http://localhost:3000/", "file:///tmp/a"} {
		if _, e := NormalizeLocalDestination(Destination{origin, "loopback"}); e == nil {
			t.Fatal("unsafe local origin allowed")
		}
	}
	for _, addr := range []string{"0.0.0.0", "169.254.1.1", "224.0.0.1", "100.64.0.1", "::", "fe80::1", "2002::1", "64:ff9b::1", "fd00:ec2::254", "fd20:ce::254", "::ffff:10.0.0.1%lo", "::ffff:127.0.0.1%lo"} {
		if LocalIP(netip.MustParseAddr(addr), "private") || LocalIP(netip.MustParseAddr(addr), "loopback") {
			t.Fatal("special-use local bypass")
		}
	}
	if !LocalIP(netip.MustParseAddr("::ffff:127.0.0.1"), "loopback") {
		t.Fatal("mapped addresses must be classified")
	}
	mixed := fixtureResolver{netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("8.8.8.8")}
	if _, e := localIPs(context.Background(), mixed, "fixture.local", "private"); e == nil {
		t.Fatal("mixed answer allowed")
	}
	d := Destination{"http://fixture.local:3000", "private"}
	if _, e := localDialer(fixtureResolver{netip.MustParseAddr("169.254.169.254")}, d)(context.Background(), "tcp", "fixture.local:3000"); e == nil {
		t.Fatal("rebound metadata dial allowed")
	}
}
func TestCanonicalReceiptVectorsAndBounds(t *testing.T) {
	raw := []byte(`{"literals":[null,true,false],"numbers":[333333333.33333329,1E30,4.50,2e-3,0.000000000000000000000000001]}`)
	got, e := jsoncanonicalizer.Transform(raw)
	want := `{"literals":[null,true,false],"numbers":[333333333.3333333,1e+30,4.5,0.002,1e-27]}`
	if e != nil || string(got) != want {
		t.Fatal("RFC 8785 number vector mismatch")
	}
	got, e = jsoncanonicalizer.Transform([]byte(`{"\u20ac":1,"\r":2,"\ufb33":3,"1":4,"\ud83d\ude00":5,"\u0080":6,"\u00f6":7}`))
	if e != nil || string(got) != `{"\r":2,"1":4,"`+"\u0080"+`":6,"ö":7,"€":1,"😀":5,"דּ":3}` {
		t.Fatal("RFC UTF-16 ordering mismatch")
	}
	r := Result{SchemaVersion: 1, LeaseID: NewID(), Fence: 1, Nonce: base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))}
	a, e := CanonicalResult(r)
	if e != nil {
		t.Fatal(e)
	}
	r.Capture.Request.Headers = []Header{}
	r.Capture.Response.Headers = []Header{}
	b, _ := CanonicalResult(r)
	if !bytes.Equal(a, b) {
		t.Fatal("null/default normalization mismatch")
	}
	text := "one"
	r.Capture.Response.Body = Body{Kind: "text", Text: &text}
	b, _ = CanonicalResult(r)
	if bytes.Equal(a, b) {
		t.Fatal("body text identity lost")
	}
	r.Fence = 9007199254740992
	if _, e = CanonicalResult(r); e == nil {
		t.Fatal("unsafe fence accepted")
	}
	for _, input := range []string{`{"schemaVersion":1,"schemaVersion":1}`, `{"extra":1}`, `{"fence":1.5}`, `{"nonce":"\ud800"}`, `{"nonce":"\udc00"}`, "{\"nonce\":\"\xff\"}"} {
		var out Result
		if StrictJSON([]byte(input), &out) == nil {
			t.Fatal("invalid result JSON accepted")
		}
	}
}
func TestDefensiveLocalSanitizationNoNetwork(t *testing.T) {
	p := fixturePlan()
	p.Target = "local"
	p.Configuration.URL = "http://localhost:3000/"
	p.Destination = &Destination{"http://localhost:3000", "loopback"}
	values := map[string]string{"s": "disposable-secret-value"}
	e := NewEvidence("project", "execution", "time", p)
	status := 200
	text := `{"authorization":"raw","value":"disposable-secret-value"}`
	e.Status = "completed"
	e.Outcome = Outcome{Certainty: "known"}
	e.Response.HTTPStatus = &status
	e.Response.Headers = []Header{{"Set-Cookie", "raw"}, {"X-Value", "disposable-secret-value"}}
	e.Response.Body = Body{Kind: "json", Text: &text}
	request, _ := LocalRequestCapture(context.Background(), p, values, *p.Destination)
	e.Request = request
	c, err := SanitizeLocalCapture(context.Background(), p, values, CaptureEvidence(e))
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(c)
	if strings.Contains(string(raw), values["s"]) || strings.Contains(string(raw), `"raw"`) {
		t.Fatal("server defensive sanitization leaked")
	}
	e.Outcome.Certainty = "bogus"
	if _, err = SanitizeLocalCapture(context.Background(), p, values, CaptureEvidence(e)); err == nil {
		t.Fatal("invalid outcome accepted")
	}
}

func TestResultWorstEscapingFitsLambdaIngress(t *testing.T) {
	w := Work{LeaseID: NewID(), Fence: 1, Nonce: base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))}
	e := NewEvidence("p", "e", "now", fixturePlan())
	text := strings.Repeat("\"", BodyLimit-2048)
	e.Response.Body = Body{Kind: "text", Text: &text}
	e.Status = "completed"
	e.Outcome = Outcome{Certainty: "known"}
	candidate, _ := json.Marshal(Result{1, w.LeaseID, 1, w.Nonce, CaptureEvidence(e)})
	if len(candidate) > EvidenceLimit || resultTransportFits(candidate) {
		t.Fatal("escaping fixture does not hit ingress boundary")
	}
	r, err := ResultFromEvidence(w, e)
	raw, _ := json.Marshal(r)
	if err != nil || !resultTransportFits(raw) || r.Capture.Response.Body.Kind != "omitted" {
		t.Fatal("ingress overflow not frozen into omission")
	}
}
