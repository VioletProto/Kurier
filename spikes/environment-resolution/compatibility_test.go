package environments

// Disposable DTO experiments, not production handlers or complete validators.
import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	execution "github.com/VioletProto/Kurier/services/request-runtime"
	wire "github.com/VioletProto/Kurier/services/request-runtime/mcpwire"
	"github.com/cyberphone/json-canonicalization/go/src/webpki.org/jsoncanonicalizer"
)

type contextFixture struct {
	EnvironmentID string `json:"environmentId"`
	Revision      int64  `json:"revision"`
}
type identityFixture struct {
	SchemaVersion            int             `json:"schemaVersion"`
	TemplateSemanticsVersion int             `json:"templateSemanticsVersion"`
	EnvironmentContext       *contextFixture `json:"environmentContext"`
}

func decodeIdentity(raw []byte, version int) (identityFixture, error) {
	var v identityFixture
	var keys map[string]json.RawMessage
	if execution.StrictJSON(raw, &keys) != nil || execution.StrictJSON(raw, &v) != nil || v.SchemaVersion != version || v.TemplateSemanticsVersion != 1 || keys["environmentContext"] == nil || v.EnvironmentContext != nil && (v.EnvironmentContext.EnvironmentID == "" || v.EnvironmentContext.Revision < 0 || v.EnvironmentContext.Revision > 9007199254740991) {
		return v, errors.New("invalid_request")
	}
	if v.EnvironmentContext != nil {
		var fields map[string]json.RawMessage
		if execution.StrictJSON(keys["environmentContext"], &fields) != nil || fields["environmentId"] == nil || fields["revision"] == nil || bytes.Equal(fields["revision"], []byte("null")) {
			return v, errors.New("invalid_request")
		}
	}
	return v, nil
}

func TestCompatibilityIdentityAndLegacyReaders(t *testing.T) {
	valid := []byte(`{"schemaVersion":2,"templateSemanticsVersion":1,"environmentContext":null}`)
	if _, e := decodeIdentity(valid, 2); e != nil {
		t.Fatal(e)
	}
	for _, raw := range []string{
		`{"schemaVersion":1,"templateSemanticsVersion":1,"environmentContext":null}`,
		`{"schemaVersion":2,"templateSemanticsVersion":1}`,
		`{"schemaVersion":2,"templateSemanticsVersion":null,"environmentContext":null}`,
		`{"schemaVersion":2,"templateSemanticsVersion":2,"environmentContext":null}`,
		`{"schemaVersion":2,"templateSemanticsVersion":1,"environmentContext":{}}`,
		`{"schemaVersion":2,"templateSemanticsVersion":1,"environmentContext":{"environmentId":"fixture"}}`,
		`{"schemaVersion":2,"templateSemanticsVersion":1,"environmentContext":{"environmentId":"fixture","revision":null}}`,
		`{"schemaVersion":2,"templateSemanticsVersion":1,"environmentContext":{"environmentId":"fixture","revision":-1}}`,
		`{"schemaVersion":2,"templateSemanticsVersion":1,"environmentContext":{"environmentId":"fixture","revision":0,"value":"canary"}}`,
		`{"schemaVersion":2,"templateSemanticsVersion":1,"environmentContext":null,"environmentContext":null}`,
	} {
		if _, e := decodeIdentity([]byte(raw), 2); e == nil {
			t.Fatalf("accepted invalid fixture %s", raw)
		}
	}
	for _, legacy := range []any{&legacyResultV1{}, &legacyWorkV1{}, &execution.LegacyEvidence{}} {
		if execution.StrictJSON(valid, legacy) == nil {
			t.Fatal("legacy reader accepted context")
		}
	}
	// Adding capabilities to the shared session Agent would break old clients;
	// the proposal uses a dedicated owner DTO and separate declaration response.
	var agent execution.Agent
	legacyAgent := compact(execution.Agent{})
	if execution.StrictJSON(legacyAgent, &agent) != nil {
		t.Fatal("legacy Agent fixture rejected")
	}
	var fields map[string]json.RawMessage
	if execution.StrictJSON(legacyAgent, &fields) != nil {
		t.Fatal("legacy Agent map rejected")
	}
	fields["capabilities"] = json.RawMessage("null")
	if execution.StrictJSON(compact(fields), &agent) == nil {
		t.Fatal("legacy session Agent accepted capabilities extension")
	}
	if _, e := execution.CanonicalResult(execution.Result{SchemaVersion: 2}); e == nil {
		t.Fatal("legacy canonicalizer accepted 2")
	}
}

