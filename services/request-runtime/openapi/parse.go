package openapi

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/netip"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/oasdiff/yaml3"
)

var grammar = map[string]map[string]string{
	"root":        {"openapi": "string", "info": "info", "servers": "servers", "paths": "paths", "components": "components"},
	"info":        {"title": "string", "version": "string", "description": "string"},
	"server":      {"url": "string", "description": "string"},
	"path":        {"get": "operation", "post": "operation", "put": "operation", "patch": "operation", "delete": "operation", "parameters": "parameters", "servers": "servers", "summary": "string", "description": "string"},
	"operation":   {"operationId": "string", "summary": "string", "description": "string", "parameters": "parameters", "requestBody": "requestBody", "responses": "responses", "servers": "servers"},
	"components":  {"schemas": "schemas", "responses": "responsesByName", "parameters": "parametersByName", "requestBodies": "requestBodies", "examples": "examples"},
	"response":    {"$ref": "ref", "description": "string", "headers": "headers", "content": "content"},
	"header":      {"$ref": "ref", "description": "string", "required": "bool", "schema": "schema"},
	"parameter":   {"$ref": "ref", "name": "string", "in": "string", "required": "bool", "description": "string", "schema": "schema", "style": "string", "explode": "bool", "allowReserved": "bool", "allowEmptyValue": "bool", "example": "exampleValue", "examples": "examples"},
	"requestBody": {"$ref": "ref", "description": "string", "required": "bool", "content": "content"},
	"media":       {"schema": "schema", "example": "exampleValue", "examples": "examples"},
	"example":     {"$ref": "ref", "summary": "string", "description": "string", "value": "exampleValue"},
	"schema":      {"$ref": "ref", "type": "string", "nullable": "bool", "description": "string", "title": "string", "required": "strings", "properties": "schemas", "items": "schema", "additionalProperties": "bool", "example": "exampleValue"},
}

func object(v any) map[string]any { m, _ := v.(map[string]any); return m }
func str(v any) string            { s, _ := v.(string); return s }
func boolean(v any) bool          { b, _ := v.(bool); return b }
func keys(m map[string]any) []string {
	a := make([]string, 0, len(m))
	for k := range m {
		a = append(a, k)
	}
	sort.Strings(a)
	return a
}
func encoded(v any) json.RawMessage { b, _ := json.Marshal(v); return b }

func grammarCheck(v any, kind string, depth int, work *int) error {
	*work++
	if depth > 32 || *work > 20000 {
		return fail("structure_limit")
	}
	switch kind {
	case "string", "ref":
		s, ok := v.(string)
		if !ok {
			return fail("shape_invalid")
		}
		if kind == "ref" && (!strings.HasPrefix(s, "#/components/") || strings.Contains(s, "%")) {
			return fail("reference_unsupported")
		}
		return nil
	case "bool":
		if _, ok := v.(bool); !ok {
			return fail("shape_invalid")
		}
		return nil
	case "exampleValue":
		return nil // The complete syntax tree was bounded before semantic traversal.
	case "servers", "parameters", "strings":
		a, ok := v.([]any)
		if !ok {
			return fail("shape_invalid")
		}
		child := "string"
		if kind == "servers" {
			child = "server"
		}
		if kind == "parameters" {
			child = "parameter"
		}
		for _, x := range a {
			if e := grammarCheck(x, child, depth+1, work); e != nil {
				return e
			}
		}
		return nil
	}
	m, ok := v.(map[string]any)
	if !ok {
		return fail("shape_invalid")
	}
	if _, ref := m["$ref"]; ref && grammar[kind]["$ref"] == "ref" && len(m) != 1 {
		return fail("reference_sibling_unsupported")
	}
	for k, x := range m {
		child := ""
		switch kind {
		case "paths":
			if !validPath(k) {
				return fail("path_unsupported")
			}
			child = "path"
		case "schemas":
			child = "schema"
		case "responsesByName":
			child = "response"
		case "parametersByName":
			child = "parameter"
		case "requestBodies":
			child = "requestBody"
		case "examples":
			child = "example"
		case "headers":
			if !headerName(k) {
				return fail("header_unsupported")
			}
			child = "header"
		case "content":
			if k != "application/json" {
				return fail("media_unsupported")
			}
			child = "media"
		case "responses":
			if !validStatus(k) {
				return fail("status_unsupported")
			}
			child = "response"
		default:
			child = grammar[kind][k]
		}
		if child == "" {
			return fail("feature_unsupported")
		}
		if e := grammarCheck(x, child, depth+1, work); e != nil {
			return e
		}
	}
	return nil
}

