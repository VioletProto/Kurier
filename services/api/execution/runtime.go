package execution

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"sort"
	"strings"
	"time"
)

type Prepared struct {
	request    *http.Request
	transport  *http.Transport
	sanitizer  *sanitizer
	policy     RedactionPolicy
	capture    RequestCapture
	timeout    time.Duration
	preparedAt time.Time
}

func Prepare(ctx context.Context, plan Plan, values map[string]string) (*Prepared, error) {
	return prepare(ctx, plan, values, net.DefaultResolver)
}
func prepare(ctx context.Context, plan Plan, values map[string]string, resolver Resolver) (*Prepared, error) {
	preparedAt := time.Now()
	c := plan.Configuration
	u, e := ValidateDestination(c.URL)
	if e != nil {
		return nil, e
	}
	if _, e = validatedIPs(ctx, resolver, u.Hostname()); e != nil {
		return nil, e
	}
	resolve := func(value string, ref *Ref, sensitive bool) (string, error) {
		if !sensitive {
			return value, nil
		}
		if ref == nil {
			return "", failure(400, "source_unavailable")
		}
		v, ok := values[ref.SecretID]
		if !ok {
			return "", failure(400, "source_unavailable")
		}
		if u.Scheme == "http" && !plan.Options.AllowInsecureSecrets {
			return "", failure(400, "source_unavailable")
		}
		return v, nil
	}
	query := u.RawQuery
	for _, f := range c.QueryParameters {
		if !f.Enabled {
			continue
		}
		v, e := resolve(f.Value, f.SecretRef, f.Sensitive)
		if e != nil {
			return nil, e
		}
		if query != "" {
			query += "&"
		}
		query += url.QueryEscape(f.Name) + "=" + url.QueryEscape(v)
	}
	u.RawQuery = query
	if len(u.String()) > ConfigLimit {
		return nil, failure(400, "input_limit_exceeded")
	}
	body := []byte{}
	if c.Body != nil {
		v, e := resolve(c.Body.Text, c.Body.SecretRef, c.Body.Sensitive)
		if e != nil {
			return nil, e
		}
		body = []byte(v)
		if len(c.Body.SecretFields) > 0 {
			var obj any
			if StrictJSON(body, &obj) != nil {
				return nil, failure(400, "source_unavailable")
			}
			for _, f := range c.Body.SecretFields {
				v, e := resolve("", f.SecretRef, true)
				if e != nil {
					return nil, e
				}
				var scalar any
				if StrictJSON([]byte(v), &scalar) != nil {
					return nil, failure(400, "source_unavailable")
				}
				obj, e = setPointer(obj, f.Pointer, scalar)
				if e != nil {
					return nil, e
				}
			}
			body, e = json.Marshal(obj)
			if e != nil {
				return nil, e
			}
		}
	}
	defer clear(body)
	if len(body) > ConfigLimit {
		return nil, failure(400, "input_limit_exceeded")
	}
	if c.Method != "GET" && c.Method != "POST" && c.Method != "PUT" && c.Method != "PATCH" && c.Method != "DELETE" {
		return nil, failure(400, "input_limit_exceeded")
	}
	req, e := http.NewRequestWithContext(ctx, c.Method, u.String(), bytes.NewReader(append([]byte(nil), body...)))
	if e != nil {
		return nil, failure(400, "input_limit_exceeded")
	}
	req.GetBody = nil
	req.Close = true
	for _, f := range c.Headers {
		if !f.Enabled {
			continue
		}
		v, e := resolve(f.Value, f.SecretRef, f.Sensitive)
		if e != nil {
			return nil, e
		}
		if !headerName(f.Name) || strings.ContainsAny(v, "\r\n\x00") || strings.EqualFold(f.Name, "Host") {
			return nil, failure(400, "input_limit_exceeded")
		}
		switch strings.ToLower(f.Name) {
		case "content-length", "transfer-encoding", "connection", "upgrade", "proxy-connection", "expect":
			return nil, failure(400, "input_limit_exceeded")
		}
		req.Header.Add(f.Name, v)
	}
	if req.Header.Get("Content-Type") == "" && c.Body != nil {
		if c.Body.Type == "json" {
			req.Header.Set("Content-Type", "application/json")
		} else {
			req.Header.Set("Content-Type", "text/plain; charset=utf-8")
		}
	}
	req.Header.Set("Accept-Encoding", "gzip")
	if headerBytes(req.Header) > ConfigLimit {
		return nil, failure(400, "input_limit_exceeded")
	}
	san := newSanitizer(values)
	capture := RequestCapture{Method: c.Method, URL: san.url(u), Headers: sanitizeHeaders(req.Header, san, nil), Body: emptyBody()}
	encodedRequestHeaders, _ := json.Marshal(capture.Headers)
	if len(encodedRequestHeaders) > ConfigLimit {
		return nil, failure(400, "sanitization_failed")
	}
	if c.Body != nil {
		if c.Body.Sensitive {
			capture.Body = omitted("sensitive_request_body")
		} else {
			media := "text/plain"
			if c.Body.Type == "json" {
				media = "application/json"
			}
			capture.Body = san.body(body, media, RedactionPolicy{})
		}
	}
	tr := &http.Transport{Proxy: nil, DialContext: safeDialer(resolver), TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}, TLSHandshakeTimeout: 5 * time.Second, ResponseHeaderTimeout: 10 * time.Second, MaxResponseHeaderBytes: ConfigLimit, DisableKeepAlives: true, DisableCompression: true, ForceAttemptHTTP2: false}
	return &Prepared{request: req, transport: tr, sanitizer: san, policy: plan.Options.ResponseRedaction, capture: capture, timeout: time.Duration(plan.Options.TimeoutSeconds) * time.Second, preparedAt: preparedAt}, nil
}
func headerBytes(h http.Header) int {
	n := 0
	for k, vs := range h {
		for _, v := range vs {
			n += len(k) + len(v) + 4
		}
	}
	return n
}
func sanitizeHeaders(h http.Header, san *sanitizer, additional []string) []Header {
	out := []Header{}
	keys := []string{}
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		masked := credentialName(k)
		for _, extra := range additional {
			if strings.EqualFold(k, extra) {
				masked = true
			}
		}
		for _, v := range h[k] {
			if masked {
				v = "[REDACTED]"
			} else {
				v = san.text(v)
			}
			out = append(out, Header{san.text(k), v})
		}
	}
	return out
}
func NewEvidence(project, execution, submitted string, plan Plan) *Evidence {
	e := &Evidence{SchemaVersion: 1, ProjectID: project, ExecutionID: execution, Source: Source{plan.RequestID, plan.Revision}, Target: "cloud", SubmittedAt: submitted, Status: "failed", Outcome: Outcome{Certainty: "not_dispatched"}, Request: RequestCapture{Method: plan.Configuration.Method, URL: plan.Configuration.URL, Headers: []Header{}, Body: emptyBody()}, Response: ResponseCapture{Headers: []Header{}, Body: emptyBody()}}
	e.Redaction.PolicyVersion = 1
	e.Redaction.Applied = true
	return e
}
func (e *Evidence) Fail(code, certainty string) {
	e.Status = "failed"
	e.Outcome = Outcome{&code, certainty}
	e.CompletedAt = stamp(time.Now())
}
func (p *Prepared) Run(ctx context.Context, e *Evidence) {
	defer p.transport.CloseIdleConnections()
	started := p.preparedAt
	ctx, cancel := context.WithDeadline(ctx, started.Add(p.timeout))
	defer cancel()
	var first *int64
	req := p.request.WithContext(httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{GotFirstResponseByte: func() { first = pointer(time.Since(started).Milliseconds()) }}))
	if req.Body != nil {
		defer req.Body.Close()
	}
	e.Request = p.capture
	e.StartedAt = pointer(stamp(started))
	e.Outcome.Certainty = "unknown"
	client := &http.Client{Transport: p.transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(req)
	e.Timing.TimeToFirstByteMS = first
	defer func() {
		e.Timing.DurationMS = pointer(time.Since(started).Milliseconds())
		e.CompletedAt = stamp(time.Now())
	}()
	if err != nil {
		code := "upstream_network_error"
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			code = "execution_timeout"
		}
		e.Fail(code, "unknown")
		return
	}
	defer response.Body.Close()
	e.Response.HTTPStatus = &response.StatusCode
	e.Response.Headers = sanitizeHeaders(response.Header, p.sanitizer, p.policy.Headers)
	encodedHeaders, _ := json.Marshal(e.Response.Headers)
	if len(encodedHeaders) > ConfigLimit {
		e.Response.Headers = []Header{}
		e.Response.Body = omitted("sanitization_failed")
		e.Fail("sanitization_failed", "known")
		return
	}
	wire := &countReader{reader: io.LimitReader(response.Body, BodyLimit+1)}
	var reader io.Reader = wire
	encoding := strings.ToLower(strings.TrimSpace(response.Header.Get("Content-Encoding")))
	var compressed *gzip.Reader
	if encoding != "" && encoding != "identity" {
		if encoding != "gzip" {
			raw, readErr := io.ReadAll(wire)
			clear(raw)
			e.Response.WireBytesRead = wire.count
			e.Response.Body = omitted("unsupported_encoding")
			if wire.count > BodyLimit {
				e.Fail("response_limit_exceeded", "known")
				return
			}
			if readErr != nil {
				code := "upstream_network_error"
				if ctx.Err() != nil {
					code = "execution_timeout"
				}
				e.Fail(code, "unknown")
				return
			}
			e.Status = statusFor(response.StatusCode)
			e.Outcome = Outcome{Certainty: "known"}
			if e.Status == "failed" {
				e.Outcome.Code = pointer("upstream_http_error")
			}
			return
		}
		compressed, err = gzip.NewReader(wire)
		if err != nil {
			e.Response.Body = omitted("invalid_encoding")
			e.Fail("sanitization_failed", "known")
			return
		}
		defer compressed.Close()
		reader = compressed
	}
	raw, err := io.ReadAll(io.LimitReader(reader, BodyLimit+1))
	defer clear(raw)
	e.Response.WireBytesRead = wire.count
	e.Response.DecodedBytesRead = int64(len(raw))
	if wire.count > BodyLimit || len(raw) > BodyLimit {
		e.Response.Body = omitted("response_limit_exceeded")
		e.Fail("response_limit_exceeded", "known")
		return
	}
	if err != nil {
		e.Response.Body = omitted("incomplete_response")
		code := "upstream_network_error"
		if ctx.Err() != nil {
			code = "execution_timeout"
		}
		e.Fail(code, "unknown")
		return
	}
	e.Response.Body = p.sanitizer.body(raw, response.Header.Get("Content-Type"), p.policy)
	e.Status = statusFor(response.StatusCode)
	e.Outcome = Outcome{Certainty: "known"}
	if e.Status == "failed" {
		e.Outcome.Code = pointer("upstream_http_error")
	}
}
func statusFor(status int) string {
	if status >= 200 && status < 400 {
		return "completed"
	}
	return "failed"
}

type countReader struct {
	reader io.Reader
	count  int64
}

func (r *countReader) Read(p []byte) (int, error) {
	n, e := r.reader.Read(p)
	r.count += int64(n)
	return n, e
}
