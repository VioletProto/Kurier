package openapi

import (
	"bytes"
	"encoding/json"
	"io"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/oasdiff/yaml3"
)

var decimal = regexp.MustCompile(`^-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?$`)

func strictJSON(raw []byte) (any, error) {
	if !utf8.Valid(raw) || !json.Valid(raw) || !unicodeEscapes(raw) {
		return nil, fail("syntax_invalid")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	work := 0
	var read func(int) (any, error)
	read = func(depth int) (any, error) {
		work++
		if depth > 32 || work > 20000 {
			return nil, fail("structure_limit")
		}
		t, e := d.Token()
		if e != nil {
			return nil, fail("syntax_invalid")
		}
		switch t {
		case json.Delim('{'):
			m := map[string]any{}
			for d.More() {
				k, e := d.Token()
				if e != nil {
					return nil, fail("syntax_invalid")
				}
				s, ok := k.(string)
				if !ok {
					return nil, fail("syntax_invalid")
				}
				if _, ok = m[s]; ok {
					return nil, fail("key_invalid")
				}
				v, e := read(depth + 1)
				if e != nil {
					return nil, e
				}
				m[s] = v
			}
			if t, e = d.Token(); e != nil || t != json.Delim('}') {
				return nil, fail("syntax_invalid")
			}
			return m, nil
		case json.Delim('['):
			a := []any{}
			for d.More() {
				v, e := read(depth + 1)
				if e != nil {
					return nil, e
				}
				a = append(a, v)
			}
			if t, e = d.Token(); e != nil || t != json.Delim(']') {
				return nil, fail("syntax_invalid")
			}
			return a, nil
		default:
			if n, ok := t.(json.Number); ok && !boundedNumber(string(n)) {
				return nil, fail("numeric_unsupported")
			}
			return t, nil
		}
	}
	v, e := read(0)
	if e != nil {
		return nil, e
	}
	if _, e = d.Token(); e != io.EOF {
		return nil, fail("syntax_invalid")
	}
	return v, nil
}

// encoding/json otherwise replaces invalid surrogate pairs with U+FFFD.
func unicodeEscapes(raw []byte) bool {
	in := false
	for i := 0; i < len(raw); i++ {
		if raw[i] == '"' {
			in = !in
			continue
		}
		if !in || raw[i] != '\\' {
			continue
		}
		i++
		if i >= len(raw) {
			return false
		}
		if raw[i] != 'u' {
			continue
		}
		if i+4 >= len(raw) {
			return false
		}
		n, e := strconv.ParseUint(string(raw[i+1:i+5]), 16, 16)
		if e != nil {
			return false
		}
		i += 4
		if n >= 0xdc00 && n <= 0xdfff {
			return false
		}
		if n >= 0xd800 && n <= 0xdbff {
			if i+6 >= len(raw) || raw[i+1] != '\\' || raw[i+2] != 'u' {
				return false
			}
			m, e := strconv.ParseUint(string(raw[i+3:i+7]), 16, 16)
			if e != nil || m < 0xdc00 || m > 0xdfff {
				return false
			}
			i += 6
		}
	}
	return true
}

func parseSyntax(raw []byte, format string) (any, error) {
	if len(raw) == 0 {
		return nil, fail("syntax_invalid")
	}
	if len(raw) > SourceLimit {
		return nil, fail("source_limit")
	}
	if format == "json" || format == "application/json" {
		return strictJSON(raw)
	}
	if format != "yaml" && format != "yml" && format != "application/yaml" && format != "text/yaml" {
		return nil, fail("format_unsupported")
	}
	if !utf8.Valid(raw) {
		return nil, fail("syntax_invalid")
	}
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	var doc, extra yaml.Node
	if dec.Decode(&doc) != nil || dec.Decode(&extra) != io.EOF || len(doc.Content) != 1 {
		return nil, fail("syntax_invalid")
	}
	work := 0
	var value func(*yaml.Node, int) (any, error)
	value = func(n *yaml.Node, depth int) (any, error) {
		work++
		if work > 20000 || depth > 32 {
			return nil, fail("structure_limit")
		}
		if n.Anchor != "" || n.Kind == yaml.AliasNode {
			return nil, fail("yaml_alias_unsupported")
		}
		switch n.Kind {
		case yaml.MappingNode:
			if n.Tag != "!!map" || len(n.Content)%2 != 0 {
				return nil, fail("shape_invalid")
			}
			m := map[string]any{}
			for i := 0; i < len(n.Content); i += 2 {
				k := n.Content[i]
				if k.Kind != yaml.ScalarNode || k.Tag != "!!str" || k.Anchor != "" || k.Value == "<<" {
					return nil, fail("key_invalid")
				}
				if _, ok := m[k.Value]; ok {
					return nil, fail("key_invalid")
				}
				v, e := value(n.Content[i+1], depth+1)
				if e != nil {
					return nil, e
				}
				m[k.Value] = v
			}
			return m, nil
		case yaml.SequenceNode:
			if n.Tag != "!!seq" {
				return nil, fail("shape_invalid")
			}
			a := []any{}
			for _, child := range n.Content {
				v, e := value(child, depth+1)
				if e != nil {
					return nil, e
				}
				a = append(a, v)
			}
			return a, nil
		case yaml.ScalarNode:
			switch n.Tag {
			case "!!str":
				return n.Value, nil
			case "!!null":
				return nil, nil
			case "!!bool":
				switch strings.ToLower(n.Value) {
				case "true":
					return true, nil
				case "false":
					return false, nil
				}
			case "!!int", "!!float":
				if boundedNumber(n.Value) {
					return json.Number(n.Value), nil
				}
			}
		}
		return nil, fail("scalar_unsupported")
	}
	return value(doc.Content[0], 0)
}

func boundedNumber(token string) bool {
	if len(token) == 0 || len(token) > 128 || !decimal.MatchString(token) {
		return false
	}
	if i := strings.IndexAny(token, "eE"); i >= 0 {
		n, e := strconv.Atoi(token[i+1:])
		if e != nil || n < -308 || n > 308 {
			return false
		}
	}
	n, ok := new(big.Rat).SetString(token)
	if !ok {
		return false
	}
	n.Abs(n)
	maximum := new(big.Rat).SetInt64(9007199254740991)
	minimum, _ := new(big.Rat).SetString("1e-308")
	return n.Cmp(maximum) <= 0 && (n.Sign() == 0 || n.Cmp(minimum) >= 0)
}
