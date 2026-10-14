package execution

import (
	"bytes"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

// StrictJSON rejects duplicate keys, trailing data and excessive nesting before
// decoding typed fields. Diagnostics deliberately never include input bytes.
func StrictJSON(raw []byte, target any) error {
	if !utf8.Valid(raw) || !validUnicodeEscapes(raw) {
		return failure(400, "invalid_request")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var read func(int) (any, error)
	read = func(depth int) (any, error) {
		if depth > 32 {
			return nil, failure(400, "invalid_request")
		}
		t, e := d.Token()
		if e != nil {
			return nil, failure(400, "invalid_request")
		}
		delim, ok := t.(json.Delim)
		if !ok {
			return t, nil
		}
		switch delim {
		case '{':
			m := map[string]any{}
			for d.More() {
				k, e := d.Token()
				if e != nil {
					return nil, failure(400, "invalid_request")
				}
				key, ok := k.(string)
				if !ok {
					return nil, failure(400, "invalid_request")
				}
				if _, exists := m[key]; exists {
					return nil, failure(400, "invalid_request")
				}
				v, e := read(depth + 1)
				if e != nil {
					return nil, e
				}
				m[key] = v
			}
			end, e := d.Token()
			if e != nil || end != json.Delim('}') {
				return nil, failure(400, "invalid_request")
			}
			return m, nil
		case '[':
			a := []any{}
			for d.More() {
				v, e := read(depth + 1)
				if e != nil {
					return nil, e
				}
				a = append(a, v)
			}
			end, e := d.Token()
			if e != nil || end != json.Delim(']') {
				return nil, failure(400, "invalid_request")
			}
			return a, nil
		}
		return nil, failure(400, "invalid_request")
	}
	if _, e := read(0); e != nil {
		return e
	}
	if _, e := d.Token(); e != io.EOF {
		return failure(400, "invalid_request")
	}
	typed := json.NewDecoder(bytes.NewReader(raw))
	typed.DisallowUnknownFields()
	typed.UseNumber()
	if e := typed.Decode(target); e != nil {
		return failure(400, "invalid_request")
	}
	return nil
}
func pointerParts(p string) ([]string, error) {
	if p == "" {
		return nil, nil
	}
	if !strings.HasPrefix(p, "/") {
		return nil, failure(400, "invalid_request")
	}
	out := strings.Split(p[1:], "/")
	for i, v := range out {
		for j := 0; j < len(v); j++ {
			if v[j] == '~' {
				if j+1 == len(v) || (v[j+1] != '0' && v[j+1] != '1') {
					return nil, failure(400, "invalid_request")
				}
				j++
			}
		}
		out[i] = strings.ReplaceAll(strings.ReplaceAll(v, "~1", "/"), "~0", "~")
	}
	return out, nil
}
func setPointer(root any, p string, v any) (any, error) {
	parts, e := pointerParts(p)
	if e != nil {
		return nil, e
	}
	if len(parts) == 0 {
		return v, nil
	}
	cur := root
	for i, part := range parts {
		last := i == len(parts)-1
		switch x := cur.(type) {
		case map[string]any:
			next, ok := x[part]
			if !ok {
				return nil, failure(400, "invalid_request")
			}
			if last {
				x[part] = v
				return root, nil
			}
			cur = next
		case []any:
			n, e := strconv.Atoi(part)
			if e != nil || n < 0 || n >= len(x) || strconv.Itoa(n) != part {
				return nil, failure(400, "invalid_request")
			}
			if last {
				x[n] = v
				return root, nil
			}
			cur = x[n]
		default:
			return nil, failure(400, "invalid_request")
		}
	}
	return nil, failure(400, "invalid_request")
}

// Policies cannot contain null anywhere; null remains valid in upstream JSON.
func NonNullObject(raw []byte) error {
	var v any
	if err := StrictJSON(raw, &v); err != nil {
		return err
	}
	if _, ok := v.(map[string]any); !ok {
		return failure(400, "invalid_request")
	}
	var valid func(any) bool
	valid = func(v any) bool {
		switch x := v.(type) {
		case nil:
			return false
		case map[string]any:
			for _, child := range x {
				if !valid(child) {
					return false
				}
			}
		case []any:
			for _, child := range x {
				if !valid(child) {
					return false
				}
			}
		}
		return true
	}
	if !valid(v) {
		return failure(400, "invalid_request")
	}
	return nil
}
func ParseOptions(raw []byte) (Options, error) {
	o := Options{TimeoutSeconds: 30, ResponseRedaction: RedactionPolicy{Headers: []string{}, JSONPointers: []string{}}}
	if e := NonNullObject(raw); e != nil {
		return o, e
	}
	if e := StrictJSON(raw, &o); e != nil {
		return o, e
	}
	if o.Target == "cloud" {
		o.Target = ""
	}
	if (o.Target != "" && o.Target != "local") || (o.Target == "" && o.Local != nil) || (o.Target == "local" && (o.Local == nil || !o.Local.AllowPrivateNetwork || o.Local.LocalAgentID == "")) {
		return o, failure(400, "invalid_request")
	}
	if o.TimeoutSeconds < 1 || o.TimeoutSeconds > 60 || o.ResponseRedaction.Headers == nil || o.ResponseRedaction.JSONPointers == nil || len(o.ResponseRedaction.Headers) > 16 || len(o.ResponseRedaction.JSONPointers) > 16 {
		return o, failure(400, "invalid_request")
	}
	for _, h := range o.ResponseRedaction.Headers {
		if !headerName(h) {
			return o, failure(400, "invalid_request")
		}
	}
	for i, p := range o.ResponseRedaction.JSONPointers {
		if len(p) > 1024 {
			return o, failure(400, "invalid_request")
		}
		if _, e := pointerParts(p); e != nil {
			return o, e
		}
		for _, q := range o.ResponseRedaction.JSONPointers[:i] {
			if p == q || strings.HasPrefix(p, q+"/") || strings.HasPrefix(q, p+"/") {
				return o, failure(400, "invalid_request")
			}
		}
	}
	return o, nil
}

// encoding/json replaces unmatched UTF-16 surrogate escapes; reject them before
// canonical identity or sanitization rather than normalize malformed input.
func validUnicodeEscapes(raw []byte) bool {
	for i := 0; i < len(raw); i++ {
		if raw[i] != '\\' {
			continue
		}
		if i+1 >= len(raw) {
			return false
		}
		if raw[i+1] != 'u' {
			i++
			continue
		}
		if i+6 > len(raw) {
			return false
		}
		n, e := strconv.ParseUint(string(raw[i+2:i+6]), 16, 16)
		if e != nil {
			return false
		}
		if n >= 0xdc00 && n <= 0xdfff {
			return false
		}
		if n >= 0xd800 && n <= 0xdbff {
			if i+12 > len(raw) || string(raw[i+6:i+8]) != "\\u" {
				return false
			}
			low, e := strconv.ParseUint(string(raw[i+8:i+12]), 16, 16)
			if e != nil || low < 0xdc00 || low > 0xdfff {
				return false
			}
			i += 11
		} else {
			i += 5
		}
	}
	return true
}
