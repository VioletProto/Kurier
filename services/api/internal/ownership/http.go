package ownership

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type Server struct {
	store        *Store
	verifier     *Verifier
	cursorSecret []byte
	mux          *http.ServeMux
}

func NewServer(store *Store, verifier *Verifier, cursorSecret []byte) (*Server, error) {
	if store == nil || verifier == nil || len(cursorSecret) < 32 {
		return nil, errors.New("store, verifier and cursor signing secret required")
	}
	a := &Server{store: store, verifier: verifier, cursorSecret: append([]byte(nil), cursorSecret...), mux: http.NewServeMux()}
	a.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 200, map[string]string{"service": "api", "status": "ok"})
	})
	a.mux.HandleFunc("GET /api/v1/users/me", a.protected(a.me))
	a.mux.HandleFunc("POST /api/v1/projects", a.protected(a.create))
	a.mux.HandleFunc("GET /api/v1/projects", a.protected(a.list))
	a.mux.HandleFunc("GET /api/v1/projects/{projectId}", a.protected(a.detail))
	a.mux.HandleFunc("PATCH /api/v1/projects/{projectId}", a.protected(a.rename))
	a.mux.HandleFunc("DELETE /api/v1/projects/{projectId}", a.protected(a.delete))
	a.mux.HandleFunc("GET /api/v1/projects/{projectId}/deletion-operations/{operationId}", a.protected(a.operation))
	for _, route := range []struct {
		pattern string
		handler endpoint
	}{
		{"POST /api/v1/projects/{projectId}/requests", a.createRequest},
		{"GET /api/v1/projects/{projectId}/requests", a.listRequests},
		{"GET /api/v1/projects/{projectId}/requests/{requestId}", a.detailRequest},
		{"PATCH /api/v1/projects/{projectId}/requests/{requestId}", a.patchRequest},
		{"DELETE /api/v1/projects/{projectId}/requests/{requestId}", a.deleteRequest},
	} {
		a.mux.HandleFunc(route.pattern, a.protected(route.handler))
	}
	return a, nil
}
func (a *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	ctx, cancel := context.WithTimeout(r.Context(), 9*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	_, pattern := a.mux.Handler(r)
	if pattern == "" {
		writeError(w, missing())
		return
	}
	a.mux.ServeHTTP(w, r)
}

type endpoint func(http.ResponseWriter, *http.Request, User) error

func (a *Server) protected(next endpoint) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		headers := r.Header.Values("Authorization")
		if len(headers) != 1 || !strings.HasPrefix(headers[0], "Bearer ") || strings.Count(headers[0], " ") != 1 {
			writeError(w, apiError(401, "unauthenticated", "Valid access token required."))
			return
		}
		identity, err := a.verifier.Verify(r.Context(), strings.TrimPrefix(headers[0], "Bearer "))
		if err != nil {
			writeError(w, err)
			return
		}
		u, err := a.store.ResolveUser(r.Context(), identity)
		if err == nil {
			err = next(w, r, u)
		}
		if err != nil {
			writeError(w, err)
		}
	}
}
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(data)
}
func success(w http.ResponseWriter, status int, data any) {
	writeJSON(w, status, map[string]any{"data": data})
}
func writeError(w http.ResponseWriter, err error) {
	var e *APIError
	if !errors.As(err, &e) {
		e = apiError(500, "internal_error", "Unexpected service failure.")
	}
	copyError := *e
	copyError.RequestID = newID()
	if e.Status == 503 || e.Status == 429 {
		w.Header().Set("Retry-After", "2")
	}
	writeJSON(w, e.Status, map[string]any{"error": copyError})
}
func (a *Server) me(w http.ResponseWriter, _ *http.Request, u User) error {
	success(w, 200, map[string]any{"user": u})
	return nil
}
func (a *Server) create(w http.ResponseWriter, r *http.Request, u User) error {
	name, err := readName(w, r)
	if err != nil {
		return err
	}
	p, err := a.store.CreateProject(r.Context(), u.UserID, name)
	if err != nil {
		return err
	}
	w.Header().Set("Location", "/api/v1/projects/"+p.ProjectID)
	projectResponse(w, 201, p)
	return nil
}
func projectResponse(w http.ResponseWriter, status int, p Project) {
	w.Header().Set("ETag", strconv.Quote(strconv.FormatInt(p.Version, 10)))
	success(w, status, map[string]any{"project": p})
}
func (a *Server) detail(w http.ResponseWriter, r *http.Request, u User) error {
	p, err := a.store.GetProject(r.Context(), u.UserID, r.PathValue("projectId"))
	if err != nil {
		return err
	}
	projectResponse(w, 200, p)
	return nil
}

var tagPattern = regexp.MustCompile(`^"(0|[1-9][0-9]*)"$`)

