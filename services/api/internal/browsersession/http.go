package browsersession

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	fail := func(status int, code string) {
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": code, "message": "Session request could not be completed.", "details": []any{}}})
	}
	handle := func(e error) {
		switch {
		case errors.Is(e, ErrForbidden):
			fail(403, "forbidden")
		case errors.Is(e, ErrEnded):
			fail(401, "unauthenticated")
		case errors.Is(e, ErrConflict):
			w.Header().Set("Retry-After", "1")
			fail(409, "session_changed")
		case errors.Is(e, ErrBusy):
			w.Header().Set("Retry-After", "1")
			fail(409, "session_busy")
		case errors.Is(e, ErrRate):
			w.Header().Set("Retry-After", "60")
			fail(429, "rate_limited")
		default:
			fail(503, "service_unavailable")
		}
	}
	if !s.Local && (s.OriginSecret == "" || !same(r.Header.Get("X-Kurier-Session-Origin"), s.OriginSecret)) {
		fail(403, "forbidden")
		return
	}
	if r.URL.RawQuery != "" || r.URL.EscapedPath() != r.URL.Path || !(r.URL.Path == Path || r.URL.Path == Path+"/token" || r.URL.Path == Path+"/bootstrap-cancellations") {
		fail(404, "not_found")
		return
	}
	if r.Header.Get("X-Kurier-Session") != "1" || r.Header.Get("Sec-Fetch-Mode") == "navigate" || (r.Header.Get("Sec-Fetch-Site") != "" && r.Header.Get("Sec-Fetch-Site") != "same-origin") || r.Header.Get("Origin") != "" && r.Header.Get("Origin") != s.Origin || r.Method != http.MethodGet && r.Header.Get("Origin") != s.Origin {
		fail(403, "forbidden")
		return
	}
	// Fixed-window source rate limiting also bounds invalid/cancellation requests.
	if e := s.Store.Rate(r.Context(), "source#"+hash(r.RemoteAddr), s.now().Unix()/60, 120); e != nil {
		handle(e)
		return
	}
	cookie, _ := r.Cookie(CookieName)
	proof := ""
	if cookie != nil {
		proof = cookie.Value
	}
	success := func(status int, data any) {
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	}
	setCookie := func(proof string, expiry int64) {
		age := int(expiry - s.now().Unix())
		if proof == "" || age <= 0 {
			age = -1
			proof = ""
		}
		http.SetCookie(w, &http.Cookie{Name: CookieName, Value: proof, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: age, Expires: time.Unix(expiry, 0).UTC()})
	}
	switch {
	case r.URL.Path == Path && r.Method == http.MethodGet:
		v, e := s.Metadata(r.Context(), proof)
		if e != nil {
			handle(e)
			return
		}
		success(200, v)
	case r.URL.Path == Path && r.Method == http.MethodPost:
		var in Bootstrap
		if !readJSON(w, r, &in) {
			fail(400, "invalid_request")
			return
		}
		access := r.Header.Get("Authorization")
		if !strings.HasPrefix(access, "Bearer ") {
			fail(401, "unauthenticated")
			return
		}
		v, e := s.Bootstrap(r.Context(), strings.TrimPrefix(access, "Bearer "), in)
		if e != nil {
			handle(e)
			return
		}
		expiry, _ := time.Parse(time.RFC3339, v.SessionExpiresAt)
		setCookie(in.SessionProof, expiry.Unix())
		success(201, v)
	case r.URL.Path == Path+"/token" && r.Method == http.MethodPost:
		var in struct{}
		if !readJSON(w, r, &in) {
			fail(400, "invalid_request")
			return
		}
		v, e := s.Token(r.Context(), proof, r.Header.Get("X-Kurier-CSRF"), r.Header.Get("X-Kurier-Session-Binding"))
		if e != nil {
			handle(e)
			return
		}
		success(200, v)
	case r.URL.Path == Path && r.Method == http.MethodDelete:
		raw, e := io.ReadAll(io.LimitReader(r.Body, 2))
		if e != nil || len(raw) != 0 {
			fail(400, "invalid_request")
			return
		}
		confirmed, e := s.Logout(r.Context(), proof, r.Header.Get("X-Kurier-CSRF"), r.Header.Get("X-Kurier-Session-Binding"))
		if e != nil {
			handle(e)
			return
		}
		setCookie("", 0)
		status := "pending"
		if confirmed {
			status = "confirmed"
		}
		success(200, map[string]any{"signedOut": true, "providerRevocation": status})
	case r.URL.Path == Path+"/bootstrap-cancellations" && r.Method == http.MethodPost:
		var in struct {
			AttemptID         string `json:"attemptId"`
			CancellationProof string `json:"cancellationProof"`
		}
		if !readJSON(w, r, &in) {
			fail(400, "invalid_request")
			return
		}
		if e := s.Cancel(r.Context(), in.AttemptID, in.CancellationProof); e != nil {
			handle(e)
			return
		}
		success(200, map[string]bool{"canceled": true})
	default:
		fail(405, "method_not_allowed")
	}
}

// Session DTOs are flat. Scan keys before decoding to reject duplicate fields,
// nested values and trailing content rather than accepting last-key-wins JSON.
func readJSON(w http.ResponseWriter, r *http.Request, out any) bool {
	media, _, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if e != nil || media != "application/json" {
		return false
	}
	raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 16*1024))
	if e != nil || !utf8.Valid(raw) {
		return false
	}
	d := json.NewDecoder(strings.NewReader(string(raw)))
	t, e := d.Token()
	if e != nil || t != json.Delim('{') {
		return false
	}
	seen := map[string]bool{}
	for d.More() {
		k, e := d.Token()
		name, ok := k.(string)
		if e != nil || !ok || seen[name] {
			return false
		}
		seen[name] = true
		v, e := d.Token()
		if e != nil {
			return false
		}
		if _, nested := v.(json.Delim); nested {
			return false
		}
	}
	if t, e = d.Token(); e != nil || t != json.Delim('}') {
		return false
	}
	if _, e = d.Token(); e != io.EOF {
		return false
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	return dec.Decode(out) == nil
}
