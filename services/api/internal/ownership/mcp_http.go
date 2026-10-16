package ownership

import (
	"context"
	"errors"
	"github.com/VioletProto/Kurier/services/api/execution"
	wire "github.com/VioletProto/Kurier/services/request-runtime/mcpwire"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func mcpManagementError(err error) error {
	var e *execution.Error
	if errors.As(err, &e) {
		return apiError(e.Status, e.Code, "MCP credential operation unavailable; refresh status before a deliberate retry.")
	}
	return err
}

func (a *Server) mcpManagement(w http.ResponseWriter, r *http.Request, u User) error {
	s := a.store.executions
	if s == nil {
		return unavailable()
	}
	switch r.Method {
	case "GET":
		v, e := s.CurrentMCP(r.Context(), u.UserID)
		if e != nil {
			return mcpManagementError(e)
		}
		success(w, 200, map[string]any{"credential": v, "serverTime": s.ServerTime()})
		return nil
	case "DELETE":
		if e := emptyAgentBody(r); e != nil {
			return e
		}
		if e := s.RevokeMCP(r.Context(), u.UserID, r.PathValue("tokenId")); e != nil {
			return mcpManagementError(e)
		}
		w.WriteHeader(204)
		return nil
	case "POST":
		raw, e := readMCPBody(w, r)
		if e != nil {
			return e
		}
		defer clear(raw)
		var input execution.MCPPair
		if execution.NonNullObject(raw) != nil || execution.StrictJSON(raw, &input) != nil {
			return apiError(400, "invalid_request", "Invalid credential request.")
		}
		keys := r.Header.Values("Idempotency-Key")
		if len(keys) != 1 {
			return apiError(400, "invalid_request", "One Idempotency-Key required.")
		}
		v, created, e := s.CreateMCP(r.Context(), u.UserID, keys[0], input)
		if e != nil {
			return mcpManagementError(e)
		}
		status := 200
		if created {
			status = 201
		}
		success(w, status, v)
		return nil
	}
	return missing()
}
func readMCPBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	media, _, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if e != nil || media != "application/json" || r.Header.Get("Content-Encoding") != "" {
		return nil, apiError(400, "invalid_request", "Use uncompressed JSON.")
	}
	b, e := io.ReadAll(http.MaxBytesReader(w, r.Body, wire.InputLimit))
	if e != nil {
		return nil, apiError(413, "payload_too_large", "Request too large.")
	}
	return b, nil
}
func (a *Server) mcpProtocol(w http.ResponseWriter, r *http.Request) {
	fail := func(e error) {
		status := 503
		var ex *execution.Error
		var ae *APIError
		if errors.As(e, &ex) {
			status = ex.Status
		} else if errors.As(e, &ae) {
			status = ae.Status
			e = &execution.Error{Status: status, Code: ae.Code}
		}
		v := execution.MCPError(e)
		if v.RetryAfterSeconds > 0 {
			w.Header().Set("Retry-After", "60")
			if v.RetryAfterSeconds == 2 {
				w.Header().Set("Retry-After", "2")
			}
		}
		writeJSON(w, status, wire.Envelope{Error: v})
	}
	if a.store.executions == nil {
		fail(unavailable())
		return
	}
	h := r.Header.Values("Authorization")
	if len(h) != 1 || !strings.HasPrefix(h[0], "Bearer ") || strings.Count(h[0], " ") != 1 {
		fail(&execution.Error{Status: 401, Code: "unauthenticated"})
		return
	}
	if e := a.store.executions.CheckMCPCredential(r.Context(), strings.TrimPrefix(h[0], "Bearer ")); e != nil {
		fail(e)
		return
	}
	raw, e := readMCPBody(w, r)
	if e != nil {
		fail(e)
		return
	}
	defer clear(raw)
	var in wire.Invocation
	if execution.NonNullObject(raw) != nil || execution.StrictJSON(raw, &in) != nil {
		fail(&execution.Error{Status: 400, Code: "invalid_request"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	env, deadline, e := a.store.executions.MCPInvoke(ctx, strings.TrimPrefix(h[0], "Bearer "), in.Tool, in.Input)
	if e != nil {
		fail(e)
		return
	}
	w.Header().Set("X-Kurier-Release-Until", deadline)
	status := 200
	if env.Error != nil {
		status = map[string]int{"invalid_request": 400, "not_found": 404, "unauthenticated": 401, "conflict": 409, "evidence_too_large": 413, "evidence_unavailable": 503, "service_unavailable": 503, "rate_limited": 429, "payload_too_large": 413, "internal_error": 500}[env.Error.Code]
		if status == 0 {
			status = 503
		}
		if env.Error.RetryAfterSeconds > 0 {
			w.Header().Set("Retry-After", strconv.Itoa(env.Error.RetryAfterSeconds))
		}
	}
	writeJSON(w, status, env)
}
