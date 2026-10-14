package execution

import (
	"context"
	"encoding/json"
	runner "github.com/VioletProto/Kurier/services/request-runtime"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dt "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"time"
)

func agentQueue(a, p string) string { return "AGENT#" + a + "#PROJECT#" + p }
func newWork(j row, p Plan, values map[string]string, nonce string) *runner.Work {
	w := &runner.Work{SchemaVersion: 1, ProjectID: j.ProjectID, JobID: j.JobID, ExecutionID: j.ExecutionID, LocalAgentID: j.LocalAgentID, RecoveryGeneration: j.RecoveryGeneration, LeaseID: j.LeaseID, Fence: j.Fence, Nonce: nonce, DispatchIntentAt: j.DispatchIntentAt, ExecuteNotAfter: j.ExecuteNotAfter, Deadline: j.Deadline, Plan: p, Destination: *p.Destination}
	w.ProtectedInputs.Values = values
	return w
}
func grantFits(w *runner.Work, serverTime string) bool {
	raw, e := json.Marshal(map[string]any{"data": runner.PollResponse{Work: w, ServerTime: serverTime}})
	return e == nil && len(raw)+1 <= runner.GrantLimit
}
func (s *Service) bindings(ctx context.Context, j row) (map[string]string, error) {
	r, e := s.get(ctx, s.Protected, j.PK, j.SK+"#BINDINGS")
	if e != nil || r.Kind != "executionBindings" || r.JobID != j.JobID || r.ProjectID != j.ProjectID {
		return nil, safeError()
	}
	raw, e := s.decrypt(ctx, j.ProjectID, j.JobID, "job-bindings", 1, r.Envelope)
	if e != nil {
		return nil, e
	}
	defer clear(raw)
	v := map[string]string{}
	if e = StrictJSON(raw, &v); e != nil {
		clear(v)
		return nil, safeError()
	}
	return v, nil
}
func (s *Service) PollAgent(ctx context.Context, token, project string) (runner.PollResponse, error) {
	response := runner.PollResponse{ServerTime: stamp(s.now())}
	stage, a, e := s.authenticateAgent(ctx, token)
	if e != nil {
		return response, e
	}
	if !uuidPattern.MatchString(project) {
		return response, failure(404, "not_found")
	}
	scoped := false
	for _, scope := range a.Scopes {
		if scope.ProjectID == project {
			scoped = true
		}
	}
	if !scoped {
		return response, failure(404, "not_found")
	}
	p, e := s.project(ctx, a.OwnerID, project)
	if e != nil {
		return response, e
	}
	if e = s.rate(ctx, a.OwnerID, "poll#"+a.LocalAgentID, s.now().UTC().Format("2006-01-02T15:04:05"), 2); e != nil {
		return response, e
	}
	// A lost response never regrants its active job. Only reconciliation can free it.
	if a.ActiveJob != "" {
		if e = s.reconcileLocal(ctx, a.ActiveProject, a.ActiveJob); e != nil {
			return response, e
		}
		_, a, e = s.authenticateAgent(ctx, token)
		if e != nil {
			return response, e
		}
		if a.ActiveJob != "" {
			response.Busy = true
			return response, s.recordAgentPresence(ctx, stage, p, a)
		}
	}
	var start map[string]dt.AttributeValue
	for page := 0; page < 5; page++ {
		out, e := s.DB.Query(ctx, &dynamodb.QueryInput{TableName: &s.Table, IndexName: aws.String("GSI2"), KeyConditionExpression: aws.String("DPK = :pk"), ExpressionAttributeValues: map[string]dt.AttributeValue{":pk": str(agentQueue(a.LocalAgentID, project))}, ExclusiveStartKey: start, Limit: aws.Int32(20)})
		if e != nil {
			return response, safeError()
		}
		for _, item := range out.Items {
			var candidate row
			if attributevalue.UnmarshalMap(item, &candidate) != nil {
				return response, safeError()
			}
			j, e := s.get(ctx, s.Table, candidate.PK, candidate.SK)
			if missingRow(e) {
				continue
			}
			if e != nil {
				return response, e
			}
			if j.ProjectID != project || j.OwnerID != a.OwnerID || j.LocalAgentID != a.LocalAgentID || j.Target != "local" || j.State != "queued" || j.RecoveryGeneration != stage.RecoveryGeneration {
				continue
			}
			submitted, e := time.Parse(time.RFC3339Nano, j.SubmittedAt)
			if e != nil {
				return response, safeError()
			}
			if !s.now().Before(submitted.Add(10 * time.Minute)) {
				_ = s.reconcileLocal(ctx, project, j.JobID)
				continue
			}
			anchor, e := s.get(ctx, s.Table, j.PK, "EXEC#"+j.ExecutionID)
			if e != nil || anchor.Plan == nil || anchor.Plan.Destination == nil {
				return response, safeError()
			}
			d, e := agentDestination(a, project, anchor.Plan.Configuration.URL)
			if e != nil || d != *anchor.Plan.Destination {
				return response, failure(409, "local_permission_required")
			}
			source, e := s.get(ctx, s.Table, j.PK, "REQ#"+anchor.Plan.RequestID)
			if e != nil || source.State != "active" || source.Kind != "request" {
				_ = s.reconcileLocal(ctx, project, j.JobID)
				continue
			}
			user, e := s.userGuard(ctx, a.OwnerID)
			if e != nil {
				return response, e
			}
			guards := []dt.TransactWriteItem{s.stageGuard(stage), s.projectGuard(p, "active"), user, s.sourceGuard(source, s.Table)}
			eligible := true
			for _, ref := range j.Sources {
				r, e := s.get(ctx, s.Protected, j.PK, "SECRET#"+ref.SecretID)
				if e != nil || r.Kind != "savedSecret" || r.State != "active" || r.SecretID != ref.SecretID || r.ProjectID != project {
					eligible = false
					break
				}
				guards = append(guards, s.sourceGuard(r, s.Protected))
			}
			if !eligible {
				_ = s.reconcileLocal(ctx, project, j.JobID)
				continue
			}
			values, e := s.bindings(ctx, j)
			if e != nil {
				return response, e
			}
			if _, e = runner.LocalRequestCapture(ctx, *anchor.Plan, values, d); e != nil {
				clear(values)
				return response, e
			}
			nonce := randomToken()
			j.Version++
			j.Fence++
			j.LeaseID = id()
			j.NonceHash = tokenHash(nonce)
			j.State = "running"
			grantTime := s.now()
			j.DispatchIntentAt = stamp(grantTime)
			j.ExecuteNotAfter = stamp(grantTime.Add(30 * time.Second))
			j.Deadline = stamp(grantTime.Add(120 * time.Second))
			j.DPK = dueShard(project)
			j.DSK = j.Deadline + "#" + j.JobID
			w := newWork(j, *anchor.Plan, values, nonce)
			if !grantFits(w, stamp(s.now())) {
				clear(values)
				return response, failure(413, "payload_too_large")
			}
			anchor.State = "running"
			anchor.Version++
			anchor.StartedAt = &j.DispatchIntentAt
			a.ActiveJob = j.JobID
			a.ActiveProject = project
			a.Version++
			seen := stamp(s.now())
			if a.LastSeenAt == nil || *a.LastSeenAt < stamp(s.now().Add(-time.Minute)) {
				a.LastSeenAt = &seen
			}
			for _, r := range []row{j, anchor, a} {
				put, e := s.put(s.Table, r, false)
				if e != nil {
					clear(values)
					return response, e
				}
				guards = append(guards, put)
			}
			wait, e := s.get(ctx, s.Table, j.PK, "LOCALWAIT#"+j.JobID)
			if e != nil {
				clear(values)
				return response, e
			}
			guards = append(guards, deleteVersion(s.Table, wait))
			// No uncertainty readback may grant executable bytes, even if commit won.
			if e = s.transact(ctx, guards); e != nil {
				clear(values)
				return response, safeError()
			}
			currentStage, current, e := s.authenticateAgent(ctx, token)
			if e != nil {
				clear(values)
				return response, e
			}
			currentProject, e := s.project(ctx, a.OwnerID, project)
			if e != nil {
				clear(values)
				return response, e
			}
			currentJob, e := s.get(ctx, s.Table, j.PK, j.SK)
			if e != nil || current.ActiveJob != j.JobID || current.ActiveProject != project || currentJob.Fence != j.Fence || currentJob.State != "running" || currentStage.RecoveryGeneration != j.RecoveryGeneration {
				clear(values)
				return response, safeError()
			}
			if e = s.transact(ctx, []dt.TransactWriteItem{s.stageGuard(currentStage), s.projectGuard(currentProject, "active"), s.rowGuard(current), s.rowGuard(currentJob)}); e != nil {
				clear(values)
				return response, safeError()
			}
			response.Work = w
			response.ServerTime = stamp(s.now())
			return response, nil
		}
		start = out.LastEvaluatedKey
		if len(start) == 0 {
			break
		}
	}
	// Presence updates are bounded and cannot overwrite a concurrently occupied slot.
	if a.LastSeenAt == nil || *a.LastSeenAt < stamp(s.now().Add(-time.Minute)) {
		a.LastSeenAt = pointer(stamp(s.now()))
		a.Version++
		put, e := s.put(s.Table, a, false)
		if e != nil {
			return response, e
		}
		user, e := s.userGuard(ctx, a.OwnerID)
		if e != nil {
			return response, e
		}
		if e = s.transact(ctx, []dt.TransactWriteItem{s.stageGuard(stage), s.projectGuard(p, "active"), user, put}); e != nil {
			return response, safeError()
		}
	} else {
		if e = s.coherentAgentRead(ctx, stage, p, a); e != nil {
			return response, e
		}
	}
	response.ServerTime = stamp(s.now())
	return response, nil
}
func (s *Service) coherentAgentRead(ctx context.Context, stage, p, a row) error {
	user, e := s.userGuard(ctx, a.OwnerID)
	if e != nil {
		return e
	}
	if e = s.transact(ctx, []dt.TransactWriteItem{s.stageGuard(stage), s.projectGuard(p, "active"), user, s.rowGuard(a)}); e != nil {
		return safeError()
	}
	return nil
}

