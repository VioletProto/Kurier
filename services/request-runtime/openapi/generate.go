package openapi

import (
	"encoding/json"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

func operations(root map[string]any) ([]Operation, error) {
	ops := []Operation{}
	paths := object(root["paths"])
	if paths == nil {
		return nil, fail("document_invalid")
	}
	for _, path := range keys(paths) {
		item := object(paths[path])
		for _, method := range []string{"get", "post", "put", "patch", "delete"} {
			raw := object(item[method])
			if raw == nil {
				continue
			}
			if len(ops) >= OperationLimit {
				return nil, fail("operation_limit")
			}
			op := Operation{OperationID: operationID(strings.ToUpper(method), path), Method: strings.ToUpper(method), Path: path, Parameters: []Parameter{}, Responses: map[string]Response{}}
			servers := root["servers"]
			if v, ok := item["servers"]; ok {
				servers = v
			}
			if v, ok := raw["servers"]; ok {
				servers = v
			}
			if a, ok := servers.([]any); ok {
				for _, v := range a {
					s := str(object(v)["url"])
					if strings.HasPrefix(s, "/") && !strings.HasPrefix(s, "//") && !strings.ContainsAny(s, "{}?#\\") && !unsafeText(s) {
						continue // Explicit absolute owner input is required before publication.
					}
					if !validServer(s) {
						return nil, fail("server_unsupported")
					}
					op.Servers = append(op.Servers, s)
				}
			}
			if len(op.Servers) > 8 {
				return nil, fail("server_limit")
			}
			effective := []map[string]any{}
			indexes := map[string]int{}
			for _, level := range []any{item["parameters"], raw["parameters"]} {
				seen := map[string]bool{}
				if a, ok := level.([]any); ok {
					for _, v := range a {
						p := object(v)
						name := str(p["name"])
						in := str(p["in"])
						key := in + ":" + name
						if in == "header" {
							key = in + ":" + strings.ToLower(name)
						}
						if seen[key] {
							return nil, fail("parameter_duplicate")
						}
						seen[key] = true
						if idx, ok := indexes[key]; ok {
							effective[idx] = p
						} else {
							indexes[key] = len(effective)
							effective = append(effective, p)
						}
					}
				}
			}
			if len(effective) > 32 {
				return nil, fail("parameter_limit")
			}
			// Count every authored effective candidate, including unselected named,
			// nested-schema and response examples, before sensitive omission.
			withoutParameters := make(map[string]any, len(raw))
			for name, value := range raw {
				if name != "parameters" {
					withoutParameters[name] = value
				}
			}
			exampleBytes, budgetError := exampleByteCount(withoutParameters, "operation")
			if budgetError != nil {
				return nil, budgetError
			}
			for _, parameter := range effective {
				size, e := exampleByteCount(parameter, "parameter")
				if e != nil {
					return nil, e
				}
				exampleBytes += size
			}
			if exampleBytes > 16<<10 {
				return nil, fail("example_limit")
			}
			for i, raw := range effective {
				p, e := parameterFrom(raw, i)
				if e != nil {
					return nil, e
				}
				op.Parameters = append(op.Parameters, p)
			}
			if b := object(raw["requestBody"]); b != nil {
				media := object(object(b["content"])["application/json"])
				if media == nil || len(object(b["content"])) != 1 {
					return nil, fail("media_unsupported")
				}
				schema, e := schemaFrom(media["schema"])
				if e != nil {
					return nil, e
				}
				example, e := exampleFrom(media, schema)
				if e != nil {
					return nil, e
				}
				op.RequestBody = &BodyDefinition{Required: boolean(b["required"]), Schema: schema, Example: example}
			}
			responses := object(raw["responses"])
			if len(responses) == 0 {
				return nil, fail("document_invalid")
			}
			for status, v := range responses {
				r := object(v)
				response := Response{Headers: map[string]HeaderDefinition{}}
				for name, h := range object(r["headers"]) {
					hm := object(h)
					s, e := schemaFrom(hm["schema"])
					if e != nil {
						return nil, e
					}
					if !primitive(s) || s.Nullable {
						return nil, fail("header_unsupported")
					}
					response.Headers[name] = HeaderDefinition{Required: boolean(hm["required"]), Schema: s}
				}
				if content, ok := r["content"]; ok {
					media := object(object(content)["application/json"])
					if media == nil || len(object(content)) != 1 {
						return nil, fail("media_unsupported")
					}
					s, e := schemaFrom(media["schema"])
					if e != nil {
						return nil, e
					}
					response.Schema = &s
					if _, e = exampleFrom(media, s); e != nil {
						return nil, e
					}
				}
				op.Responses[status] = response
			}
			if e := ValidateOperation(op); e != nil {
				return nil, e
			}
			ops = append(ops, op)
		}
	}
	if len(ops) == 0 {
		return nil, fail("operations_required")
	}
	return ops, nil
}

func exampleByteCount(v any, kind string) (int, error) {
	if kind == "exampleValue" {
		size := len(encoded(v))
		if size > 8192 {
			return 0, fail("example_limit")
		}
		return size, nil
	}
	total := 0
	switch value := v.(type) {
	case map[string]any:
		if kind == "examples" && len(value) > 8 {
			return 0, fail("example_limit")
		}
		for name, child := range value {
			size, e := exampleByteCount(child, childKind(kind, name))
			if e != nil {
				return 0, e
			}
			total += size
			if total > 16<<10 {
				return 0, fail("example_limit")
			}
		}
	case []any:
		child := "string"
		if kind == "parameters" {
			child = "parameter"
		}
		if kind == "servers" {
			child = "server"
		}
		for _, value := range value {
			size, e := exampleByteCount(value, child)
			if e != nil {
				return 0, e
			}
			total += size
			if total > 16<<10 {
				return 0, fail("example_limit")
			}
		}
	}
	return total, nil
}
func primitive(s Schema) bool {
	return s.Type == "string" || s.Type == "number" || s.Type == "integer" || s.Type == "boolean"
}
func parameterFrom(raw map[string]any, i int) (Parameter, error) {
	p := Parameter{InputID: "p" + strconv.Itoa(i), Name: str(raw["name"]), In: str(raw["in"]), Required: boolean(raw["required"])}
	if p.Name == "" || len(p.Name) > 128 || unsafeText(p.Name) {
		return p, fail("parameter_unsupported")
	}
	s, e := schemaFrom(raw["schema"])
	if e != nil {
		return p, e
	}
	p.Schema = s
	if s.Nullable || !primitive(s) && !(p.In == "query" && s.Type == "array" && s.Items != nil && primitive(*s.Items) && !s.Items.Nullable) {
		return p, fail("parameter_unsupported")
	}
	style := "simple"
	explode := false
	if p.In == "query" {
		style = "form"
		explode = true
	}
	if v, ok := raw["style"]; ok && v != style {
		return p, fail("serialization_unsupported")
	}
	if v, ok := raw["explode"]; ok && v != explode {
		return p, fail("serialization_unsupported")
	}
	if boolean(raw["allowReserved"]) || boolean(raw["allowEmptyValue"]) {
		return p, fail("serialization_unsupported")
	}
	switch p.In {
	case "path":
		if !p.Required || credentialName(p.Name) {
			return p, fail("path_unsupported")
		}
	case "query":
		p.Sensitive = credentialName(p.Name)
		if p.Sensitive && s.Type == "array" {
			return p, fail("parameter_unsupported")
		}
	case "header":
		if !headerName(p.Name) || reservedHeader(p.Name) {
			return p, fail("header_unsupported")
		}
		p.Sensitive = credentialName(p.Name)
	default:
		return p, fail("parameter_unsupported")
	}
	p.Example, e = exampleFrom(raw, s)
	if e != nil {
		return p, e
	}
	if e = parameterExampleBounds(raw); e != nil {
		return p, e
	}
	if p.Sensitive {
		p.Example = nil
	}
	return p, nil
}

func parameterExampleBounds(raw map[string]any) error {
	values := []any{}
	if value, ok := raw["example"]; ok {
		values = append(values, value)
	}
	if value, ok := object(raw["schema"])["example"]; ok {
		values = append(values, value)
	}
	for _, example := range object(raw["examples"]) {
		if value, ok := object(example)["value"]; ok {
			values = append(values, value)
		}
	}
	for _, value := range values {
		scalars := []any{value}
		if array, ok := value.([]any); ok {
			if len(array) < 1 || len(array) > 16 {
				return fail("example_limit")
			}
			scalars = array
		}
		for _, scalar := range scalars {
			text, e := scalarString(scalar)
			if e != nil {
				return fail("example_invalid")
			}
			if len(text) > 2048 {
				return fail("example_limit")
			}
			if unsafeText(text) {
				return fail("example_invalid")
			}
		}
	}
	return nil
}
func reservedHeader(name string) bool {
	switch strings.ToLower(name) {
	case "accept", "content-type", "authorization", "host", "content-length", "transfer-encoding", "connection", "proxy-connection", "keep-alive", "upgrade", "trailer", "te", "expect", "proxy-authorization":
		return true
	}
	return false
}
func exampleFrom(m map[string]any, s Schema) (json.RawMessage, error) {
	direct, has := m["example"]
	named, names := m["examples"]
	if has && names {
		return nil, fail("example_invalid")
	}
	if has {
		b := encoded(direct)
		return b, validateValue(s, b)
	}
	if names {
		examples := object(named)
		if examples == nil || len(examples) > 8 {
			return nil, fail("example_limit")
		}
		var first json.RawMessage
		aggregate := 0
		for _, k := range keys(examples) {
			example := object(examples[k])
			v, ok := example["value"]
			if !ok || v == nil {
				return nil, fail("example_invalid")
			}
			raw := encoded(v)
			aggregate += len(raw)
			if aggregate > 16<<10 {
				return nil, fail("example_limit")
			}
			if e := validateValue(s, raw); e != nil {
				return nil, e
			}
			if first == nil {
				first = raw
			}
		}
		return first, nil
	}
	if schema := object(m["schema"]); schema != nil {
		if v, ok := schema["example"]; ok {
			b := encoded(v)
			return b, validateValue(s, b)
		}
	}
	return nil, nil
}

// ValidateOperation applies bounds again to persisted/transported bundles.
func ValidateOperation(op Operation) error {
	if op.OperationID != operationID(op.Method, op.Path) || !strings.Contains("|GET|POST|PUT|PATCH|DELETE|", "|"+op.Method+"|") || !validPath(op.Path) {
		return fail("operation_invalid")
	}
	if len(op.Servers) > 8 || len(op.Parameters) > 32 || len(op.Responses) == 0 || len(op.Responses) > 501 {
		return fail("bundle_limit")
	}
	for _, s := range op.Servers {
		if !validServer(s) {
			return fail("server_unsupported")
		}
	}
	seen, inputs := map[string]bool{}, map[string]bool{}
	pathParams := map[string]bool{}
	for i, p := range op.Parameters {
		if p.InputID != "p"+strconv.Itoa(i) || inputs[p.InputID] || p.Name == "" || len(p.Name) > 128 || unsafeText(p.Name) {
			return fail("parameter_invalid")
		}
		inputs[p.InputID] = true
		key := p.In + ":" + p.Name
		if p.In == "header" {
			key = p.In + ":" + strings.ToLower(p.Name)
		}
		if seen[key] {
			return fail("parameter_duplicate")
		}
		seen[key] = true
		work := 0
		if e := validSchema(p.Schema, 0, &work); e != nil {
			return e
		}
		if p.Schema.Nullable {
			return fail("parameter_unsupported")
		}
		switch p.In {
		case "path":
			if !p.Required || p.Sensitive || credentialName(p.Name) || !primitive(p.Schema) {
				return fail("path_unsupported")
			}
			pathParams[p.Name] = true
		case "query":
			if p.Sensitive && p.Schema.Type == "array" {
				return fail("parameter_unsupported")
			}
			if !primitive(p.Schema) && !(p.Schema.Type == "array" && p.Schema.Items != nil && primitive(*p.Schema.Items) && !p.Schema.Items.Nullable) {
				return fail("parameter_unsupported")
			}
		case "header":
			if !headerName(p.Name) || reservedHeader(p.Name) || !primitive(p.Schema) {
				return fail("header_unsupported")
			}
		default:
			return fail("parameter_unsupported")
		}
		if p.In != "path" && p.Sensitive != credentialName(p.Name) {
			return fail("parameter_invalid")
		}
		if p.Sensitive && p.Example != nil {
			return fail("source_unsafe")
		}
		if len(p.Example) > 0 {
			if e := validateValue(p.Schema, p.Example); e != nil {
				return e
			}
		}
	}
	matches := placeholder.FindAllStringSubmatch(op.Path, -1)
	used := map[string]bool{}
	for _, m := range matches {
		if !pathParams[m[1]] {
			return fail("path_unsupported")
		}
		used[m[1]] = true
	}
	if len(used) != len(pathParams) {
		return fail("path_unsupported")
	}
	if op.RequestBody != nil {
		work := 0
		if e := validSchema(op.RequestBody.Schema, 0, &work); e != nil {
			return e
		}
		if len(op.RequestBody.Example) > 0 {
			if e := validateValue(op.RequestBody.Schema, op.RequestBody.Example); e != nil {
				return e
			}
		}
	}
	for status, r := range op.Responses {
		if !validStatus(status) || len(r.Headers) > 64 {
			return fail("response_invalid")
		}
		names := map[string]bool{}
		for name, h := range r.Headers {
			if !headerName(name) || names[strings.ToLower(name)] {
				return fail("header_unsupported")
			}
			names[strings.ToLower(name)] = true
			work := 0
			if e := validSchema(h.Schema, 0, &work); e != nil {
				return e
			}
			if !primitive(h.Schema) || h.Schema.Nullable {
				return fail("header_unsupported")
			}
		}
		if r.Schema != nil {
			work := 0
			if e := validSchema(*r.Schema, 0, &work); e != nil {
				return e
			}
		}
	}
	bytes, err := json.Marshal(op)
	if err != nil || len(bytes) > BundleLimit {
		return fail("bundle_limit")
	}
	return nil
}

var uuid = regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-4[a-f0-9]{3}-[89ab][a-f0-9]{3}-[a-f0-9]{12}$`)

func Generate(op Operation, choices Choices) (Generated, error) {
	g := Generated{Name: op.Method + " " + op.Path, Method: op.Method, QueryParameters: []Field{}, Headers: []Field{}}
	if utf8.RuneCountInString(g.Name) > 100 {
		g.Name = op.Method + " " + op.OperationID
	}
	if e := ValidateOperation(op); e != nil {
		return g, e
	}
	if e := validateProvidedChoices(op, choices); e != nil {
		return g, e
	}
	server := choices.Server
	if server == "" && len(op.Servers) == 1 {
		server = op.Servers[0]
	}
	if server == "" {
		return g, fail("inputs_required")
	}
	if !validServer(server) {
		return g, fail("server_unsupported")
	}
	if len(op.Servers) > 0 {
		found := false
		for _, s := range op.Servers {
			if server == s {
				found = true
			}
		}
		if !found {
			return g, fail("server_unsupported")
		}
	}
	allowed := map[string]bool{}
	for _, p := range op.Parameters {
		allowed[p.InputID] = true
	}
	for k := range choices.Parameters {
		if !allowed[k] {
			return g, fail("input_invalid")
		}
	}
	for k := range choices.SecretRefs {
		if !allowed[k] {
			return g, fail("input_invalid")
		}
	}
	path := op.Path
	for _, p := range op.Parameters {
		value, provided := choices.Parameters[p.InputID]
		secret := choices.SecretRefs[p.InputID]
		if p.Sensitive {
			if provided {
				return g, fail("source_unsafe")
			}
			if secret == "" {
				if p.Required {
					return g, fail("inputs_required")
				}
				continue
			}
			if !uuid.MatchString(secret) {
				return g, fail("input_invalid")
			}
			f := Field{Name: p.Name, Enabled: true, Sensitive: true, SecretRef: &Ref{secret}}
			if p.In == "query" {
				g.QueryParameters = append(g.QueryParameters, f)
			} else {
				g.Headers = append(g.Headers, f)
			}
			continue
		}
		if secret != "" {
			return g, fail("input_invalid")
		}
		if !provided {
			value = p.Example
		}
		if len(value) == 0 {
			if p.Required {
				return g, fail("inputs_required")
			}
			continue
		}
		if e := validateValue(p.Schema, value); e != nil {
			return g, e
		}
		v, e := strictJSON(value)
		if e != nil || sensitiveExample(v, Policy{}) {
			return g, fail("source_unsafe")
		}
		values := []any{v}
		if a, ok := v.([]any); ok {
			if len(a) < 1 || len(a) > 16 {
				return g, fail("example_limit")
			}
			values = a
		}
		for _, v := range values {
			text, e := scalarString(v)
			if e != nil || len(text) > 2048 || unsafeText(text) {
				return g, fail("input_invalid")
			}
			f := Field{Name: p.Name, Value: text, Enabled: true}
			switch p.In {
			case "path":
				escaped := url.PathEscape(text)
				if text == "." || text == ".." || text == "" {
					return g, fail("path_unsupported")
				}
				path = strings.ReplaceAll(path, "{"+p.Name+"}", escaped)
			case "query":
				g.QueryParameters = append(g.QueryParameters, f)
			case "header":
				g.Headers = append(g.Headers, f)
			}
		}
	}
	if strings.ContainsAny(path, "{}") {
		return g, fail("inputs_required")
	}
	g.URL = strings.TrimRight(server, "/") + path
	if op.RequestBody != nil {
		if choices.BodySecretRef != "" {
			if len(choices.Body) > 0 || !uuid.MatchString(choices.BodySecretRef) {
				return g, fail("input_invalid")
			}
			g.Body = &RequestBody{Type: "json", Sensitive: true, SecretRef: &Ref{choices.BodySecretRef}}
		} else {
			body := choices.Body
			if len(body) == 0 {
				body = op.RequestBody.Example
			}
			if len(body) == 0 {
				if op.RequestBody.Required {
					return g, fail("inputs_required")
				}
			} else {
				if e := validateValue(op.RequestBody.Schema, body); e != nil {
					return g, e
				}
				v, _ := strictJSON(body)
				if sensitiveExample(v, Policy{}) {
					return g, fail("source_unsafe")
				}
				g.Body = &RequestBody{Type: "json", Text: string(body)}
			}
		}
	} else if len(choices.Body) > 0 || choices.BodySecretRef != "" {
		return g, fail("input_invalid")
	}
	if len(encoded(g)) > 64<<10 {
		return g, fail("request_limit")
	}
	return g, nil
}

// Partial completions can be persisted by callers on inputs_required. Validate
// all supplied values first so an earlier missing input never hides an unsafe
// later field behind that resumable status.
func validateProvidedChoices(op Operation, choices Choices) error {
	allowed := map[string]Parameter{}
	for _, p := range op.Parameters {
		allowed[p.InputID] = p
	}
	for input, raw := range choices.Parameters {
		p, ok := allowed[input]
		if !ok {
			return fail("input_invalid")
		}
		if p.Sensitive {
			return fail("source_unsafe")
		}
		if e := validateValue(p.Schema, raw); e != nil {
			return e
		}
		value, e := strictJSON(raw)
		if e != nil {
			return fail("input_invalid")
		}
		if sensitiveExample(value, Policy{}) {
			return fail("source_unsafe")
		}
		if e := parameterExampleBounds(map[string]any{"example": value}); e != nil {
			return e
		}
		if p.In == "path" {
			text, e := scalarString(value)
			if e != nil || text == "" || text == "." || text == ".." {
				return fail("path_unsupported")
			}
		}
	}
	for input, secret := range choices.SecretRefs {
		p, ok := allowed[input]
		if !ok || !p.Sensitive || !uuid.MatchString(secret) {
			return fail("input_invalid")
		}
	}
	if len(choices.Body) > 0 {
		if op.RequestBody == nil || choices.BodySecretRef != "" {
			return fail("input_invalid")
		}
		if e := validateValue(op.RequestBody.Schema, choices.Body); e != nil {
			return e
		}
		value, e := strictJSON(choices.Body)
		if e != nil {
			return fail("input_invalid")
		}
		if sensitiveExample(value, Policy{}) {
			return fail("source_unsafe")
		}
	}
	if choices.BodySecretRef != "" && (op.RequestBody == nil || !uuid.MatchString(choices.BodySecretRef)) {
		return fail("input_invalid")
	}
	if choices.Server != "" {
		if !validServer(choices.Server) {
			return fail("server_unsupported")
		}
		if len(op.Servers) > 0 {
			found := false
			for _, server := range op.Servers {
				if server == choices.Server {
					found = true
				}
			}
			if !found {
				return fail("server_unsupported")
			}
		}
	}
	return nil
}
func scalarString(v any) (string, error) {
	switch x := v.(type) {
	case string:
		return x, nil
	case bool:
		return strconv.FormatBool(x), nil
	case json.Number:
		return string(x), nil
	}
	return "", fail("input_invalid")
}

// StableOperations helps callers display the parser's deterministic order.
func StableOperations(ops []Operation) {
	sort.Slice(ops, func(i, j int) bool { return ops[i].OperationID < ops[j].OperationID })
}
