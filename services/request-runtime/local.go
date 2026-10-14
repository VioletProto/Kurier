package execution

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const GrantLimit = 128 * 1024

type Destination struct {
	Origin       string `json:"origin"`
	AddressClass string `json:"addressClass"`
}
type Scope struct {
	ProjectID    string        `json:"projectId"`
	Destinations []Destination `json:"destinations"`
}
type LocalOptions struct {
	LocalAgentID        string `json:"localAgentId"`
	AllowPrivateNetwork bool   `json:"allowPrivateNetwork"`
}
type Agent struct {
	LocalAgentID       string  `json:"localAgentId"`
	Name               string  `json:"name"`
	Scopes             []Scope `json:"scopes"`
	CreatedAt          string  `json:"createdAt"`
	ExpiresAt          string  `json:"expiresAt"`
	RevokedAt          *string `json:"revokedAt"`
	LastSeenAt         *string `json:"lastSeenAt"`
	RecoveryGeneration string  `json:"recoveryGeneration"`
}
type Work struct {
	SchemaVersion      int    `json:"schemaVersion"`
	ProjectID          string `json:"projectId"`
	JobID              string `json:"jobId"`
	ExecutionID        string `json:"executionId"`
	LocalAgentID       string `json:"localAgentId"`
	RecoveryGeneration string `json:"recoveryGeneration"`
	LeaseID            string `json:"leaseId"`
	Fence              int64  `json:"fence"`
	Nonce              string `json:"nonce"`
	DispatchIntentAt   string `json:"dispatchIntentAt"`
	ExecuteNotAfter    string `json:"executeNotAfter"`
	Deadline           string `json:"deadline"`
	Plan               Plan   `json:"plan"`
	ProtectedInputs    struct {
		Values map[string]string `json:"values"`
	} `json:"protectedInputs"`
	Destination Destination `json:"destination"`
}
type PollResponse struct {
	Work       *Work  `json:"work"`
	Busy       bool   `json:"busy"`
	ServerTime string `json:"serverTime"`
}
type Capture struct {
	Status    string          `json:"status"`
	Outcome   Outcome         `json:"outcome"`
	Request   RequestCapture  `json:"request"`
	Response  ResponseCapture `json:"response"`
	Timing    Timing          `json:"timing"`
	Redaction Redaction       `json:"redaction"`
}
type Result struct {
	SchemaVersion int     `json:"schemaVersion"`
	LeaseID       string  `json:"leaseId"`
	Fence         int64   `json:"fence"`
	Nonce         string  `json:"nonce"`
	Capture       Capture `json:"capture"`
}
type Receipt struct {
	ExecutionID string `json:"executionId"`
	AcceptedAt  string `json:"acceptedAt"`
}

