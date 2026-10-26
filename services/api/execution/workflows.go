package execution

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	runner "github.com/VioletProto/Kurier/services/request-runtime"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dt "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type WorkflowDefinition struct {
	Name  string                `json:"name"`
	Steps []runner.WorkflowStep `json:"steps"`
}

// Keep preactivation/legacy stored configurations readable by older binaries.
// Owner DTOs always normalize omission to [], while legacy frozen runs stay v1.
func workflowDefinitionJSON(d WorkflowDefinition, assertions bool) []byte {
	raw, _ := json.Marshal(d)
	if assertions {
		return raw
	}
	var v map[string]json.RawMessage
	_ = json.Unmarshal(raw, &v)
	var steps []map[string]json.RawMessage
	_ = json.Unmarshal(v["steps"], &steps)
	for _, step := range steps {
		delete(step, "assertions")
	}
	v["steps"], _ = json.Marshal(steps)
	raw, _ = json.Marshal(v)
	return raw
}

// Reserve metadata headroom for terminal timestamps, counters, outcome and
// retention before the first send. Assertions add no finalization actions.
func workflowAdmissionHeadroom(r row) error {
	item, err := attributevalue.MarshalMap(r)
	if err != nil {
		return safeError()
	}
	raw, err := json.Marshal(item)
	if err != nil || len(raw) > 128*1024-4096 {
		return failure(413, "payload_too_large")
	}
	return nil
}

type Workflow struct {
	WorkflowDefinition
	WorkflowID string `json:"workflowId"`
	Revision   int64  `json:"revision"`
	CreatedAt  string `json:"createdAt"`
	UpdatedAt  string `json:"updatedAt"`
}
type RunStep struct {
	runner.WorkflowStep
	Position        int        `json:"position"`
	ExecutionID     *string    `json:"executionId"`
	State           string     `json:"state"`
	Summary         *Summary   `json:"summary"`
	Retention       *Retention `json:"retention"`
	EvidenceExpired bool       `json:"evidenceExpired"`
	LiveRequestID   *string    `json:"liveRequestId"`
}

func (s RunStep) MarshalJSON() ([]byte, error) {
	// Do not promote WorkflowStep.MarshalJSON: it would hide run metadata.
	type stepFields runner.WorkflowStep
	step := s.WorkflowStep
	if step.Assertions == nil {
		step.Assertions = []runner.Assertion{}
	}
	return json.Marshal(struct {
		stepFields
		Assertions      []runner.Assertion `json:"assertions"`
		Position        int                `json:"position"`
		ExecutionID     *string            `json:"executionId"`
		State           string             `json:"state"`
		Summary         *Summary           `json:"summary"`
		Retention       *Retention         `json:"retention"`
		EvidenceExpired bool               `json:"evidenceExpired"`
		LiveRequestID   *string            `json:"liveRequestId"`
	}{stepFields(step), step.Assertions, s.Position, s.ExecutionID, s.State, s.Summary, s.Retention, s.EvidenceExpired, s.LiveRequestID})
}

func (s *RunStep) UnmarshalJSON(raw []byte) error {
	// Separate the embedded authoring fields from the run-only DTO fields.
	var fields map[string]json.RawMessage
	if StrictJSON(raw, &fields) != nil {
		return safeError()
	}
	step := map[string]json.RawMessage{}
	for _, key := range []string{"stepId", "requestId", "requestRevision", "extract", "bindings", "assertions", "environment"} {
		if v, ok := fields[key]; ok {
			step[key] = v
			delete(fields, key)
		}
	}
	encoded, _ := json.Marshal(step)
	if StrictJSON(encoded, &s.WorkflowStep) != nil {
		return safeError()
	}
	var details struct {
		Position        int        `json:"position"`
		ExecutionID     *string    `json:"executionId"`
		State           string     `json:"state"`
		Summary         *Summary   `json:"summary"`
		Retention       *Retention `json:"retention"`
		EvidenceExpired bool       `json:"evidenceExpired"`
		LiveRequestID   *string    `json:"liveRequestId"`
	}
	encoded, _ = json.Marshal(fields)
	if StrictJSON(encoded, &details) != nil {
		return safeError()
	}
	s.Position, s.ExecutionID, s.State, s.Summary, s.Retention, s.EvidenceExpired, s.LiveRequestID = details.Position, details.ExecutionID, details.State, details.Summary, details.Retention, details.EvidenceExpired, details.LiveRequestID
	return nil
}

