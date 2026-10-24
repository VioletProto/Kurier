package execution

import (
	"bytes"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
)

type templateScalar struct {
	Raw       json.RawMessage
	Protected bool
	Enabled   bool
}

func templateCompact(v any) []byte {
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	_ = e.Encode(v)
	return bytes.TrimSuffix(b.Bytes(), []byte{'\n'})
}

func parseScalar(raw []byte) (any, error) {
	if len(raw) > 2048 {
		return nil, failure(400, "input_limit_exceeded")
	}
	var v any
	if StrictJSON(raw, &v) != nil {
		return nil, failure(400, "template_invalid")
	}
	switch x := v.(type) {
	case nil, string, bool:
	case json.Number:
		if _, err := ParseAssertionDecimal(string(x)); err != nil {
			return nil, failure(400, "template_invalid")
		}
	default:
		return nil, failure(400, "variable_type")
	}
	return v, nil
}

type part struct{ literal, name string }

// A bounded single pass; values are never parsed as templates recursively.
func scanTemplate(text string) ([]part, error) {
	if len(text) > 65536 {
		return nil, failure(400, "input_limit_exceeded")
	}
	parts := []part{}
	refs := 0
	for len(text) > 0 {
		i := strings.Index(text, "{{")
		if i < 0 {
			if strings.Contains(text, "}}") {
				return nil, failure(400, "template_invalid")
			}
			parts = append(parts, part{literal: text})
			break
		}
		if strings.Contains(text[:i], "}}") {
			return nil, failure(400, "template_invalid")
		}
		parts = append(parts, part{literal: text[:i]})
		text = text[i+2:]
		end := strings.Index(text, "}}")
		if end < 0 || !variableName.MatchString(text[:end]) {
			return nil, failure(400, "template_invalid")
		}
		refs++
		if refs > 32 {
			return nil, failure(400, "input_limit_exceeded")
		}
		parts = append(parts, part{name: text[:end]})
		text = text[end+2:]
	}
	return parts, nil
}

func renderTemplate(text, location string, vars map[string]templateScalar) (any, bool, error) {
	parts, err := scanTemplate(text)
	if err != nil {
		return nil, false, err
	}
	sole := len(parts) == 2 && parts[0].literal == "" && parts[1].name != ""
	var b strings.Builder
	protected := false
	for _, p := range parts {
		piece := p.literal
		if p.name != "" {
			s, ok := vars[p.name]
			if !ok || !s.Enabled {
				return nil, false, failure(400, "variable_missing")
			}
			value, err := parseScalar(s.Raw)
			if err != nil {
				return nil, false, err
			}
			protected = protected || s.Protected
			if sole && location == "json" {
				return value, protected, nil
			}
			if str, ok := value.(string); ok {
				piece = str
			} else if sole && (location == "query" || location == "header") && value != nil {
				piece = string(s.Raw)
			} else {
				return nil, false, failure(400, "variable_type")
			}
		}
		if b.Len()+len(piece) > 65536 {
			return nil, false, failure(400, "input_limit_exceeded")
		}
		b.WriteString(piece)
	}
	value := b.String()
	if location == "header" && (len(value) > 2048 || !HeaderValueValid(value)) {
		return nil, false, failure(400, "template_invalid")
	}
	return value, protected, nil
}

