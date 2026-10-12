package execution

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dt "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const ProjectPinLimit int64 = 100
const StagePinLimit int64 = 200
const ProjectPinBytes int64 = 256 * 1024 * 1024
const StagePinBytes int64 = 512 * 1024 * 1024

func (s *Service) drainGuards(ctx context.Context, stage, p row, execution string, retention bool) ([]dt.TransactWriteItem, error) {
	expected := "deleting"
	if retention {
		expected = "active"
	}
	if p.State != expected {
		return nil, failure(409, "conflict")
	}
	guards := []dt.TransactWriteItem{s.stageGuard(stage), s.projectGuard(p, expected)}
	if retention {
		ret, err := s.get(ctx, s.Table, p.PK, "EXEC#"+execution+"#RET")
		if err != nil || ret.State != "deleting" || ret.Pinned {
			return nil, failure(409, "conflict")
		}
		guard := s.rowGuard(ret)
		guard.ConditionCheck.ExpressionAttributeNames["#s"] = "state"
		guard.ConditionCheck.ExpressionAttributeValues[":deleting"] = str("deleting")
		guard.ConditionCheck.ExpressionAttributeValues[":no"] = &dt.AttributeValueMemberBOOL{Value: false}
		guard.ConditionCheck.ConditionExpression = aws.String(*guard.ConditionCheck.ConditionExpression + " AND #s = :deleting AND (attribute_not_exists(pinned) OR pinned = :no)")
		guards = append(guards, guard)
	}
	return guards, nil
}

func (s *Service) releaseProjectPin(ctx context.Context, p, ret row) error {
	stage, err := s.stage(ctx)
	if err != nil {
		return err
	}
	if p.State != "deleting" || stage.EvidencePinningSchemaVersion != 1 || !ret.Pinned {
		return failure(409, "cleanup_pending")
	}
	snap, err := s.get(ctx, s.Table, ret.PK, "EXEC#"+ret.ExecutionID+"#SNAP")
	if err != nil || snap.Kind != "executionSnapshot" || snap.Bytes < 1 || snap.Bytes > EvidenceLimit {
		return safeError()
	}
	quota, err := s.get(ctx, s.Table, "STAGE#"+s.Stage, "QUOTA")
	if err != nil || quota.Kind != "executionQuota" {
		return safeError()
	}
	pg, err := s.pinCounters(p, -1, -snap.Bytes, true)
	if err != nil {
		return err
	}
	qg, err := s.pinCounters(quota, -1, -snap.Bytes, false)
	if err != nil {
		return err
	}
	ret.Version++
	ret.Pinned = false
	ret.PinnedAt = nil
	ret.State = "deleting"
	change, err := s.put(s.Table, ret, false)
	if err != nil {
		return err
	}
	if err = s.transact(ctx, []dt.TransactWriteItem{s.stageGuard(stage), pg, s.rowGuard(snap), change, qg}); err != nil {
		return safeError()
	}
	return failure(409, "cleanup_pending")
}

type PinResult struct {
	Retention  *Retention `json:"retention"`
	ServerTime string     `json:"serverTime"`
}

func retentionView(r row, now time.Time) *Retention {
	status := "retained"
	if r.State == "deleting" {
		status = "deleting"
	} else if r.Pinned {
		status = "pinned"
	} else if r.NormalExpiresAt <= stamp(now) {
		status = "expired"
	}
	return &Retention{NormalExpiresAt: r.NormalExpiresAt, Pinned: r.Pinned, EvidenceAvailability: r.EvidenceAvailability, Version: r.Version, PinnedAt: r.PinnedAt, State: r.State, EffectiveStatus: status, PinEligible: r.State == "live" && r.EvidenceAvailability == "available" && (r.Pinned || r.NormalExpiresAt > stamp(now.Add(5*time.Second)))}
}

