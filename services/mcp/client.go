package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	runner "github.com/VioletProto/Kurier/services/request-runtime"
	wire "github.com/VioletProto/Kurier/services/request-runtime/mcpwire"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var strict = runner.StrictJSON

func validOrigin(raw string) bool {
	u, e := url.Parse(raw)
	return e == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && u.Path == "" && u.RawQuery == "" && u.Fragment == "" && u.Opaque == "" && u.RawPath == "" && u.Hostname() != ""
}

type bridge struct {
	credential storedCredential
	client     *http.Client
}

func newBridge(c storedCredential) (*bridge, error) {
	if !validOrigin(c.APIOrigin) || len(c.Token) != 43 || strings.ContainsAny(c.Token, "\r\n\t ") {
		return nil, errors.New("invalid MCP credential configuration")
	}
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.Proxy = nil
	return &bridge{credential: c, client: &http.Client{Transport: t, Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (b *bridge) invoke(ctx context.Context, tool string, input json.RawMessage) (wire.Envelope, time.Time, error) {
	bad := errors.New("service_unavailable")
	if _, e := wire.Parse(tool, input); e != nil {
		return wire.Envelope{}, time.Time{}, errors.New("invalid_request")
	}
	raw, e := json.Marshal(wire.Invocation{Tool: tool, Input: input})
	if e != nil || len(raw) > wire.InputLimit {
		return wire.Envelope{}, time.Time{}, errors.New("invalid_request")
	}
	start := time.Now()
	req, e := http.NewRequestWithContext(ctx, "POST", b.credential.APIOrigin+"/api/v1/mcp/invocations", bytes.NewReader(raw))
	if e != nil {
		return wire.Envelope{}, time.Time{}, bad
	}
	req.Header.Set("Authorization", "Bearer "+b.credential.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, e := b.client.Do(req)
	if e != nil {
		return wire.Envelope{}, time.Time{}, bad
	}
	defer resp.Body.Close()
	if resp.Header.Get("Content-Encoding") != "" || !strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") {
		return wire.Envelope{}, time.Time{}, bad
	}
	raw, e = io.ReadAll(io.LimitReader(resp.Body, wire.OutputLimit+1))
	if e != nil || len(raw) > wire.OutputLimit {
		return wire.Envelope{}, time.Time{}, bad
	}
	env, e := wire.Decode(tool, raw)
	if e != nil {
		return wire.Envelope{}, time.Time{}, bad
	}
	if env.Error != nil {
		return safeError(env.Error.Code), time.Time{}, nil
	}
	if resp.StatusCode != 200 {
		return wire.Envelope{}, time.Time{}, bad
	}
	server, e := time.Parse(time.RFC3339Nano, env.Data.ServerTime)
	if e != nil {
		return wire.Envelope{}, time.Time{}, bad
	}
	until, e := time.Parse(time.RFC3339Nano, resp.Header.Get("X-Kurier-Release-Until"))
	if e != nil || !until.After(server) || until.Sub(server) > 5*time.Second {
		return wire.Envelope{}, time.Time{}, bad
	}
	deadline := start.Add(until.Sub(server) - 100*time.Millisecond)
	if !time.Now().Before(deadline) {
		return wire.Envelope{}, time.Time{}, bad
	}
	return env, deadline, nil
}
func safeError(code string) wire.Envelope {
	messages := map[string]string{"internal_error": "Unexpected service failure.", "payload_too_large": "Request too large.", "invalid_request": "Invalid read request.", "unauthenticated": "Valid MCP credential required; reauthorize in the browser.", "not_found": "Resource not found.", "conflict": "Evidence is not published.", "evidence_too_large": "Complete evidence exceeds the MCP limit; request summary view.", "rate_limited": "Read budget exceeded; wait before retrying.", "evidence_unavailable": "Published evidence unavailable.", "service_unavailable": "Service temporarily unavailable."}
	msg, ok := messages[code]
	if !ok {
		code = "service_unavailable"
		msg = messages[code]
	}
	retry := 0
	if code == "rate_limited" {
		retry = 60
	}
	if code == "service_unavailable" {
		retry = 2
	}
	return wire.Envelope{Error: &wire.Error{Code: code, Message: msg, Details: []any{}, RequestID: runner.NewID(), RetryAfterSeconds: retry}}
}