// JSON string VALUES only: property names and untouched number tokens survive.
func renderTemplateJSON(raw []byte, vars map[string]templateScalar) ([]byte, bool, error) {
	if len(raw) > 65536 {
		return nil, false, failure(400, "input_limit_exceeded")
	}
	var root any
	if StrictJSON(raw, &root) != nil {
		return nil, false, failure(400, "template_invalid")
	}
	if !strings.Contains(string(raw), "{{") && !strings.Contains(string(raw), "}}") {
		return append([]byte(nil), raw...), false, nil
	}
	protected := false
	nodes := 0
	var visit func(any) (any, error)
	visit = func(v any) (any, error) {
		nodes++
		if nodes > 16384 {
			return nil, failure(413, "variable_limit")
		}
		switch x := v.(type) {
		case string:
			r, s, err := renderTemplate(x, "json", vars)
			protected = protected || s
			return r, err
		case map[string]any:
			for k, child := range x {
				if strings.Contains(k, "{{") || strings.Contains(k, "}}") {
					return nil, failure(400, "template_invalid")
				}
				r, err := visit(child)
				if err != nil {
					return nil, err
				}
				x[k] = r
			}
		case []any:
			for i, child := range x {
				r, err := visit(child)
				if err != nil {
					return nil, err
				}
				x[i] = r
			}
		}
		return v, nil
	}
	root, err := visit(root)
	if err != nil {
		return nil, false, err
	}
	out := templateCompact(root)
	if len(out) > 65536 {
		return nil, false, failure(400, "input_limit_exceeded")
	}
	return out, protected, nil
}

// URL experiment: a public leading base URL plus public whole path segments.
// This does syntax/taint checks only; it does not perform DNS or authorize a target.
func renderTemplateURL(text string, vars map[string]templateScalar) (string, error) {
	if len(text) > 65536 {
		return "", failure(400, "input_limit_exceeded")
	}
	if strings.HasPrefix(text, "{{") {
		i := strings.Index(text, "}}")
		if i < 0 {
			return "", failure(400, "template_invalid")
		}
		name := text[2:i]
		if !variableName.MatchString(name) {
			return "", failure(400, "template_invalid")
		}
		v, ok := vars[name]
		if !ok || !v.Enabled {
			return "", failure(400, "variable_missing")
		}
		if v.Protected {
			return "", failure(400, "sensitive_url")
		}
		value, err := parseScalar(v.Raw)
		base, ok := value.(string)
		if err != nil || !ok || strings.ContainsAny(base, "{}") {
			return "", failure(400, "template_invalid")
		}
		u, err := url.Parse(base)
		if err != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Host == "" || u.Scheme != "http" && u.Scheme != "https" {
			return "", failure(400, "template_invalid")
		}
		suffix := text[i+2:]
		if suffix != "" && !strings.HasPrefix(suffix, "/") {
			return "", failure(400, "template_invalid")
		}
		text = strings.TrimSuffix(base, "/") + suffix
	}
	u, err := url.Parse(text)
	if err != nil || u.User != nil || u.Host == "" || strings.ContainsAny(u.Host, "{}") || u.RawQuery != "" || u.Fragment != "" || u.Scheme != "http" && u.Scheme != "https" {
		return "", failure(400, "template_invalid")
	}
	// Read template delimiters before net/url percent-escapes them. Percent
	// escapes authored as literal data do not turn into template syntax.
	prefix := len(u.Scheme) + 3
	start := strings.Index(text[prefix:], "/")
	sourcePath := ""
	if start >= 0 {
		sourcePath = text[prefix+start:]
	}
	segments := strings.Split(sourcePath, "/")
	for i, p := range segments {
		if !strings.ContainsAny(p, "{}") {
			decoded, err := url.PathUnescape(p)
			if err != nil {
				return "", failure(400, "template_invalid")
			}
			segments[i] = url.PathEscape(decoded)
			continue
		}
		if !strings.HasPrefix(p, "{{") || !strings.HasSuffix(p, "}}") {
			return "", failure(400, "template_invalid")
		}
		name := p[2 : len(p)-2]
		if !variableName.MatchString(name) {
			return "", failure(400, "template_invalid")
		}
		v, ok := vars[name]
		if !ok || !v.Enabled {
			return "", failure(400, "variable_missing")
		}
		if v.Protected {
			return "", failure(400, "sensitive_url")
		}
		x, err := parseScalar(v.Raw)
		s, ok := x.(string)
		if err != nil || !ok || s == "." || s == ".." || strings.ContainsAny(s, "/\\%?#{}") {
			return "", failure(400, "template_invalid")
		}
		segments[i] = url.PathEscape(s)
	}
	path := strings.Join(segments, "/")
	out := u.Scheme + "://" + u.Host + path
	if len(out) > 65536 {
		return "", failure(400, "input_limit_exceeded")
	}
	return out, nil
}

