package execution

import (
	"context"
	"fmt"
	runner "github.com/VioletProto/Kurier/services/request-runtime"
	dt "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"time"
)

const (
	workflowDefinitionSubmission = "definition"
	workflowFrozenRerun          = "frozen_rerun"
)

// Only server-side rerun admission sets this provenance after retained-context
// authorization. Legacy runs without provenance remain definition-backed.
func validWorkflowSubmission(run row) bool {
	switch run.WorkflowSubmission {
	case "", workflowDefinitionSubmission:
		return run.RerunSourceRunID == ""
	case workflowFrozenRerun:
		return uuidPattern.MatchString(run.RerunSourceRunID) && run.RerunSourceRunID != run.RunID
	default:
		return false
	}
}

func (s *Service) workflowRuntime(ctx context.Context, job row, plan Plan) (Plan, map[string]string, *runner.ExtractionCapture, error) {
	run, e := s.get(ctx, s.Table, job.PK, "RUN#"+job.RunID)
	if e != nil || terminal(run.State) || run.CurrentPosition != job.Position || run.RecoveryGeneration != job.RecoveryGeneration {
		return plan, nil, nil, failure(409, "conflict")
	}
	if !validWorkflowSubmission(run) {
		return plan, nil, nil, safeError()
	}
	if run.StartedAt == nil && run.WorkflowSubmission != workflowFrozenRerun {
		wf, e := s.get(ctx, s.Table, job.PK, "WF#"+run.WorkflowID)
		if e != nil || wf.Kind != "executionWorkflow" || wf.State != "active" {
			return plan, nil, nil, failure(400, "source_unavailable")
		}
	}
	if _, e = s.workflowRequestGuards(ctx, run, ""); e != nil {
		return plan, nil, nil, e
	}
	var definition WorkflowDefinition
	if StrictJSON([]byte(run.ConfigurationJSON), &definition) != nil {
		return plan, nil, nil, safeError()
	}
	steps, e := s.runSteps(ctx, run)
	if e != nil {
		return plan, nil, nil, e
	}
	plans := []Plan{}
	for _, step := range steps {
		plans = append(plans, *step.Plan)
	}
	initial, e := s.get(ctx, s.Protected, job.PK, "RUN#"+job.RunID+"#INPUTS")
	if e != nil {
		return plan, nil, nil, e
	}
	raw, e := s.decrypt(ctx, job.ProjectID, job.RunID, "workflow-inputs", 1, initial.Envelope)
	if e != nil {
		return plan, nil, nil, e
	}
	values := map[string]string{}
	e = StrictJSON(raw, &values)
	clear(raw)
	if e != nil {
		return plan, nil, nil, e
	}
	outputs := map[string]runner.RuntimeValue{}
	for i := 0; i < job.Position; i++ {
		r, e := s.get(ctx, s.Protected, job.PK, outputKey(job.RunID, i))
		if e != nil {
			clear(values)
			return plan, nil, nil, e
		}
		raw, e := s.decrypt(ctx, job.ProjectID, job.RunID+fmt.Sprintf("/%02d", i), "workflow-output", 1, r.Envelope)
		if e != nil {
			clear(values)
			return plan, nil, nil, e
		}
		var bundle map[string]runner.RuntimeValue
		e = StrictJSON(raw, &bundle)
		clear(raw)
		if e != nil {
			clear(values)
			return plan, nil, nil, e
		}
		for k, v := range bundle {
			outputs[k] = v
			values["runtime/"+k] = string(v.Value)
		}
	}
	// Recompute every historical rendered variant for containment in subsequent steps.
	for _, step := range definition.Steps {
		for _, b := range step.Bindings {
			if b.To.Kind != "header" {
				continue
			}
			v, ok := outputs[runner.RuntimeKey(b.From.StepID, b.From.Name)]
			if !ok {
				continue
			}
			var text string
			if StrictJSON(v.Value, &text) == nil {
				if rendered, e := runner.RenderHeader(text, b.To); e == nil {
					values["variant/"+step.StepID+"/"+fmt.Sprint(*b.To.Index)] = rendered
				}
			}
		}
	}
	resolved, e := runner.ApplyWorkflowBindings(plan, definition.Steps[job.Position], outputs, values)
	if e != nil {
		clear(values)
		return plan, nil, nil, e
	}
	capture := &runner.ExtractionCapture{Step: definition.Steps[job.Position], Steps: definition.Steps, Plans: plans, Outputs: map[string]runner.RuntimeValue{}, Values: values}
	return resolved, values, capture, nil
}
func (s *Service) workflowIntent(ctx context.Context, job row) ([]dt.TransactWriteItem, error) {
	run, e := s.get(ctx, s.Table, job.PK, "RUN#"+job.RunID)
	if e != nil || run.CurrentPosition != job.Position || terminal(run.State) {
		return nil, failure(409, "conflict")
	}
	step, e := s.get(ctx, s.Table, job.PK, stepKey(job.RunID, job.Position))
	if e != nil || step.State != "queued" {
		return nil, failure(409, "conflict")
	}
	if !validWorkflowSubmission(run) {
		return nil, safeError()
	}
	// The worker already guards the current request and every secret. Guard
	// other required requests here without duplicate transaction targets.
	actions, e := s.workflowRequestGuards(ctx, run, step.Plan.RequestID)
	if e != nil {
		return nil, e
	}
	if run.StartedAt == nil && run.WorkflowSubmission != workflowFrozenRerun {
		wf, e := s.get(ctx, s.Table, job.PK, "WF#"+run.WorkflowID)
		if e != nil || wf.Kind != "executionWorkflow" || wf.State != "active" {
			return nil, failure(400, "source_unavailable")
		}
		actions = append(actions, s.rowGuard(wf))
	}
	if run.StartedAt == nil {
		run.StartedAt = &job.DispatchIntentAt
	}
	run.State = "running"
	run.Version++
	step.State = "running"
	step.Version++
	step.StartedAt = &job.DispatchIntentAt
	for _, r := range []row{run, step} {
		a, e := s.put(s.Table, r, false)
		if e != nil {
			return nil, e
		}
		actions = append(actions, a)
	}
	return actions, nil
}
func (s *Service) workflowRequestGuards(ctx context.Context, run row, exclude string) ([]dt.TransactWriteItem, error) {
	steps, e := s.runSteps(ctx, run)
	if e != nil {
		return nil, e
	}
	actions := []dt.TransactWriteItem{}
	seen := map[string]bool{exclude: true}
	for _, step := range steps {
		if seen[step.Plan.RequestID] {
			continue
		}
		r, e := s.get(ctx, s.Table, run.PK, "REQ#"+step.Plan.RequestID)
		if e != nil || r.Kind != "request" || r.State != "active" || r.ProjectID != run.ProjectID {
			return nil, failure(400, "source_unavailable")
		}
		actions = append(actions, s.sourceGuard(r, s.Table))
		seen[step.Plan.RequestID] = true
	}
	return actions, nil
}
func (s *Service) workflowSourceGuards(ctx context.Context, run row) ([]dt.TransactWriteItem, error) {
	actions, e := s.workflowRequestGuards(ctx, run, "")
	if e != nil {
		return nil, e
	}
	for _, source := range run.Sources {
		r, e := s.get(ctx, s.Protected, run.PK, "SECRET#"+source.SecretID)
		if e != nil || r.Kind != "savedSecret" || r.State != "active" {
			return nil, failure(400, "source_unavailable")
		}
		actions = append(actions, s.sourceGuard(r, s.Protected))
	}
	return actions, nil
}
func (s *Service) workflowFinalActions(ctx context.Context, job row, evidence *Evidence, output *Envelope) ([]dt.TransactWriteItem, bool, error) {
	run, e := s.get(ctx, s.Table, job.PK, "RUN#"+job.RunID)
	if e != nil || run.CurrentPosition != job.Position || terminal(run.State) {
		return nil, false, failure(409, "conflict")
	}
	steps, e := s.runSteps(ctx, run)
	if e != nil {
		return nil, false, e
	}
	step := steps[job.Position]
	if step.ExecutionID != job.ExecutionID || step.JobID != job.JobID {
		return nil, false, safeError()
	}
	continuing := evidence.Status == "completed" && job.Position+1 < run.StepCount
	var definition WorkflowDefinition
	if StrictJSON([]byte(run.ConfigurationJSON), &definition) != nil {
		return nil, false, safeError()
	}
	if evidence.Status == "completed" && len(definition.Steps[job.Position].Extract) > 0 && output == nil {
		return nil, false, safeError()
	}
	actions := []dt.TransactWriteItem{}
	if evidence.Status == "completed" {
		guards, e := s.workflowSourceGuards(ctx, run)
		if e != nil {
			return nil, false, e
		}
		actions = append(actions, guards...)
		user, e := s.userGuard(ctx, run.OwnerID)
		if e != nil {
			return nil, false, e
		}
		actions = append(actions, user)
	}
	step.State = evidence.Status
	step.CompletedAt = &evidence.CompletedAt
	step.StartedAt = evidence.StartedAt
	step.Summary = &Summary{HTTPStatus: evidence.Response.HTTPStatus, DurationMS: evidence.Timing.DurationMS, Outcome: evidence.Outcome}
	step.Version++
	run.Version++
	run.AvailableCount++
	if continuing {
		if output == nil {
			output, e = s.encryptPurpose(ctx, job.ProjectID, job.RunID+fmt.Sprintf("/%02d", job.Position), "workflow-output", map[string]runner.RuntimeValue{})
			if e != nil {
				return nil, false, e
			}
		}
		bundle := row{PK: job.PK, SK: outputKey(job.RunID, job.Position), Kind: "executionWorkflowOutput", SchemaVersion: 1, ProjectID: job.ProjectID, RunID: job.RunID, Position: job.Position, Envelope: output}
		a, e := s.put(s.Protected, bundle, true)
		if e != nil {
			return nil, false, e
		}
		actions = append(actions, a)
		next := steps[job.Position+1]
		next.State = "queued"
		next.ExecutionID = workflowIdentity(job.RunID, next.Position, "execution")
		next.JobID = workflowIdentity(job.RunID, next.Position, "job")
		next.Version++
		a, e = s.put(s.Table, next, false)
		if e != nil {
			return nil, false, e
		}
		actions = append(actions, a)
		nextJob, e := s.workflowJobActions(run, *next.Plan, next.Position, stamp(s.now()))
		if e != nil {
			return nil, false, e
		}
		actions = append(actions, nextJob...)
		run.CurrentPosition = next.Position
		run.State = "running"
	} else {
		run.State = evidence.Status
		run.CompletedAt = &evidence.CompletedAt
		completed, e := time.Parse(time.RFC3339Nano, evidence.CompletedAt)
		if e != nil {
			return nil, false, e
		}
		run.NormalExpiresAt = stamp(completed.Add(30 * 24 * time.Hour))
		run.DPK = dueShard(run.ProjectID)
		run.DSK = run.NormalExpiresAt + "#" + run.RunID
		if evidence.Status == "failed" {
			run.FailedPosition = &job.Position
		}
		for i := job.Position + 1; i < len(steps); i++ {
			next := steps[i]
			next.State = "skipped"
			next.Version++
			a, e := s.put(s.Table, next, false)
			if e != nil {
				return nil, false, e
			}
			actions = append(actions, a)
		}
		actions = append(actions, dt.TransactWriteItem{Delete: &dt.Delete{TableName: &s.Protected, Key: keys(job.PK, "RUN#"+job.RunID+"#INPUTS")}})
		for i := 0; i < run.StepCount; i++ {
			actions = append(actions, dt.TransactWriteItem{Delete: &dt.Delete{TableName: &s.Protected, Key: keys(job.PK, outputKey(job.RunID, i))}})
		}
	}
	for _, r := range []row{step, run} {
		a, e := s.put(s.Table, r, false)
		if e != nil {
			return nil, false, e
		}
		actions = append(actions, a)
	}
	return actions, continuing, nil
}
func (s *Service) expireRunStep(ctx context.Context, ret row) ([]dt.TransactWriteItem, error) {
	run, e := s.get(ctx, s.Table, ret.PK, "RUN#"+ret.RunID)
	if e != nil {
		return nil, e
	}
	step, e := s.get(ctx, s.Table, ret.PK, stepKey(ret.RunID, ret.Position))
	if e != nil || step.EvidenceExpired || run.AvailableCount <= 0 {
		return nil, safeError()
	}
	run.AvailableCount--
	run.Version++
	step.EvidenceExpired = true
	step.Version++
	actions := []dt.TransactWriteItem{}
	for _, r := range []row{run, step} {
		a, e := s.put(s.Table, r, false)
		if e != nil {
			return nil, e
		}
		actions = append(actions, a)
	}
	return actions, nil
}
func (s *Service) expireRun(ctx context.Context, run row) error {
	stage, e := s.stage(ctx)
	if e != nil {
		return e
	}
	p, e := s.project(ctx, run.OwnerID, run.ProjectID)
	if e != nil {
		return e
	}
	if !terminal(run.State) || run.NormalExpiresAt > stamp(s.now()) || run.AvailableCount != 0 || run.RunPinnedCount != 0 {
		return failure(409, "cleanup_pending")
	}
	steps, e := s.runSteps(ctx, run)
	if e != nil {
		return e
	}
	actions := []dt.TransactWriteItem{s.stageGuard(stage), s.projectGuard(p, "active")}
	for _, step := range steps {
		if terminal(step.State) && !step.EvidenceExpired {
			return failure(409, "cleanup_pending")
		}
		if step.ExecutionID != "" {
			ret, e := s.get(ctx, s.Table, run.PK, "EXEC#"+step.ExecutionID+"#RET")
			if e == nil {
				if ret.Pinned || ret.State == "live" {
					return failure(409, "cleanup_pending")
				}
				actions = append(actions, s.rowGuard(ret))
			} else if !missingRow(e) {
				return e
			}
		}
		actions = append(actions, conditionalDelete(s.Table, step))
	}
	actions = append(actions, conditionalDelete(s.Table, run))
	return s.transact(ctx, actions)
}
func conditionalDelete(table string, r row) dt.TransactWriteItem {
	return dt.TransactWriteItem{Delete: &dt.Delete{TableName: &table, Key: keys(r.PK, r.SK), ConditionExpression: pointer("#v = :v"), ExpressionAttributeNames: map[string]string{"#v": "version"}, ExpressionAttributeValues: map[string]dt.AttributeValue{":v": num(r.Version)}}}
}

