package ownership

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
)

// LocalCORS permits only the configured loopback frontend. It never grants
// cookie credentials or authenticates a request; the wrapped API verifies JWTs.
func LocalCORS(next http.Handler, origin string) (http.Handler, error) {
	u, err := url.Parse(origin)
	if err != nil || u.Scheme != "http" || u.User != nil || u.Port() == "" ||
		(u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1" && u.Hostname() != "::1") ||
		u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.String() != origin {
		return nil, errors.New("frontend origin must be an explicit HTTP loopback origin with port")
	}
	methods := map[string]bool{"GET": true, "POST": true, "PATCH": true, "DELETE": true}
	headers := map[string]bool{"authorization": true, "content-type": true, "if-match": true, "idempotency-key": true}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Origin")
		w.Header().Set("Cache-Control", "no-store")
		origins := r.Header.Values("Origin")
		if len(origins) == 0 {
			next.ServeHTTP(w, r)
			return
		}
		if len(origins) != 1 || origins[0] != origin {
			writeError(w, apiError(403, "forbidden", "Frontend origin is not allowed."))
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Expose-Headers", "ETag, Location, Retry-After")
		if r.Method == "OPTIONS" {
			w.Header().Add("Vary", "Access-Control-Request-Method")
			w.Header().Add("Vary", "Access-Control-Request-Headers")
			if !strings.HasPrefix(r.URL.Path, "/api/v1/") || !methods[r.Header.Get("Access-Control-Request-Method")] {
				writeError(w, missing())
				return
			}
			for _, field := range strings.Split(r.Header.Get("Access-Control-Request-Headers"), ",") {
				if field != "" && !headers[strings.ToLower(strings.TrimSpace(field))] {
					writeError(w, apiError(403, "forbidden", "Request header is not allowed."))
					return
				}
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, If-Match, Idempotency-Key")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	}), nil
}
