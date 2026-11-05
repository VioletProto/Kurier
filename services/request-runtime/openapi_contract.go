package execution

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"

	contract "github.com/VioletProto/Kurier/services/request-runtime/openapi"
)

// OperationRef is an owner-selected association. The executable response bundle
// is resolved by admission, never accepted from an owner-supplied configuration.
type OperationRef struct {
	ImportID    string `json:"importId"`
	OperationID string `json:"operationId"`
}

func (r *OperationRef) UnmarshalJSON(raw []byte) error {
	type plain OperationRef
	var v plain
	var fields map[string]json.RawMessage
	if StrictJSON(raw, &fields) != nil || StrictJSON(raw, &v) != nil || len(fields) != 2 || fields["importId"] == nil || fields["operationId"] == nil {
		return failure(400, "openapi_reference_invalid")
	}
	ref := OperationRef(v)
	if ref.Validate() != nil {
		return failure(400, "openapi_reference_invalid")
	}
	*r = ref
	return nil
}

func (r *OperationRef) Validate() error {
	if r == nil || !workflowUUID.MatchString(r.ImportID) || len(r.OperationID) == 0 || len(r.OperationID) > 128 {
		return failure(400, "openapi_reference_invalid")
	}
	for _, c := range r.OperationID {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return failure(400, "openapi_reference_invalid")
		}
	}
	return nil
}

type FrozenContract struct {
	SchemaVersion int                `json:"schemaVersion"`
	ImportID      string             `json:"importId"`
	OperationID   string             `json:"operationId"`
	Revision      int64              `json:"revision"`
	Operation     contract.Operation `json:"operation"`
}

func (f *FrozenContract) Validate(ref *OperationRef) error {
	if f == nil || ref.Validate() != nil || f.SchemaVersion != 1 || f.ImportID != ref.ImportID || f.OperationID != ref.OperationID || f.Revision < 0 || f.Revision > 9007199254740991 || f.Operation.OperationID != ref.OperationID || contract.ValidateOperation(f.Operation) != nil {
		return failure(400, "openapi_reference_invalid")
	}
	raw, err := json.Marshal(f)
	if err != nil || len(raw) > 40*1024 {
		return failure(413, "payload_too_large")
	}
	return nil
}

type ContractEvaluation struct {
	ImportID    string          `json:"importId"`
	OperationID string          `json:"operationId"`
	Revision    int64           `json:"revision"`
	Result      contract.Result `json:"result"`
}

func (c *ContractEvaluation) UnmarshalJSON(raw []byte) error {
	type plain ContractEvaluation
	var v plain
	var fields map[string]json.RawMessage
	if StrictJSON(raw, &fields) != nil || StrictJSON(raw, &v) != nil || len(fields) != 4 || fields["importId"] == nil || fields["operationId"] == nil || fields["revision"] == nil || fields["result"] == nil {
		return workflowInvalid()
	}
	for _, field := range fields {
		if bytes.Equal(bytes.TrimSpace(field), []byte("null")) {
			return workflowInvalid()
		}
	}
	value := ContractEvaluation(v)
	if validateContractEvaluation(&value) != nil {
		return workflowInvalid()
	}
	*c = value
	return nil
}

func UnavailableContract(f *FrozenContract, reason string) *ContractEvaluation {
	checks := []contract.Check{}
	for _, kind := range []string{"status", "headers", "body"} {
		checks = append(checks, contract.Check{Kind: kind, Status: "not_evaluated", Code: reason})
	}
	return &ContractEvaluation{ImportID: f.ImportID, OperationID: f.OperationID, Revision: f.Revision, Result: contract.Result{SemanticsVersion: 1, Status: "not_evaluated", Checks: checks}}
}

func validateContractEvaluation(c *ContractEvaluation) error {
	if c == nil || (&OperationRef{c.ImportID, c.OperationID}).Validate() != nil || c.Revision < 0 || c.Revision > 9007199254740991 || !contract.ValidResult(c.Result) {
		return workflowInvalid()
	}
	return nil
}

// Any protected input makes reflected-response validation unavailable, without
// revealing whether a particular protected value matched. Recognized credential
// fields and explicitly suppressed response content are equally conservative.
func contractSuppressed(raw []byte, headers http.Header, san *sanitizer, policy RedactionPolicy) bool {
	if len(san.values) != 0 || policy.OmitBody || len(policy.JSONPointers) != 0 || len(policy.Headers) != 0 {
		return true
	}
	for name := range headers {
		if credentialName(name) {
			return true
		}
	}
	var value any
	if StrictJSON(raw, &value) != nil {
		return false
	}
	var sensitive func(any) bool
	sensitive = func(v any) bool {
		switch x := v.(type) {
		case map[string]any:
			for k, v := range x {
				if credentialName(k) || sensitive(v) {
					return true
				}
			}
		case []any:
			for _, v := range x {
				if sensitive(v) {
					return true
				}
			}
		case string:
			lower := strings.ToLower(x)
			return strings.HasPrefix(lower, "bearer ") || strings.Contains(lower, "-----begin ")
		}
		return false
	}
	return sensitive(value)
}

func OpenAPIStageVersions() map[string]int64 {
	v := EnvironmentStageVersions()
	v["savedRequestsSchemaVersion"] = 10
	v["openapiSchemaVersion"] = 1
	return v
}
