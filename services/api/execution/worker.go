package execution

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	runner "github.com/VioletProto/Kurier/services/request-runtime"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dt "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"io"
	"regexp"
	"time"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// Process acknowledges duplicate identifiers without granting a second HTTP
// attempt. Only a freshly fenced claimant can commit its own dispatch intent.
func (s *Service) Process(ctx context.Context, m Message) error {
	if !uuidPattern.MatchString(m.ProjectID) || !uuidPattern.MatchString(m.JobID) || m.OutboxID != "OUT#"+m.JobID {
		return nil
	}
	stage, e := s.stage(ctx)
	if e != nil {
		return e
	}
	job, e := s.get(ctx, s.Table, "P#"+m.ProjectID, "JOB#"+m.JobID)
	if missingRow(e) {
		return nil
	}
	if e != nil {
		return e
	}
	if job.Target == "local" {
		return nil
	}
	if job.Kind == "executionJob" && job.RunID != "" && job.RecoveryGeneration != stage.RecoveryGeneration {
		return s.interruptRestoredWorkflow(ctx, stage, job)
	}
	if job.Kind != "executionJob" || job.SchemaVersion != 1 || job.RecoveryGeneration != stage.RecoveryGeneration {
		return nil
	}
	if terminal(job.State) {
		return nil
	}
	p, e := s.project(ctx, job.OwnerID, job.ProjectID)
	if e != nil {
		if missingRow(e) {
			return nil
		}
		return e
	}
	anchor, e := s.get(ctx, s.Table, job.PK, "EXEC#"+job.ExecutionID)
	if e != nil || anchor.Plan == nil {
		return safeError()
	}
	if (job.State == "running" || job.State == "claimed") && job.Deadline > stamp(s.now()) {
		return nil
	}
	recovery := job.State == "running"
	if job.State != "queued" && job.State != "claimed" && !recovery {
		return safeError()
	}
	job.Version++
	job.Fence++
	job.LeaseID = id()
	job.Deadline = stamp(s.now().Add(120 * time.Second))
	job.DPK = dueShard(job.ProjectID)
	job.DSK = job.Deadline + "#" + job.JobID
	if !recovery {
		job.State = "claimed"
		job.Attempts++
	}
	anchor.Version++
	anchor.State = job.State
	jput, e := s.put(s.Table, job, false)
	if e != nil {
		return e
	}
	aput, e := s.put(s.Table, anchor, false)
	if e != nil {
		return e
	}
	if e = s.transact(ctx, []dt.TransactWriteItem{s.stageGuard(stage), s.projectGuard(p, "active"), jput, aput}); e != nil {
		read, readErr := s.get(ctx, s.Table, job.PK, job.SK)
		if readErr != nil || read.LeaseID != job.LeaseID || read.Fence != job.Fence {
			return safeError()
		}
		job = read
	}
	s.tracePublication(job, row{}, "claimed", nil)
	evidence := NewEvidence(job.ProjectID, job.ExecutionID, anchor.SubmittedAt, *anchor.Plan)
	if recovery {
		if recovered, err := s.recoverPrepared(ctx, job); err != nil {
			return err
		} else if recovered {
			return nil
		}
		evidence.StartedAt = pointer(job.DispatchIntentAt)
		evidence.Fail("execution_outcome_unknown", "unknown")
		return s.publish(ctx, job, evidence)
	}
	submitted, err := time.Parse(time.RFC3339Nano, job.SubmittedAt)
	if err != nil {
		return safeError()
	}
	if s.now().Sub(submitted) > 10*time.Minute || job.Attempts > 5 {
		evidence.Fail("queued_expired", "not_dispatched")
		return s.publish(ctx, job, evidence)
	}
	if job.RunID != "" && p.StorageReservedBytes >= 1024*1024*1024 {
		evidence.Fail("input_limit_exceeded", "not_dispatched")
		return s.publish(ctx, job, evidence)
	}
	user, e := s.userGuard(ctx, job.OwnerID)
	if e != nil {
		evidence.Fail("source_unavailable", "not_dispatched")
		return s.publish(ctx, job, evidence)
	}
	source, e := s.get(ctx, s.Table, job.PK, "REQ#"+anchor.Plan.RequestID)
	if e != nil || source.Kind != "request" || source.State != "active" {
		evidence.Fail("source_unavailable", "not_dispatched")
		return s.publish(ctx, job, evidence)
	}
	guards := []dt.TransactWriteItem{user, s.sourceGuard(source, s.Table)}
	for _, ref := range job.Sources {
		r, e := s.get(ctx, s.Protected, job.PK, "SECRET#"+ref.SecretID)
		if e != nil || !validateEnvironmentSecret(*anchor.Plan, r) || r.ProjectID != job.ProjectID || r.SecretID != ref.SecretID {
			evidence.Fail("source_unavailable", "not_dispatched")
			return s.publish(ctx, job, evidence)
		}
		guards = append(guards, s.sourceGuard(r, s.Protected))
	}
	envGuards, envErr := s.environmentGuards(ctx, job.ProjectID, *anchor.Plan)
	if envErr != nil {
		evidence.Fail("source_unavailable", "not_dispatched")
		return s.publish(ctx, job, evidence)
	}
	guards = append(guards, envGuards...)
	values := map[string]string{}
	var capture *runner.ExtractionCapture
	var output *Envelope
	runtimePlan := *anchor.Plan
	if job.RunID != "" {
		runtimePlan, values, capture, e = s.workflowRuntime(ctx, job, runtimePlan)
		if e != nil {
			code := "source_unavailable"
			var err *Error
			if errors.As(e, &err) {
				code = err.Code
			}
			evidence.Fail(code, "not_dispatched")
			return s.publish(ctx, job, evidence)
		}
	} else {
		bindings, e := s.get(ctx, s.Protected, job.PK, job.SK+"#BINDINGS")
		if e != nil || bindings.Kind != "executionBindings" || bindings.ProjectID != job.ProjectID {
			return safeError()
		}
		raw, e := s.decrypt(ctx, job.ProjectID, job.JobID, "job-bindings", 1, bindings.Envelope)
		if e != nil {
			return e
		}
		err = StrictJSON(raw, &values)
		clear(raw)
		if err != nil {
			return safeError()
		}
	}
	defer clear(values)
	prepareRequest := s.prepare
	if prepareRequest == nil {
		prepareRequest = Prepare
	}
	prepared, e := prepareRequest(ctx, runtimePlan, values)
	if e != nil {
		code := "input_limit_exceeded"
		var a *Error
		if errors.As(e, &a) {
			code = a.Code
		}
		evidence.Fail(code, "not_dispatched")
		return s.publish(ctx, job, evidence)
	}
	if capture != nil {
		capture.Seal = func(values map[string]runner.RuntimeValue) error {
			var err error
			output, err = s.encryptPurpose(ctx, job.ProjectID, job.RunID+fmt.Sprintf("/%02d", job.Position), "workflow-output", values)
			return err
		}
		prepared.WorkflowCapture(capture)
	}
	// Deletion, source revocation/replacement and recovery races are fenced in
	// the same durable intent transaction. Uncertain intent never grants HTTP.
	stage, p, job, e = s.liveWriter(ctx, job)
	if e != nil {
		return e
	}
	anchor, e = s.get(ctx, s.Table, job.PK, "EXEC#"+job.ExecutionID)
	if e != nil {
		return e
	}
	job.State = "running"
	job.Version++
	job.DispatchIntentAt = stamp(s.now())
	anchor.State = "running"
	anchor.Version++
	anchor.StartedAt = &job.DispatchIntentAt
	jput, _ = s.put(s.Table, job, false)
	aput, _ = s.put(s.Table, anchor, false)
	actions := append([]dt.TransactWriteItem{s.stageGuard(stage), s.projectGuard(p, "active"), jput, aput}, guards...)
	if job.RunID != "" {
		extra, err := s.workflowIntent(ctx, job)
		if err != nil {
			return err
		}
		actions = append(actions, extra...)
	}
	if e = s.transact(ctx, actions); e != nil {
		read, readErr := s.get(ctx, s.Table, job.PK, job.SK)
		if readErr != nil || read.State != "running" || read.LeaseID != job.LeaseID || read.Fence != job.Fence || read.Version != job.Version {
			return safeError()
		}
	}
	s.tracePublication(job, row{}, "intent", nil)
	prepared.Run(ctx, evidence)
	evidence.StartedAt = &job.DispatchIntentAt

	return s.publish(ctx, job, evidence, output)
}
func (s *Service) liveWriter(ctx context.Context, expected row) (row, row, row, error) {
	stage, e := s.stage(ctx)
	if e != nil {
		return row{}, row{}, row{}, e
	}
	p, e := s.project(ctx, expected.OwnerID, expected.ProjectID)
	if e != nil {
		return row{}, row{}, row{}, e
	}
	job, e := s.get(ctx, s.Table, expected.PK, expected.SK)
	if e != nil || job.RecoveryGeneration != stage.RecoveryGeneration || job.LeaseID != expected.LeaseID || job.Fence != expected.Fence || job.State != expected.State || terminal(job.State) || job.Deadline <= stamp(s.now()) {
		return row{}, row{}, row{}, failure(409, "conflict")
	}
	if job.Target == "local" && !job.RecoveryWriter {
		if job.Deadline <= stamp(s.now().Add(5*time.Second)) {
			return row{}, row{}, row{}, failure(409, "conflict")
		}
		if _, e = s.agentByID(ctx, job.OwnerID, job.LocalAgentID); e != nil {
			return row{}, row{}, row{}, failure(401, "unauthenticated")
		}
	}
	return stage, p, job, nil
}
func (s *Service) publish(ctx context.Context, job row, evidence *Evidence, outputs ...*Envelope) error {
	if job.Target == "local" && !job.RecoveryWriter {
		return s.publishLocal(ctx, job, evidence)
	}
	if e := s.workflowEvidence(ctx, job, evidence, true); e != nil {
		return e
	}
	raw, _, e := EncodeEvidence(evidence)
	if e != nil {
		return e
	}
	stage, p, job, e := s.liveWriter(ctx, job)
	if e != nil {
		return e
	}
	if p.StorageReservedBytes+int64(len(raw)) > 1024*1024*1024 {
		return safeError()
	}
	quota, e := s.get(ctx, s.Table, "STAGE#"+s.Stage, "QUOTA")
	if e != nil || quota.StorageReservedBytes+int64(len(raw)) > 2*1024*1024*1024 {
		return safeError()
	}
	quota.StorageReservedBytes += int64(len(raw))
	quota.Version++
	qput, _ := s.put(s.Table, quota, false)
	hash := sha256.Sum256(raw)
	ticketID := id()
	ticket := row{PK: job.PK, SK: "UPLOAD#" + ticketID, Kind: "executionUpload", SchemaVersion: 1, ProjectID: job.ProjectID, OwnerID: job.OwnerID, JobID: job.JobID, ExecutionID: job.ExecutionID, State: "pending", RecoveryWriter: job.RecoveryWriter, Fence: job.Fence, LeaseID: job.LeaseID, Deadline: job.Deadline, DeletionEpoch: p.DeletionEpoch, RecoveryGeneration: stage.RecoveryGeneration, ObjectKey: s.Stage + "/projects/" + job.ProjectID + "/evidence/" + job.ExecutionID + "/" + ticketID + ".json", Checksum: hex.EncodeToString(hash[:]), Bytes: int64(len(raw)), DPK: dueShard(job.ProjectID), DSK: job.Deadline + "#" + ticketID}
	put, _ := s.put(s.Table, ticket, true)
	// CAS on the authoritative job version registers before any S3 attempt.
	job.Version++
	jput, _ := s.put(s.Table, job, false)
	if e = s.transact(ctx, []dt.TransactWriteItem{s.stageGuard(stage), s.projectUpdate(p, ticket.Bytes, 0), jput, put, qput}); e != nil {
		s.tracePublication(job, ticket, "register", e)
		read, readErr := s.get(ctx, s.Table, ticket.PK, ticket.SK)
		if readErr != nil || read.Checksum != ticket.Checksum {
			return safeError()
		}
		ticket = read
	}
	s.tracePublication(job, ticket, "registered", nil)
	// Persist writing intent separately. A crash/timeout after it is unresolved
	// until the PUT outcome is durably settled; absence alone is not settlement.
	stage, p, job, e = s.liveWriter(ctx, job)
	if e != nil {
		return e
	}
	ticket.State = "writing"
	ticket.Version++
	tput, _ := s.put(s.Table, ticket, false)
	job.Version++
	jput, _ = s.put(s.Table, job, false)
	if e = s.transact(ctx, []dt.TransactWriteItem{s.stageGuard(stage), s.projectGuard(p, "active"), jput, tput}); e != nil {
		s.tracePublication(job, ticket, "writing", e)
		return safeError()
	}
	s.tracePublication(job, ticket, "writing", nil)
	upload, cancel := context.WithTimeout(ctx, 15*time.Second)
	_, e = s.Objects.PutObject(upload, &s3.PutObjectInput{Bucket: &s.Bucket, Key: &ticket.ObjectKey, Body: bytes.NewReader(raw), ContentType: aws.String("application/json"), IfNoneMatch: aws.String("*"), ChecksumSHA256: aws.String(base64.StdEncoding.EncodeToString(hash[:]))})
	cancel()
	s.tracePublication(job, ticket, "put_ack", e)
	if e != nil {
		return safeError()
	}
	// Acknowledged PUT is durable even if deletion now denies publication.
	ticket.WriteSettled = true
	ticket.State = "uploaded"
	ticket.Version++
	tput, _ = s.put(s.Table, ticket, false)
	if e = s.transact(ctx, []dt.TransactWriteItem{tput}); e != nil {
		s.tracePublication(job, ticket, "settle", e)
		read, readErr := s.get(ctx, s.Table, ticket.PK, ticket.SK)
		if readErr != nil || !read.WriteSettled {
			return safeError()
		}
		ticket = read
	}
	s.tracePublication(job, ticket, "settled", nil)
	e = s.finalize(ctx, job, ticket, evidence, outputs...)
	s.tracePublication(job, ticket, "finalize", e)
	return e
}
func (s *Service) finalize(ctx context.Context, expected, ticket row, evidence *Evidence, outputs ...*Envelope) error {
	if e := s.workflowEvidence(ctx, expected, evidence, false); e != nil {
		return e
	}
	stage, p, job, e := s.liveWriter(ctx, expected)
	if e != nil {
		return e
	}
	current, e := s.get(ctx, s.Table, ticket.PK, ticket.SK)
	if e != nil || current.State != "uploaded" || !current.WriteSettled || current.Fence != job.Fence || current.LeaseID != job.LeaseID || current.DeletionEpoch != p.DeletionEpoch {
		return failure(409, "conflict")
	}
	ticket = current
	anchor, e := s.get(ctx, s.Table, job.PK, "EXEC#"+job.ExecutionID)
	if e != nil {
		return e
	}
	completed, e := time.Parse(time.RFC3339Nano, evidence.CompletedAt)
	if e != nil {
		return safeError()
	}
	summary := &Summary{HTTPStatus: evidence.Response.HTTPStatus, DurationMS: evidence.Timing.DurationMS, Outcome: evidence.Outcome}
	job.State = evidence.Status
	job.Version++
	job.DPK = ""
	job.DSK = ""
	anchor.State = evidence.Status
	anchor.Version++
	anchor.CompletedAt = &evidence.CompletedAt
	anchor.Summary = summary
	anchor.StartedAt = evidence.StartedAt
	ticket.State = "published"
	ticket.Version++
	ticket.DPK = ""
	ticket.DSK = ""
	manifest := row{PK: job.PK, SK: anchor.SK + "#SNAP", Kind: "executionSnapshot", SchemaVersion: 1, ProjectID: job.ProjectID, ExecutionID: job.ExecutionID, ObjectKey: ticket.ObjectKey, Checksum: ticket.Checksum, Bytes: ticket.Bytes, Summary: summary, CompletedAt: &evidence.CompletedAt}
	ret := row{PK: job.PK, SK: anchor.SK + "#RET", Kind: "executionRetention", SchemaVersion: 1, ProjectID: job.ProjectID, ExecutionID: job.ExecutionID, State: "live", NormalExpiresAt: stamp(completed.Add(30 * 24 * time.Hour)), EvidenceAvailability: "available", DPK: dueShard(job.ProjectID), DSK: stamp(completed.Add(30*24*time.Hour)) + "#" + job.ExecutionID}
	actions := []dt.TransactWriteItem{s.stageGuard(stage), s.projectUpdate(p, 0, -1)}
	continuing := false
	if job.RunID != "" {
		if p.InflightCount < 1 {
			return safeError()
		}
		var output *Envelope
		if len(outputs) > 0 {
			output = outputs[0]
		}
		extra, more, err := s.workflowFinalActions(ctx, job, evidence, output)
		if err != nil {
			return err
		}
		continuing = more
		actions = append(actions, extra...)
		if continuing {
			actions[1] = s.projectUpdate(p, 0, 0)
		}
		ret.RunID = job.RunID
		ret.Position = job.Position
	}
	if job.Target == "local" {
		a, e := s.get(ctx, s.Table, "U#"+job.OwnerID, "CRED#"+job.LocalAgentID)
		if e != nil {
			return e
		}
		if !job.RecoveryWriter {
			user, e := s.userGuard(ctx, job.OwnerID)
			if e != nil {
				return e
			}
			actions = append(actions, user)
			if a.State != "active" || a.ExpiresAt <= stamp(s.now().Add(5*time.Second)) || a.RecoveryGeneration != stage.RecoveryGeneration || a.ActiveJob != job.JobID || a.ActiveProject != job.ProjectID || job.CandidateDigest == "" {
				return failure(401, "unauthenticated")
			}
			receipt := row{PK: job.PK, SK: job.SK + "#RECEIPT", Kind: "executionLocalReceipt", SchemaVersion: 1, ProjectID: job.ProjectID, OwnerID: job.OwnerID, ExecutionID: job.ExecutionID, JobID: job.JobID, LocalAgentID: job.LocalAgentID, LeaseID: job.LeaseID, Fence: job.Fence, NonceHash: job.NonceHash, InputDigest: job.CandidateDigest, CanonicalizationVersion: job.CanonicalizationVersion, TemplateSemanticsVersion: job.TemplateSemanticsVersion, EnvironmentContext: job.EnvironmentContext, HMACKeyID: job.HMACKeyID, AcceptedAt: stamp(s.now()), RecoveryGeneration: stage.RecoveryGeneration}
			put, e := s.put(s.Table, receipt, true)
			if e != nil {
				return e
			}
			actions = append(actions, put)
		}
		if a.ActiveJob == job.JobID && a.ActiveProject == job.ProjectID {
			a.ActiveJob = ""
			a.ActiveProject = ""
			a.Version++
			put, e := s.put(s.Table, a, false)
			if e != nil {
				return e
			}
			actions = append(actions, put)
		} else if !job.RecoveryWriter {
			return failure(409, "conflict")
		}
	}
	for _, spec := range []struct {
		r     row
		fresh bool
	}{{job, false}, {anchor, false}, {ticket, false}, {manifest, true}, {ret, true}} {
		a, e := s.put(s.Table, spec.r, spec.fresh)
		if e != nil {
			return e
		}
		actions = append(actions, a)
	}
	actions = append(actions, dt.TransactWriteItem{Delete: &dt.Delete{TableName: &s.Protected, Key: keys(job.PK, job.SK+"#BINDINGS")}})
	if job.RunID != "" && len(actions) > 60 {
		return safeError()
	}
	if e = s.transact(ctx, actions); e != nil {
		read, readErr := s.get(ctx, s.Table, job.PK, manifest.SK)
		if readErr == nil && read.Checksum == ticket.Checksum && read.ObjectKey == ticket.ObjectKey {
			return nil
		}
		return safeError()
	}
	if continuing {
		s.fastNotify(ctx, job.ProjectID, workflowIdentity(job.RunID, job.Position+1, "job"))
	}
	return nil
}
func (s *Service) Evidence(ctx context.Context, owner, project, execution string) ([]byte, error) {
	v, e := s.Detail(ctx, owner, project, execution)
	if e != nil {
		return nil, e
	}
	if !terminal(v.Status) {
		return nil, failure(409, "conflict")
	}
	manifest, e := s.get(ctx, s.Table, "P#"+project, "EXEC#"+execution+"#SNAP")
	if e != nil || manifest.Kind != "executionSnapshot" || manifest.Bytes > EvidenceLimit || manifest.Bytes < 1 {
		return nil, safeError()
	}
	out, e := s.Objects.GetObject(ctx, &s3.GetObjectInput{Bucket: &s.Bucket, Key: &manifest.ObjectKey})
	if e != nil {
		return nil, s.evidenceUnavailable(ctx, owner, project, execution)
	}
	defer out.Body.Close()
	raw, e := io.ReadAll(io.LimitReader(out.Body, EvidenceLimit+1))
	if e != nil {
		return nil, s.evidenceUnavailable(ctx, owner, project, execution)
	}
	hash := sha256.Sum256(raw)
	if int64(len(raw)) != manifest.Bytes || hex.EncodeToString(hash[:]) != manifest.Checksum {
		return nil, s.evidenceUnavailable(ctx, owner, project, execution)
	}
	var stored Evidence
	if StrictJSON(raw, &stored) != nil {
		return nil, s.evidenceUnavailable(ctx, owner, project, execution)
	}
	anchor, err := s.get(ctx, s.Table, "P#"+project, "EXEC#"+execution)
	if err != nil || s.workflowEvidence(ctx, anchor, &stored, false) != nil {
		return nil, s.evidenceUnavailable(ctx, owner, project, execution)
	}
	api, e := json.Marshal(map[string]any{"data": map[string]any{"evidence": json.RawMessage(raw)}})
	if e != nil || len(api) > EvidenceLimit {
		return nil, s.evidenceUnavailable(ctx, owner, project, execution)
	}
	if _, e = s.Detail(ctx, owner, project, execution); e != nil {
		return nil, e
	}
	return api, nil
}