func precondition(r *http.Request) (int64, error) {
	values := r.Header.Values("If-Match")
	if len(values) == 0 {
		return 0, apiError(428, "precondition_required", "If-Match is required.")
	}
	if len(values) != 1 || !tagPattern.MatchString(values[0]) {
		return 0, apiError(400, "invalid_request", "Expected one quoted decimal If-Match.")
	}
	v, err := strconv.ParseInt(strings.Trim(values[0], "\""), 10, 64)
	if err != nil || v == 9223372036854775807 {
		return 0, apiError(400, "invalid_request", "Invalid project version.")
	}
	return v, nil
}
func (a *Server) rename(w http.ResponseWriter, r *http.Request, u User) error {
	// Authorize before returning precondition/field information.
	if _, err := a.store.GetProject(r.Context(), u.UserID, r.PathValue("projectId")); err != nil {
		return err
	}
	v, err := precondition(r)
	if err != nil {
		return err
	}
	name, err := readName(w, r)
	if err != nil {
		return err
	}
	p, err := a.store.RenameProject(r.Context(), u.UserID, r.PathValue("projectId"), name, v)
	if err != nil {
		return err
	}
	projectResponse(w, 200, p)
	return nil
}
func (a *Server) delete(w http.ResponseWriter, r *http.Request, u User) error {
	if _, err := a.store.owned(r.Context(), u.UserID, r.PathValue("projectId"), true); err != nil {
		return err
	}
	v, err := precondition(r)
	if err != nil {
		return err
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 8192))
	if err != nil {
		var large *http.MaxBytesError
		if errors.As(err, &large) {
			return apiError(413, "payload_too_large", "Request body exceeds 8 KiB.")
		}
		return apiError(400, "invalid_request", "Invalid request body.")
	}
	if len(body) != 0 {
		return apiError(400, "invalid_request", "DELETE does not accept a body.")
	}
	op, err := a.store.DeleteProject(r.Context(), u.UserID, r.PathValue("projectId"), v)
	if err != nil {
		return err
	}
	w.Header().Set("Location", "/api/v1/projects/"+op.ProjectID+"/deletion-operations/"+op.OperationID)
	success(w, 202, map[string]any{"deletionOperation": op})
	return nil
}
func (a *Server) operation(w http.ResponseWriter, r *http.Request, u User) error {
	op, err := a.store.GetDeletion(r.Context(), u.UserID, r.PathValue("projectId"), r.PathValue("operationId"))
	if err != nil {
		return err
	}
	success(w, 200, map[string]any{"deletionOperation": op})
	return nil
}
func (a *Server) list(w http.ResponseWriter, r *http.Request, u User) error {
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
	if values, ok := query["limit"]; ok {
		limit, err = strconv.Atoi(values[0])
		if err != nil || limit < 1 || limit > 100 {
			return apiError(400, "invalid_request", "Limit must be between 1 and 100.")
		}
	}
	if values, ok := query["cursor"]; ok && values[0] == "" {
		return apiError(400, "invalid_request", "Invalid or expired cursor.")
	}
	items, err := a.store.ListProjects(r.Context(), u.UserID, limit, query.Get("cursor"), a.cursorSecret)
	if err != nil {
		return err
	}
	success(w, 200, items)
	return nil
}
func readName(w http.ResponseWriter, r *http.Request) (string, error) {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		return "", apiError(415, "invalid_request", "Use application/json.")
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 8192))
	if err != nil {
		var large *http.MaxBytesError
		if errors.As(err, &large) {
			return "", apiError(413, "payload_too_large", "Request body exceeds 8 KiB.")
		}
		return "", apiError(400, "invalid_request", "Invalid JSON body.")
	}
	bad := apiError(400, "invalid_request", "Invalid JSON body.")
	invalid := apiError(400, "validation_failed", "Provide only a name of 1–100 characters without control characters.")
	if !utf8.Valid(body) {
		return "", bad
	}
	d := json.NewDecoder(bytes.NewReader(body))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return "", bad
	}
	seen := false
	var name *string
	for d.More() {
		field, e := d.Token()
		if e != nil {
			return "", bad
		}
		if field != "name" {
			return "", invalid
		}
		if seen {
			return "", bad
		}
		seen = true
		if d.Decode(&name) != nil {
			return "", invalid
		}
	}
	if token, err = d.Token(); err != nil || token != json.Delim('}') {
		return "", bad
	}
	if _, err = d.Token(); err != io.EOF {
		return "", bad
	}
	if !seen || name == nil {
		return "", invalid
	}
	for _, c := range *name {
		if unicode.IsControl(c) {
			return "", invalid
		}
	}
	trimmed := strings.TrimSpace(*name)
	if utf8.RuneCountInString(trimmed) < 1 || utf8.RuneCountInString(trimmed) > 100 {
		return "", invalid
	}
	return trimmed, nil
}
