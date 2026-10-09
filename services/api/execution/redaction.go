package execution

import (
	"encoding/base64"
	"encoding/json"
	"mime"
	"net/url"
	"sort"
	"strings"
	"unicode/utf8"
)

func credentialName(name string) bool {
	v := strings.ToLower(strings.NewReplacer("-", "", "_", "", ".", "", " ", "").Replace(name))
	for _, p := range []string{"authorization", "cookie", "apikey", "token", "secret", "password", "passwd", "credential", "privatekey", "signature", "sessionid"} {
		if strings.Contains(v, p) {
			return true
		}
	}
	return v == "key" || v == "auth" || v == "pwd"
}
func headerName(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("!#$%&'*+-.^_`|~", r)) {
			return false
		}
	}
	return true
}

type sanitizer struct {
	values   []string
	replacer *strings.Replacer
}

func newSanitizer(values map[string]string) *sanitizer {
	unique := map[string]bool{}
	var collect func(any)
	collect = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for _, v := range x {
				collect(v)
			}
		case []any:
			for _, v := range x {
				collect(v)
			}
		case string:
			if x != "" {
				unique[x] = true
			}
		case json.Number:
			unique[x.String()] = true
		case bool:
			if x {
				unique["true"] = true
			} else {
				unique["false"] = true
			}
		case nil:
			unique["null"] = true
		}
	}
	for _, v := range values {
		if v == "" {
			continue
		}
		unique[v] = true
		var parsed any
		if StrictJSON([]byte(v), &parsed) == nil {
			collect(parsed)
		}
		if strings.HasPrefix(strings.ToLower(v), "bearer ") {
			unique[v[7:]] = true
		}
		for _, c := range strings.Split(v, ";") {
			if _, value, ok := strings.Cut(strings.TrimSpace(c), "="); ok && value != "" {
				unique[value] = true
			}
		}
	}
	original := []string{}
	for v := range unique {
		original = append(original, v)
	}
	for _, v := range original {
		raw, _ := json.Marshal(v)
		if len(raw) > 2 {
			unique[string(raw[1:len(raw)-1])] = true
		}
		unique[url.QueryEscape(v)] = true
		unique[url.PathEscape(v)] = true
		unique[base64.StdEncoding.EncodeToString([]byte(v))] = true
		unique[base64.RawURLEncoding.EncodeToString([]byte(v))] = true
		unique[base64.URLEncoding.EncodeToString([]byte(v))] = true
		unique[base64.RawStdEncoding.EncodeToString([]byte(v))] = true
	}
	out := &sanitizer{}
	for v := range unique {
		if v != "" {
			out.values = append(out.values, v)
		}
	}
	sort.Slice(out.values, func(i, j int) bool { return len(out.values[i]) > len(out.values[j]) })
	pairs := []string{}
	for _, v := range out.values {
		pairs = append(pairs, v, "[REDACTED]")
	}
	out.replacer = strings.NewReplacer(pairs...)
	return out
}
func (s *sanitizer) text(v string) string {
	return s.replacer.Replace(v)
}
func (s *sanitizer) json(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, value := range x {
			if credentialName(k) {
				out[s.text(k)] = "[REDACTED]"
			} else {
				out[s.text(k)] = s.json(value)
			}
		}
		return out
	case []any:
		for i, v := range x {
			x[i] = s.json(v)
		}
		return x
	case string:
		return s.text(x)
	case json.Number:
		if s.text(x.String()) != x.String() {
			return "[REDACTED]"
		}
	case bool:
		raw, _ := json.Marshal(x)
		if s.text(string(raw)) != string(raw) {
			return "[REDACTED]"
		}
	case nil:
		if s.text("null") != "null" {
			return "[REDACTED]"
		}
	}
	return v
}
func (s *sanitizer) body(raw []byte, contentType string, policy RedactionPolicy) Body {
	if len(raw) == 0 {
		return emptyBody()
	}
	if policy.OmitBody {
		return omitted("policy_omit_body")
	}
	media, params, e := mime.ParseMediaType(contentType)
	if e != nil {
		return omitted("unsupported_content")
	}
	if charset := params["charset"]; charset != "" && !strings.EqualFold(charset, "utf-8") {
		return omitted("unsupported_charset")
	}
	if !utf8.Valid(raw) {
		return omitted("invalid_utf8")
	}
	if media == "application/json" || strings.HasPrefix(media, "application/") && strings.HasSuffix(media, "+json") {
		var v any
		if StrictJSON(raw, &v) != nil {
			return omitted("invalid_json")
		}
		for _, p := range policy.JSONPointers {
			v, e = setPointer(v, p, "[REDACTED]")
			if e != nil {
				return omitted("redaction_path_unavailable")
			}
		}
		b, e := json.Marshal(s.json(v))
		if e != nil {
			return omitted("sanitization_failed")
		}
		text := string(b)
		return Body{Kind: "json", Text: &text}
	}
	if media == "text/plain" {
		if len(policy.JSONPointers) > 0 {
			return omitted("redaction_path_unavailable")
		}
		text := s.text(string(raw))
		return Body{Kind: "text", Text: &text}
	}
	return omitted("unsupported_content")
}

func (s *sanitizer) url(source *url.URL) string {
	copy := *source
	q, err := url.ParseQuery(copy.RawQuery)
	if err != nil {
		copy.RawQuery = ""
		return s.text(copy.String())
	}
	for name, values := range q {
		for i, value := range values {
			if credentialName(name) {
				values[i] = "[REDACTED]"
			} else {
				values[i] = s.text(value)
			}
		}
		q[name] = values
	}
	copy.RawQuery = q.Encode()
	return s.text(copy.String())
}