func (s *Service) evidenceUnavailable(ctx context.Context, owner, project, execution string) error {
	stage, e := s.stage(ctx)
	if e != nil {
		return failure(503, "evidence_unavailable")
	}
	p, e := s.project(ctx, owner, project)
	if e != nil {
		return e
	}
	ret, e := s.get(ctx, s.Table, p.PK, "EXEC#"+execution+"#RET")
	if e == nil && ret.State == "live" && (ret.Pinned || ret.NormalExpiresAt > stamp(s.now())) && ret.EvidenceAvailability != "unavailable" {
		ret.EvidenceAvailability = "unavailable"
		ret.Version++
		a, err := s.put(s.Table, ret, false)
		if err == nil {
			_ = s.transact(ctx, []dt.TransactWriteItem{s.stageGuard(stage), s.projectGuard(p, "active"), a})
		}
	}
	return failure(503, "evidence_unavailable")
}

// An acknowledged upload can be republished under the new fence. This retries
// only the immutable sanitized result; there is no outbound HTTP here.
func (s *Service) recoverPrepared(ctx context.Context, job row) (bool, error) {
	if job.RunID != "" {
		return false, nil
	}
	var start map[string]dt.AttributeValue
	for page := 0; page < 5; page++ {
		rows, e := s.DB.Query(ctx, &dynamodb.QueryInput{TableName: &s.Table, KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK,:prefix)"), ExpressionAttributeValues: map[string]dt.AttributeValue{":pk": str(job.PK), ":prefix": str("UPLOAD#")}, ConsistentRead: aws.Bool(true), Limit: aws.Int32(20), ExclusiveStartKey: start})
		if e != nil {
			return false, safeError()
		}
		for _, item := range rows.Items {
			var ticket row
			if attributevalue.UnmarshalMap(item, &ticket) != nil {
				return false, safeError()
			}
			if ticket.ExecutionID != job.ExecutionID || ticket.State != "uploaded" || !ticket.WriteSettled || (job.Target == "local" && !ticket.RecoveryWriter) {
				continue
			}
			if !knownExecution(item, ticket) {
				return false, safeError()
			}
			object, e := s.Objects.GetObject(ctx, &s3.GetObjectInput{Bucket: &s.Bucket, Key: &ticket.ObjectKey})
			if e != nil {
				return false, safeError()
			}
			raw, e := io.ReadAll(io.LimitReader(object.Body, EvidenceLimit+1))
			_ = object.Body.Close()
			hash := sha256.Sum256(raw)
			if e != nil || int64(len(raw)) != ticket.Bytes || hex.EncodeToString(hash[:]) != ticket.Checksum {
				return false, safeError()
			}
			var evidence Evidence
			if StrictJSON(raw, &evidence) != nil || runner.ValidateEvidenceVersion(&evidence) != nil || evidence.ExecutionID != job.ExecutionID || evidence.ProjectID != job.ProjectID || !terminal(evidence.Status) {
				return false, safeError()
			}
			final, _, e := EncodeEvidence(&evidence)
			if e != nil || !bytes.Equal(final, raw) {
				return false, safeError()
			}
			stage, p, current, e := s.liveWriter(ctx, job)
			if e != nil {
				return false, e
			}
			if ticket.DeletionEpoch != p.DeletionEpoch || ticket.RecoveryGeneration != stage.RecoveryGeneration {
				return false, failure(409, "conflict")
			}
			ticket.Version++
			ticket.Fence = current.Fence
			ticket.LeaseID = current.LeaseID
			ticket.Deadline = current.Deadline
			tput, _ := s.put(s.Table, ticket, false)
			current.Version++
			jput, _ := s.put(s.Table, current, false)
			if e = s.transact(ctx, []dt.TransactWriteItem{s.stageGuard(stage), s.projectGuard(p, "active"), tput, jput}); e != nil {
				return false, safeError()
			}
			return true, s.finalize(ctx, current, ticket, &evidence)
		}
		start = rows.LastEvaluatedKey
		if len(start) == 0 {
			return false, nil
		}
	}
	return false, safeError()
}
