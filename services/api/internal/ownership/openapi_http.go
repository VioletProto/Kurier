package ownership

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"

	runner "github.com/VioletProto/Kurier/services/request-runtime"
	oapi "github.com/VioletProto/Kurier/services/request-runtime/openapi"
)

func readOpenAPIJSON(w http.ResponseWriter, r *http.Request, limit int64) ([]byte, error) {
	media, _, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if e != nil || media != "application/json" {
		return nil, apiError(415, "invalid_request", "Use application/json.")
	}
	raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, limit))
	if e != nil {
		var large *http.MaxBytesError
		if errors.As(e, &large) {
			return nil, apiError(413, "payload_too_large", "OpenAPI upload exceeds the transport limit.")
		}
		return nil, invalidConfiguration()
	}
	return raw, nil
}

// Decode exact wire keys as well as typed values; encoding/json's case-folded
// struct field matching must not create hidden duplicate semantic fields.
func decodeOpenAPIInput(raw []byte, target any, required, optional []string) error {
	var fields map[string]json.RawMessage
	if runner.StrictJSON(raw, &fields) != nil || fields == nil {
		return invalidConfiguration()
	}
	allowed := map[string]bool{}
	for _, key := range required {
		allowed[key] = true
		if len(fields[key]) == 0 || string(fields[key]) == "null" {
			return invalidConfiguration()
		}
	}
	for _, key := range optional {
		allowed[key] = true
	}
	for key := range fields {
		if !allowed[key] {
			return invalidConfiguration()
		}
	}
	if choices, ok := fields["choices"]; ok {
		if _, e := decodeOpenAPIChoices(choices); e != nil {
			return invalidConfiguration()
		}
	}
	if runner.StrictJSON(raw, target) != nil {
		return invalidConfiguration()
	}
	return nil
}
func openapiResponse(w http.ResponseWriter, status int, value OpenAPIImport, offset int) error {
	if offset < 0 || offset > len(value.Operations) {
		return invalidConfiguration()
	}
	all := value.Operations
	value.Operations = []OpenAPICandidate{}
	value.NextOperationOffset = nil
	for n := offset; n < len(all) && len(value.Operations) < 20; n++ {
		value.Operations = append(value.Operations, all[n])
		raw, e := json.Marshal(map[string]any{"data": map[string]any{"import": value}})
		if e != nil {
			return unavailable()
		}
		if len(raw) > 2*1024*1024 {
			value.Operations = value.Operations[:len(value.Operations)-1]
			break
		}
	}
	next := offset + len(value.Operations)
	if next < len(all) {
		if len(value.Operations) == 0 {
			return unavailable()
		}
		value.NextOperationOffset = &next
	}
	w.Header().Set("ETag", strconv.Quote(strconv.FormatInt(value.Revision, 10)))
	success(w, status, map[string]any{"import": value})
	return nil
}