func (s *Service) rowGuard(r row) dt.TransactWriteItem {
	return dt.TransactWriteItem{ConditionCheck: &dt.ConditionCheck{TableName: &s.Table, Key: keys(r.PK, r.SK), ConditionExpression: aws.String("#v = :v AND #k = :k"), ExpressionAttributeNames: map[string]string{"#v": "version", "#k": "kind"}, ExpressionAttributeValues: map[string]dt.AttributeValue{":v": num(r.Version), ":k": str(r.Kind)}}}
}

// Only known conditional failures allow a new decision/time. Unknown outcomes
// must reuse the exact transaction token or reconcile, never reapply intention.
func pinConflict(err error) bool {
	var cancelled *dt.TransactionCanceledException
	if errors.As(err, &cancelled) {
		found := false
		for _, reason := range cancelled.CancellationReasons {
			switch aws.ToString(reason.Code) {
			case "ConditionalCheckFailed", "TransactionConflict":
				found = true
			case "None", "":
			default:
				return false
			}
		}
		return found
	}
	var conflict *dt.TransactionConflictException
	return errors.As(err, &conflict)
}

func healthyPinCounters(r row) bool {
	return r.PinnedCount != nil && r.PinnedBytes != nil && *r.PinnedCount >= 0 && *r.PinnedBytes >= 0 && ((*r.PinnedCount == 0) == (*r.PinnedBytes == 0))
}

// Update, rather than Put, keeps unrelated project metadata and immutable bytes.
func (s *Service) pinCounters(r row, count, bytes int64, project bool) (dt.TransactWriteItem, error) {
	if !healthyPinCounters(r) || *r.PinnedCount+count < 0 || *r.PinnedBytes+bytes < 0 {
		return dt.TransactWriteItem{}, safeError()
	}
	guard := s.rowGuard(r).ConditionCheck
	if project {
		guard = s.projectGuard(r, r.State).ConditionCheck
	}
	guard.ExpressionAttributeValues = mergeValues(guard.ExpressionAttributeValues, map[string]dt.AttributeValue{":pc": num(*r.PinnedCount), ":pb": num(*r.PinnedBytes), ":nc": num(*r.PinnedCount + count), ":nb": num(*r.PinnedBytes + bytes), ":next": num(r.Version + 1)})
	guard.ConditionExpression = aws.String(*guard.ConditionExpression + " AND pinnedCount = :pc AND pinnedBytes = :pb")
	return dt.TransactWriteItem{Update: &dt.Update{TableName: guard.TableName, Key: guard.Key, ConditionExpression: guard.ConditionExpression, ExpressionAttributeNames: guard.ExpressionAttributeNames, ExpressionAttributeValues: guard.ExpressionAttributeValues, UpdateExpression: aws.String("SET #v = :next, pinnedCount = :nc, pinnedBytes = :nb")}}, nil
}

// Verification reads only already published sanitized bytes. It never returns
// them to a pin caller, logs diagnostics, uploads or changes SNAP/checksum.
func (s *Service) verifyPinObject(ctx context.Context, manifest row) error {
	if manifest.Bytes < 1 || manifest.Bytes > EvidenceLimit || manifest.ObjectKey == "" || len(manifest.Checksum) != 64 {
		return failure(503, "evidence_unavailable")
	}
	object, err := s.Objects.GetObject(ctx, &s3.GetObjectInput{Bucket: &s.Bucket, Key: &manifest.ObjectKey})
	if err != nil {
		return failure(503, "evidence_unavailable")
	}
	defer object.Body.Close()
	hash := sha256.New()
	n, err := io.Copy(hash, io.LimitReader(object.Body, EvidenceLimit+1))
	if err != nil || n != manifest.Bytes || hex.EncodeToString(hash.Sum(nil)) != manifest.Checksum {
		return failure(503, "evidence_unavailable")
	}
	return nil
}