// TemplateLocation identifies a bounded source, never a diagnostic payload.
type TemplateLocation struct {
	Kind       string
	Index      int
	Pointer    string
	Text       string
	Enabled    bool
	Credential bool
}

func HasTemplate(s string) bool { return strings.Contains(s, "{{") || strings.Contains(s, "}}") }
func wholeTemplate(text string, bearer bool) bool {
	if bearer {
		text = strings.TrimPrefix(text, "Bearer ")
	}
	p, e := scanTemplate(text)
	return e == nil && len(p) == 2 && p[0].literal == "" && p[1].name != ""
}
func TemplateLocations(c Configuration) ([]TemplateLocation, error) {
	if c.TemplateSemanticsVersion != 1 || HasTemplate(c.Name) || HasTemplate(c.Method) || c.OperationRef != nil {
		return nil, failure(400, "template_invalid")
	}
	out := []TemplateLocation{{Kind: "url", Text: c.URL, Enabled: true}}
	for _, g := range []struct {
		kind   string
		fields []Field
	}{{"header", c.Headers}, {"query", c.QueryParameters}} {
		for i, f := range g.fields {
			if HasTemplate(f.Name) || f.Sensitive && HasTemplate(f.Value) {
				return nil, failure(400, "template_target_overlap")
			}
			if credentialName(f.Name) && !f.Sensitive && !wholeTemplate(f.Value, g.kind == "header") && !(g.kind == "header" && f.Value == "") {
				return nil, failure(400, "sensitive_input")
			}
			out = append(out, TemplateLocation{Kind: g.kind, Index: i, Text: f.Value, Enabled: f.Enabled, Credential: credentialName(f.Name)})
		}
	}
	if b := c.Body; b != nil && !b.Sensitive {
		if b.Type == "text" {
			out = append(out, TemplateLocation{Kind: "text", Text: b.Text, Enabled: true})
		} else if b.Type == "json" {
			var v any
			if StrictJSON([]byte(b.Text), &v) != nil {
				return nil, failure(400, "template_invalid")
			}
			nodes := 0
			var visit func(any, string, bool) error
			visit = func(v any, path string, sensitive bool) error {
				nodes++
				if nodes > 16384 {
					return failure(413, "variable_limit")
				}
				switch x := v.(type) {
				case string:
					if sensitive && !wholeTemplate(x, false) {
						return failure(400, "sensitive_input")
					}
					if HasTemplate(x) {
						for _, f := range b.SecretFields {
							if path == f.Pointer || strings.HasPrefix(path, f.Pointer+"/") || strings.HasPrefix(f.Pointer, path+"/") {
								return failure(400, "template_target_overlap")
							}
						}
					}
					out = append(out, TemplateLocation{Kind: "json", Pointer: path, Text: x, Enabled: true, Credential: sensitive})
				case map[string]any:
					for k, child := range x {
						if HasTemplate(k) {
							return failure(400, "template_invalid")
						}
						next := path + "/" + strings.ReplaceAll(strings.ReplaceAll(k, "~", "~0"), "/", "~1")
						if e := visit(child, next, sensitive || credentialName(k)); e != nil {
							return e
						}
					}
				case []any:
					for i, child := range x {
						if e := visit(child, path+"/"+strconv.Itoa(i), sensitive); e != nil {
							return e
						}
					}
				default:
					if sensitive {
						found := false
						for _, f := range b.SecretFields {
							if f.Pointer == path {
								found = true
							}
						}
						if !found {
							return failure(400, "sensitive_input")
						}
					}
				}
				return nil
			}
			if e := visit(v, "", false); e != nil {
				return nil, e
			}
		} else {
			return nil, failure(400, "template_invalid")
		}
	}
	total := 0
	distinct := map[string]bool{}
	for _, loc := range out {
		p, e := scanTemplate(loc.Text)
		if e != nil {
			return nil, e
		}
		for _, part := range p {
			if part.name != "" {
				total++
				distinct[part.name] = true
			}
		}
	}
	if total > 128 || len(distinct) > 32 {
		return nil, failure(413, "variable_limit")
	}
	return out, nil
}
func UsedTemplateNames(c Configuration) ([]string, error) {
	locations, e := TemplateLocations(c)
	if e != nil {
		return nil, e
	}
	seen := map[string]bool{}
	out := []string{}
	for _, loc := range locations {
		if !loc.Enabled {
			continue
		}
		parts, _ := scanTemplate(loc.Text)
		for _, p := range parts {
			if p.name != "" && !seen[p.name] {
				seen[p.name] = true
				out = append(out, p.name)
			}
		}
	}
	return out, nil
}
func snapshotScalars(p Plan, values map[string]string, publicOnly bool) (map[string]templateScalar, error) {
	if e := p.TemplateSnapshot.Validate(); e != nil {
		return nil, e
	}
	vars := map[string]templateScalar{}
	for _, v := range p.TemplateSnapshot.Variables {
		var raw []byte
		if v.Sensitive {
			if publicOnly {
				raw = []byte(`"unused"`)
			} else {
				value, ok := values[v.SecretRef.SecretID]
				if !ok {
					return nil, failure(400, "source_unavailable")
				}
				kind := "jsonScalar"
				if v.ValueType == "string" {
					kind = "string"
				}
				if e := ValidateTypedSecret(kind, v.ValueType, value); e != nil {
					return nil, e
				}
				if kind == "string" {
					raw, _ = json.Marshal(value)
				} else {
					// Decode JSON framing once; preserve the numeric token itself.
					// String payloads above remain literal and are never trimmed.
					var primitive any
					if StrictJSON([]byte(value), &primitive) != nil {
						return nil, failure(400, "variable_type")
					}
					raw = templateCompact(primitive)
				}
			}
		} else {
			var e error
			raw, e = v.Value.Raw()
			if e != nil {
				return nil, e
			}
		}
		vars[v.Name] = templateScalar{Raw: raw, Protected: v.Sensitive, Enabled: true}
	}
	used, e := UsedTemplateNames(p.Configuration)
	if e != nil {
		return nil, e
	}
	if len(used) != len(vars) {
		return nil, failure(400, "source_unavailable")
	}
	for _, name := range used {
		if _, ok := vars[name]; !ok {
			return nil, failure(400, "variable_missing")
		}
	}
	return vars, nil
}
func ResolvedTemplateURL(p Plan) (string, error) {
	if p.Configuration.TemplateSemanticsVersion == 0 {
		return p.Configuration.URL, nil
	}
	vars, e := snapshotScalars(p, nil, true)
	if e != nil {
		return "", e
	}
	if !HasTemplate(p.Configuration.URL) {
		return p.Configuration.URL, nil
	}
	return renderTemplateURL(p.Configuration.URL, vars)
}