func validStatus(s string) bool {
	return s == "default" || len(s) == 3 && s[0] >= '1' && s[0] <= '5' && s[1] >= '0' && s[1] <= '9' && s[2] >= '0' && s[2] <= '9'
}

var placeholder = regexp.MustCompile(`\{([A-Za-z0-9_.-]{1,128})\}`)

func validPath(p string) bool {
	if len(p) > 2048 || !strings.HasPrefix(p, "/") || strings.HasPrefix(p, "//") || strings.ContainsAny(p, "?#\\") || unsafeText(p) {
		return false
	}
	rest := placeholder.ReplaceAllString(p, "")
	if strings.ContainsAny(rest, "{}") {
		return false
	}
	decoded, e := url.PathUnescape(rest)
	if e != nil || strings.ContainsAny(decoded, "\\?#") || unsafeText(decoded) {
		return false
	}
	for _, s := range strings.Split(decoded, "/") {
		if s == "." || s == ".." {
			return false
		}
	}
	return true
}
func unsafeText(s string) bool {
	return strings.ContainsFunc(s, func(r rune) bool { return unicode.IsControl(r) || r == unicode.ReplacementChar })
}
func headerName(s string) bool {
	if s == "" || len(s) > 128 {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("!#$%&'*+-.^_`|~", r)) {
			return false
		}
	}
	return true
}
func credentialName(s string) bool {
	s = strings.ToLower(strings.NewReplacer("-", "", "_", "", ".", "", " ", "").Replace(s))
	for _, v := range []string{"authorization", "cookie", "apikey", "token", "secret", "password", "passwd", "credential", "privatekey", "signature", "sessionid"} {
		if strings.Contains(s, v) {
			return true
		}
	}
	return s == "key" || s == "auth" || s == "pwd"
}

var credentialText = regexp.MustCompile(`(?i)(bearer\s+\S+|-----BEGIN [A-Z ]*PRIVATE KEY-----|AKIA[A-Z0-9]{16}|(?:sk|ghp|github_pat)[_-][A-Za-z0-9_-]{16,})`)

func designated(s string, p Policy) bool {
	if credentialText.MatchString(s) {
		return true
	}
	for _, v := range p.SensitiveValues {
		if v == "" {
			continue
		}
		for _, variant := range []string{v, url.QueryEscape(v), url.PathEscape(v), base64.StdEncoding.EncodeToString([]byte(v)), base64.RawURLEncoding.EncodeToString([]byte(v))} {
			if strings.Contains(s, variant) {
				return true
			}
		}
	}
	return false
}
func sensitiveExample(v any, p Policy) bool {
	if len(p.SensitiveJSONPointers) > 0 {
		return true
	}
	switch x := v.(type) {
	case string:
		return unsafeText(x) || designated(x, p)
	case map[string]any:
		for k, v := range x {
			if credentialName(k) || designated(k, p) || sensitiveExample(v, p) {
				return true
			}
		}
	case []any:
		for _, v := range x {
			if sensitiveExample(v, p) {
				return true
			}
		}
	}
	return false
}

func validServer(s string) bool {
	if s == "" || len(s) > 2048 || unsafeText(s) || strings.ContainsAny(s, "{}\\") {
		return false
	}
	u, e := url.Parse(s)
	if e != nil || u.Scheme != "https" && u.Scheme != "http" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Opaque != "" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || host == "metadata.google.internal" {
		return false
	}
	if a, e := netip.ParseAddr(host); e == nil {
		if !a.IsGlobalUnicast() || a.IsPrivate() || a.IsLoopback() || a.IsLinkLocalUnicast() || a.IsUnspecified() {
			return false
		}
	} else if !strings.Contains(host, ".") || strings.ContainsAny(host, " %") {
		return false
	}
	if _, e := url.PathUnescape(u.EscapedPath()); e != nil {
		return false
	}
	if u.Port() != "" {
		n, e := strconv.Atoi(u.Port())
		if e != nil || n < 1 || n > 65535 {
			return false
		}
	}
	return net.ParseIP(host) == nil || !strings.Contains(host, "%")
}

