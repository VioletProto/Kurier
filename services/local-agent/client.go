package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	runner "github.com/VioletProto/Kurier/services/request-runtime"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type protocolError struct {
	Status     int
	RetryAfter time.Duration
}

func (e *protocolError) Error() string { return "hosted agent operation unavailable" }

type client struct {
	origin, token string
	http          *http.Client
	agent         runner.Agent
	seen          map[string]bool
	now           func() time.Time
	pause         func(context.Context, time.Duration) error
}

func newClient(origin, token string) (*client, error) {
	u, e := url.Parse(origin)
	if e != nil || u == nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return nil, errors.New("choose an explicit HTTPS Kurier API origin")
	}
	transport := &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 5 * time.Second}).DialContext, TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}, TLSHandshakeTimeout: 5 * time.Second, ResponseHeaderTimeout: 10 * time.Second, MaxResponseHeaderBytes: 32 * 1024, DisableKeepAlives: true, DisableCompression: true}
	return &client{origin: origin, token: token, http: &http.Client{Transport: transport, Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, seen: map[string]bool{}, now: time.Now, pause: wait}, nil
}
func (c *client) call(ctx context.Context, method, path string, input any, output any, limit int) error {
	var raw []byte
	var e error
	if input != nil {
		raw, e = json.Marshal(input)
		if e != nil || len(raw) > runner.EvidenceLimit {
			return errors.New("agent packet exceeds limit")
		}
	}
	defer clear(raw)
	request, e := http.NewRequestWithContext(ctx, method, c.origin+path, bytes.NewReader(raw))
	if e != nil {
		return errors.New("hosted request unavailable")
	}
	request.GetBody = nil
	request.Close = true
	request.Header.Set("Authorization", "Bearer "+c.token)
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, e := c.http.Do(request)
	if e != nil {
		return errors.New("hosted connection unavailable")
	}
	defer response.Body.Close()
	body, e := io.ReadAll(io.LimitReader(response.Body, int64(limit+1)))
	defer clear(body)
	if e != nil || len(body) > limit {
		return errors.New("incomplete hosted response")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		retry, _ := strconv.Atoi(response.Header.Get("Retry-After"))
		if retry < 0 {
			retry = 0
		}
		if retry > 60 {
			retry = 60
		}
		return &protocolError{response.StatusCode, time.Duration(retry) * time.Second}
	}
	if output == nil {
		return nil
	}
	media := response.Header.Get("Content-Type")
	if !strings.HasPrefix(media, "application/json") {
		return errors.New("invalid hosted response")
	}
	if e = runner.StrictJSON(body, output); e != nil {
		return errors.New("invalid hosted response")
	}
	return nil
}
func (c *client) session(ctx context.Context) error {
	var response struct {
		Data struct {
			Agent      runner.Agent `json:"agent"`
			ServerTime string       `json:"serverTime"`
		} `json:"data"`
	}
	if e := c.call(ctx, "GET", "/api/v1/local-agent/session", nil, &response, 32*1024); e != nil {
		return e
	}
	a := response.Data.Agent
	if a.LocalAgentID == "" || a.RecoveryGeneration == "" || len(a.Scopes) < 1 || len(a.Scopes) > 10 {
		return errors.New("invalid agent scope")
	}
	for _, scope := range a.Scopes {
		if scope.ProjectID == "" || len(scope.Destinations) < 1 || len(scope.Destinations) > 16 {
			return errors.New("invalid agent scope")
		}
		for _, d := range scope.Destinations {
			normalized, e := runner.NormalizeLocalDestination(d)
			if e != nil || normalized != d {
				return errors.New("invalid destination permission")
			}
		}
	}
	c.agent = a
	return nil
}
func (c *client) workAllowed(w runner.Work) bool {
	nonce, e := base64.RawURLEncoding.DecodeString(w.Nonce)
	if e != nil || len(nonce) != 32 || base64.RawURLEncoding.EncodeToString(nonce) != w.Nonce || w.Fence > 9007199254740991 {
		return false
	}
	intent, e := time.Parse(time.RFC3339Nano, w.DispatchIntentAt)
	if e != nil {
		return false
	}
	start, e := time.Parse(time.RFC3339Nano, w.ExecuteNotAfter)
	if e != nil || start.Sub(intent) != 30*time.Second {
		return false
	}
	deadline, e := time.Parse(time.RFC3339Nano, w.Deadline)
	if e != nil || deadline.Sub(intent) != 120*time.Second {
		return false
	}
	if w.SchemaVersion != 1 || w.LocalAgentID != c.agent.LocalAgentID || w.RecoveryGeneration != c.agent.RecoveryGeneration || w.Plan.Target != "local" || w.Plan.LocalAgentID != w.LocalAgentID || w.Plan.Destination == nil || *w.Plan.Destination != w.Destination || w.Plan.Options.Local == nil || !w.Plan.Options.Local.AllowPrivateNetwork || w.Plan.Options.Local.LocalAgentID != w.LocalAgentID || w.JobID == "" || w.ExecutionID == "" || w.LeaseID == "" || w.Fence < 1 || w.ProtectedInputs.Values == nil {
		return false
	}
	if _, e := runner.ValidateLocalURL(w.Plan.Configuration.URL, w.Destination); e != nil {
		return false
	}
	if w.Plan.Options.TimeoutSeconds < 1 || w.Plan.Options.TimeoutSeconds > 60 {
		return false
	}
	for _, scope := range c.agent.Scopes {
		if scope.ProjectID == w.ProjectID {
			for _, d := range scope.Destinations {
				if d == w.Destination {
					return true
				}
			}
		}
	}
	return false
}
func remaining(server, deadline string, elapsed time.Duration) (time.Duration, error) {
	s, e := time.Parse(time.RFC3339Nano, server)
	if e != nil {
		return 0, errors.New("invalid server time")
	}
	d, e := time.Parse(time.RFC3339Nano, deadline)
	if e != nil {
		return 0, errors.New("invalid deadline")
	}
	return d.Sub(s) - elapsed, nil
}
func (c *client) execute(ctx context.Context, w runner.Work, serverTime string, sent time.Time) error {
	defer clear(w.ProtectedInputs.Values)
	identity := w.JobID + "/" + w.LeaseID + "/" + strconv.FormatInt(w.Fence, 10)
	if c.seen[identity] {
		return nil
	}
	if len(c.seen) >= 4096 {
		return errors.New("agent session memory limit reached; restart without restoring jobs")
	}
	c.seen[identity] = true // In-memory only; never restore jobs after restart.
	if !c.workAllowed(w) {
		return errors.New("grant violates confirmed scope")
	}
	start, e := remaining(serverTime, w.ExecuteNotAfter, c.now().Sub(sent))
	if e != nil || start <= 5*time.Second || start > 30*time.Second {
		return errors.New("start window not provably open")
	}
	lease, e := remaining(serverTime, w.Deadline, c.now().Sub(sent))
	if e != nil || lease <= 10*time.Second || lease > 120*time.Second {
		return errors.New("invalid lease window")
	}
	deadline := c.now().Add(lease)
	timeout := time.Duration(w.Plan.Options.TimeoutSeconds) * time.Second
	if timeout > lease-10*time.Second {
		timeout = lease - 10*time.Second
	}
	evidence := runner.NewEvidence(w.ProjectID, w.ExecutionID, w.DispatchIntentAt, w.Plan)
	executionCtx, cancel := context.WithTimeout(ctx, timeout)
	prepared, e := runner.PrepareLocal(executionCtx, w.Plan, w.ProtectedInputs.Values, w.Destination)
	if e != nil {
		code := "destination_blocked"
		var fixed *runner.Error
		if errors.As(e, &fixed) {
			code = fixed.Code
		}
		evidence.Fail(code, "not_dispatched")
	} else {
		// DNS/preparation consumes the same conservative start and lease budget.
		start, e = remaining(serverTime, w.ExecuteNotAfter, c.now().Sub(sent))
		if e != nil || start <= 5*time.Second {
			cancel()
			return errors.New("start window elapsed during preparation")
		}
		prepared.Run(executionCtx, evidence)
	}
	cancel()
	if evidence.Timing.DurationMS != nil && *evidence.Timing.DurationMS > 60000 {
		*evidence.Timing.DurationMS = 60000
	}
	if evidence.Timing.TimeToFirstByteMS != nil && evidence.Timing.DurationMS != nil && *evidence.Timing.TimeToFirstByteMS > *evidence.Timing.DurationMS {
		*evidence.Timing.TimeToFirstByteMS = *evidence.Timing.DurationMS
	}
	packet, e := runner.ResultFromEvidence(w, evidence)
	if e != nil {
		return e
	}
	clear(w.ProtectedInputs.Values)
	// Only this immutable sanitized packet can retry. Acknowledgment retries never
	// re-enter Prepare/Run, including after the lease expired.
	var ack struct {
		Data runner.Receipt `json:"data"`
	}
	for attempt := 0; attempt < 10; attempt++ {
		e = c.call(ctx, "POST", "/api/v1/projects/"+url.PathEscape(w.ProjectID)+"/local-agent/jobs/"+url.PathEscape(w.JobID)+"/result", packet, &ack, 16*1024)
		if e == nil {
			if ack.Data.ExecutionID != w.ExecutionID || ack.Data.AcceptedAt == "" {
				return errors.New("invalid result acknowledgment")
			}
			return nil
		}
		var p *protocolError
		if errors.As(e, &p) && (p.Status == 401 || p.Status == 404 || p.Status == 409 || p.Status == 400 || p.Status == 413) {
			return e
		}
		if !c.now().Before(deadline) {
			return e
		} // This attempt can discover an already accepted receipt after expiry.
		delay := time.Duration(6+rand.IntN(3)) * time.Second
		if errors.As(e, &p) && p.RetryAfter > delay {
			delay = p.RetryAfter
		}
		if e = c.pause(ctx, delay); e != nil {
			return e
		}
	}
	return e
}
func wait(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
func (c *client) run(ctx context.Context, report func(string)) error {
	interval := 10 * time.Second / time.Duration(len(c.agent.Scopes))
	backoff := interval
	index := 0
	for ctx.Err() == nil {
		project := c.agent.Scopes[index%len(c.agent.Scopes)].ProjectID
		index++
		sent := c.now()
		var response struct {
			Data runner.PollResponse `json:"data"`
		}
		e := c.call(ctx, "POST", "/api/v1/projects/"+url.PathEscape(project)+"/local-agent/poll", nil, &response, runner.GrantLimit)
		if e == nil && response.Data.Work != nil {
			report("Executing authorized local job.")
			e = c.execute(ctx, *response.Data.Work, response.Data.ServerTime, sent)
			if e == nil {
				report("Sanitized evidence acknowledged.")
			} else {
				report("Execution result unresolved; target HTTP will not replay.")
			}
		}
		if e != nil {
			var p *protocolError
			if errors.As(e, &p) && p.Status == 401 {
				return errors.New("credential revoked or expired; re-pair in browser")
			}
			backoff *= 2
			if backoff > 60*time.Second {
				backoff = 60 * time.Second
			}
			if errors.As(e, &p) && p.RetryAfter > backoff {
				backoff = p.RetryAfter
			}
			report("Agent offline or busy; retrying polling only.")
		} else {
			backoff = interval
		}
		if e = c.pause(ctx, backoff+time.Duration(rand.IntN(500))*time.Millisecond); e != nil {
			return e
		}
	}
	return ctx.Err()
}