type capabilitiesFixture struct {
	SchemaVersion    int   `json:"schemaVersion"`
	WorkVersions     []int `json:"workVersions"`
	ResultVersions   []int `json:"resultVersions"`
	TemplateVersions []int `json:"templateSemanticsVersions"`
	ContextVersions  []int `json:"environmentContextVersions"`
}

func TestCompatibilityRefusalBeforeAdmission(t *testing.T) {
	wanted := capabilitiesFixture{1, []int{1, 2}, []int{1, 2}, []int{1}, []int{1}}
	// Spy proves fixture ordering only, not repository atomicity or a handshake.
	admit := func(raw []byte, effects *int) error {
		var c capabilitiesFixture
		if execution.StrictJSON(raw, &c) != nil || !reflect.DeepEqual(c, wanted) {
			return errors.New("agent_incompatible")
		}
		*effects++
		return nil
	}
	for _, raw := range [][]byte{[]byte(`null`), []byte(`{}`), []byte(`{"schemaVersion":1,"workVersions":[1],"resultVersions":[1],"templateSemanticsVersions":[],"environmentContextVersions":[]}`)} {
		effects := 0
		if admit(raw, &effects) == nil || effects != 0 {
			t.Fatal("incompatible admission created effects")
		}
	}
	effects := 0
	if admit(compact(wanted), &effects) != nil || effects != 1 {
		t.Fatal("supported declaration rejected")
	}
}

func TestCompatibilityIdentityCannotDrift(t *testing.T) {
	original := identityFixture{2, 1, &contextFixture{"22222222-2222-4222-8222-222222222222", 7}}
	canonical := func(v identityFixture) []byte {
		raw, e := jsoncanonicalizer.Transform(compact(v))
		if e != nil {
			t.Fatal(e)
		}
		return raw
	}
	packet := canonical(original)
	decoded, e := decodeIdentity(packet, 2)
	if e != nil {
		t.Fatal(e)
	}
	receiptIdentity := append([]byte(nil), packet...)
	// Exercise RFC8785 on identity fields; this is not result HMAC verification.
	for _, mutate := range []func(*identityFixture){
		func(v *identityFixture) { v.EnvironmentContext.Revision++ },
		func(v *identityFixture) { v.EnvironmentContext.EnvironmentID = "another" },
		func(v *identityFixture) { v.EnvironmentContext = nil },
	} {
		candidate, _ := decodeIdentity(packet, 2)
		mutate(&candidate)
		if reflect.DeepEqual(candidate, decoded) || bytes.Equal(canonical(candidate), receiptIdentity) {
			t.Fatal("identity drift survived comparison")
		}
	}
	packet[0] = '!'
	if !bytes.Equal(receiptIdentity, canonical(original)) {
		t.Fatal("retained identity aliased inputs")
	}
}

func typedPatchFixture(kind, scalarType string, raw []byte) error {
	var p struct {
		Value *string `json:"value"`
	}
	if execution.StrictJSON(raw, &p) != nil || p.Value == nil || *p.Value == "" {
		return errors.New("invalid_request")
	}
	var value any
	var e error
	if kind == "string" && scalarType == "string" {
		value = *p.Value
		_, e = parseScalar(compact(value))
	} else if kind == "jsonScalar" {
		value, e = parseScalar([]byte(*p.Value))
	} else {
		return errors.New("variable_type")
	}
	if e != nil {
		return errors.New("variable_type")
	}
	actual := ""
	switch value.(type) {
	case string:
		actual = "string"
	case json.Number:
		actual = "number"
	case bool:
		actual = "boolean"
	case nil:
		actual = "null"
	}
	if actual != scalarType || kind == "jsonScalar" && scalarType == "string" {
		return errors.New("variable_type")
	}
	return nil
}

