package execution

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	runner "github.com/VioletProto/Kurier/services/request-runtime"
	"github.com/aws/aws-sdk-go-v2/aws"
	dt "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"io"
	"time"
)

func (s *Service) receiptKeyID() string { return s.digest("local-result-key-id-v1", s.Stage) }
func (s *Service) resultDigest(project, job, agent, keyID string, canonical []byte) (string, error) {
	key := s.SigningKey
	if keyID != s.receiptKeyID() {
		key = s.ReceiptKeys[keyID]
	}
	if len(key) < 32 {
		return "", safeError()
	}
	h := hmac.New(sha256.New, key)
	scope, _ := json.Marshal([]string{"local-result-v1", s.Stage, project, job, agent, keyID})
	h.Write(scope)
	h.Write([]byte{0})
	h.Write(canonical)
	return hex.EncodeToString(h.Sum(nil)), nil
}
func (s *Service) UploadAgentResult(ctx context.Context, token, project, jobID string, result runner.Result) (runner.Receipt, error) {
	stage, a, e := s.authenticateAgent(ctx, token)
	if e != nil {
		return runner.Receipt{}, e
	}
	p, e := s.project(ctx, a.OwnerID, project)
	if e != nil {
		return runner.Receipt{}, e
	}
	scoped := false
	for _, scope := range a.Scopes {
		if scope.ProjectID == project {
			scoped = true
		}
	}
	if !scoped || !uuidPattern.MatchString(jobID) {
		return runner.Receipt{}, failure(404, "not_found")
	}
	canonical, e := runner.CanonicalResult(result)
	if e != nil {
		return runner.Receipt{}, e
	}
	defer clear(canonical)
	receipt, e := s.get(ctx, s.Table, p.PK, "JOB#"+jobID+"#RECEIPT")
	if e == nil {
		if receipt.LocalAgentID != a.LocalAgentID || receipt.RecoveryGeneration != stage.RecoveryGeneration {
			return runner.Receipt{}, failure(409, "conflict")
		}
		digest, e := s.resultDigest(project, jobID, a.LocalAgentID, receipt.HMACKeyID, canonical)
		if e != nil {
			return runner.Receipt{}, e
		}
		if digest != receipt.InputDigest || receipt.LeaseID != result.LeaseID || receipt.Fence != result.Fence || receipt.NonceHash != tokenHash(result.Nonce) {
			return runner.Receipt{}, failure(409, "conflict")
		}
		if e = s.coherentAgentRead(ctx, stage, p, a); e != nil {
			return runner.Receipt{}, e
		}
		if e = s.transact(ctx, []dt.TransactWriteItem{s.stageGuard(stage), s.projectGuard(p, "active"), s.rowGuard(a), s.rowGuard(receipt)}); e != nil {
			return runner.Receipt{}, safeError()
		}
		return runner.Receipt{ExecutionID: receipt.ExecutionID, AcceptedAt: receipt.AcceptedAt}, nil
	}
	if !missingRow(e) {
		return runner.Receipt{}, e
	}
	if e = s.rate(ctx, a.OwnerID, "upload#"+a.LocalAgentID, s.now().UTC().Format("2006-01-02T15:04"), 10); e != nil {
		return runner.Receipt{}, e
	}
	j, e := s.get(ctx, s.Table, p.PK, "JOB#"+jobID)
	if e != nil {
		return runner.Receipt{}, e
	}
	if j.Target != "local" || j.LocalAgentID != a.LocalAgentID || j.RecoveryWriter || j.State != "running" || j.RecoveryGeneration != stage.RecoveryGeneration || j.LeaseID != result.LeaseID || j.Fence != result.Fence || j.NonceHash != tokenHash(result.Nonce) || j.Deadline <= stamp(s.now().Add(5*time.Second)) || a.ActiveJob != jobID || a.ActiveProject != project {
		return runner.Receipt{}, failure(409, "conflict")
	}
	keyID := j.HMACKeyID
	if keyID == "" {
		keyID = s.receiptKeyID()
	}
	digest, e := s.resultDigest(project, jobID, a.LocalAgentID, keyID, canonical)
	if e != nil {
		return runner.Receipt{}, e
	}
	if j.CandidateDigest != "" && j.CandidateDigest != digest {
		return runner.Receipt{}, failure(409, "conflict")
	}
	anchor, e := s.get(ctx, s.Table, p.PK, "EXEC#"+j.ExecutionID)
	if e != nil || anchor.Plan == nil || anchor.Plan.Destination == nil {
		return runner.Receipt{}, safeError()
	}
	if _, e = agentDestination(a, project, anchor.Plan.Configuration.URL); e != nil {
		return runner.Receipt{}, e
	}
	values, e := s.bindings(ctx, j)
	if e != nil {
		return runner.Receipt{}, e
	}
	defer clear(values)
	capture, e := runner.SanitizeLocalCapture(ctx, *anchor.Plan, values, result.Capture)
	if e != nil {
		return runner.Receipt{}, e
	}
	evidence := NewEvidence(project, j.ExecutionID, j.SubmittedAt, *anchor.Plan)
	evidence.StartedAt = &j.DispatchIntentAt
	evidence.CompletedAt = stamp(s.now())
	evidence.Status = capture.Status
	evidence.Outcome = capture.Outcome
	evidence.Request = capture.Request
	evidence.Response = capture.Response
	evidence.Timing = capture.Timing
	evidence.Redaction = capture.Redaction
	j.CandidateDigest = digest
	j.HMACKeyID = keyID
	if e = s.publishLocal(ctx, j, evidence); e != nil {
		return runner.Receipt{}, e
	}
	// Re-authentication means revocation can also deny an ACK after a winning commit.
	return s.UploadAgentResult(ctx, token, project, jobID, result)
}

