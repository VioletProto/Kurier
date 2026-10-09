package execution

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestCredentialNamesWithoutProtectedInputs(t *testing.T) {
	for _, name := range []string{"key", "KEY", "K_E-Y", "auth", "Auth", "a.u_t-h", "pwd", "PWD", "p-w_d", "X-Api-Key", "Set-Cookie"} {
		t.Run(name, func(t *testing.T) {
			san := newSanitizer(map[string]string{})
			input, err := json.Marshal(map[string]any{
				"nested": []any{map[string]any{name: "unmapped-json-value"}},
				"public": "keep-json", "monkey": "keep-noncredential",
			})
			if err != nil {
				t.Fatal(err)
			}
			body := san.body(input, "application/json", RedactionPolicy{})
			if body.Text == nil {
				t.Fatal("expected sanitized JSON")
			}
			var decoded map[string]any
			if err := json.Unmarshal([]byte(*body.Text), &decoded); err != nil {
				t.Fatal(err)
			}
			field := decoded["nested"].([]any)[0].(map[string]any)[name]
			if field != "[REDACTED]" || decoded["public"] != "keep-json" || decoded["monkey"] != "keep-noncredential" {
				t.Fatal("credential JSON field was not masked independently of protected inputs")
			}
			headers := sanitizeHeaders(http.Header{name: {"unmapped-header-value", "another-unmapped-header-value"}, "Monkey": {"keep-header"}}, san, nil)
			for _, h := range headers {
				if h.Name == name && h.Value != "[REDACTED]" || h.Name == "Monkey" && h.Value != "keep-header" {
					t.Fatal("credential header masking or public header preservation failed")
				}
			}
			source := &url.URL{Scheme: "https", Host: "fixture.example", RawQuery: url.Values{name: {"unmapped-query-value", "another-unmapped-query-value"}, "monkey": {"keep-query"}}.Encode()}
			masked, err := url.Parse(san.url(source))
			if err != nil {
				t.Fatal(err)
			}
			query := masked.Query()
			if len(query[name]) != 2 || query[name][0] != "[REDACTED]" || query[name][1] != "[REDACTED]" || query.Get("monkey") != "keep-query" {
				t.Fatal("credential URL query masking or public query preservation failed")
			}
		})
	}
}

func TestExactCredentialNamesMaskJSONSubtreesAndScalars(t *testing.T) {
	san := newSanitizer(nil)
	body := san.body([]byte(`{"key":{"nested":"unmapped-value"},"auth":12345,"pwd":null,"rows":[{"KEY":true}],"monkey":"keep","author":"keep","pwdHint":"keep"}`), "application/json", RedactionPolicy{})
	if body.Text == nil {
		t.Fatal("expected sanitized JSON")
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(*body.Text), &decoded); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"key", "auth", "pwd"} {
		if decoded[name] != "[REDACTED]" {
			t.Fatal("credential subtree or scalar survived")
		}
	}
	if decoded["rows"].([]any)[0].(map[string]any)["KEY"] != "[REDACTED]" {
		t.Fatal("credential inside array survived")
	}
	for _, name := range []string{"monkey", "author", "pwdHint"} {
		if decoded[name] != "keep" {
			t.Fatal("exact credential aliases incorrectly matched a longer name")
		}
	}
	if strings.Contains(*body.Text, "unmapped-value") || strings.Contains(*body.Text, "12345") {
		t.Fatal("credential data survived JSON sanitization")
	}
}
