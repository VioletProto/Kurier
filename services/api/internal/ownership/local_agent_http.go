package ownership

import (
	"errors"
	"github.com/VioletProto/Kurier/services/api/execution"
	runner "github.com/VioletProto/Kurier/services/request-runtime"
	"io"
	"mime"
	"net/http"
	"strings"
)

func localAgentError(err error) error {
	var e *execution.Error
	if errors.As(err, &e) {
		return apiError(e.Status, e.Code, "Agent operation could not be completed. Refresh status before a deliberate retry.")
	}
	return err
}
func emptyAgentBody(r *http.Request) error {
	raw, e := io.ReadAll(io.LimitReader(r.Body, 1))
	if e != nil || len(raw) > 0 {
		return apiError(400, "invalid_request", "An empty body is required.")
	}
	return nil
}
func (a *Server) agentManagement(w http.ResponseWriter, r *http.Request, u User) error {
	s := a.store.executions
	if s == nil {
		return unavailable()
	}
	switch r.Method {
	case "GET":
		v, e := s.CurrentAgent(r.Context(), u.UserID)
		if e != nil {
			return localAgentError(e)
		}
		expected := []string{}
		if v != nil {
			expected = append(expected, v.LocalAgentID)
		}
		caps, enabled, e := s.OwnerAgentCapabilities(r.Context(), u.UserID, expected...)
		if e != nil {
			return localAgentError(e)
		}
		var ownerView any = v
		if enabled && v != nil {
			ownerView = struct {
				runner.Agent
				Capabilities *runner.AgentCapabilities `json:"capabilities"`
			}{*v, caps}
		}
		success(w, 200, map[string]any{"agent": ownerView, "serverTime": s.ServerTime()})
		return nil
	case "DELETE":
		if e := emptyAgentBody(r); e != nil {
			return e
		}
		if e := s.RevokeAgent(r.Context(), u.UserID, r.PathValue("localAgentId")); e != nil {
			return localAgentError(e)
		}
		w.WriteHeader(204)
		return nil
	case "POST":
		if r.ContentLength > 32*1024 {
			return apiError(413, "payload_too_large", "Pairing input too large.")
		}
		raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 32*1024))
		if e != nil {
			return apiError(413, "payload_too_large", "Pairing input too large.")
		}
		defer clear(raw)
		if e = execution.NonNullObject(raw); e != nil {
			return localAgentError(e)
		}
		var input execution.PairInput
		if e = execution.StrictJSON(raw, &input); e != nil {
			return localAgentError(e)
		}
		keys := r.Header.Values("Idempotency-Key")
		if len(keys) != 1 {
			return apiError(400, "invalid_request", "One Idempotency-Key is required.")
		}
		response, created, e := s.CreateAgent(r.Context(), u.UserID, keys[0], input)
		if e != nil {
			return localAgentError(e)
		}
		status := 200
		if created {
			status = 201
		}
		var result any = response
		capabilities, enabled, e := s.OwnerAgentCapabilities(r.Context(), u.UserID, response.Agent.LocalAgentID)
		if e != nil {
			return localAgentError(e)
		}
		if enabled {
			result = struct {
				Agent any     `json:"agent"`
				Token *string `json:"token"`
			}{struct {
				runner.Agent
				Capabilities *runner.AgentCapabilities `json:"capabilities"`
			}{response.Agent, capabilities}, response.Token}
		}
		success(w, status, result)
		return nil
	}
	return missing()
}
func (a *Server) agentProtocol(w http.ResponseWriter, r *http.Request) {
	fail := func(err error) { writeError(w, localAgentError(err)) }
	s := a.store.executions
	if s == nil {
		fail(unavailable())
		return
	}
	h := r.Header.Values("Authorization")
	if len(h) != 1 || !strings.HasPrefix(h[0], "Bearer ") || strings.Count(h[0], " ") != 1 {
		fail(apiError(401, "unauthenticated", "Valid agent credential required."))
		return
	}
	token := strings.TrimPrefix(h[0], "Bearer ")
	switch r.Pattern {
	case "POST /api/v1/local-agent/capabilities":
		if e := s.CheckAgentCredential(r.Context(), token); e != nil {
			fail(e)
			return
		}
		if r.Header.Get("Content-Encoding") != "" {
			fail(invalidConfiguration())
			return
		}
		media, _, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if e != nil || media != "application/json" {
			fail(invalidConfiguration())
			return
		}
		raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 1024))
		if e != nil {
			fail(apiError(413, "payload_too_large", "Capability declaration exceeds its limit."))
			return
		}
		var c runner.AgentCapabilities
		if e = runner.StrictJSON(raw, &c); e != nil {
			fail(e)
			return
		}
		result, e := s.DeclareAgentCapabilities(r.Context(), token, c)
		if e != nil {
			fail(e)
			return
		}
		success(w, 200, result)

	case "DELETE /api/v1/local-agent/session":
		if e := emptyAgentBody(r); e != nil {
			fail(e)
			return
		}
		agent, _, e := s.AgentSession(r.Context(), token)
		if e != nil {
			fail(e)
			return
		}
		if e = s.RevokeAuthenticatedAgent(r.Context(), token, agent.LocalAgentID); e != nil {
			fail(e)
			return
		}
		w.WriteHeader(204)
	case "GET /api/v1/local-agent/session":
		agent, serverTime, e := s.AgentSession(r.Context(), token)
		if e != nil {
			fail(e)
			return
		}
		success(w, 200, map[string]any{"agent": agent, "serverTime": serverTime})
	case "POST /api/v1/projects/{projectId}/local-agent/poll":
		if e := emptyAgentBody(r); e != nil {
			fail(e)
			return
		}
		response, e := s.PollAgent(r.Context(), token, r.PathValue("projectId"))
		if e != nil {
			fail(e)
			return
		}
		if response.Work != nil {
			defer clear(response.Work.ProtectedInputs.Values)
		}
		success(w, 200, response)
	case "POST /api/v1/projects/{projectId}/local-agent/jobs/{jobId}/result":
		if e := s.CheckAgentCredential(r.Context(), token); e != nil {
			fail(e)
			return
		}
		media, _, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if e != nil || media != "application/json" || r.Header.Get("Content-Encoding") != "" {
			fail(apiError(400, "invalid_request", "Use uncompressed JSON."))
			return
		}
		raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, execution.EvidenceLimit))
		if e != nil {
			fail(apiError(413, "payload_too_large", "Result too large."))
			return
		}
		defer clear(raw)
		var result runner.Result
		if e = execution.StrictJSON(raw, &result); e != nil {
			fail(e)
			return
		}
		receipt, e := s.UploadAgentResult(r.Context(), token, r.PathValue("projectId"), r.PathValue("jobId"), result)
		if e != nil {
			fail(e)
			return
		}
		success(w, 200, receipt)
	default:
		fail(missing())
	}
}
