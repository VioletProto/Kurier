package ownership

import (
	"encoding/json"
	"strings"
	"testing"
)

func publicConfiguration() RequestConfiguration {
	return RequestConfiguration{Name: "Public", Method: "GET", URL: "https://example.com/public", QueryParameters: []RequestField{}, Headers: []RequestField{}}
}
func TestSavedConfigurationValidationAndCompleteLimit(t *testing.T) {
	c := publicConfiguration()
	c.Name = "\tPublic"
	assertStatus(t, validateConfiguration(&c), 400)
	c = publicConfiguration()
	c.Body = &RequestBody{Type: "text", Text: ""}
	raw, _ := configurationJSON(c)
	c.Body.Text = strings.Repeat("x", savedConfigurationLimit-len(raw))
	if err := validateConfiguration(&c); err != nil {
		t.Fatal(err)
	}
	raw, _ = configurationJSON(c)
	if len(raw) != 65536 {
		t.Fatal(len(raw))
	}
	c.Headers = []RequestField{{Name: "Accept", Value: "application/json", Enabled: true}}
	assertStatus(t, validateConfiguration(&c), 413)
	c = publicConfiguration()
	c.Body = &RequestBody{Type: "text", Text: strings.Repeat("\"", 40000)}
	assertStatus(t, validateConfiguration(&c), 413)
	for _, url := range []string{"ftp://example.com", "https://user:pass@example.com", "https://example.com/#fragment", "https://example.com/?api_key=fixture", "https://example.com/?access_token=fixture", "{{base}}/users"} {
		c = publicConfiguration()
		c.URL = url
		assertStatus(t, validateConfiguration(&c), 400)
	}
	for _, name := range []string{"Authorization", "Cookie", "Set-Cookie", "X-API-Key", "x-auth-token", "X-Amz-Security-Token", "Password"} {
		c = publicConfiguration()
		c.Headers = []RequestField{{Name: name, Value: "fixture", Enabled: false}}
		assertStatus(t, validateConfiguration(&c), 400)
	}
	c = publicConfiguration()
	c.Body = &RequestBody{Type: "json", Text: `{"password":"fixture"}`}
	assertStatus(t, validateConfiguration(&c), 400)
	c = publicConfiguration()
	c.Body = &RequestBody{Type: "json", Text: ` { "ok" : [1,2] } `}
	if err := validateConfiguration(&c); err != nil {
		t.Fatal(err)
	}
	if c.Body.Text != ` { "ok" : [1,2] } ` {
		t.Fatal("body rewritten")
	}
}
func TestRequestPatchClearingReplacementAndStrictJSON(t *testing.T) {
	c := publicConfiguration()
	c.Headers = []RequestField{{Name: "Accept", Value: "text/plain", Enabled: true}}
	c.Body = &RequestBody{Type: "text", Text: "public"}
	next, err := mergeConfiguration([]byte(`{"name":"  New  "}`), &c)
	if err != nil || next.Name != "New" || next.Body.Text != "public" || len(next.Headers) != 1 {
		t.Fatal(next, err)
	}
	next, err = mergeConfiguration([]byte(`{"body":null,"headers":[],"queryParameters":[],"operationRef":null}`), &c)
	if err != nil || next.Body != nil || len(next.Headers) != 0 || next.Headers == nil {
		t.Fatal(next, err)
	}
	next, err = mergeConfiguration([]byte(`{"headers":[{"name":"Accept","value":"application/json","enabled":false,"sensitive":false}]}`), &c)
	if err != nil || len(next.Headers) != 1 || next.Headers[0].Value != "application/json" || next.Headers[0].Enabled {
		t.Fatal(next, err)
	}
	for _, raw := range []string{`{}`, `null`, `{"name":null}`, `{"headers":null}`, `{"method":"GET","method":"POST"}`, `{"unknown":1}`, `{"headers":[{"name":"Accept","value":"x","enabled":null,"sensitive":false}]}`, `{"headers":[{"name":"Accept","name":"Other","value":"x","enabled":true,"sensitive":false}]}`, `{"body":{"type":"text","text":"public","sensitive":true}}`, `{"headers":[{"name":"custom","value":"public","enabled":true,"sensitive":true}]}`, `{"body":{"type":"json","text":"{\"x\":1,\"x\":2}","sensitive":false}}`, `{"secretWrite":{"action":"set","value":"fixture"}}`, `{"secretRef":"fixture"}`, `{"operationRef":{"importId":"fixture"}}`, `{"name":"ok"} {}`} {
		_, err := mergeConfiguration([]byte(raw), &c)
		assertStatus(t, err, 400)
	}
	_, err = mergeConfiguration([]byte(`{"name":"x"}`), nil)
	assertStatus(t, err, 400)
	raw, _ := json.Marshal(publicConfiguration())
	if _, err = mergeConfiguration(raw, nil); err != nil {
		t.Fatal(err)
	}
}
