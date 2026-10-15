package execution

import (
	"encoding/json"
	"fmt"
	"math"
	"mime"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

type Extraction struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Type      string `json:"type"`
	Sensitive bool   `json:"sensitive"`
}
type Producer struct {
	StepID string `json:"stepId"`
	Name   string `json:"name"`
}
type BindingTarget struct {
	Kind    string  `json:"kind"`
	Index   *int    `json:"index,omitempty"`
	Pointer *string `json:"pointer,omitempty"`
	Prefix  *string `json:"prefix,omitempty"`
	Suffix  *string `json:"suffix,omitempty"`
}
type Binding struct {
	From Producer      `json:"from"`
	To   BindingTarget `json:"to"`
}
type WorkflowStep struct {
	StepID          string       `json:"stepId"`
	RequestID       string       `json:"requestId"`
	RequestRevision int64        `json:"requestRevision"`
	Extract         []Extraction `json:"extract"`
	Bindings        []Binding    `json:"bindings"`
}
type RuntimeValue struct {
	Value     json.RawMessage `json:"value"`
	Sensitive bool            `json:"sensitive"`
}
type PathSelector struct {
	Property string
	Index    int
	Array    bool
}

var variableName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,63}$`)
var workflowUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func workflowInvalid() error { return failure(400, "validation_failed") }
func ParseExtractionPath(path string) ([]PathSelector, error) {
	if len(path) == 0 || len(path) > 512 || path[0] != '$' || !utf8.ValidString(path) {
		return nil, workflowInvalid()
	}
	out := []PathSelector{}
	for i := 1; i < len(path); {
		if len(out) >= 32 {
			return nil, workflowInvalid()
		}
		if path[i] == '.' {
			start := i + 1
			i = start
			for i < len(path) && (path[i] >= 'a' && path[i] <= 'z' || path[i] >= 'A' && path[i] <= 'Z' || path[i] == '_' || i > start && path[i] >= '0' && path[i] <= '9') {
				i++
			}
			if i == start {
				return nil, workflowInvalid()
			}
			out = append(out, PathSelector{Property: path[start:i]})
			continue
		}
		if path[i] != '[' {
			return nil, workflowInvalid()
		}
		i++
		if i < len(path) && path[i] == '"' {
			start := i
			i++
			escaped := false
			for i < len(path) {
				if !escaped && path[i] == '"' {
					break
				}
				if !escaped && path[i] == '\\' {
					escaped = true
				} else {
					escaped = false
				}
				i++
			}
			if i >= len(path) {
				return nil, workflowInvalid()
			}
			i++
			var property string
			if StrictJSON([]byte(path[start:i]), &property) != nil || i >= len(path) || path[i] != ']' {
				return nil, workflowInvalid()
			}
			i++
			out = append(out, PathSelector{Property: property})
			continue
		}
		start := i
		for i < len(path) && path[i] >= '0' && path[i] <= '9' {
			i++
		}
		if start == i || i >= len(path) || path[i] != ']' {
			return nil, workflowInvalid()
		}
		n, e := strconv.Atoi(path[start:i])
		if e != nil || strconv.Itoa(n) != path[start:i] {
			return nil, workflowInvalid()
		}
		i++
		out = append(out, PathSelector{Array: true, Index: n})
	}
	return out, nil
}
func selected(root any, parts []PathSelector) (any, string, error) {
	p := ""
	v := root
	for _, part := range parts {
		if part.Array {
			a, ok := v.([]any)
			if !ok || part.Index >= len(a) {
				return nil, "", workflowInvalid()
			}
			v = a[part.Index]
			p += "/" + strconv.Itoa(part.Index)
		} else {
			m, ok := v.(map[string]any)
			if !ok {
				return nil, "", workflowInvalid()
			}
			v, ok = m[part.Property]
			if !ok {
				return nil, "", workflowInvalid()
			}
			p += "/" + strings.ReplaceAll(strings.ReplaceAll(part.Property, "~", "~0"), "/", "~1")
		}
	}
	return v, p, nil
}
func scalar(v any) (string, error) {
	switch x := v.(type) {
	case string:
		return "string", nil
	case bool:
		return "boolean", nil
	case nil:
		return "null", nil
	case json.Number:
		n, e := x.Float64()
		if e != nil || math.IsNaN(n) || math.IsInf(n, 0) || math.Trunc(n) == n && math.Abs(n) > 9007199254740991 {
			return "", workflowInvalid()
		}
		return "number", nil
	}
	return "", workflowInvalid()
}
func HeaderValueValid(v string) bool {
	if !utf8.ValidString(v) {
		return false
	}
	for _, b := range []byte(v) {
		if b == 127 || b < 32 && b != '\t' {
			return false
		}
	}
	return true
}
func literal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
func RenderHeader(v string, t BindingTarget) (string, error) {
	if len(literal(t.Prefix)) > 256 || len(literal(t.Suffix)) > 256 {
		return "", failure(413, "payload_too_large")
	}
	if !HeaderValueValid(literal(t.Prefix)) || !HeaderValueValid(literal(t.Suffix)) || !HeaderValueValid(v) {
		return "", workflowInvalid()
	}
	if len(v) > 2048-len(literal(t.Prefix))-len(literal(t.Suffix)) {
		return "", failure(400, "input_limit_exceeded")
	}
	return literal(t.Prefix) + v + literal(t.Suffix), nil
}
func ValidateWorkflow(steps []WorkflowStep, plans []Plan) error {
	if len(steps) < 2 || len(steps) > 10 || len(plans) != len(steps) {
		return workflowInvalid()
	}
	producers := map[string]map[string]string{}
	revisions := map[string]int64{}
	for pos, step := range steps {
		if !workflowUUID.MatchString(step.StepID) || !workflowUUID.MatchString(step.RequestID) || step.RequestRevision < 0 || producers[step.StepID] != nil || len(step.Extract) > 8 || len(step.Bindings) > 8 || step.Extract == nil || step.Bindings == nil {
			return workflowInvalid()
		}
		if rev, ok := revisions[step.RequestID]; ok && rev != step.RequestRevision {
			return workflowInvalid()
		}
		revisions[step.RequestID] = step.RequestRevision
		names := map[string]string{}
		for _, x := range step.Extract {
			if !variableName.MatchString(x.Name) || names[x.Name] != "" {
				return workflowInvalid()
			}
			if _, e := ParseExtractionPath(x.Path); e != nil {
				return e
			}
			switch x.Type {
			case "string", "number", "boolean", "null":
			default:
				return workflowInvalid()
			}
			names[x.Name] = x.Type
		}
		targets := map[string]bool{}
		pointers := []string{}
		for bindingIndex, b := range step.Bindings {
			typ := producers[b.From.StepID][b.From.Name]
			if typ == "" {
				return workflowInvalid()
			}
			t := b.To
			key := t.Kind
			config := plans[pos].Configuration
			if t.Kind != "header" && (t.Prefix != nil || t.Suffix != nil) || t.Kind == "header" && typ != "string" && (t.Prefix != nil || t.Suffix != nil) {
				return workflowInvalid()
			}
			switch t.Kind {
			case "header", "query":
				if t.Index == nil || t.Pointer != nil || *t.Index < 0 || typ == "null" {
					return workflowInvalid()
				}
				fields := config.QueryParameters
				if t.Kind == "header" {
					fields = config.Headers
				}
				if *t.Index >= len(fields) || !fields[*t.Index].Enabled || fields[*t.Index].Sensitive || fields[*t.Index].SecretRef != nil {
					return workflowInvalid()
				}
				key += strconv.Itoa(*t.Index)
				if t.Kind == "header" {
					if _, e := RenderHeader("", t); e != nil {
						return e
					}
					n := strings.ToLower(fields[*t.Index].Name)
					if !headerName(n) {
						return workflowInvalid()
					}
					switch n {
					case "host", "content-length", "transfer-encoding", "connection", "upgrade", "proxy-connection", "expect":
						return workflowInvalid()
					}
					if typ == "string" {
						prefix, suffix := literal(t.Prefix), literal(t.Suffix)
						steps[pos].Bindings[bindingIndex].To.Prefix = &prefix
						steps[pos].Bindings[bindingIndex].To.Suffix = &suffix
					}
				}
			case "jsonBody":
				if t.Pointer == nil || t.Index != nil || config.Body == nil || config.Body.Type != "json" || config.Body.Sensitive {
					return workflowInvalid()
				}
				parts, e := pointerParts(*t.Pointer)
				if e != nil {
					return e
				}
				_ = parts
				var obj any
				if StrictJSON([]byte(config.Body.Text), &obj) != nil {
					return workflowInvalid()
				}
				v, e := getPointer(obj, *t.Pointer)
				if e != nil {
					return e
				}
				if _, e = scalar(v); e != nil {
					return e
				}
				for _, f := range config.Body.SecretFields {
					if overlaps(f.Pointer, *t.Pointer) {
						return workflowInvalid()
					}
				}
				for _, p := range pointers {
					if overlaps(p, *t.Pointer) {
						return workflowInvalid()
					}
				}
				pointers = append(pointers, *t.Pointer)
				key += *t.Pointer
			case "textBody":
				if typ != "string" || t.Index != nil || t.Pointer != nil || config.Body == nil || config.Body.Type != "text" || config.Body.Sensitive {
					return workflowInvalid()
				}
			default:
				return workflowInvalid()
			}
			if targets[key] {
				return workflowInvalid()
			}
			targets[key] = true
		}
		producers[step.StepID] = names
	}
	return nil
}
func overlaps(a, b string) bool {
	return a == b || strings.HasPrefix(a, b+"/") || strings.HasPrefix(b, a+"/")
}
func getPointer(root any, p string) (any, error) {
	parts, e := pointerParts(p)
	if e != nil {
		return nil, e
	}
	v := root
	for _, part := range parts {
		switch x := v.(type) {
		case map[string]any:
			var ok bool
			v, ok = x[part]
			if !ok {
				return nil, workflowInvalid()
			}
		case []any:
			n, e := strconv.Atoi(part)
			if e != nil || strconv.Itoa(n) != part || n < 0 || n >= len(x) {
				return nil, workflowInvalid()
			}
			v = x[n]
		default:
			return nil, workflowInvalid()
		}
	}
	return v, nil
}
func RuntimeKey(step, name string) string { return step + "/" + name }
func ApplyWorkflowBindings(plan Plan, step WorkflowStep, outputs map[string]RuntimeValue, values map[string]string) (Plan, error) {
	raw, _ := json.Marshal(plan)
	var p Plan
	_ = StrictJSON(raw, &p)
	for _, b := range step.Bindings {
		rv, ok := outputs[RuntimeKey(b.From.StepID, b.From.Name)]
		if !ok {
			return p, failure(400, "source_unavailable")
		}
		var v any
		if StrictJSON(rv.Value, &v) != nil {
			return p, workflowInvalid()
		}
		kind, e := scalar(v)
		if e != nil {
			return p, e
		}
		text := string(rv.Value)
		if kind == "string" {
			text = v.(string)
		}
		sensitive := rv.Sensitive
		t := b.To
		switch t.Kind {
		case "header":
			f := &p.Configuration.Headers[*t.Index]
			sensitive = sensitive || credentialName(f.Name)
			if kind == "string" {
				text, e = RenderHeader(text, t)
				if e != nil {
					return p, e
				}
			} else if !HeaderValueValid(text) {
				return p, workflowInvalid()
			}
			f.Value = text
		case "query":
			p.Configuration.QueryParameters[*t.Index].Value = text
			sensitive = sensitive || credentialName(p.Configuration.QueryParameters[*t.Index].Name)
		case "jsonBody":
			var obj any
			if StrictJSON([]byte(p.Configuration.Body.Text), &obj) != nil {
				return p, workflowInvalid()
			}
			obj, e = setPointer(obj, *t.Pointer, v)
			if e != nil {
				return p, e
			}
			r, _ := json.Marshal(obj)
			p.Configuration.Body.Text = string(r)
		case "textBody":
			p.Configuration.Body.Text = text
		}
		u, e := url.Parse(p.Configuration.URL)
		if e != nil {
			return p, workflowInvalid()
		}
		if sensitive && u.Scheme == "http" && !p.Options.AllowInsecureSecrets {
			return p, failure(400, "insecure_secret_transport")
		}
		values["runtime-render/"+RuntimeKey(b.From.StepID, b.From.Name)+"/"+fmt.Sprint(len(values))] = text
	}
	return p, nil
}

// ExtractionCapture stays in worker memory and is never a wire DTO or Plan field.
type ExtractionCapture struct {
	// Seal is a worker-memory callback; encryption must precede raw-buffer disposal.
	Seal    func(map[string]RuntimeValue) error
	Step    WorkflowStep
	Steps   []WorkflowStep
	Plans   []Plan
	Outputs map[string]RuntimeValue
	Values  map[string]string
}

func (p *Prepared) WorkflowCapture(c *ExtractionCapture) { p.workflow = c }
func (c *ExtractionCapture) capture(raw []byte, contentType string) ([]string, error) {
	media, params, e := mime.ParseMediaType(contentType)
	if e != nil || params["charset"] != "" && !strings.EqualFold(params["charset"], "utf-8") || !(media == "application/json" || strings.HasPrefix(media, "application/") && strings.HasSuffix(media, "+json")) {
		return nil, workflowInvalid()
	}
	var root any
	if StrictJSON(raw, &root) != nil {
		return nil, workflowInvalid()
	}
	paths := []string{}
	failed := false
	for _, x := range c.Step.Extract {
		parts, _ := ParseExtractionPath(x.Path)
		v, path, e := selected(root, parts)
		if e != nil {
			failed = true
			continue
		}
		paths = append(paths, path)
		r, e := json.Marshal(v)
		if e != nil {
			failed = true
			continue
		}
		typ, e := scalar(v)
		if e != nil {
			failed = true
			continue
		}
		key := RuntimeKey(c.Step.StepID, x.Name)
		sensitive := x.Sensitive || newSanitizer(c.Values).text(string(r)) != string(r)
		c.Values["runtime/"+key] = string(r)
		for _, part := range parts {
			sensitive = sensitive || credentialName(part.Property)
		}
		for i, step := range c.Steps {
			for _, b := range step.Bindings {
				if b.From.StepID != c.Step.StepID || b.From.Name != x.Name {
					continue
				}
				if b.To.Kind == "header" {
					sensitive = sensitive || credentialName(c.Plans[i].Configuration.Headers[*b.To.Index].Name)
					if typ == "string" {
						if rendered, e := RenderHeader(v.(string), b.To); e == nil {
							c.Values["runtime-render/"+key+"/"+fmt.Sprint(i)+"/"+fmt.Sprint(*b.To.Index)] = rendered
						}
					}
				}
			}
		}
		if typ != x.Type || len(r) > 2048 {
			failed = true
			continue
		}
		c.Outputs[key] = RuntimeValue{r, sensitive}
	}
	if failed {
		return paths, workflowInvalid()
	}
	return paths, nil
}
func CredentialName(v string) bool { return credentialName(v) }