func LocalIP(ip netip.Addr, class string) bool {
	if ip.Zone() != "" {
		return false
	}
	ip = ip.Unmap()
	if !ip.IsValid() || ip.Zone() != "" || netip.MustParsePrefix("fd00:ec2::/32").Contains(ip) || ip == netip.MustParseAddr("fd20:ce::254") {
		return false
	}
	if class == "loopback" {
		return ip.IsLoopback()
	}
	if class != "private" {
		return false
	}
	for _, p := range []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "fc00::/7"} {
		if netip.MustParsePrefix(p).Contains(ip) {
			return true
		}
	}
	return false
}
func NormalizeLocalDestination(d Destination) (Destination, error) {
	u, e := url.Parse(d.Origin)
	if e != nil || u == nil || u.Opaque != "" || u.User != nil || u.Fragment != "" || u.RawQuery != "" || u.Path != "" || u.Host == "" || strings.HasSuffix(u.Host, ":") || (u.Scheme != "http" && u.Scheme != "https") || (d.AddressClass != "loopback" && d.AddressClass != "private") {
		return d, failure(400, "destination_blocked")
	}
	host := strings.ToLower(u.Hostname())
	port := u.Port()
	if port == "" {
		port = "80"
		if u.Scheme == "https" {
			port = "443"
		}
	}
	n, e := strconv.Atoi(port)
	if e != nil || n < 1 || n > 65535 || strconv.Itoa(n) != port {
		return d, failure(400, "destination_blocked")
	}
	if ip, e := netip.ParseAddr(host); e == nil {
		if !LocalIP(ip, d.AddressClass) || ip.String() != host {
			return d, failure(400, "destination_blocked")
		}
	} else {
		if !hostPattern.MatchString(host) || numericHost.MatchString(host) || strings.HasPrefix(host, "0x") || strings.HasSuffix(host, ".") || len(host) > 253 {
			return d, failure(400, "destination_blocked")
		}
		for _, label := range strings.Split(host, ".") {
			if len(label) == 0 || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
				return d, failure(400, "destination_blocked")
			}
		}
		if host == "localhost" && d.AddressClass != "loopback" {
			return d, failure(400, "destination_blocked")
		}
	}
	d.Origin = u.Scheme + "://" + net.JoinHostPort(host, port)
	return d, nil
}
func ValidateLocalURL(raw string, d Destination) (*url.URL, error) {
	u, e := url.Parse(raw)
	if e != nil || u == nil || u.Opaque != "" || u.User != nil || u.Fragment != "" || strings.ContainsAny(raw, "\r\n\x00 {}#") || len(raw) > ConfigLimit {
		return nil, failure(400, "destination_blocked")
	}
	actual, e := NormalizeLocalDestination(Destination{Origin: u.Scheme + "://" + u.Host, AddressClass: d.AddressClass})
	if e != nil {
		return nil, e
	}
	expected, e := NormalizeLocalDestination(d)
	if e != nil || actual != expected {
		return nil, failure(400, "destination_blocked")
	}
	return u, nil
}
func localIPs(ctx context.Context, r Resolver, host, class string) ([]netip.Addr, error) {
	if ip, e := netip.ParseAddr(host); e == nil {
		if !LocalIP(ip, class) {
			return nil, failure(400, "destination_blocked")
		}
		return []netip.Addr{ip.Unmap()}, nil
	}
	dns, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	ips, e := r.LookupNetIP(dns, "ip", host)
	if e != nil || len(ips) == 0 {
		return nil, failure(502, "upstream_network_error")
	}
	for i, ip := range ips {
		if !LocalIP(ip, class) || (strings.EqualFold(host, "localhost") && !ip.Unmap().IsLoopback()) {
			return nil, failure(400, "destination_blocked")
		}
		ips[i] = ip.Unmap()
	}
	return ips, nil
}
func localDialer(r Resolver, d Destination) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, e := net.SplitHostPort(address)
		if e != nil {
			return nil, failure(400, "destination_blocked")
		}
		if _, e = ValidateLocalURL(strings.Split(d.Origin, "://")[0]+"://"+net.JoinHostPort(host, port), d); e != nil {
			return nil, e
		}
		ips, e := localIPs(ctx, r, host, d.AddressClass)
		if e != nil {
			return nil, e
		}
		return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
	}
}
func PrepareLocal(ctx context.Context, p Plan, v map[string]string, d Destination) (*Prepared, error) {
	return preparePolicy(ctx, p, v, net.DefaultResolver, &d, true)
}
func LocalRequestCapture(ctx context.Context, p Plan, v map[string]string, d Destination) (RequestCapture, error) {
	prepared, e := preparePolicy(ctx, p, v, net.DefaultResolver, &d, false)
	if e != nil {
		return RequestCapture{}, e
	}
	if prepared.request.Body != nil {
		_ = prepared.request.Body.Close()
	}
	return prepared.capture, nil
}
func Scrub(v string, values map[string]string) string { return newSanitizer(values).text(v) }
func CaptureEvidence(e *Evidence) Capture {
	return Capture{e.Status, e.Outcome, e.Request, e.Response, e.Timing, e.Redaction}
}

