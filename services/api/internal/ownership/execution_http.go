package ownership

import (
	"errors"
	"github.com/VioletProto/Kurier/services/api/execution"
	"net/http"
	"net/url"
	"strconv"
)

func (st *Store) ConfigureExecutions(service *execution.Service) { st.executions = service }
func (a *Server) executionEndpoint(w http.ResponseWriter, r *http.Request, u User) error {
	if a.store.executions == nil {
		return unavailable()
	}
	service := a.store.executions
	project, executionID := r.PathValue("projectId"), r.PathValue("executionId")
	if _, _, _, err := a.store.requestContext(r.Context(), u.UserID, project); err != nil {
		return err
	}
	wrap := func(err error) error {
		var e *execution.Error
		if errors.As(err, &e) {
			return apiError(e.Status, e.Code, "Execution operation could not be completed. Refresh status before a deliberate retry.")
		}
		return err
	}
	if r.Method == "POST" {
		raw, err := readRequestJSON(w, r)
		if err != nil {
			return err
		}
		rerun := executionID != ""
		var options execution.Options
		var revision int64
		source := r.PathValue("requestId")
		if rerun {
			var input struct {
				AllowInsecureSecrets bool `json:"allowInsecureSecrets"`
			}
			if err = execution.NonNullObject(raw); err != nil {
				return wrap(err)
			}
			if err = execution.StrictJSON(raw, &input); err != nil {
				return wrap(err)
			}
			options.AllowInsecureSecrets = input.AllowInsecureSecrets
			source = executionID
		} else {
			revision, err = precondition(r)
			if err != nil {
				return err
			}
			options, err = execution.ParseOptions(raw)
			if err != nil {
				return wrap(err)
			}
		}
		keys := r.Header.Values("Idempotency-Key")
		if len(keys) != 1 {
			return apiError(400, "invalid_request", "One Idempotency-Key is required.")
		}
		view, err := service.Submit(r.Context(), u.UserID, project, source, keys[0], revision, options, rerun)
		if err != nil {
			return wrap(err)
		}
		w.Header().Set("Location", "/api/v1/projects/"+project+"/executions/"+view.ExecutionID)
		w.Header().Set("Retry-After", "10")
		success(w, 202, map[string]any{"execution": view})
		return nil
	}
	if executionID == "" {
		q, queryErr := url.ParseQuery(r.URL.RawQuery)
		if queryErr != nil {
			return apiError(400, "invalid_request", "Invalid history query.")
		}
		for k, v := range q {
			if len(v) != 1 || (k != "limit" && k != "cursor" && k != "requestId" && k != "status") {
				return apiError(400, "invalid_request", "Invalid history query.")
			}
		}
		limit := 25
		if q.Has("limit") {
			var err error
			limit, err = strconv.Atoi(q.Get("limit"))
			if err != nil {
				return apiError(400, "invalid_request", "Invalid history limit.")
			}
		}
		result, err := service.History(r.Context(), u.UserID, project, q.Get("requestId"), q.Get("status"), limit, q.Get("cursor"))
		if err != nil {
			return wrap(err)
		}
		success(w, 200, result)
		return nil
	}
	if r.Pattern == "GET /api/v1/projects/{projectId}/executions/{executionId}/evidence" {
		raw, err := service.Evidence(r.Context(), u.UserID, project, executionID)
		if err != nil {
			return wrap(err)
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(200)
		_, err = w.Write(raw)
		return err
	}
	view, err := service.Detail(r.Context(), u.UserID, project, executionID)
	if err != nil {
		return wrap(err)
	}
	if r.Pattern == "GET /api/v1/projects/{projectId}/executions/{executionId}/status" {
		view.Configuration = nil
		success(w, 200, map[string]any{"status": view})
	} else {
		success(w, 200, map[string]any{"execution": view})
	}
	return nil
}
