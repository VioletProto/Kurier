package environments

import execution "github.com/VioletProto/Kurier/services/request-runtime"

// Frozen field sets from reviewed head d8be596a. Research must not accidentally
// relabel the now environment-aware production readers as legacy readers.
type legacyResultV1 struct {
	SchemaVersion int               `json:"schemaVersion"`
	LeaseID       string            `json:"leaseId"`
	Fence         int64             `json:"fence"`
	Nonce         string            `json:"nonce"`
	Capture       execution.Capture `json:"capture"`
}
type legacyWorkV1 struct {
	SchemaVersion      int            `json:"schemaVersion"`
	ProjectID          string         `json:"projectId"`
	JobID              string         `json:"jobId"`
	ExecutionID        string         `json:"executionId"`
	LocalAgentID       string         `json:"localAgentId"`
	RecoveryGeneration string         `json:"recoveryGeneration"`
	LeaseID            string         `json:"leaseId"`
	Fence              int64          `json:"fence"`
	Nonce              string         `json:"nonce"`
	DispatchIntentAt   string         `json:"dispatchIntentAt"`
	ExecuteNotAfter    string         `json:"executeNotAfter"`
	Deadline           string         `json:"deadline"`
	Plan               execution.Plan `json:"plan"`
	ProtectedInputs    struct {
		Values map[string]string `json:"values"`
	} `json:"protectedInputs"`
	Destination execution.Destination `json:"destination"`
}
type legacyOptionsV1 struct {
	Target               string                    `json:"target,omitempty"`
	Local                *execution.LocalOptions   `json:"local,omitempty"`
	TimeoutSeconds       int                       `json:"timeoutSeconds"`
	AllowInsecureSecrets bool                      `json:"allowInsecureSecrets"`
	ResponseRedaction    execution.RedactionPolicy `json:"responseRedaction"`
}
type legacyWorkflowStepV1 struct {
	StepID          string                 `json:"stepId"`
	RequestID       string                 `json:"requestId"`
	RequestRevision int64                  `json:"requestRevision"`
	Extract         []execution.Extraction `json:"extract"`
	Bindings        []execution.Binding    `json:"bindings"`
	Assertions      []execution.Assertion  `json:"assertions,omitempty"`
}
type legacyConfigurationV1 struct {
	Name            string                 `json:"name"`
	Method          string                 `json:"method"`
	URL             string                 `json:"url"`
	QueryParameters []execution.Field      `json:"queryParameters"`
	Headers         []execution.Field      `json:"headers"`
	Body            *execution.RequestBody `json:"body"`
	OperationRef    any                    `json:"operationRef"`
}