// ResolveTemplatePlan returns transient copies only, never a persistable plan.
func ResolveTemplatePlan(p Plan, values map[string]string) (Plan, map[string]string, error) {
	if p.Configuration.TemplateSemanticsVersion == 0 {
		if p.TemplateSnapshot != nil {
			return p, nil, failure(400, "template_invalid")
		}
		return p, values, nil
	}
	vars, e := snapshotScalars(p, values, false)
	if e != nil {
		return p, nil, e
	}
	raw, _ := json.Marshal(p.Configuration)
	var c Configuration
	if StrictJSON(raw, &c) != nil {
		return p, nil, failure(400, "template_invalid")
	}
	corpus := map[string]string{}
	for k, v := range values {
		corpus[k] = v
	}
	c.URL, e = ResolvedTemplateURL(p)
	if e != nil {
		return p, nil, e
	}
	locations, e := TemplateLocations(p.Configuration)
	if e != nil {
		return p, nil, e
	}
	sourceSanitizer := newSanitizer(values)
	if c.Body != nil && !c.Body.Sensitive && c.Body.Type == "json" {
		var root any
		if StrictJSON([]byte(c.Body.Text), &root) != nil {
			return p, nil, failure(400, "template_invalid")
		}
		var publicKeys func(any) bool
		publicKeys = func(v any) bool {
			switch x := v.(type) {
			case map[string]any:
				for k, v := range x {
					if sourceSanitizer.text(k) != k || !publicKeys(v) {
						return false
					}
				}
			case []any:
				for _, v := range x {
					if !publicKeys(v) {
						return false
					}
				}
			}
			return true
		}
		if !publicKeys(root) {
			return p, nil, failure(400, "sensitive_input")
		}
	}

	for _, text := range []string{c.Name, c.Method, c.URL} {
		if sourceSanitizer.text(text) != text {
			return p, nil, failure(400, "sensitive_input")
		}
	}
	for _, fields := range [][]Field{c.Headers, c.QueryParameters} {
		for _, field := range fields {
			if sourceSanitizer.text(field.Name) != field.Name {
				return p, nil, failure(400, "sensitive_input")
			}
		}
	}
	for _, loc := range locations {
		if !loc.Enabled || loc.Kind == "url" {
			continue
		}
		value, secret, e := renderTemplate(loc.Text, loc.Kind, vars)
		if e != nil {
			return p, nil, e
		}
		if !secret {
			text := string(templateCompact(value))
			if s, ok := value.(string); ok {
				text = s
			}
			if sourceSanitizer.text(text) != text {
				return p, nil, failure(400, "sensitive_input")
			}
		}
		if loc.Credential && !secret && loc.Text != "" {
			return p, nil, failure(400, "sensitive_input")
		}
		if loc.Kind == "header" || loc.Kind == "query" {
			fields := c.Headers
			if loc.Kind == "query" {
				fields = c.QueryParameters
			}
			f := &fields[loc.Index]
			if f.Sensitive {
				continue
			}
			f.Value = value.(string)
			if secret {
				key := "environment:" + loc.Kind + ":" + strconv.Itoa(loc.Index)
				corpus[key] = f.Value
				f.Sensitive = true
				f.SecretRef = &Ref{SecretID: key}
				f.Value = ""
			}
		}
	}
	if b := c.Body; b != nil && !b.Sensitive {
		if b.Type == "json" {
			rendered, _, err := renderTemplateJSON([]byte(b.Text), vars)
			if err != nil {
				return p, nil, err
			}
			b.Text = string(rendered)
		} else {
			value, secret, err := renderTemplate(b.Text, "text", vars)
			if err != nil {
				return p, nil, err
			}
			b.Text = value.(string)
			if secret {
				corpus["environment:body"] = b.Text
			}
		}
	}
	san := newSanitizer(corpus)
	for _, v := range p.TemplateSnapshot.Variables {
		if !v.Sensitive {
			raw, _ := v.Value.Raw()
			if san.text(string(raw)) != string(raw) {
				return p, nil, failure(400, "sensitive_input")
			}
		}
	}
	if san.text(c.URL) != c.URL {
		return p, nil, failure(400, "sensitive_url")
	}
	c.TemplateSemanticsVersion = 0
	p.Configuration = c
	p.TemplateSnapshot = nil
	encoded, _ := json.Marshal(c)
	if len(encoded) > ConfigLimit {
		return p, nil, failure(400, "input_limit_exceeded")
	}
	return p, corpus, nil
}

// ValidateTemplateURL checks source structure without a live environment or DNS.
func ValidateTemplateURL(text string) error {
	parts, e := scanTemplate(text)
	if e != nil {
		return e
	}
	vars := map[string]templateScalar{}
	for _, p := range parts {
		if p.name != "" {
			vars[p.name] = templateScalar{Raw: json.RawMessage(`"segment"`), Enabled: true}
		}
	}
	if strings.HasPrefix(text, "{{") {
		end := strings.Index(text, "}}")
		if end < 0 {
			return failure(400, "template_invalid")
		}
		name := text[2:end]
		v := vars[name]
		v.Raw = json.RawMessage(`"https://template.invalid"`)
		vars[name] = v
	}
	if !HasTemplate(text) {
		return nil
	}
	_, e = renderTemplateURL(text, vars)
	return e
}