func expand(root any) (any, error) {
	work := 0
	active := map[string]bool{}
	var walk func(any, string, int) (any, error)
	walk = func(v any, kind string, depth int) (any, error) {
		*(&work)++
		if depth > 32 || work > 20000 {
			return nil, fail("reference_limit")
		}
		if kind == "exampleValue" {
			return v, nil
		}
		switch x := v.(type) {
		case map[string]any:
			if ref, ok := x["$ref"].(string); ok && grammar[kind]["$ref"] == "ref" {
				if active[ref] {
					return nil, fail("reference_cycle")
				}
				parts := strings.Split(strings.TrimPrefix(ref, "#/"), "/")
				expected := map[string]string{"schema": "schemas", "parameter": "parameters", "response": "responses", "requestBody": "requestBodies", "example": "examples"}[kind]
				if len(parts) != 3 || parts[0] != "components" || expected == "" || parts[1] != expected {
					return nil, fail("reference_unsupported")
				}
				var target any = root
				for _, part := range parts {
					if strings.Contains(strings.ReplaceAll(strings.ReplaceAll(part, "~1", ""), "~0", ""), "~") {
						return nil, fail("reference_invalid")
					}
					part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
					m, ok := target.(map[string]any)
					if !ok {
						return nil, fail("reference_invalid")
					}
					target, ok = m[part]
					if !ok {
						return nil, fail("reference_invalid")
					}
				}
				active[ref] = true
				r, e := walk(target, kind, depth+1)
				delete(active, ref)
				return r, e
			}
			m := map[string]any{}
			for k, c := range x {
				r, e := walk(c, childKind(kind, k), depth+1)
				if e != nil {
					return nil, e
				}
				m[k] = r
			}
			return m, nil
		case []any:
			a := make([]any, 0, len(x))
			child := "string"
			if kind == "servers" {
				child = "server"
			}
			if kind == "parameters" {
				child = "parameter"
			}
			for _, c := range x {
				r, e := walk(c, child, depth+1)
				if e != nil {
					return nil, e
				}
				a = append(a, r)
			}
			return a, nil
		default:
			return v, nil
		}
	}
	return walk(root, "root", 0)
}

func childKind(kind, key string) string {
	switch kind {
	case "paths":
		return "path"
	case "schemas":
		return "schema"
	case "responses", "responsesByName":
		return "response"
	case "parametersByName":
		return "parameter"
	case "requestBodies":
		return "requestBody"
	case "examples":
		return "example"
	case "headers":
		return "header"
	case "content":
		return "media"
	}
	return grammar[kind][key]
}