func (s *Service) localWriterActions(ctx context.Context, stage, p, j row) ([]dt.TransactWriteItem, error) {
	a, e := s.agentByID(ctx, j.OwnerID, j.LocalAgentID)
	if e != nil {
		return nil, failure(401, "unauthenticated")
	}
	if a.ActiveJob != j.JobID || a.ActiveProject != j.ProjectID {
		return nil, failure(409, "conflict")
	}
	user, e := s.userGuard(ctx, j.OwnerID)
	if e != nil {
		return nil, e
	}
	return []dt.TransactWriteItem{s.stageGuard(stage), s.projectGuard(p, "active"), user, s.rowGuard(a)}, nil
}
func (s *Service) publishLocal(ctx context.Context, expected row, evidence *Evidence) error {
	if expected.CandidateCompletedAt != "" {
		evidence.CompletedAt = expected.CandidateCompletedAt
	}
	raw, _, e := EncodeEvidence(evidence)
	if e != nil {
		return e
	}
	stage, p, j, e := s.liveWriter(ctx, expected)
	if e != nil {
		return e
	}
	if j.CandidateDigest != "" && j.CandidateDigest != expected.CandidateDigest {
		return failure(409, "conflict")
	}
	var ticket row
	if j.CandidateTicket != "" {
		ticket, e = s.get(ctx, s.Table, j.PK, j.CandidateTicket)
		if e != nil {
			return e
		}
	} else {
		if expected.CandidateDigest == "" {
			return safeError()
		}
		hash := sha256.Sum256(raw)
		ticketID := id()
		ticket = row{PK: j.PK, SK: "UPLOAD#" + ticketID, Kind: "executionUpload", SchemaVersion: 1, ProjectID: j.ProjectID, OwnerID: j.OwnerID, JobID: j.JobID, ExecutionID: j.ExecutionID, State: "pending", Fence: j.Fence, LeaseID: j.LeaseID, Deadline: j.Deadline, DeletionEpoch: p.DeletionEpoch, RecoveryGeneration: stage.RecoveryGeneration, ObjectKey: s.Stage + "/projects/" + j.ProjectID + "/evidence/" + j.ExecutionID + "/" + ticketID + ".json", Checksum: hex.EncodeToString(hash[:]), Bytes: int64(len(raw)), DPK: dueShard(j.ProjectID), DSK: j.Deadline + "#" + ticketID}
		quota, e := s.get(ctx, s.Table, "STAGE#"+s.Stage, "QUOTA")
		if e != nil {
			return e
		}
		if quota.StorageReservedBytes+ticket.Bytes > 2*1024*1024*1024 || p.StorageReservedBytes+ticket.Bytes > 1024*1024*1024 {
			return safeError()
		}
		quota.StorageReservedBytes += ticket.Bytes
		quota.Version++
		j.CandidateCompletedAt = evidence.CompletedAt
		j.CandidateDigest = expected.CandidateDigest
		j.HMACKeyID = expected.HMACKeyID
		j.CanonicalizationVersion = 1
		j.CandidateTicket = ticket.SK
		j.Version++
		actions, e := s.localWriterActions(ctx, stage, p, j)
		if e != nil {
			return e
		}
		actions[1] = s.projectUpdate(p, ticket.Bytes, 0)
		for _, entry := range []struct {
			r     row
			fresh bool
		}{{j, false}, {ticket, true}, {quota, false}} {
			put, e := s.put(s.Table, entry.r, entry.fresh)
			if e != nil {
				return e
			}
			actions = append(actions, put)
		}
		if e = s.transact(ctx, actions); e != nil {
			return safeError()
		}
	}
	if ticket.Fence != j.Fence || ticket.LeaseID != j.LeaseID || ticket.DeletionEpoch != p.DeletionEpoch || ticket.RecoveryGeneration != stage.RecoveryGeneration {
		return failure(409, "conflict")
	}
	if ticket.State == "writing" && !ticket.WriteSettled {
		return safeError()
	} // Uncertain PUT is retained for reviewed settlement; no second writer.
	if ticket.State == "pending" {
		stage, p, j, e = s.liveWriter(ctx, j)
		if e != nil {
			return e
		}
		actions, e := s.localWriterActions(ctx, stage, p, j)
		if e != nil {
			return e
		}
		ticket.State = "writing"
		ticket.Version++
		j.Version++
		for _, r := range []row{j, ticket} {
			put, e := s.put(s.Table, r, false)
			if e != nil {
				return e
			}
			actions = append(actions, put)
		}
		if e = s.transact(ctx, actions); e != nil {
			return safeError()
		}
		upload, cancel := context.WithTimeout(ctx, 3*time.Second)
		hash := sha256.Sum256(raw)
		_, e = s.Objects.PutObject(upload, &s3.PutObjectInput{Bucket: &s.Bucket, Key: &ticket.ObjectKey, Body: bytes.NewReader(raw), ContentType: aws.String("application/json"), IfNoneMatch: aws.String("*"), ChecksumSHA256: aws.String(base64.StdEncoding.EncodeToString(hash[:]))})
		cancel()
		if e != nil {
			return safeError()
		}
		ticket.State = "uploaded"
		ticket.WriteSettled = true
		ticket.Version++
		put, e := s.put(s.Table, ticket, false)
		if e != nil {
			return e
		}
		if e = s.transact(ctx, []dt.TransactWriteItem{put}); e != nil {
			return safeError()
		}
	} else if ticket.State == "uploaded" && ticket.WriteSettled {
		object, e := s.Objects.GetObject(ctx, &s3.GetObjectInput{Bucket: &s.Bucket, Key: &ticket.ObjectKey})
		if e != nil {
			return safeError()
		}
		stored, e := io.ReadAll(io.LimitReader(object.Body, EvidenceLimit+1))
		_ = object.Body.Close()
		hash := sha256.Sum256(stored)
		if e != nil || int64(len(stored)) != ticket.Bytes || hex.EncodeToString(hash[:]) != ticket.Checksum {
			return safeError()
		}
		if e = StrictJSON(stored, evidence); e != nil {
			return safeError()
		}
	} else {
		return failure(409, "conflict")
	}
	return s.finalize(ctx, j, ticket, evidence)
}