// Reconciliation publishes only a safe failure. It never calls Prepare/Run,
// never promotes a pending local candidate, and never creates a client receipt.
func (s *Service) reconcileLocal(ctx context.Context, project, jobID string) error {
	stage, e := s.localStage(ctx)
	if e != nil {
		return e
	}
	j, e := s.get(ctx, s.Table, "P#"+project, "JOB#"+jobID)
	if missingRow(e) {
		return nil
	}
	if e != nil {
		return e
	}
	if j.Target != "local" || terminal(j.State) {
		return nil
	}
	p, e := s.project(ctx, j.OwnerID, project)
	if e != nil {
		return e
	}
	anchor, e := s.get(ctx, s.Table, j.PK, "EXEC#"+j.ExecutionID)
	if e != nil || anchor.Plan == nil {
		return safeError()
	}
	a, e := s.get(ctx, s.Table, "U#"+j.OwnerID, "CRED#"+j.LocalAgentID)
	if e != nil {
		return e
	}
	reason := ""
	certainty := "not_dispatched"
	if j.RecoveryGeneration != stage.RecoveryGeneration {
		reason = "recovery_interrupted"
		if j.DispatchIntentAt != "" {
			certainty = "unknown"
		}
	}
	if reason == "" && j.State == "running" {
		if j.Deadline > stamp(s.now()) && a.State == "active" && a.ExpiresAt > stamp(s.now()) && a.RecoveryGeneration == stage.RecoveryGeneration {
			return nil
		}
		reason = "execution_outcome_unknown"
		certainty = "unknown"
	}
	if reason == "" && j.State == "queued" {
		if a.State != "active" || a.ExpiresAt <= stamp(s.now()) || a.RecoveryGeneration != stage.RecoveryGeneration {
			reason = "source_unavailable"
		}
		source, se := s.get(ctx, s.Table, j.PK, "REQ#"+anchor.Plan.RequestID)
		if missingRow(se) || se == nil && (source.State != "active" || source.Kind != "request") {
			reason = "source_unavailable"
		} else if se != nil {
			return se
		}
		for _, ref := range j.Sources {
			r, se := s.get(ctx, s.Protected, j.PK, "SECRET#"+ref.SecretID)
			if missingRow(se) || se == nil && r.State != "active" {
				reason = "source_unavailable"
			} else if se != nil {
				return se
			}
		}
		if _, se := s.userGuard(ctx, j.OwnerID); se != nil {
			reason = "source_unavailable"
		}
		if reason == "" && j.SubmittedAt <= stamp(s.now().Add(-10*time.Minute)) {
			reason = "queued_expired"
		}
		if reason == "" {
			return nil
		}
	}
	if reason == "" {
		return safeError()
	}
	resumeRecovery := j.RecoveryWriter && j.RecoveryGeneration == stage.RecoveryGeneration
	j.RecoveryGeneration = stage.RecoveryGeneration
	j.Version++
	j.Fence++
	j.LeaseID = id()
	j.Deadline = stamp(s.now().Add(120 * time.Second))
	j.State = "running"
	j.RecoveryWriter = true
	j.CandidateDigest = ""
	j.CandidateTicket = ""
	j.DPK = dueShard(project)
	j.DSK = j.Deadline + "#" + j.JobID
	anchor.Version++
	anchor.State = "running"
	guards := []dt.TransactWriteItem{s.stageGuard(stage), s.projectGuard(p, "active"), s.rowGuard(a)}
	for _, r := range []row{j, anchor} {
		put, e := s.put(s.Table, r, false)
		if e != nil {
			return e
		}
		guards = append(guards, put)
	}
	wait, e := s.get(ctx, s.Table, j.PK, "LOCALWAIT#"+j.JobID)
	if e == nil {
		guards = append(guards, deleteVersion(s.Table, wait))
	} else if !missingRow(e) {
		return e
	}
	if e = s.transact(ctx, guards); e != nil {
		return safeError()
	}
	if resumeRecovery {
		if recovered, e := s.recoverPrepared(ctx, j); e != nil {
			return e
		} else if recovered {
			return nil
		}
	}
	capture := NewEvidence(project, j.ExecutionID, j.SubmittedAt, *anchor.Plan)
	capture.Fail(reason, certainty)
	capture.CompletedAt = stamp(s.now())
	if j.DispatchIntentAt != "" {
		capture.StartedAt = &j.DispatchIntentAt
	}
	return s.publish(ctx, j, capture)
}

func (s *Service) recordAgentPresence(ctx context.Context, stage, p, a row) error {
	if a.LastSeenAt != nil && *a.LastSeenAt >= stamp(s.now().Add(-time.Minute)) {
		return s.coherentAgentRead(ctx, stage, p, a)
	}
	a.LastSeenAt = pointer(stamp(s.now()))
	a.Version++
	put, e := s.put(s.Table, a, false)
	if e != nil {
		return e
	}
	user, e := s.userGuard(ctx, a.OwnerID)
	if e != nil {
		return e
	}
	if e = s.transact(ctx, []dt.TransactWriteItem{s.stageGuard(stage), s.projectGuard(p, "active"), user, put}); e != nil {
		return safeError()
	}
	return nil
}
