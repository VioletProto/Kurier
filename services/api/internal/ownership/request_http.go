package ownership

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
)

func readRequestJSON(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		return nil, apiError(415, "invalid_request", "Use application/json.")
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 128*1024))
	if err != nil {
		var large *http.MaxBytesError
		if errors.As(err, &large) {
			return nil, apiError(413, "payload_too_large", "Transport JSON exceeds 128 KiB.")
		}
		return nil, apiError(400, "invalid_request", "Invalid JSON body.")
	}
	return body, nil
}
func requestResponse(w http.ResponseWriter, status int, r SavedRequest) {
	w.Header().Set("ETag", strconv.Quote(strconv.FormatInt(r.Revision, 10)))
	success(w, status, map[string]any{"request": r})
}
func (a *Server) createRequest(w http.ResponseWriter, r *http.Request, u User) error {
	projectID := r.PathValue("projectId")
	if _, _, _, err := a.store.requestContext(r.Context(), u.UserID, projectID); err != nil {
		return err
	}
	raw, err := readRequestJSON(w, r)
	if err != nil {
		return err
	}
	c, err := mergeConfiguration(raw, nil)
	if err != nil {
		return err
	}
	saved, err := a.store.CreateRequest(r.Context(), u.UserID, projectID, c)
	if err != nil {
		return err
	}
	w.Header().Set("Location", "/api/v1/projects/"+projectID+"/requests/"+saved.RequestID)
	requestResponse(w, 201, saved)
	return nil
}
func (a *Server) detailRequest(w http.ResponseWriter, r *http.Request, u User) error {
	saved, err := a.store.GetRequest(r.Context(), u.UserID, r.PathValue("projectId"), r.PathValue("requestId"))
	if err != nil {
		return err
	}
	requestResponse(w, 200, saved)
	return nil
}
func (a *Server) patchRequest(w http.ResponseWriter, r *http.Request, u User) error {
	if _, err := a.store.GetRequest(r.Context(), u.UserID, r.PathValue("projectId"), r.PathValue("requestId")); err != nil {
		return err
	}
	v, err := precondition(r)
	if err != nil {
		return err
	}
	raw, err := readRequestJSON(w, r)
	if err != nil {
		return err
	}
	saved, err := a.store.PatchRequest(r.Context(), u.UserID, r.PathValue("projectId"), r.PathValue("requestId"), raw, v)
	if err != nil {
		return err
	}
	requestResponse(w, 200, saved)
	return nil
}
func (a *Server) deleteRequest(w http.ResponseWriter, r *http.Request, u User) error {
	if _, err := a.store.GetRequest(r.Context(), u.UserID, r.PathValue("projectId"), r.PathValue("requestId")); err != nil {
		return err
	}
	v, err := precondition(r)
	if err != nil {
		return err
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1))
	if err != nil || len(body) != 0 {
		return apiError(400, "invalid_request", "DELETE does not accept a body.")
	}
	if err = a.store.DeleteRequest(r.Context(), u.UserID, r.PathValue("projectId"), r.PathValue("requestId"), v); err != nil {
		return err
	}
	w.WriteHeader(204)
	return nil
}
func (a *Server) listRequests(w http.ResponseWriter, r *http.Request, u User) error {
	if _, _, _, err := a.store.requestContext(r.Context(), u.UserID, r.PathValue("projectId")); err != nil {
		return err
	}
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return apiError(400, "invalid_request", "Invalid pagination parameters.")
	}
	for k, v := range query {
		if (k != "limit" && k != "cursor") || len(v) != 1 {
			return apiError(400, "invalid_request", "Invalid pagination parameters.")
		}
	}
	limit := 25
	if v, ok := query["limit"]; ok {
		limit, err = strconv.Atoi(v[0])
		if err != nil || limit < 1 || limit > 100 {
			return apiError(400, "invalid_request", "Limit must be between 1 and 100.")
		}
	}
	if v, ok := query["cursor"]; ok && v[0] == "" {
		return apiError(400, "invalid_request", "Invalid cursor.")
	}
	page, err := a.store.ListRequests(r.Context(), u.UserID, r.PathValue("projectId"), limit, query.Get("cursor"), a.cursorSecret)
	if err != nil {
		return err
	}
	success(w, 200, page)
	return nil
}