func TestCompatibilityTypedSecretPatch(t *testing.T) {
	for _, tc := range []struct {
		kind, typ, raw string
		ok             bool
	}{
		{"string", "string", `{"value":""}`, false},
		{"string", "string", `{"value":"null"}`, true},
		{"jsonScalar", "number", `{"value":"1.0000000000000001"}`, true},
		{"jsonScalar", "number", `{"value":"\"1\""}`, false},
		{"jsonScalar", "number", `{"value":"9007199254740992"}`, false},
		{"jsonScalar", "boolean", `{"value":"false"}`, true},
		{"jsonScalar", "boolean", `{"value":"0"}`, false},
		{"jsonScalar", "null", `{"value":"null"}`, true},
		{"jsonScalar", "null", `{"value":null}`, false},
		{"jsonScalar", "untyped", `{"value":"1"}`, false},
		{"jsonBody", "number", `{"value":"1"}`, false},
		{"string", "string", `{"value":"canary","scalarType":"number"}`, false},
		{"jsonScalar", "number", `{"value":"[]"}`, false},
		{"jsonScalar", "number", `{"value":"1","value":"2"}`, false},
	} {
		if e := typedPatchFixture(tc.kind, tc.typ, []byte(tc.raw)); (e == nil) != tc.ok {
			t.Fatalf("kind %s type %s acceptance mismatch", tc.kind, tc.typ)
		}
	}
}

func TestCompatibilityMCPProjectionPreservesOwnerEvidence(t *testing.T) {
	base := execution.Evidence{SchemaVersion: 1, ProjectID: "project", ExecutionID: "execution", Target: "cloud"}
	legacy := compact(base.LegacyProjection())
	// Only a fixture: actual owner evidence must be completely validated first.
	owner := struct {
		execution.LegacyEvidence
		TemplateSemanticsVersion int             `json:"templateSemanticsVersion"`
		EnvironmentContext       *contextFixture `json:"environmentContext"`
		Assertions               json.RawMessage `json:"assertions"`
	}{base.LegacyProjection(), 1, &contextFixture{"environment", 7}, json.RawMessage(`{"schemaVersion":1,"state":"not_configured","results":[]}`)}
	owner.SchemaVersion = 3
	stored := compact(owner)
	before := append([]byte(nil), stored...)
	projected := owner.LegacyEvidence
	projected.SchemaVersion = 1
	reason := "unsupported_context_not_exposed"
	envelope := wire.Evidence{View: "complete", Capture: &projected, Complete: true, Truncated: false, OmissionReason: &reason, OmittedFields: []string{"assertions", "templateSemanticsVersion", "environmentContext"}}
	// Exercise the current wire boundary, not just the Evidence struct: View is
	// a string in the DTO, but Parse/Decode enforce summary|complete.
	for _, tc := range []struct {
		name   string
		reason string
		fields []string
	}{
		{"environment", "environment_context_not_exposed", []string{"templateSemanticsVersion", "environmentContext"}},
		{"environment_and_assertions", reason, envelope.OmittedFields},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := envelope
			candidate.OmissionReason = &tc.reason
			candidate.OmittedFields = tc.fields
			packet := wire.Envelope{Data: &wire.Data{SchemaVersion: 1, ServerTime: "2026-10-22T22:00:00Z", Evidence: &candidate}}
			decoded, err := wire.Decode("get_execution_evidence", compact(packet))
			if err != nil {
				t.Fatalf("current wire contract rejected complete projection: %v", err)
			}
			if !reflect.DeepEqual(decoded.Data.Evidence, &candidate) {
				t.Fatal("wire round-trip changed capture or explicit omission metadata")
			}
			candidate.View = "capture"
			if _, err := wire.Decode("get_execution_evidence", compact(packet)); err == nil {
				t.Fatal("current wire contract accepted capture as a view")
			}
			candidate.View = "complete"
			candidate.Capture = nil
			if _, err := wire.Decode("get_execution_evidence", compact(packet)); err == nil {
				t.Fatal("complete view accepted without capture field")
			}
		})
	}
	for _, tc := range []struct {
		view  string
		valid bool
	}{{"complete", true}, {"summary", true}, {"capture", false}} {
		input := compact(map[string]string{"projectId": "11111111-1111-4111-8111-111111111111", "executionId": "22222222-2222-4222-8222-222222222222", "view": tc.view})
		if _, err := wire.Parse("get_execution_evidence", input); (err == nil) != tc.valid {
			t.Fatalf("current wire request view acceptance mismatch: %s", tc.view)
		}
	}
	if !bytes.Equal(compact(*envelope.Capture), legacy) || !bytes.Equal(stored, before) {
		t.Fatal("projection changed capture or stored evidence")
	}
	var fields map[string]json.RawMessage
	if execution.StrictJSON(compact(*envelope.Capture), &fields) != nil {
		t.Fatal("invalid projection")
	}
	for _, key := range envelope.OmittedFields {
		if _, ok := fields[key]; ok {
			t.Fatalf("unsupported field %s exposed", key)
		}
	}
	if !envelope.Complete || envelope.Truncated {
		t.Fatal("omission mislabeled truncation")
	}
}