// Reopening a reconciled recovery generation never replays restored workflow HTTP.
func (s *Service) interruptRestoredWorkflow(ctx context.Context, stage, job row) error {
	if terminal(job.State) {
		return nil
	}
	p, e := s.project(ctx, job.OwnerID, job.ProjectID)
	if e != nil {
		return e
	}
	run, e := s.get(ctx, s.Table, job.PK, "RUN#"+job.RunID)
	if e != nil || terminal(run.State) || run.CurrentPosition != job.Position {
		return safeError()
	}
	anchor, e := s.get(ctx, s.Table, job.PK, "EXEC#"+job.ExecutionID)
	if e != nil || anchor.Plan == nil {
		return safeError()
	}
	certainty := "not_dispatched"
	if job.DispatchIntentAt != "" {
		certainty = "unknown"
	}
	job.RecoveryGeneration = stage.RecoveryGeneration
	job.State = "claimed"
	job.Fence++
	job.LeaseID = id()
	job.Deadline = stamp(s.now().Add(120 * time.Second))
	job.Version++
	anchor.RecoveryGeneration = stage.RecoveryGeneration
	anchor.Version++
	run.RecoveryGeneration = stage.RecoveryGeneration
	run.Version++
	actions := []dt.TransactWriteItem{s.stageGuard(stage), s.projectGuard(p, "active")}
	for _, r := range []row{job, anchor, run} {
		a, e := s.put(s.Table, r, false)
		if e != nil {
			return e
		}
		actions = append(actions, a)
	}
	if e = s.transact(ctx, actions); e != nil {
		return safeError()
	}
	evidence := NewEvidence(job.ProjectID, job.ExecutionID, anchor.SubmittedAt, *anchor.Plan)
	evidence.StartedAt = anchor.StartedAt
	evidence.Fail("recovery_interrupted", certainty)
	return s.publish(ctx, job, evidence)
}