func (s *Service) Pin(ctx context.Context, owner, project, execution string, pinned bool) (PinResult, error) {
	for attempt := 0; attempt < 3; attempt++ {
		stage, err := s.stage(ctx)
		if err != nil {
			return PinResult{}, err
		}
		p, err := s.project(ctx, owner, project)
		if err != nil {
			return PinResult{}, err
		}
		if stage.EvidencePinningSchemaVersion != 1 {
			return PinResult{}, safeError()
		}
		user, err := s.userGuard(ctx, owner)
		if err != nil {
			return PinResult{}, err
		}
		anchor, err := s.get(ctx, s.Table, p.PK, "EXEC#"+execution)
		if err != nil {
			return PinResult{}, err
		}
		if anchor.Kind != "execution" || anchor.OwnerID != owner {
			return PinResult{}, failure(404, "not_found")
		}
		if !terminal(anchor.State) {
			return PinResult{}, failure(409, "conflict")
		}
		ret, err := s.get(ctx, s.Table, p.PK, anchor.SK+"#RET")
		if err != nil {
			return PinResult{}, err
		}
		if ret.Kind != "executionRetention" || (ret.State != "live" && ret.State != "deleting") {
			return PinResult{}, safeError()
		}
		if pinned && ret.State != "live" {
			return PinResult{}, failure(409, "conflict")
		}
		original := ret
		actions := []dt.TransactWriteItem{s.stageGuard(stage), user, s.projectGuard(p, "active"), s.rowGuard(anchor)}
		transition := ret.Pinned != pinned
		if _, err := time.Parse(time.RFC3339Nano, ret.NormalExpiresAt); err != nil || ret.Version < 0 {
			return PinResult{}, safeError()
		}
		quota, err := s.get(ctx, s.Table, "STAGE#"+s.Stage, "QUOTA")
		if err != nil || quota.Kind != "executionQuota" || !healthyPinCounters(p) || !healthyPinCounters(quota) {
			return PinResult{}, safeError()
		}
		if transition {
			manifest, err := s.get(ctx, s.Table, p.PK, anchor.SK+"#SNAP")
			if missingRow(err) {
				_ = s.evidenceUnavailable(ctx, owner, project, execution)
				return PinResult{}, failure(409, "conflict")
			}
			if err != nil {
				return PinResult{}, err
			}
			if manifest.Kind != "executionSnapshot" || manifest.Bytes < 1 || manifest.Bytes > EvidenceLimit {
				return PinResult{}, failure(503, "evidence_unavailable")
			}
			if pinned {
				if ret.EvidenceAvailability != "available" {
					return PinResult{}, failure(503, "evidence_unavailable")
				}
				if ret.NormalExpiresAt <= stamp(s.now().Add(5*time.Second)) {
					return PinResult{}, failure(409, "pin_window_closed")
				}
				// Publication ticket is insert-only and protected by the same project/RET
				// gate after finalization; validate its exact settled publication identity.
				job, err := s.get(ctx, s.Table, p.PK, "JOB#"+anchor.JobID)
				if err != nil || !terminal(job.State) {
					return PinResult{}, failure(409, "conflict")
				}
				ticketID := strings.TrimSuffix(path.Base(manifest.ObjectKey), ".json")
				if !uuidPattern.MatchString(ticketID) {
					return PinResult{}, failure(409, "conflict")
				}
				ticket, err := s.get(ctx, s.Table, p.PK, "UPLOAD#"+ticketID)
				if err != nil || ticket.JobID != anchor.JobID || ticket.ExecutionID != execution || ticket.State != "published" || !ticket.WriteSettled || ticket.ObjectKey != manifest.ObjectKey || ticket.Checksum != manifest.Checksum || ticket.Bytes != manifest.Bytes {
					return PinResult{}, failure(409, "conflict")
				}
				if err = s.verifyPinObject(ctx, manifest); err != nil {
					_ = s.evidenceUnavailable(ctx, owner, project, execution)
					return PinResult{}, err
				}
			}
			delta := int64(-1)
			if pinned {
				delta = 1
			}
			if pinned && (*p.PinnedCount >= ProjectPinLimit || *quota.PinnedCount >= StagePinLimit || *p.PinnedBytes+manifest.Bytes > ProjectPinBytes || *quota.PinnedBytes+manifest.Bytes > StagePinBytes) {
				return PinResult{}, failure(409, "pin_quota_exceeded")
			}
			projectAction, err := s.pinCounters(p, delta, delta*manifest.Bytes, true)
			if err != nil {
				return PinResult{}, err
			}
			quotaAction, err := s.pinCounters(quota, delta, delta*manifest.Bytes, false)
			if err != nil {
				return PinResult{}, err
			}
			actions[2] = projectAction
			now := s.now()
			if pinned && ret.NormalExpiresAt <= stamp(now.Add(5*time.Second)) {
				return PinResult{}, failure(409, "pin_window_closed")
			}
			ret.Version++
			ret.Pinned = pinned
			ret.DecisionTime = stamp(now)
			ret.PinnedAt = nil
			ret.DPK = dueShard(project)
			ret.DSK = ret.NormalExpiresAt + "#" + execution
			if pinned {
				ret.PinnedAt = pointer(stamp(now))
				ret.DPK = ""
				ret.DSK = ""
			}
			change, err := s.put(s.Table, ret, false)
			if err != nil {
				return PinResult{}, err
			}
			change.Put.ExpressionAttributeNames["#s"] = "state"
			change.Put.ExpressionAttributeValues[":live"] = str("live")
			change.Put.ExpressionAttributeValues[":before"] = &dt.AttributeValueMemberBOOL{Value: original.Pinned}
			change.Put.ConditionExpression = aws.String(*change.Put.ConditionExpression + " AND #s = :live AND (pinned = :before" + func() string {
				if !original.Pinned {
					return " OR attribute_not_exists(pinned)"
				}
				return ""
			}() + ")")
			if pinned {
				change.Put.ExpressionAttributeValues[":cutoff"] = str(stamp(now.Add(5 * time.Second)))
				change.Put.ConditionExpression = aws.String(*change.Put.ConditionExpression + " AND normalExpiresAt > :cutoff")
			}
			actions = append(actions, s.rowGuard(manifest), change, quotaAction)
		} else {
			actions = append(actions, s.rowGuard(ret), s.rowGuard(quota))
		}
		raw, _ := json.Marshal(actions)
		if len(actions) > 8 || len(raw) > 2*1024*1024 {
			return PinResult{}, safeError()
		}
		input := &dynamodb.TransactWriteItemsInput{TransactItems: actions, ClientRequestToken: pointer(id())}
		_, err = s.DB.TransactWriteItems(ctx, input)
		if err != nil && pinConflict(err) {
			continue
		}
		if err != nil {
			// Reuse this exact immutable token/input once, preserving decisionTime.
			_, err = s.DB.TransactWriteItems(ctx, input)
			if err != nil {
				current, readErr := s.get(ctx, s.Table, ret.PK, ret.SK)
				if !transition || readErr != nil || current.Version != ret.Version || current.Pinned != ret.Pinned || current.DecisionTime != ret.DecisionTime {
					return PinResult{}, safeError()
				}
			}
		}
		// Guard current authorization/retention after commit, including lost-ACK
		// resolution. Do not claim success if an opposite transition obscures it.
		currentProject, err := s.project(ctx, owner, project)
		if err != nil {
			return PinResult{}, err
		}
		currentStage, err := s.stage(ctx)
		if err != nil {
			return PinResult{}, err
		}
		if currentStage.RecoveryGeneration != stage.RecoveryGeneration {
			return PinResult{}, safeError()
		}
		if err = s.transact(ctx, []dt.TransactWriteItem{s.stageGuard(stage), user, s.projectGuard(currentProject, "active"), s.rowGuard(anchor), s.rowGuard(ret)}); err != nil {
			return PinResult{}, safeError()
		}
		now := s.now()
		return PinResult{retentionView(ret, now), stamp(now)}, nil
	}
	return PinResult{}, failure(409, "conflict")
}
