package ownership

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

func descriptorJSON(v any) ([]byte, error) {
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	err := e.Encode(v)
	return bytes.TrimSuffix(b.Bytes(), []byte("\n")), err
}

var bindingUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func strictDecode(raw []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	return d.Decode(v)
}
func descriptor(m map[string]json.RawMessage, body, stored bool) error {
	var sensitive bool
	if v, ok := m["sensitive"]; !ok || bytes.Equal(v, []byte("null")) || json.Unmarshal(v, &sensitive) != nil {
		return invalidConfiguration()
	}
	required := []string{"name", "enabled", "sensitive", "value"}
	allowed := map[string]bool{}
	if body {
		required = []string{"type", "text", "sensitive"}
		allowed["secretFields"] = true
	}
	if sensitive {
		if body {
			required = []string{"type", "sensitive", "bindingId"}
		} else {
			required = []string{"name", "enabled", "sensitive", "bindingId"}
		}
		if stored {
			required = append(required, "masked", "secretRef")
		} else {
			required = append(required, "secretWrite")
		}
	}
	for _, k := range required {
		v, ok := m[k]
		if !ok || bytes.Equal(v, []byte("null")) {
			return invalidConfiguration()
		}
		allowed[k] = true
	}
	for k := range m {
		if !allowed[k] {
			return invalidConfiguration()
		}
	}
	return nil
}

// Public marshaling retains the existing exact field order and empty strings.
func (f RequestField) MarshalJSON() ([]byte, error) {
	if !f.Sensitive {
		return descriptorJSON(struct {
			Name      string `json:"name"`
			Value     string `json:"value"`
			Enabled   bool   `json:"enabled"`
			Sensitive bool   `json:"sensitive"`
		}{f.Name, f.Value, f.Enabled, false})
	}
	return descriptorJSON(struct {
		Name      string           `json:"name"`
		Enabled   bool             `json:"enabled"`
		Sensitive bool             `json:"sensitive"`
		BindingID string           `json:"bindingId"`
		Masked    bool             `json:"masked,omitempty"`
		Ref       *SecretReference `json:"secretRef,omitempty"`
		Write     *SecretWrite     `json:"secretWrite,omitempty"`
	}{f.Name, f.Enabled, true, f.BindingID, f.Masked, f.SecretRef, f.SecretWrite})
}
func (b RequestBody) MarshalJSON() ([]byte, error) {
	if !b.Sensitive {
		return descriptorJSON(struct {
			Type      string        `json:"type"`
			Text      string        `json:"text"`
			Sensitive bool          `json:"sensitive"`
			Fields    []SecretField `json:"secretFields,omitempty"`
		}{b.Type, b.Text, false, b.SecretFields})
	}
	return descriptorJSON(struct {
		Type      string           `json:"type"`
		Sensitive bool             `json:"sensitive"`
		BindingID string           `json:"bindingId"`
		Masked    bool             `json:"masked,omitempty"`
		Ref       *SecretReference `json:"secretRef,omitempty"`
		Write     *SecretWrite     `json:"secretWrite,omitempty"`
	}{b.Type, true, b.BindingID, b.Masked, b.SecretRef, b.SecretWrite})
}

type secretSlot struct {
	id, locator, kind string
	ref               **SecretReference
	write             **SecretWrite
	masked            *bool
}

