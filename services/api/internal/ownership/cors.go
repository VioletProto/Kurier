package ownership

import (
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// LocalCORS preserves the local-only adapter used by development fixtures.
func LocalCORS(next http.Handler, origin string) (http.Handler, error) {
	return FrontendCORS(next, origin, "")
}

// FrontendCORS adds one explicit hosted CloudFront origin to deliberate local access.
// It never grants cookie credentials or authenticates a request.
func FrontendCORS(next http.Handler, local, hosted string) (http.Handler, error) {
	// Local development remains an explicit loopback origin.
	u, err := url.Parse(local)
	if err != nil || u.Scheme != "http" || u.User != nil || u.Port() == "" ||
		(u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1" && u.Hostname() != "::1") ||
		u.Path != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.String() != local {
		return nil, errors.New("frontend origin must be an explicit HTTP loopback origin with port")
	}
	allowed := map[string]bool{local: true}
	if hosted != "" {
		u, err := url.Parse(hosted)
		if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" ||
			!hostedFrontendHost.MatchString(u.Host) || u.Path != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.String() != hosted {
			return nil, errors.New("hosted frontend must be one exact HTTPS CloudFront origin")
		}
		allowed[hosted] = true
	}
	methods := map[string]bool{"GET": true, "POST": true, "PATCH": true, "PUT": true, "DELETE": true}
	headers := map[string]bool{"authorization": true, "content-type": true, "if-match": true, "idempotency-key": true}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Origin")
		w.Header().Set("Cache-Control", "no-store")
		origins := r.Header.Values("Origin")
		if len(origins) == 0 {
			next.ServeHTTP(w, r)
			return
		}
		if len(origins) != 1 || !allowed[origins[0]] {
			writeError(w, apiError(403, "forbidden", "Frontend origin is not allowed."))
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", origins[0])
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
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, If-Match, Idempotency-Key")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	}), nil
}

var hostedFrontendHost = regexp.MustCompile(`^d[a-z0-9]+\.cloudfront\.net$`)
