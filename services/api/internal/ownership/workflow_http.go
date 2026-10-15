package ownership

import (
	"encoding/json"
	"errors"
	"github.com/VioletProto/Kurier/services/api/execution"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func (a *Server) workflowEndpoint(w http.ResponseWriter, r *http.Request, u User) error {
	if a.store.executions == nil {
		return unavailable()
	}
	service := a.store.executions
	project, wf, run := r.PathValue("projectId"), r.PathValue("workflowId"), r.PathValue("workflowRunId")
	if _, _, _, e := a.store.requestContext(r.Context(), u.UserID, project); e != nil {
		return e
	}
	wrap := func(err error) error {
		var e *execution.Error
		if errors.As(err, &e) {
			return apiError(e.Status, e.Code, "Workflow operation could not be completed. Refresh before a deliberate retry.")
		}
		return err
	}
	runRoute := run != "" || strings.Contains(r.Pattern, "workflow-runs") || strings.HasSuffix(r.Pattern, "/runs")
	if r.Method == "POST" && runRoute {
		raw, e := readRequestJSON(w, r)
		if e != nil {
			return e
		}
		var options execution.RunOptions
		if execution.NonNullObject(raw) != nil || execution.StrictJSON(raw, &options) != nil {
			return apiError(400, "invalid_request", "Invalid workflow options.")
		}
		rev := int64(0)
		rerun := run != ""
		source := wf
		if rerun {
			source = run
		} else {
			rev, e = precondition(r)
			if e != nil {
				return e
			}
		}
		keys := r.Header.Values("Idempotency-Key")
		if len(keys) != 1 {
			return apiError(400, "invalid_request", "One Idempotency-Key is required.")
		}
		v, e := service.SubmitWorkflow(r.Context(), u.UserID, project, source, keys[0], rev, options, rerun)
		if e != nil {
			return wrap(e)
		}
		w.Header().Set("Location", "/api/v1/projects/"+project+"/workflow-runs/"+v.WorkflowRunID)
		w.Header().Set("Retry-After", "10")
		success(w, 202, map[string]any{"run": v, "serverTime": timestamp(a.store.now())})
		return nil
	}
	if r.Method == "POST" || r.Method == "PATCH" {
		raw, e := readRequestJSON(w, r)
		if e != nil {
			return e
		}
		if execution.NonNullObject(raw) != nil {
			return apiError(400, "invalid_request", "Invalid workflow definition.")
		}
		rev := int64(0)
		var d execution.WorkflowDefinition
		if r.Method == "PATCH" {
			rev, e = precondition(r)
			if e != nil {
				return e
			}
			old, e := service.WorkflowDetail(r.Context(), u.UserID, project, wf)
			if e != nil {
				return wrap(e)
			}
			d = old.WorkflowDefinition
		}
		var fields map[string]json.RawMessage
		if execution.StrictJSON(raw, &fields) != nil {
			return apiError(400, "invalid_request", "Invalid workflow definition.")
		}
		if len(fields) == 0 {
			return apiError(400, "invalid_request", "Empty workflow update.")
		}
		for k := range fields {
			if k != "name" && k != "steps" {
				return apiError(400, "invalid_request", "Unknown workflow field.")
			}
		}
		if execution.StrictJSON(raw, &d) != nil {
			return apiError(400, "invalid_request", "Invalid workflow definition.")
		}
		v, e := service.SaveWorkflow(r.Context(), u.UserID, project, wf, rev, d, r.Method == "POST")
		if e != nil {
			return wrap(e)
		}
		w.Header().Set("ETag", strconv.Quote(strconv.FormatInt(v.Revision, 10)))
		status := 200
		if r.Method == "POST" {
			status = 201
			w.Header().Set("Location", "/api/v1/projects/"+project+"/workflows/"+v.WorkflowID)
		}
		success(w, status, map[string]any{"workflow": v})
		return nil
	}
	if r.Method == "DELETE" {
		rev, e := precondition(r)
		if e != nil {
			return e
		}
		body, e := io.ReadAll(io.LimitReader(r.Body, 1))
		if e != nil || len(body) != 0 {
			return apiError(400, "invalid_request", "Empty body required.")
		}
		if e = service.DeleteWorkflow(r.Context(), u.UserID, project, wf, rev); e != nil {
			return wrap(e)
		}
		w.WriteHeader(204)
		return nil
	}
	if run != "" {
		v, e := service.RunDetail(r.Context(), u.UserID, project, run)
		if e != nil {
			return wrap(e)
		}
		success(w, 200, map[string]any{"run": v, "serverTime": timestamp(a.store.now())})
		return nil
	}
	if wf != "" {
		v, e := service.WorkflowDetail(r.Context(), u.UserID, project, wf)
		if e != nil {
			return wrap(e)
		}
		w.Header().Set("ETag", strconv.Quote(strconv.FormatInt(v.Revision, 10)))
		success(w, 200, map[string]any{"workflow": v})
		return nil
	}
	q, e := url.ParseQuery(r.URL.RawQuery)
	if e != nil {
		return apiError(400, "invalid_request", "Invalid list query.")
	}
	for k, v := range q {
		if len(v) != 1 || k != "limit" && k != "cursor" && !(runRoute && k == "workflowId") {
			return apiError(400, "invalid_request", "Invalid list query.")
		}
	}
	limit := 25
	if q.Has("limit") {
		limit, e = strconv.Atoi(q.Get("limit"))
		if e != nil {
			return apiError(400, "invalid_request", "Invalid limit.")
		}
	}
	if runRoute {
		v, e := service.Runs(r.Context(), u.UserID, project, q.Get("workflowId"), limit, q.Get("cursor"))
		if e != nil {
			return wrap(e)
		}
		success(w, 200, v)
	} else {
		v, e := service.Workflows(r.Context(), u.UserID, project, limit, q.Get("cursor"))
		if e != nil {
			return wrap(e)
		}
		success(w, 200, v)
	}
	return nil
}
