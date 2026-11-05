package ownership

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	oapi "github.com/VioletProto/Kurier/services/request-runtime/openapi"
)

func TestOpenAPIDetailPagingMeasuresEscapedEnvelope(t *testing.T) {
	choices := oapi.Choices{Body: json.RawMessage(strconvQuote(strings.Repeat("<", 10000)))}
	encoded, e := json.Marshal(choices)
	if e != nil || len(encoded) > 64*1024 {
		t.Fatal("invalid bounded fixture")
	}
	candidate := OpenAPICandidate{Operation: oapi.Operation{OperationID: "op-fixture", Method: "POST", Path: "/items"}, Choices: choices, ChoicesJSON: string(encoded), RequestID: "request-fixture", Ready: true}
	value := OpenAPIImport{OpenAPIImportSummary: OpenAPIImportSummary{ImportID: "import-fixture", OperationCount: 100, Revision: 7}, ReadyOperationCount: 100}
	for n := 0; n < 100; n++ {
		value.Operations = append(value.Operations, candidate)
	}
	response := httptest.NewRecorder()
	if e = openapiResponse(response, 200, value, 0); e != nil {
		t.Fatal(e)
	}
	if response.Body.Len() > 2*1024*1024 {
		t.Fatal("escaped envelope exceeds bound")
	}
	var decoded struct {
		Data struct {
			Import OpenAPIImport `json:"import"`
		} `json:"data"`
	}
	if json.Unmarshal(response.Body.Bytes(), &decoded) != nil {
		t.Fatal("invalid JSON")
	}
	page := decoded.Data.Import
	if page.NextOperationOffset == nil || *page.NextOperationOffset != len(page.Operations) || len(page.Operations) < 1 || len(page.Operations) >= 20 || page.ReadyOperationCount != 100 || page.OperationCount != 100 {
		t.Fatal("page silently lost candidates or total readiness")
	}
	response = httptest.NewRecorder()
	if e = openapiResponse(response, 200, value, *page.NextOperationOffset); e != nil || response.Body.Len() > 2*1024*1024 {
		t.Fatal("second page invalid", e)
	}
	if e = openapiResponse(httptest.NewRecorder(), 200, value, 101); e == nil {
		t.Fatal("invalid offset accepted")
	}
}
func strconvQuote(s string) string { b, _ := json.Marshal(s); return string(b) }
func TestOpenAPIJSONTransportBoundIsFixedAndSafe(t *testing.T) {
	r := httptest.NewRequest("POST", "/", strings.NewReader(`{"source":"`+strings.Repeat("sensitive-canary", 100)+`"}`))
	r.Header.Set("Content-Type", "application/json")
	_, e := readOpenAPIJSON(httptest.NewRecorder(), r, 50)
	ae, ok := e.(*APIError)
	if !ok || ae.Status != 413 || strings.Contains(ae.Message, "sensitive-canary") {
		t.Fatal("unsafe transport rejection")
	}
}
func TestOpenAPIChoicesPublicBoundary(t *testing.T) {
	for _, choices := range []oapi.Choices{{Body: json.RawMessage(`{"password":"private-canary"}`)}, {Parameters: map[string]json.RawMessage{"p0": json.RawMessage(`"Bearer private-canary"`)}}} {
		if publicOpenAPIChoices(choices) {
			t.Fatal("sensitive choices accepted")
		}
	}
}

func TestOpenAPIEnvelopeRejectsCaseAliasesAndMissingRequiredFields(t *testing.T) {
	var input struct {
		Format              string `json:"format"`
		Source              string `json:"source"`
		PublicDataConfirmed bool   `json:"publicDataConfirmed"`
	}
	for _, raw := range []string{`{"Format":"json","source":"{}","publicDataConfirmed":true}`, `{"format":"json","source":"{}","publicDataConfirmed":false,"PublicDataConfirmed":true}`, `{"format":"json","source":"{}"}`, `{"format":"json","source":"{}","publicDataConfirmed":null}`} {
		if decodeOpenAPIInput([]byte(raw), &input, []string{"format", "source", "publicDataConfirmed"}, nil) == nil {
			t.Fatal("invalid closed envelope accepted")
		}
	}
}

func TestOpenAPIChoicesRejectCaseAliasesButPreserveNullableBody(t *testing.T) {
	for _, raw := range []string{`{"Parameters":{}}`, `{"parameters":{},"Parameters":{}}`, `{"secretRefs":null}`, `{"future":true}`} {
		if _, e := decodeOpenAPIChoices([]byte(raw)); e == nil {
			t.Fatal("invalid closed choice accepted")
		}
	}
	choices, e := decodeOpenAPIChoices([]byte(`{"body":null}`))
	if e != nil || string(choices.Body) != "null" {
		t.Fatal("explicit JSON null body lost")
	}
}