// Defensive server sanitization never needs to connect to a private destination.
func SanitizeLocalCapture(ctx context.Context, p Plan, values map[string]string, c Capture) (Capture, error) {
	if c.Status != "completed" && c.Status != "failed" || c.Redaction.PolicyVersion != 1 || !c.Redaction.Applied || c.Request.Method != p.Configuration.Method {
		return c, failure(400, "invalid_request")
	}
	if c.Outcome.Certainty != "known" && c.Outcome.Certainty != "unknown" && c.Outcome.Certainty != "not_dispatched" {
		return c, failure(400, "invalid_request")
	}
	codes := map[string]bool{"upstream_http_error": true, "execution_timeout": true, "upstream_network_error": true, "execution_outcome_unknown": true, "response_limit_exceeded": true, "destination_blocked": true, "source_unavailable": true, "input_limit_exceeded": true, "sanitization_failed": true}
	if c.Outcome.Code != nil && !codes[*c.Outcome.Code] || c.Status == "failed" && c.Outcome.Code == nil || c.Status == "completed" && (c.Outcome.Code != nil || c.Outcome.Certainty != "known") {
		return c, failure(400, "invalid_request")
	}
	if c.Response.HTTPStatus != nil && (*c.Response.HTTPStatus < 100 || *c.Response.HTTPStatus > 599) || c.Status == "completed" && (c.Response.HTTPStatus == nil || *c.Response.HTTPStatus < 200 || *c.Response.HTTPStatus >= 400) {
		return c, failure(400, "invalid_request")
	}
	if c.Response.WireBytesRead < 0 || c.Response.WireBytesRead > BodyLimit+1 || c.Response.DecodedBytesRead < 0 || c.Response.DecodedBytesRead > BodyLimit+1 {
		return c, failure(400, "invalid_request")
	}
	for _, n := range []*int64{c.Timing.DurationMS, c.Timing.TimeToFirstByteMS} {
		if n != nil && (*n < 0 || *n > 60000) {
			return c, failure(400, "invalid_request")
		}
	}
	if c.Timing.TimeToFirstByteMS != nil && (c.Timing.DurationMS == nil || *c.Timing.TimeToFirstByteMS > *c.Timing.DurationMS) {
		return c, failure(400, "invalid_request")
	}
	if p.Destination == nil {
		return c, failure(400, "invalid_request")
	}
	request, e := LocalRequestCapture(ctx, p, values, *p.Destination)
	if e != nil {
		return c, e
	}
	if c.Outcome.Certainty != "not_dispatched" {
		incoming := c.Request
		if incoming.Headers == nil {
			incoming.Headers = []Header{}
		}
		if incoming.Body.Kind == "omitted" && incoming.Body.Text == nil && incoming.Body.OmissionReason != nil && *incoming.Body.OmissionReason == "encoded_evidence_limit" {
			request.Body = incoming.Body
		}
		expected, _ := json.Marshal(request)
		actual, _ := json.Marshal(incoming)
		if !bytes.Equal(expected, actual) {
			return c, failure(400, "invalid_request")
		}
	}
	c.Request = request
	san := newSanitizer(values)
	h := mapHeader(c.Response.Headers)
	if h == nil {
		return c, failure(400, "invalid_request")
	}
	c.Response.Headers = sanitizeHeaders(h, san, p.Options.ResponseRedaction.Headers)
	raw, _ := json.Marshal(c.Response.Headers)
	if len(raw) > ConfigLimit {
		return c, failure(413, "payload_too_large")
	}
	b := c.Response.Body
	switch b.Kind {
	case "none":
		if b.Text != nil || b.OmissionReason != nil {
			return c, failure(400, "invalid_request")
		}
	case "omitted":
		if b.Text != nil || b.OmissionReason == nil || !omissionReasons[*b.OmissionReason] {
			return c, failure(400, "invalid_request")
		}
	case "json", "text":
		if b.Text == nil || b.OmissionReason != nil || len(*b.Text) > LocalSanitizedBodyLimit {
			return c, failure(400, "invalid_request")
		}
		media := "text/plain"
		if b.Kind == "json" {
			media = "application/json"
		}
		c.Response.Body = san.body([]byte(*b.Text), media, p.Options.ResponseRedaction)
	default:
		return c, failure(400, "invalid_request")
	}
	return c, nil
}

func mapHeader(headers []Header) http.Header {
	h := http.Header{}
	for _, v := range headers {
		if !headerName(strings.ReplaceAll(v.Name, "[REDACTED]", "x")) || strings.ContainsAny(v.Value, "\r\n\x00") {
			return nil
		}
		h.Add(v.Name, v.Value)
	}
	if headerBytes(h) > ConfigLimit {
		return nil
	}
	return h
}

var omissionReasons = map[string]bool{"sensitive_request_body": true, "encoded_evidence_limit": true, "unsupported_encoding": true, "invalid_encoding": true, "sanitization_failed": true, "response_limit_exceeded": true, "incomplete_response": true, "unsupported_content_type": true, "unsupported_charset": true, "invalid_utf8": true, "invalid_json": true, "redaction_policy_unapplied": true, "body_omitted_by_policy": true, "policy_omit_body": true, "unsupported_content": true, "redaction_path_unavailable": true}