type WorkflowRun struct {
	WorkflowRunID    string    `json:"workflowRunId"`
	WorkflowID       string    `json:"workflowId"`
	WorkflowRevision int64     `json:"workflowRevision"`
	LiveWorkflowID   *string   `json:"liveWorkflowId"`
	Target           string    `json:"target"`
	State            string    `json:"state"`
	Version          int64     `json:"version"`
	SubmittedAt      string    `json:"submittedAt"`
	StartedAt        *string   `json:"startedAt"`
	CompletedAt      *string   `json:"completedAt"`
	NormalExpiresAt  *string   `json:"normalExpiresAt"`
	CurrentPosition  int       `json:"currentPosition"`
	FailedPosition   *int      `json:"failedPosition"`
	Steps            []RunStep `json:"steps"`
}
type RunOptions struct {
	Environment          *runner.EnvironmentContext `json:"environment,omitempty"`
	Target               string                     `json:"target"`
	AllowInsecureSecrets bool                       `json:"allowInsecureSecrets"`
}

func (s *Service) workflowContext(ctx context.Context, owner, project string) (row, row, dt.TransactWriteItem, error) {
	stage, e := s.stage(ctx)
	if e != nil || stage.SequentialWorkflowsSchemaVersion != 1 {
		return row{}, row{}, dt.TransactWriteItem{}, safeError()
	}
	p, e := s.project(ctx, owner, project)
	if e != nil {
		return stage, p, dt.TransactWriteItem{}, e
	}
	user, e := s.userGuard(ctx, owner)
	return stage, p, user, e
}
func stepKey(run string, pos int) string   { return fmt.Sprintf("RUN#%s#STEP#%02d", run, pos) }
func outputKey(run string, pos int) string { return fmt.Sprintf("RUN#%s#OUTPUT#%02d", run, pos) }
func workflowDTO(r row) (Workflow, error) {
	var d WorkflowDefinition
	if StrictJSON([]byte(r.ConfigurationJSON), &d) != nil {
		return Workflow{}, safeError()
	}
	return Workflow{d, r.WorkflowID, r.Revision, r.CreatedAt, r.UpdatedAt}, nil
}
func (s *Service) workflowPlans(ctx context.Context, p row, d *WorkflowDefinition, checkRevision bool) ([]Plan, []row, error) {
	plans := []Plan{}
	requests := []row{}
	seen := map[string]bool{}
	for _, step := range d.Steps {
		request, e := s.get(ctx, s.Table, p.PK, "REQ#"+step.RequestID)
		if e != nil || request.Kind != "request" || request.State != "active" {
			return nil, nil, failure(400, "source_unavailable")
		}
		if checkRevision && request.Revision != step.RequestRevision {
			return nil, nil, failure(412, "precondition_failed")
		}
		var c Configuration
		if StrictJSON([]byte(request.ConfigurationJSON), &c) != nil {
			return nil, nil, safeError()
		}
		plans = append(plans, Plan{Configuration: c, RequestID: step.RequestID, Revision: step.RequestRevision, Options: Options{TimeoutSeconds: 30, ResponseRedaction: RedactionPolicy{Headers: []string{}, JSONPointers: []string{}}}})
		if !seen[request.SK] {
			requests = append(requests, request)
			seen[request.SK] = true
		}
	}
	if e := runner.ValidateWorkflow(d.Steps, plans); e != nil {
		return nil, nil, e
	}
	return plans, requests, nil
}
func validWorkflowName(v string) bool {
	if utf8.RuneCountInString(v) < 1 || utf8.RuneCountInString(v) > 100 {
		return false
	}
	for _, r := range v {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
func (s *Service) SaveWorkflow(ctx context.Context, owner, project, workflow string, expected int64, d WorkflowDefinition, create bool) (Workflow, error) {
	stage, p, user, e := s.workflowContext(ctx, owner, project)
	if e != nil {
		return Workflow{}, e
	}
	if stage.WorkflowAssertionsSchemaVersion != 1 {
		for _, step := range d.Steps {
			if len(step.Assertions) > 0 {
				return Workflow{}, safeError()
			}
		}
	}
	d.Name = strings.TrimSpace(d.Name)
	if !validWorkflowName(d.Name) {
		return Workflow{}, failure(400, "validation_failed")
	}
	_, requests, e := s.workflowPlans(ctx, p, &d, true)
	if e != nil {
		return Workflow{}, e
	}
	for _, step := range d.Steps {
		if step.Environment != nil {
			if stage.ProjectEnvironmentsSchemaVersion != 1 {
				return Workflow{}, safeError()
			}
			if step.Environment.Mode == "select" {
				env, e := s.get(ctx, s.Table, p.PK, "ENV#"+step.Environment.EnvironmentID)
				if e != nil || env.Kind != "environment" || env.State != "active" {
					return Workflow{}, failure(400, "source_unavailable")
				}
				if env.Revision != step.Environment.Revision {
					return Workflow{}, failure(409, "environment_revision_conflict")
				}
				exists := false
				for _, r := range requests {
					if r.SK == env.SK {
						if r.Revision != env.Revision {
							return Workflow{}, failure(409, "environment_revision_conflict")
						}
						exists = true
					}
				}
				if !exists {
					requests = append(requests, env)
				}
			}
		}
	}
	raw := workflowDefinitionJSON(d, stage.WorkflowAssertionsSchemaVersion == 1)
	if len(raw) > ConfigLimit {
		return Workflow{}, failure(413, "payload_too_large")
	}
	now := stamp(s.now())
	r := row{PK: p.PK, SK: "WF#" + workflow, Kind: "executionWorkflow", SchemaVersion: 1, ProjectID: project, OwnerID: owner, WorkflowID: workflow, State: "active", ConfigurationJSON: string(raw), CreatedAt: now, UpdatedAt: now, LPK: p.PK + "#WF", LSK: now + "#" + workflow}
	if create {
		r.WorkflowID = id()
		r.SK = "WF#" + r.WorkflowID
		r.LSK = now + "#" + r.WorkflowID
	} else {
		old, err := s.get(ctx, s.Table, p.PK, r.SK)
		if err != nil || old.Kind != "executionWorkflow" || old.State != "active" {
			return Workflow{}, failure(404, "not_found")
		}
		if old.Revision != expected {
			return Workflow{}, failure(412, "precondition_failed")
		}
		r.Version = old.Version + 1
		r.Revision = old.Revision + 1
		r.CreatedAt = old.CreatedAt
		r.LSK = old.LSK
	}
	a, e := s.put(s.Table, r, create)
	if e != nil {
		return Workflow{}, e
	}
	actions := []dt.TransactWriteItem{s.stageGuard(stage), user, s.projectUpdate(p, 0, 0), a}
	for _, request := range requests {
		actions = append(actions, s.sourceGuard(request, s.Table))
	}
	if e = s.transact(ctx, actions); e != nil {
		return Workflow{}, safeError()
	}
	return workflowDTO(r)
}
func (s *Service) WorkflowDetail(ctx context.Context, owner, project, workflow string) (Workflow, error) {
	stage, p, user, e := s.workflowContext(ctx, owner, project)
	if e != nil {
		return Workflow{}, e
	}
	r, e := s.get(ctx, s.Table, p.PK, "WF#"+workflow)
	if e != nil || r.Kind != "executionWorkflow" || r.State != "active" {
		return Workflow{}, failure(404, "not_found")
	}
	if e = s.transact(ctx, []dt.TransactWriteItem{s.stageGuard(stage), user, s.projectGuard(p, "active"), s.rowGuard(r)}); e != nil {
		return Workflow{}, safeError()
	}
	return workflowDTO(r)
}
func (s *Service) DeleteWorkflow(ctx context.Context, owner, project, workflow string, expected int64) error {
	stage, p, user, e := s.workflowContext(ctx, owner, project)
	if e != nil {
		return e
	}
	r, e := s.get(ctx, s.Table, p.PK, "WF#"+workflow)
	if e != nil || r.Kind != "executionWorkflow" {
		return failure(404, "not_found")
	}
	if r.State == "deleted" && r.Revision == expected+1 {
		return nil
	}
	if r.State != "active" || r.Revision != expected {
		return failure(412, "precondition_failed")
	}
	r.Version++
	r.Revision++
	r.State = "deleted"
	r.ConfigurationJSON = ""
	r.Name = ""
	r.LPK = ""
	r.LSK = ""
	r.UpdatedAt = stamp(s.now())
	a, _ := s.put(s.Table, r, false)
	return s.transact(ctx, []dt.TransactWriteItem{s.stageGuard(stage), user, s.projectUpdate(p, 0, 0), a})
}
func (s *Service) SubmitWorkflow(ctx context.Context, owner, project, source, key string, revision int64, options RunOptions, rerun bool) (WorkflowRun, error) {
	stage, p, user, e := s.workflowContext(ctx, owner, project)
	if e != nil {
		return WorkflowRun{}, e
	}

	if rerun && options.Environment != nil {
		return WorkflowRun{}, failure(400, "invalid_request")
	}
	if !uuidPattern.MatchString(key) || !uuidPattern.MatchString(source) || len(s.SigningKey) < 32 || options.Target != "cloud" {
		return WorkflowRun{}, failure(400, "invalid_request")
	}
	purpose := "workflow-submit"
	if rerun {
		purpose = "workflow-rerun"
		revision = 0
	}
	receiptSK := "IDEM#execution-admission#" + s.digest("execution-admission-key-v1", []string{s.Stage, owner, project, key})
	input := s.digest("workflow-admission-input-v1", []any{purpose, source, revision, options})
	receipt, e := s.get(ctx, s.Table, p.PK, receiptSK)
	if e == nil {
		if receipt.TemplateSemanticsVersion == 1 {
			input = s.digest("workflow-environment-admission-input-v1", []any{input, 1})
		}
		if receipt.InputDigest != input || receipt.RunID == "" || receipt.ExpiresAt <= stamp(s.now()) {
			return WorkflowRun{}, failure(409, "conflict")
		}
		return s.RunDetail(ctx, owner, project, receipt.RunID)
	}
	if !missingRow(e) {
		return WorkflowRun{}, e
	}
	if p.StorageReservedBytes >= 1024*1024*1024 {
		return WorkflowRun{}, failure(429, "rate_limited")
	}
	if p.StorageReservedBytes < 0 || p.InflightCount < 0 {
		return WorkflowRun{}, safeError()
	}
	var definition WorkflowDefinition
	var plans []Plan
	var requests []row
	var wf row
	if rerun {
		old, e := s.get(ctx, s.Table, p.PK, "RUN#"+source)
		if e != nil || old.Kind != "executionWorkflowRun" || old.OwnerID != owner || old.ProjectID != project {
			return WorkflowRun{}, failure(404, "not_found")
		}
		if _, e = s.RunDetail(ctx, owner, project, source); e != nil {
			return WorkflowRun{}, e
		}
		if StrictJSON([]byte(old.ConfigurationJSON), &definition) != nil {
			return WorkflowRun{}, safeError()
		}
		steps, e := s.runSteps(ctx, old)
		if e != nil {
			return WorkflowRun{}, e
		}
		for _, step := range steps {
			if step.Plan == nil {
				return WorkflowRun{}, safeError()
			}
			plans = append(plans, *step.Plan)
		}
		wf = old
		seen := map[string]bool{}
		for _, plan := range plans {
			if seen[plan.RequestID] {
				continue
			}
			r, e := s.get(ctx, s.Table, p.PK, "REQ#"+plan.RequestID)
			if e != nil || r.Kind != "request" || r.State != "active" {
				return WorkflowRun{}, failure(400, "source_unavailable")
			}
			requests = append(requests, r)
			seen[plan.RequestID] = true
		}
	} else {
		wf, e = s.get(ctx, s.Table, p.PK, "WF#"+source)
		if e != nil || wf.Kind != "executionWorkflow" || wf.State != "active" {
			return WorkflowRun{}, failure(404, "not_found")
		}
		if wf.Revision != revision {
			return WorkflowRun{}, failure(412, "precondition_failed")
		}
		if StrictJSON([]byte(wf.ConfigurationJSON), &definition) != nil {
			return WorkflowRun{}, safeError()
		}
		plans, requests, e = s.workflowPlans(ctx, p, &definition, true)
		if e != nil {
			return WorkflowRun{}, e
		}
	}
	if e = runner.ValidateWorkflow(definition.Steps, plans); e != nil {
		return WorkflowRun{}, e
	}
	environmentActions := []dt.TransactWriteItem{}
	environmentSeen := map[string]int64{}
	appendEnvironment := func(guards []dt.TransactWriteItem) error {
		for _, g := range guards {
			if g.ConditionCheck == nil {
				continue
			}
			sk := g.ConditionCheck.Key["SK"].(*dt.AttributeValueMemberS).Value
			r := g.ConditionCheck.ExpressionAttributeValues[":r"].(*dt.AttributeValueMemberN).Value
			var revision int64
			fmt.Sscan(r, &revision)
			if old, ok := environmentSeen[sk]; ok {
				if old != revision {
					return failure(409, "environment_revision_conflict")
				}
				continue
			}
			environmentSeen[sk] = revision
			environmentActions = append(environmentActions, g)
		}
		if len(environmentSeen) > 4 {
			return failure(413, "variable_limit")
		}
		return nil
	}
	if options.Environment != nil && !rerun {
		dummy := Plan{Configuration: Configuration{TemplateSemanticsVersion: 1, URL: "https://example.test/", QueryParameters: []Field{}, Headers: []Field{}}}
		guards, e := s.freezeEnvironment(ctx, project, &dummy, options.Environment, false)
		if e != nil {
			return WorkflowRun{}, e
		}
		if e = appendEnvironment(guards); e != nil {
			return WorkflowRun{}, e
		}
	}
	for i := range plans {
		selection := options.Environment
		if step := definition.Steps[i].Environment; step != nil {
			switch step.Mode {
			case "none":
				selection = nil
			case "select":
				selection = &runner.EnvironmentContext{EnvironmentID: step.EnvironmentID, Revision: step.Revision}
			}
		}
		if plans[i].Configuration.TemplateSemanticsVersion == 1 && stage.ProjectEnvironmentsSchemaVersion != 1 {
			return WorkflowRun{}, safeError()
		}
		guards, e := s.freezeEnvironment(ctx, project, &plans[i], selection, rerun)
		if e != nil {
			return WorkflowRun{}, e
		}
		if e = appendEnvironment(guards); e != nil {
			return WorkflowRun{}, e
		}
	}
	environmentSemantics := 0
	for _, plan := range plans {
		if plan.TemplateSnapshot != nil {
			environmentSemantics = 1
		}
	}
	if environmentSemantics == 1 {
		input = s.digest("workflow-environment-admission-input-v1", []any{input, 1})
	}
	runID := id()
	values := map[string]string{}
	defer clear(values)
	sources := []SecretSource{}
	secretRows := []row{}
	seen := map[string]bool{}
	total := 0
	for i := range plans {
		plans[i].Options.AllowInsecureSecrets = options.AllowInsecureSecrets
		plans[i].Target = ""
		plans[i].Options.Target = ""
		plans[i].Options.Local = nil
		raw, _ := json.Marshal(struct {
			Plan Plan
			Step runner.WorkflowStep
		}{plans[i], definition.Steps[i]})
		if len(raw) > ConfigLimit {
			return WorkflowRun{}, failure(413, "payload_too_large")
		}
		total += len(raw)
		refs := requiredPlanRefs(plans[i])
		if len(refs) > 8 {
			return WorkflowRun{}, failure(413, "payload_too_large")
		}
		resolvedURL, e := runner.ResolvedTemplateURL(plans[i])
		if e != nil {
			return WorkflowRun{}, e
		}
		destination, e := ValidateDestination(resolvedURL)
		if e != nil {
			return WorkflowRun{}, e
		}
		if destination.Scheme == "http" && !options.AllowInsecureSecrets && (len(refs) > 0 || workflowStepSensitive(definition.Steps[i], plans[i], definition.Steps)) {
			return WorkflowRun{}, failure(400, "insecure_secret_transport")
		}
		for _, ref := range refs {
			if seen[ref] {
				continue
			}
			r, e := s.get(ctx, s.Protected, p.PK, "SECRET#"+ref)
			if e != nil || !validateEnvironmentSecret(plans[i], r) || r.ProjectID != project {
				return WorkflowRun{}, failure(400, "source_unavailable")
			}
			raw, e := s.decrypt(ctx, project, ref, "saved-secret", r.Revision, r.Envelope)
			if e != nil {
				return WorkflowRun{}, e
			}
			values[ref] = string(raw)
			clear(raw)
			sources = append(sources, SecretSource{ref, r.Revision})
			secretRows = append(secretRows, r)
			seen[ref] = true
		}
	}
	for _, plan := range plans {
		if e = s.preflightEnvironment(ctx, plan, values); e != nil {
			return WorkflowRun{}, e
		}
	}
	if len(sources) > 16 || len(requests)+len(secretRows)+len(environmentActions)+1 > 18 {
		return WorkflowRun{}, failure(413, "payload_too_large")
	}
	envelope, e := s.encryptPurpose(ctx, project, runID, "workflow-inputs", values)
	if e != nil {
		return WorkflowRun{}, e
	}
	enc, _ := json.Marshal(envelope)
	total += len(enc)
	if total > 256*1024 {
		return WorkflowRun{}, failure(413, "payload_too_large")
	}
	now := stamp(s.now())
	semantics := stage.WorkflowAssertionsSchemaVersion
	if rerun {
		semantics = wf.AssertionSemanticsVersion
	}
	defRaw := workflowDefinitionJSON(definition, semantics == 1)
	if len(defRaw) > ConfigLimit {
		return WorkflowRun{}, failure(413, "payload_too_large")
	}
	run := row{PK: p.PK, SK: "RUN#" + runID, Kind: "executionWorkflowRun", SchemaVersion: 1, ProjectID: project, OwnerID: owner, RunID: runID, WorkflowID: wf.WorkflowID, WorkflowRevision: wf.Revision, State: "queued", StepCount: len(plans), ConfigurationJSON: string(defRaw), Sources: sources, SubmittedAt: now, RecoveryGeneration: stage.RecoveryGeneration, LPK: p.PK + "#RUN", LSK: now + "#" + runID}
	run.AssertionSemanticsVersion = stage.WorkflowAssertionsSchemaVersion
	run.WorkflowSubmission = workflowDefinitionSubmission
	if rerun {
		run.AssertionSemanticsVersion = wf.AssertionSemanticsVersion
		run.WorkflowRevision = wf.WorkflowRevision
		run.WorkflowSubmission = workflowFrozenRerun
		run.RerunSourceRunID = source
	}
	if err := workflowAdmissionHeadroom(run); err != nil {
		return WorkflowRun{}, err
	}
	actions := []dt.TransactWriteItem{s.stageGuard(stage), user, s.projectUpdate(p, 0, 1)}
	actions = append(actions, environmentActions...)
	// Fence the definition or retained source run against concurrent deletion.
	actions = append(actions, s.rowGuard(wf))
	for _, r := range requests {
		actions = append(actions, s.sourceGuard(r, s.Table))
	}
	for _, r := range secretRows {
		actions = append(actions, s.sourceGuard(r, s.Protected))
	}
	for i, plan := range plans {
		step := row{PK: p.PK, SK: stepKey(runID, i), Kind: "executionWorkflowStep", SchemaVersion: 1, ProjectID: project, OwnerID: owner, RunID: runID, Position: i, State: "pending", Plan: &plan}
		if err := workflowAdmissionHeadroom(step); err != nil {
			return WorkflowRun{}, err
		}
		if i == 0 {
			step.State = "queued"
			step.ExecutionID = workflowIdentity(runID, i, "execution")
			step.JobID = workflowIdentity(runID, i, "job")
		}
		a, e := s.put(s.Table, step, true)
		if e != nil {
			return WorkflowRun{}, e
		}
		actions = append(actions, a)
	}
	initial := row{PK: p.PK, SK: "RUN#" + runID + "#INPUTS", Kind: "executionWorkflowInputs", SchemaVersion: 1, ProjectID: project, RunID: runID, Envelope: envelope}
	receipt = row{PK: p.PK, SK: receiptSK, Kind: "executionReceipt", SchemaVersion: 1, ProjectID: project, OwnerID: owner, InputDigest: input, RunID: runID, ExpiresAt: stamp(s.now().Add(7 * 24 * time.Hour))}
	receipt.TemplateSemanticsVersion = environmentSemantics
	for _, spec := range []struct {
		table string
		r     row
	}{{s.Table, run}, {s.Protected, initial}, {s.Table, receipt}} {
		a, e := s.put(spec.table, spec.r, true)
		if e != nil {
			return WorkflowRun{}, e
		}
		actions = append(actions, a)
	}
	first, e := s.workflowJobActions(run, plans[0], 0, now)
	if e != nil {
		return WorkflowRun{}, e
	}
	actions = append(actions, first...)
	quota, e := s.admissionQuotaCount(ctx, owner, s.now(), int64(len(plans)))
	if e != nil {
		return WorkflowRun{}, e
	}
	actions = append(actions, quota...)
	if len(actions) > 60 {
		return WorkflowRun{}, failure(413, "payload_too_large")
	}
	if e = s.transact(ctx, actions); e != nil {
		r, err := s.get(ctx, s.Table, p.PK, receiptSK)
		if err == nil {
			if r.InputDigest != input {
				return WorkflowRun{}, failure(409, "conflict")
			}
			return s.RunDetail(ctx, owner, project, r.RunID)
		}
		var c *dt.TransactionCanceledException
		if errors.As(e, &c) {
			return WorkflowRun{}, failure(409, "conflict")
		}
		return WorkflowRun{}, safeError()
	}
	s.fastNotify(ctx, project, workflowIdentity(runID, 0, "job"))
	return s.RunDetail(ctx, owner, project, runID)
}
func workflowStepSensitive(step runner.WorkflowStep, plan Plan, steps []runner.WorkflowStep) bool {
	for _, b := range step.Bindings {
		if b.To.Kind == "header" && runner.CredentialName(plan.Configuration.Headers[*b.To.Index].Name) || b.To.Kind == "query" && runner.CredentialName(plan.Configuration.QueryParameters[*b.To.Index].Name) {
			return true
		}
		for _, producer := range steps {
			if producer.StepID == b.From.StepID {
				for _, x := range producer.Extract {
					if x.Name == b.From.Name && (x.Sensitive || extractionCredentialPath(x.Path)) {
						return true
					}
				}
			}
		}
	}
	return false
}
func workflowIdentity(run string, pos int, purpose string) string { // UUID-form deterministic per immutable run position.
	return runner.DeterministicID(run + "/" + fmt.Sprint(pos) + "/" + purpose)
}
func (s *Service) workflowJobActions(run row, plan Plan, pos int, now string) ([]dt.TransactWriteItem, error) {
	exec, job := workflowIdentity(run.RunID, pos, "execution"), workflowIdentity(run.RunID, pos, "job")
	anchor := row{PK: run.PK, SK: "EXEC#" + exec, Kind: "execution", SchemaVersion: 1, ProjectID: run.ProjectID, OwnerID: run.OwnerID, RunID: run.RunID, Position: pos, State: "queued", ExecutionID: exec, JobID: job, Plan: &plan, SubmittedAt: now, RecoveryGeneration: run.RecoveryGeneration, LPK: run.PK + "#EXEC", LSK: now + "#" + exec, HPK: run.PK + "#REQ#" + plan.RequestID, HSK: now + "#" + exec}
	if err := workflowAdmissionHeadroom(anchor); err != nil {
		return nil, err
	}
	j := row{PK: run.PK, SK: "JOB#" + job, Kind: "executionJob", SchemaVersion: 1, ProjectID: run.ProjectID, OwnerID: run.OwnerID, RunID: run.RunID, Position: pos, State: "queued", ExecutionID: exec, JobID: job, Sources: run.Sources, SubmittedAt: now, RecoveryGeneration: run.RecoveryGeneration, DPK: dueShard(run.ProjectID), DSK: stamp(s.now().Add(10*time.Minute)) + "#" + job}
	out := row{PK: run.PK, SK: "OUT#" + job, Kind: "executionOutbox", SchemaVersion: 1, ProjectID: run.ProjectID, OwnerID: run.OwnerID, ExecutionID: exec, JobID: job, RunID: run.RunID, Position: pos, State: "pending", RecoveryGeneration: run.RecoveryGeneration, DPK: dueShard(run.ProjectID), DSK: now + "#" + job}
	actions := []dt.TransactWriteItem{}
	for _, r := range []row{anchor, j, out} {
		a, e := s.put(s.Table, r, true)
		if e != nil {
			return nil, e
		}
		actions = append(actions, a)
	}
	return actions, nil
}
func (s *Service) runSteps(ctx context.Context, run row) ([]row, error) {
	out := []row{}
	if run.StepCount < 2 || run.StepCount > 10 {
		return nil, safeError()
	}
	for i := 0; i < run.StepCount; i++ {
		r, e := s.get(ctx, s.Table, run.PK, stepKey(run.RunID, i))
		if e != nil || r.Kind != "executionWorkflowStep" || r.RunID != run.RunID || r.Position != i || r.Plan == nil {
			return nil, safeError()
		}
		out = append(out, r)
	}
	return out, nil
}
func (s *Service) RunDetail(ctx context.Context, owner, project, runID string) (WorkflowRun, error) {
	stage, p, user, e := s.workflowContext(ctx, owner, project)
	if e != nil {
		return WorkflowRun{}, e
	}
	run, e := s.get(ctx, s.Table, p.PK, "RUN#"+runID)
	if e != nil || run.Kind != "executionWorkflowRun" || run.OwnerID != owner {
		return WorkflowRun{}, failure(404, "not_found")
	}
	steps, e := s.runSteps(ctx, run)
	if e != nil {
		return WorkflowRun{}, e
	}
	var def WorkflowDefinition
	if StrictJSON([]byte(run.ConfigurationJSON), &def) != nil || len(def.Steps) != len(steps) {
		return WorkflowRun{}, safeError()
	}
	out := WorkflowRun{WorkflowRunID: runID, WorkflowID: run.WorkflowID, WorkflowRevision: run.WorkflowRevision, Target: "cloud", State: run.State, Version: run.Version, SubmittedAt: run.SubmittedAt, StartedAt: run.StartedAt, CompletedAt: run.CompletedAt, CurrentPosition: run.CurrentPosition, FailedPosition: run.FailedPosition, Steps: []RunStep{}}
	if run.NormalExpiresAt != "" {
		out.NormalExpiresAt = &run.NormalExpiresAt
	}
	if wf, err := s.get(ctx, s.Table, p.PK, "WF#"+run.WorkflowID); err == nil && wf.State == "active" {
		out.LiveWorkflowID = &run.WorkflowID
	}
	actions := []dt.TransactWriteItem{s.stageGuard(stage), user, s.projectGuard(p, "active"), s.rowGuard(run)}
	protected := !terminal(run.State) || run.NormalExpiresAt > stamp(s.now())
	for i, step := range steps {
		actions = append(actions, s.rowGuard(step))
		v := RunStep{WorkflowStep: def.Steps[i], Position: i, State: step.State, Summary: step.Summary, EvidenceExpired: step.EvidenceExpired}
		if step.ExecutionID != "" {
			v.ExecutionID = &step.ExecutionID
		}
		if request, err := s.get(ctx, s.Table, p.PK, "REQ#"+step.Plan.RequestID); err == nil && request.State == "active" {
			v.LiveRequestID = &step.Plan.RequestID
		}
		if terminal(step.State) && !step.EvidenceExpired {
			ret, err := s.get(ctx, s.Table, p.PK, "EXEC#"+step.ExecutionID+"#RET")
			if err != nil {
				return WorkflowRun{}, safeError()
			}
			actions = append(actions, s.rowGuard(ret))
			live := ret.State == "live" && (ret.Pinned || ret.NormalExpiresAt > stamp(s.now()))
			if live {
				protected = true
				v.Retention = retentionView(ret, s.now())
			} else {
				v.EvidenceExpired = true
			}
		}
		out.Steps = append(out.Steps, v)
	}
	if !protected {
		return WorkflowRun{}, failure(404, "not_found")
	}
	if e = s.transact(ctx, actions); e != nil {
		return WorkflowRun{}, safeError()
	}
	return out, nil
}

// Signed bounded list cursors pin project, collection, generation and optional filter.
type workflowCursor struct {
	Owner, Project, Collection, Filter, Generation, High string
	Expires                                              int64
	Last                                                 map[string]string
}
type WorkflowList struct {
	Items      []Workflow `json:"items"`
	NextCursor *string    `json:"nextCursor"`
}
type RunList struct {
	Items      []WorkflowRun `json:"items"`
	NextCursor *string       `json:"nextCursor"`
}

func (s *Service) workflowList(ctx context.Context, owner, project, collection, filter string, limit int, token string) ([]row, *string, error) {
	stage, p, _, e := s.workflowContext(ctx, owner, project)
	if e != nil {
		return nil, nil, e
	}
	if limit < 1 || limit > 100 || len(token) > 8192 || filter != "" && !uuidPattern.MatchString(filter) {
		return nil, nil, failure(400, "invalid_request")
	}
	// A 25-record read budget keeps worst-case 64 KiB definitions/run rules
	// below the existing API/Lambda delivery bounds, even when limit=100.
	limit = min(limit, 25)
	c := workflowCursor{Owner: owner, Project: project, Collection: collection, Filter: filter, Generation: stage.RecoveryGeneration, High: stamp(s.now()) + "#~", Expires: s.now().Add(24 * time.Hour).Unix()}
	var start map[string]dt.AttributeValue
	if token != "" {
		parts := strings.Split(token, ".")
		if len(parts) != 2 {
			return nil, nil, failure(400, "invalid_request")
		}
		raw, e := base64.RawURLEncoding.DecodeString(parts[0])
		if e != nil || parts[1] != s.digest("workflow-cursor-v1", string(raw)) || StrictJSON(raw, &c) != nil || c.Owner != owner || c.Project != project || c.Collection != collection || c.Filter != filter || c.Generation != stage.RecoveryGeneration || c.Expires <= s.now().Unix() || c.Expires > s.now().Add(24*time.Hour).Unix() || c.Last["PK"] != p.PK || !strings.HasPrefix(c.Last["SK"], collection+"#") {
			return nil, nil, failure(400, "invalid_request")
		}
		start = map[string]dt.AttributeValue{}
		for k, v := range c.Last {
			if k != "PK" && k != "SK" && k != "LPK" && k != "LSK" {
				return nil, nil, failure(400, "invalid_request")
			}
			start[k] = str(v)
		}
	}
	out := []row{}
	for page := 0; page < 5; page++ {
		r, e := s.DB.Query(ctx, &dynamodb.QueryInput{TableName: &s.Table, IndexName: aws.String("GSI1"), KeyConditionExpression: aws.String("LPK = :pk AND LSK <= :hi"), ExpressionAttributeValues: map[string]dt.AttributeValue{":pk": str(p.PK + "#" + collection), ":hi": str(c.High)}, ExclusiveStartKey: start, ScanIndexForward: aws.Bool(false), Limit: aws.Int32(int32(limit - len(out)))})
		if e != nil {
			return nil, nil, safeError()
		}
		for _, item := range r.Items {
			var candidate row
			if attributevalue.UnmarshalMap(item, &candidate) != nil {
				return nil, nil, safeError()
			}
			strong, e := s.get(ctx, s.Table, candidate.PK, candidate.SK)
			if missingRow(e) {
				continue
			}
			if e != nil {
				return nil, nil, e
			}
			if filter == "" || strong.WorkflowID == filter {
				out = append(out, strong)
			}
		}
		start = r.LastEvaluatedKey
		if len(start) == 0 || len(out) >= limit {
			break
		}
	}
	var next *string
	if len(start) > 0 {
		c.Last = map[string]string{}
		for k, v := range start {
			a, ok := v.(*dt.AttributeValueMemberS)
			if !ok {
				return nil, nil, safeError()
			}
			c.Last[k] = a.Value
		}
		raw, _ := json.Marshal(c)
		next = pointer(base64.RawURLEncoding.EncodeToString(raw) + "." + s.digest("workflow-cursor-v1", string(raw)))
	}
	return out, next, nil
}
func (s *Service) Workflows(ctx context.Context, owner, project string, limit int, token string) (WorkflowList, error) {
	rows, next, e := s.workflowList(ctx, owner, project, "WF", "", limit, token)
	out := WorkflowList{Items: []Workflow{}, NextCursor: next}
	if e != nil {
		return out, e
	}
	for _, r := range rows {
		v, e := s.WorkflowDetail(ctx, owner, project, r.WorkflowID)
		if missingRow(e) {
			continue
		}
		if e != nil {
			return out, e
		}
		out.Items = append(out.Items, v)
	}
	return out, nil
}
func (s *Service) Runs(ctx context.Context, owner, project, filter string, limit int, token string) (RunList, error) {
	rows, next, e := s.workflowList(ctx, owner, project, "RUN", filter, limit, token)
	out := RunList{Items: []WorkflowRun{}, NextCursor: next}
	if e != nil {
		return out, e
	}
	for _, r := range rows {
		v, e := s.RunDetail(ctx, owner, project, r.RunID)
		if missingRow(e) {
			continue
		}
		if e != nil {
			return out, e
		}
		out.Items = append(out.Items, v)
	}
	return out, nil
}

func extractionCredentialPath(path string) bool {
	parts, e := runner.ParseExtractionPath(path)
	if e != nil {
		return true
	}
	for _, part := range parts {
		if !part.Array && runner.CredentialName(part.Property) {
			return true
		}
	}
	return false
}