func slots(c *RequestConfiguration) []secretSlot {
	var out []secretSlot
	for _, group := range []struct {
		fields     []RequestField
		collection string
	}{{c.Headers, "headers"}, {c.QueryParameters, "query"}} {
		for i := range group.fields {
			f := &group.fields[i]
			if f.Sensitive {
				out = append(out, secretSlot{f.BindingID, group.collection + "\x00" + f.Name, "string", &f.SecretRef, &f.SecretWrite, &f.Masked})
			}
		}
	}
	if b := c.Body; b != nil {
		if b.Sensitive {
			kind := "string"
			if b.Type == "json" {
				kind = "jsonBody"
			}
			out = append(out, secretSlot{b.BindingID, "body:" + b.Type, kind, &b.SecretRef, &b.SecretWrite, &b.Masked})
		} else {
			for i := range b.SecretFields {
				f := &b.SecretFields[i]
				out = append(out, secretSlot{f.BindingID, "pointer:" + b.Type + ":" + f.Pointer, "jsonScalar", &f.SecretRef, &f.SecretWrite, &f.Masked})
			}
		}
	}
	return out
}
func validateSlots(c *RequestConfiguration) error {
	seen := map[string]bool{}
	all := slots(c)
	if len(all) > 16 {
		return invalidConfiguration()
	}
	for _, f := range all {
		if !bindingUUID.MatchString(f.id) || seen[f.id] {
			return invalidConfiguration()
		}
		seen[f.id] = true
		if *f.write == nil {
			if *f.ref == nil || !bindingUUID.MatchString((*f.ref).SecretID) || !*f.masked {
				return invalidConfiguration()
			}
		} else {
			w := *f.write
			if *f.ref != nil || *f.masked {
				return invalidConfiguration()
			}
			switch w.Action {
			case "set":
				if w.Value == nil || w.SecretRef != nil || len(*w.Value) == 0 || len(*w.Value) > 8192 {
					return invalidConfiguration()
				}
			case "preserve", "secretRef":
				if w.Value != nil || w.SecretRef == nil || !bindingUUID.MatchString(w.SecretRef.SecretID) {
					return invalidConfiguration()
				}
			case "remove":
				if w.Value != nil || w.SecretRef != nil {
					return invalidConfiguration()
				}
			default:
				return invalidConfiguration()
			}
		}
	}
	if b := c.Body; b != nil {
		if b.Type != "text" && b.Type != "json" {
			return invalidConfiguration()
		}
		if b.Sensitive {
			if b.Text != "" || len(b.SecretFields) > 0 {
				return invalidConfiguration()
			}
		} else if len(b.SecretFields) > 0 {
			if b.Type != "json" || uniqueJSON([]byte(b.Text)) != nil {
				return invalidConfiguration()
			}
			var v any
			d := json.NewDecoder(strings.NewReader(b.Text))
			d.UseNumber()
			if d.Decode(&v) != nil {
				return invalidConfiguration()
			}
			paths := map[string]bool{}
			for _, f := range b.SecretFields {
				if f.SecretWrite != nil && f.SecretWrite.Action == "remove" {
					continue
				}
				if paths[f.Pointer] {
					return invalidConfiguration()
				}
				paths[f.Pointer] = true
				leaf, ok := pointerValue(v, f.Pointer)
				if !ok || leaf != nil {
					return invalidConfiguration()
				}
				for p := range paths {
					if p != f.Pointer && (strings.HasPrefix(p, f.Pointer+"/") || strings.HasPrefix(f.Pointer, p+"/")) {
						return invalidConfiguration()
					}
				}
			}
		}
	}
	return nil
}
func pointerTokens(p string) ([]string, bool) {
	if p == "" {
		return []string{}, true
	}
	if !strings.HasPrefix(p, "/") {
		return nil, false
	}
	out := strings.Split(p[1:], "/")
	for i, s := range out {
		for n := 0; n < len(s); n++ {
			if s[n] == '~' {
				if n+1 == len(s) || (s[n+1] != '0' && s[n+1] != '1') {
					return nil, false
				}
				n++
			}
		}
		out[i] = strings.ReplaceAll(strings.ReplaceAll(s, "~1", "/"), "~0", "~")
	}
	return out, true
}
func pointerValue(v any, p string) (any, bool) {
	tokens, ok := pointerTokens(p)
	if !ok {
		return nil, false
	}
	for _, t := range tokens {
		switch a := v.(type) {
		case map[string]any:
			v, ok = a[t]
			if !ok {
				return nil, false
			}
		case []any:
			i, e := strconv.Atoi(t)
			if e != nil || i < 0 || strconv.Itoa(i) != t || i >= len(a) {
				return nil, false
			}
			v = a[i]
		default:
			return nil, false
		}
	}
	return v, true
}
func publicBodyJSON(v any, fields []SecretField) bool {
	paths := map[string]bool{}
	for _, f := range fields {
		paths[f.Pointer] = true
	}
	var walk func(any, string) bool
	walk = func(v any, p string) bool {
		if paths[p] {
			return v == nil
		}
		switch a := v.(type) {
		case map[string]any:
			for k, v := range a {
				q := p + "/" + strings.ReplaceAll(strings.ReplaceAll(k, "~", "~0"), "/", "~1")
				if credentialName(k) && !paths[q] {
					return false
				}
				if !walk(v, q) {
					return false
				}
			}
		case []any:
			for i, v := range a {
				if !walk(v, p+"/"+strconv.Itoa(i)) {
					return false
				}
			}
		case string:
			return !obviousCredential(a)
		}
		return true
	}
	return walk(v, "")
}

func configurationSize(c RequestConfiguration) (int, error) {
	c.Headers = append([]RequestField{}, c.Headers...)
	c.QueryParameters = append([]RequestField{}, c.QueryParameters...)
	if c.Body != nil {
		b := *c.Body
		b.SecretFields = append([]SecretField{}, b.SecretFields...)
		c.Body = &b
	}
	for _, f := range slots(&c) {
		if *f.write != nil {
			w := *f.write
			*f.write = nil
			*f.masked = true
			if w.SecretRef != nil {
				*f.ref = w.SecretRef
			} else {
				*f.ref = &SecretReference{"00000000-0000-4000-8000-000000000000"}
			}
		}
	}
	raw, err := configurationJSON(c)
	return len(raw), err
}
