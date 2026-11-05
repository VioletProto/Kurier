// Package openapi implements Kurier's bounded, network-free OpenAPI subset.
// Uploaded text and validator errors must never be used as diagnostic messages.
package openapi

import "encoding/json"

const SourceLimit = 2 << 20
const OperationLimit = 100
const BundleLimit = 32 << 10

type Error struct {
	Code string `json:"code"`
}

func (e *Error) Error() string { return e.Code }
func fail(code string) error   { return &Error{Code: code} }

type Policy struct {
	SensitiveValues       []string `json:"sensitiveValues,omitempty"`
	SensitiveJSONPointers []string `json:"sensitiveJsonPointers,omitempty"`
}
type Diagnostic struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
}
type Document struct {
	Source      json.RawMessage `json:"source"`
	Operations  []Operation     `json:"operations"`
	Diagnostics []Diagnostic    `json:"diagnostics"`
}
type Schema struct {
	Type                 string            `json:"type"`
	Nullable             bool              `json:"nullable,omitempty"`
	Required             []string          `json:"required,omitempty"`
	Properties           map[string]Schema `json:"properties,omitempty"`
	Items                *Schema           `json:"items,omitempty"`
	AdditionalProperties *bool             `json:"additionalProperties,omitempty"`
}
type Parameter struct {
	InputID   string          `json:"inputId"`
	Name      string          `json:"name"`
	In        string          `json:"in"`
	Required  bool            `json:"required"`
	Sensitive bool            `json:"sensitive"`
	Schema    Schema          `json:"schema"`
	Example   json.RawMessage `json:"example,omitempty"`
}
type BodyDefinition struct {
	Required bool            `json:"required"`
	Schema   Schema          `json:"schema"`
	Example  json.RawMessage `json:"example,omitempty"`
}
type HeaderDefinition struct {
	Required bool   `json:"required"`
	Schema   Schema `json:"schema"`
}
type Response struct {
	Headers map[string]HeaderDefinition `json:"headers,omitempty"`
	Schema  *Schema                     `json:"schema,omitempty"`
}
type Operation struct {
	OperationID string              `json:"operationId"`
	Method      string              `json:"method"`
	Path        string              `json:"path"`
	Servers     []string            `json:"servers"`
	Parameters  []Parameter         `json:"parameters"`
	RequestBody *BodyDefinition     `json:"requestBody,omitempty"`
	Responses   map[string]Response `json:"responses"`
}
type Choices struct {
	Server        string                     `json:"server,omitempty"`
	Parameters    map[string]json.RawMessage `json:"parameters,omitempty"`
	SecretRefs    map[string]string          `json:"secretRefs,omitempty"`
	Body          json.RawMessage            `json:"body,omitempty"`
	BodySecretRef string                     `json:"bodySecretRef,omitempty"`
}
type Ref struct {
	SecretID string `json:"secretId"`
}
type Field struct {
	Name      string `json:"name"`
	Value     string `json:"value,omitempty"`
	Enabled   bool   `json:"enabled"`
	Sensitive bool   `json:"sensitive"`
	SecretRef *Ref   `json:"secretRef,omitempty"`
}
type RequestBody struct {
	Type      string `json:"type"`
	Text      string `json:"text,omitempty"`
	Sensitive bool   `json:"sensitive"`
	SecretRef *Ref   `json:"secretRef,omitempty"`
}
type Generated struct {
	Name            string       `json:"name"`
	Method          string       `json:"method"`
	URL             string       `json:"url"`
	QueryParameters []Field      `json:"queryParameters"`
	Headers         []Field      `json:"headers"`
	Body            *RequestBody `json:"body"`
}
type Check struct {
	Kind   string `json:"kind"`
	Status string `json:"status"`
	Code   string `json:"code,omitempty"`
}
type Result struct {
	SemanticsVersion int     `json:"semanticsVersion"`
	Status           string  `json:"status"`
	Checks           []Check `json:"checks"`
}
