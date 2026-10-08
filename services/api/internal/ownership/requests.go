package ownership

import (
	"bytes"
	"encoding/json"
	"io"
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const savedConfigurationLimit = 64 * 1024

type RequestField struct {
	Name        string           `json:"name"`
	Value       string           `json:"value"`
	Enabled     bool             `json:"enabled"`
	Sensitive   bool             `json:"sensitive"`
	BindingID   string           `json:"bindingId,omitempty"`
	Masked      bool             `json:"masked,omitempty"`
	SecretRef   *SecretReference `json:"secretRef,omitempty"`
	SecretWrite *SecretWrite     `json:"secretWrite,omitempty"`
}
type SecretReference struct {
	SecretID string `json:"secretId"`
}
type SecretWrite struct {
	Action    string           `json:"action"`
	Value     *string          `json:"value,omitempty"`
	SecretRef *SecretReference `json:"secretRef,omitempty"`
}
type SecretField struct {
	Pointer     string           `json:"pointer"`
	BindingID   string           `json:"bindingId"`
	Masked      bool             `json:"masked,omitempty"`
	SecretRef   *SecretReference `json:"secretRef,omitempty"`
	SecretWrite *SecretWrite     `json:"secretWrite,omitempty"`
}
type RequestBody struct {
	Type         string           `json:"type"`
	Text         string           `json:"text"`
	SecretFields []SecretField    `json:"secretFields,omitempty"`
	Sensitive    bool             `json:"sensitive"`
	BindingID    string           `json:"bindingId,omitempty"`
	Masked       bool             `json:"masked,omitempty"`
	SecretRef    *SecretReference `json:"secretRef,omitempty"`
	SecretWrite  *SecretWrite     `json:"secretWrite,omitempty"`
}
type RequestConfiguration struct {
	Name            string         `json:"name"`
	Method          string         `json:"method"`
	URL             string         `json:"url"`
	QueryParameters []RequestField `json:"queryParameters"`
	Headers         []RequestField `json:"headers"`
	Body            *RequestBody   `json:"body"`
	OperationRef    any            `json:"operationRef"`
}
type SavedRequest struct {
	RequestID string `json:"requestId"`
	ProjectID string `json:"projectId"`
	Revision  int64  `json:"revision"`
	RequestConfiguration
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

func configurationJSON(c RequestConfiguration) ([]byte, error) {
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(c); err != nil {
		return nil, unavailable()
	}
	return bytes.TrimSuffix(out.Bytes(), []byte("\n")), nil
}
func invalidConfiguration() error {
	return apiError(400, "validation_failed", "Use a public HTTP(S) URL, supported method, public headers/query and a text or JSON body. Use protected inputs for credentials and designated sensitive fields.")
}

var headerToken = regexp.MustCompile("^[!#$%&'*+.^_`|~0-9A-Za-z-]+$")

// Case/separator normalization catches common credential aliases. This is a
// conservative named-field boundary, not a promise to identify arbitrary secrets.
func credentialName(name string) bool {
	name = strings.ToLower(name)
	name = strings.NewReplacer("-", "", "_", "", ".", "", " ", "").Replace(name)
	for _, part := range []string{"authorization", "cookie", "apikey", "token", "secret", "password", "passwd", "credential", "privatekey", "signature", "sessionid"} {
		if strings.Contains(name, part) {
			return true
		}
	}
	return name == "key" || name == "auth" || name == "pwd"
}
func control(value string, allowTab bool) bool {
	for _, c := range value {
		if unicode.IsControl(c) && !(allowTab && c == '\t') {
			return true
		}
	}
	return false
}

// uniqueJSON bounds nesting and rejects duplicate keys before ordinary decoding.
func uniqueJSON(raw []byte) error {
	if !utf8.Valid(raw) {
		return apiError(400, "invalid_request", "Invalid JSON.")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var value func(int) error
	value = func(depth int) error {
		if depth > 32 {
			return invalidConfiguration()
		}
		token, e := d.Token()
		if e != nil {
			return e
		}
		delim, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		if delim == '{' {
			seen := map[string]bool{}
			for d.More() {
				k, e := d.Token()
				if e != nil {
					return e
				}
				name, ok := k.(string)
				if !ok || seen[name] {
					return invalidConfiguration()
				}
				seen[name] = true
				if e = value(depth + 1); e != nil {
					return e
				}
			}
			end, e := d.Token()
			if e != nil || end != json.Delim('}') {
				return invalidConfiguration()
			}
		} else if delim == '[' {
			for d.More() {
				if e = value(depth + 1); e != nil {
					return e
				}
			}
			end, e := d.Token()
			if e != nil || end != json.Delim(']') {
				return invalidConfiguration()
			}
		} else {
			return invalidConfiguration()
		}
		return nil
	}
	if err := value(0); err != nil {
		return apiError(400, "invalid_request", "Invalid JSON, duplicate keys or excessive nesting.")
	}
	if _, err := d.Token(); err != io.EOF {
		return apiError(400, "invalid_request", "Invalid JSON.")
	}
	return nil
}
func publicJSON(value any) bool {
	switch v := value.(type) {
	case map[string]any:
		for key, value := range v {
			if credentialName(key) || !publicJSON(value) {
				return false
			}
		}
	case []any:
		for _, value := range v {
			if !publicJSON(value) {
				return false
			}
		}
	case string:
		return !obviousCredential(v)
	}
	return true
}
func obviousCredential(value string) bool {
	lower := strings.ToLower(value)
	return strings.Contains(lower, "bearer ") || strings.Contains(lower, "-----begin") || strings.HasPrefix(lower, "basic ")
}
func validateConfiguration(c *RequestConfiguration) error {
	if control(c.Name, false) {
		return invalidConfiguration()
	}
	c.Name = strings.TrimSpace(c.Name)
	if utf8.RuneCountInString(c.Name) < 1 || utf8.RuneCountInString(c.Name) > 100 || control(c.Name, false) {
		return invalidConfiguration()
	}
	switch c.Method {
	case "GET", "POST", "PUT", "PATCH", "DELETE":
	default:
		return invalidConfiguration()
	}
	u, err := url.Parse(c.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.Fragment != "" || strings.Contains(c.URL, "#") || control(c.URL, false) || strings.ContainsAny(c.URL, " {}") {
		return invalidConfiguration()
	}
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return invalidConfiguration()
	}
	for name, values := range query {
		if credentialName(name) {
			return invalidConfiguration()
		}
		for _, v := range values {
			if obviousCredential(v) {
				return invalidConfiguration()
			}
		}
	}
	if c.OperationRef != nil || c.QueryParameters == nil || c.Headers == nil || len(c.Headers) > 100 || len(c.QueryParameters) > 100 {
		return invalidConfiguration()
	}
	for _, entry := range []struct {
		fields []RequestField
		header bool
	}{{c.Headers, true}, {c.QueryParameters, false}} {
		for _, f := range entry.fields {
			if f.Sensitive && f.Value != "" {
				return invalidConfiguration()
			}
			if !f.Sensitive && (f.BindingID != "" || f.SecretWrite != nil || f.SecretRef != nil || f.Masked) {
				return invalidConfiguration()
			}
			if f.Name == "" || control(f.Name, false) || control(f.Value, entry.header) || (!f.Sensitive && (credentialName(f.Name) || obviousCredential(f.Value))) || (entry.header && !headerToken.MatchString(f.Name)) {
				return invalidConfiguration()
			}
		}
	}
	if err := validateSlots(c); err != nil {
		return err
	}
	if c.Body != nil && !c.Body.Sensitive {
		if !c.Body.Sensitive && obviousCredential(c.Body.Text) {
			return invalidConfiguration()
		}
		switch c.Body.Type {
		case "text":
		case "json":
			if uniqueJSON([]byte(c.Body.Text)) != nil {
				return invalidConfiguration()
			}
			var v any
			d := json.NewDecoder(strings.NewReader(c.Body.Text))
			d.UseNumber()
			if d.Decode(&v) != nil || (!c.Body.Sensitive && !publicBodyJSON(v, c.Body.SecretFields)) {
				return invalidConfiguration()
			}
		default:
			return invalidConfiguration()
		}
	}
	size, err := configurationSize(*c)
	if err != nil {
		return err
	}
	if size > savedConfigurationLimit {
		return apiError(413, "payload_too_large", "Complete saved configuration exceeds 64 KiB, including URL, headers, query, body and JSON overhead.")
	}
	return nil
}

// mergeConfiguration also validates descriptor presence/nulls; Go's ordinary
// struct decoder otherwise accepts null booleans/strings as zero values.
func mergeConfiguration(raw []byte, previous *RequestConfiguration) (RequestConfiguration, error) {
	return mergeConfigurationMode(raw, previous, false)
}
func mergeConfigurationMode(raw []byte, previous *RequestConfiguration, stored bool) (RequestConfiguration, error) {
	c := RequestConfiguration{QueryParameters: []RequestField{}, Headers: []RequestField{}}
	if previous != nil {
		c = *previous
	}
	if err := uniqueJSON(raw); err != nil {
		return c, err
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil || len(fields) == 0 {
		return c, invalidConfiguration()
	}
	if previous == nil {
		for _, k := range []string{"name", "method", "url"} {
			if _, ok := fields[k]; !ok {
				return c, invalidConfiguration()
			}
		}
	}
	for k, v := range fields {
		switch k {
		case "name", "method", "url":
			var value string
			if bytes.Equal(v, []byte("null")) || json.Unmarshal(v, &value) != nil {
				return c, invalidConfiguration()
			}
			switch k {
			case "name":
				c.Name = value
			case "method":
				c.Method = value
			case "url":
				c.URL = value
			}
		case "headers", "queryParameters":
			var entries []map[string]json.RawMessage
			if json.Unmarshal(v, &entries) != nil || entries == nil {
				return c, invalidConfiguration()
			}
			for _, entry := range entries {
				if err := descriptor(entry, false, stored); err != nil {
					return c, err
				}
			}
			var values []RequestField
			if strictDecode(v, &values) != nil {
				return c, invalidConfiguration()
			}
			if k == "headers" {
				c.Headers = values
			} else {
				c.QueryParameters = values
			}
		case "body":
			if bytes.Equal(v, []byte("null")) {
				c.Body = nil
				continue
			}
			var body map[string]json.RawMessage
			if json.Unmarshal(v, &body) != nil {
				return c, invalidConfiguration()
			}
			if v, ok := body["secretFields"]; ok {
				var entries []map[string]json.RawMessage
				if json.Unmarshal(v, &entries) != nil || entries == nil {
					return c, invalidConfiguration()
				}
				for _, entry := range entries {
					expected := 3
					if stored {
						expected = 4
					}
					if len(entry) != expected {
						return c, invalidConfiguration()
					}
					for _, k := range []string{"pointer", "bindingId"} {
						if value, ok := entry[k]; !ok || bytes.Equal(value, []byte("null")) {
							return c, invalidConfiguration()
						}
					}
					if stored {
						if _, ok := entry["secretRef"]; !ok {
							return c, invalidConfiguration()
						}
					} else {
						if _, ok := entry["secretWrite"]; !ok {
							return c, invalidConfiguration()
						}
					}
				}
			}
			if err := descriptor(body, true, stored); err != nil {
				return c, err
			}
			var parsed RequestBody
			if strictDecode(v, &parsed) != nil {
				return c, invalidConfiguration()
			}
			c.Body = &parsed
		case "operationRef":
			if !bytes.Equal(v, []byte("null")) {
				return c, invalidConfiguration()
			}
			c.OperationRef = nil
		default:
			return c, invalidConfiguration()
		}
	}
	err := validateConfiguration(&c)
	return c, err
}
func (r record) savedRequest() (SavedRequest, error) {
	if r.Kind != "request" || r.State != "active" || r.RequestID == "" || r.SK != "REQ#"+r.RequestID || r.PK != "P#"+r.ProjectID || r.Revision < 0 {
		return SavedRequest{}, missing()
	}
	c, err := mergeConfigurationMode([]byte(r.ConfigurationJSON), nil, true)
	if err != nil {
		return SavedRequest{}, unavailable()
	}
	return SavedRequest{r.RequestID, r.ProjectID, r.Revision, c, r.CreatedAt, r.UpdatedAt}, nil
}