func Parse(raw []byte, format string, policy Policy) (Document, error) {
	empty := Document{}
	if len(policy.SensitiveValues) > 64 || len(policy.SensitiveJSONPointers) > 64 {
		return empty, fail("policy_limit")
	}
	for _, s := range append(append([]string{}, policy.SensitiveValues...), policy.SensitiveJSONPointers...) {
		if len(s) > 2048 || unsafeText(s) {
			return empty, fail("policy_invalid")
		}
	}
	v, e := parseSyntax(raw, format)
	if e != nil {
		return empty, e
	}
	root := object(v)
	version := str(root["openapi"])
	if version != "3.0.0" && version != "3.0.1" && version != "3.0.2" && version != "3.0.3" && version != "3.0.4" {
		return empty, fail("dialect_unsupported")
	}
	work := 0
	if e = grammarCheck(root, "root", 0, &work); e != nil {
		return empty, e
	}
	// Bound and reject cycles before invoking library traversal. Checking the
	// expanded tree again rejects references whose targets have the wrong kind.
	expanded, e := expand(root)
	if e != nil {
		return empty, e
	}
	root = object(expanded)
	work = 0
	if e = grammarCheck(root, "root", 0, &work); e != nil {
		return empty, e
	}
	// Enforce all generation/example budgets before omission or library validation.
	if _, e = operations(root); e != nil {
		return empty, e
	}
	// No URI/file loader can run, even if future library behavior changes.
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = false
	loader.ReadFromURIFunc = func(*openapi3.Loader, *url.URL) ([]byte, error) { return nil, fail("reference_unsupported") }
	doc, e := loader.LoadFromData(encoded(root))
	if e != nil {
		return empty, fail("document_invalid")
	}
	if e = doc.Validate(context.Background()); e != nil {
		return empty, fail("document_invalid")
	}
	warnings := []Diagnostic{}
	omitted := false
	var sanitize func(any, string) (any, error)
	sanitize = func(v any, kind string) (any, error) {
		if kind == "exampleValue" {
			return v, nil
		}
		switch x := v.(type) {
		case map[string]any:
			m := map[string]any{}
			blockedDefault := false
			if direct, ok := x["example"]; ok && grammar[kind]["example"] == "exampleValue" && sensitiveExample(direct, policy) {
				blockedDefault = true
			}
			if examples, ok := x["examples"]; ok && grammar[kind]["examples"] == "examples" {
				names := keys(object(examples))
				if len(names) > 0 && sensitiveExample(object(object(examples)[names[0]])["value"], policy) {
					blockedDefault = true
				}
			}
			for _, k := range keys(x) {
				c := x[k]
				if grammar[kind][k] == "string" && (k == "description" || k == "summary" || k == "title" || k == "operationId") {
					continue
				}
				child := childKind(kind, k)
				if k == "example" && child == "exampleValue" {
					if len(encoded(c)) > 8192 {
						return nil, fail("example_limit")
					}
					if sensitiveExample(c, policy) || kind == "parameter" && credentialName(str(x["name"])) {
						omitted = true
						continue
					}
				}
				if kind == "example" && k == "value" {
					if len(encoded(c)) > 8192 {
						return nil, fail("example_limit")
					}
					if sensitiveExample(c, policy) {
						omitted = true
						return nil, nil
					}
				}
				if kind == "examples" {
					if len(x) > 8 {
						return nil, fail("example_limit")
					}
					child = "example"
				}
				if child != "exampleValue" {
					if s, ok := c.(string); ok && designated(s, policy) {
						return nil, fail("source_unsafe")
					}
					if kind == "schemas" || kind == "paths" || kind == "headers" || kind == "parametersByName" || kind == "requestBodies" {
						if designated(k, policy) {
							return nil, fail("source_unsafe")
						}
					}
				}
				r, e := sanitize(c, child)
				if e != nil {
					return nil, e
				}
				if kind == "examples" {
					if r != nil {
						m["example"+strconv.Itoa(len(m))] = r
					}
				} else {
					m[k] = r
				}
			}
			if kind == "response" {
				m["description"] = ""
			}
			if kind == "root" {
				m["info"] = map[string]any{"title": "Imported API", "version": "1"}
			}
			if blockedDefault {
				delete(m, "example")
				if grammar[kind]["examples"] == "examples" {
					m["examples"] = map[string]any{}
				}
				if schema := object(m["schema"]); schema != nil {
					delete(schema, "example")
				}
				omitted = true
			}
			if kind == "parameter" && credentialName(str(x["name"])) {
				if _, ok := m["examples"]; ok {
					delete(m, "examples")
					omitted = true
				}
				if s := object(m["schema"]); s != nil {
					if _, ok := s["example"]; ok {
						delete(s, "example")
						omitted = true
					}
				}
			}
			return m, nil
		case []any:
			a := []any{}
			child := "string"
			if kind == "servers" {
				child = "server"
			}
			if kind == "parameters" {
				child = "parameter"
			}
			for _, c := range x {
				r, e := sanitize(c, child)
				if e != nil {
					return nil, e
				}
				a = append(a, r)
			}
			return a, nil
		default:
			if s, ok := v.(string); ok && designated(s, policy) {
				return nil, fail("source_unsafe")
			}
			return v, nil
		}
	}
	sanitized, e := sanitize(root, "root")
	if e != nil {
		return empty, e
	}
	root = object(sanitized)
	if omitted {
		warnings = append(warnings, Diagnostic{"example_omitted_sensitive", "warning"})
	}
	ops, e := operations(root)
	if e != nil {
		return empty, e
	}
	for _, op := range ops {
		if len(op.Servers) == 0 {
			warnings = append(warnings, Diagnostic{"server_input_required", "warning"})
			break
		}
	}
	source := encoded(root)
	if len(source) > SourceLimit {
		return empty, fail("source_limit")
	}
	return Document{Source: source, Operations: ops, Diagnostics: warnings}, nil
}

func Export(doc Document, format string) ([]byte, error) {
	// Revalidate stored source independently of caller-supplied operation objects.
	checked, e := Parse(doc.Source, "json", Policy{})
	if e != nil {
		return nil, e
	}
	switch format {
	case "json", "application/json":
		return append([]byte{}, checked.Source...), nil
	case "yaml", "yml", "application/yaml", "text/yaml":
		v, e := strictJSON(checked.Source)
		if e != nil {
			return nil, e
		}
		raw, e := yaml.Marshal(yamlNode(v))
		if e != nil {
			return nil, fail("export_unavailable")
		}
		return raw, nil
	default:
		return nil, fail("format_unsupported")
	}
}

func yamlNode(v any) *yaml.Node {
	switch x := v.(type) {
	case map[string]any:
		n := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		for _, k := range keys(x) {
			n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k}, yamlNode(x[k]))
		}
		return n
	case []any:
		n := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		for _, child := range x {
			n.Content = append(n.Content, yamlNode(child))
		}
		return n
	case json.Number:
		tag := "!!int"
		if strings.ContainsAny(string(x), ".eE") {
			tag = "!!float"
		}
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: string(x)}
	case string:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: x}
	case bool:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: strconv.FormatBool(x)}
	default:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}
	}
}

func operationID(method, path string) string {
	h := sha256.Sum256([]byte(method + " " + path))
	return "op_" + hex.EncodeToString(h[:16])
}