func (a *Server) openapiEndpoint(w http.ResponseWriter, r *http.Request, u User) error {
	project, id := r.PathValue("projectId"), r.PathValue("importId")
	if _, _, _, e := a.store.openapiContext(r.Context(), u.UserID, project); e != nil {
		return e
	}
	q, e := url.ParseQuery(r.URL.RawQuery)
	if e != nil {
		return invalidConfiguration()
	}
	if r.Method == "GET" && id == "" {
		for k, v := range q {
			if k != "cursor" || len(v) != 1 {
				return invalidConfiguration()
			}
		}
		items, next, e := a.store.listOpenAPI(r.Context(), u.UserID, project, q.Get("cursor"), a.cursorSecret)
		if e != nil {
			return e
		}
		success(w, 200, map[string]any{"items": items, "nextCursor": next})
		return nil
	}
	if r.Method == "GET" {
		offset := 0
		for k, v := range q {
			if k != "operationOffset" || len(v) != 1 {
				return invalidConfiguration()
			}
		}
		if q.Has("operationOffset") {
			offset, e = strconv.Atoi(q.Get("operationOffset"))
			if e != nil || offset < 0 {
				return invalidConfiguration()
			}
		}

		out, e := a.store.openapiDetail(r.Context(), u.UserID, project, id)
		if e != nil {
			return e
		}
		return openapiResponse(w, 200, out, offset)
	}
	if len(q) != 0 {
		return invalidConfiguration()
	}
	if r.Method == "POST" && id == "" {
		raw, e := readOpenAPIJSON(w, r, 5*1024*1024)
		if e != nil {
			return e
		}
		var input struct {
			Format                string   `json:"format"`
			Source                string   `json:"source"`
			PublicDataConfirmed   bool     `json:"publicDataConfirmed"`
			SensitiveValues       []string `json:"sensitiveValues"`
			SensitiveJSONPointers []string `json:"sensitiveJSONPointers"`
		}
		if decodeOpenAPIInput(raw, &input, []string{"format", "source", "publicDataConfirmed"}, []string{"sensitiveValues", "sensitiveJSONPointers"}) != nil || !input.PublicDataConfirmed {
			return invalidConfiguration()
		}
		if len(input.Source) > 2*1024*1024 {
			return apiError(413, "payload_too_large", "OpenAPI source exceeds 2 MiB.")
		}
		out, e := a.store.createOpenAPI(r.Context(), u.UserID, project, input.Format, input.Source, oapi.Policy{SensitiveValues: input.SensitiveValues, SensitiveJSONPointers: input.SensitiveJSONPointers})
		if e != nil {
			return e
		}
		w.Header().Set("Location", "/api/v1/projects/"+project+"/openapi-imports/"+out.ImportID)
		return openapiResponse(w, 201, out, 0)
	}
	expected, e := precondition(r)
	if e != nil {
		return e
	}
	if expected > 9007199254740991 {
		return invalidConfiguration()
	}
	if r.Method == "DELETE" {
		if r.ContentLength != 0 || r.TransferEncoding != nil {
			return invalidConfiguration()
		}
		if e = a.store.deleteOpenAPI(r.Context(), u.UserID, project, id, expected); e != nil {
			return e
		}
		w.WriteHeader(204)
		return nil
	}
	if r.Method == "PATCH" {
		raw, e := readOpenAPIJSON(w, r, 128*1024)
		if e != nil {
			return e
		}
		var input struct {
			OperationID         string       `json:"operationId"`
			Choices             oapi.Choices `json:"choices"`
			PublicDataConfirmed bool         `json:"publicDataConfirmed"`
		}
		if decodeOpenAPIInput(raw, &input, []string{"operationId", "choices", "publicDataConfirmed"}, nil) != nil || !input.PublicDataConfirmed || input.OperationID == "" {
			return invalidConfiguration()
		}
		out, e := a.store.updateOpenAPICandidate(r.Context(), u.UserID, project, id, input.OperationID, input.Choices, expected)
		if e != nil {
			return e
		}
		return openapiResponse(w, 200, out, 0)
	}
	return invalidConfiguration()
}
func (a *Server) openapiPublish(w http.ResponseWriter, r *http.Request, u User) error {
	project, id := r.PathValue("projectId"), r.PathValue("importId")
	if _, _, _, e := a.store.openapiContext(r.Context(), u.UserID, project); e != nil {
		return e
	}
	if r.URL.RawQuery != "" {
		return invalidConfiguration()
	}
	expected, e := precondition(r)
	if e != nil {
		return e
	}
	raw, e := readOpenAPIJSON(w, r, 1024)
	if e != nil {
		return e
	}
	var input struct{}
	if runner.StrictJSON(raw, &input) != nil {
		return invalidConfiguration()
	}
	out, e := a.store.publishOpenAPI(r.Context(), u.UserID, project, id, expected)
	if e != nil {
		return e
	}
	return openapiResponse(w, 200, out, 0)
}
func (a *Server) openapiExport(w http.ResponseWriter, r *http.Request, u User) error {
	q, e := url.ParseQuery(r.URL.RawQuery)
	if e != nil || len(q) != 1 || len(q["format"]) != 1 || q.Get("format") != "json" && q.Get("format") != "yaml" {
		return invalidConfiguration()
	}
	format := q.Get("format")
	raw, e := a.store.openapiSource(r.Context(), u.UserID, r.PathValue("projectId"), r.PathValue("importId"), format)
	if e != nil {
		return e
	}
	content := "application/json"
	if format == "yaml" {
		content = "application/yaml"
	}
	w.Header().Set("Content-Type", content)
	w.Header().Set("Content-Disposition", "attachment; filename=\"kurier-sanitized-openapi."+format+"\"")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(200)
	_, _ = w.Write(raw)
	return nil
}
