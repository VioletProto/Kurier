package execution

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dt "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/smithy-go"
	"reflect"
	"strings"
	"time"
)

func (s *Service) publishOutbox(ctx context.Context, r row) error {
	stage, e := s.stage(ctx)
	if e != nil {
		return e
	}
	if r.RecoveryGeneration != stage.RecoveryGeneration {
		return nil
	}
	p, e := s.project(ctx, r.OwnerID, r.ProjectID)
	if e != nil {
		return e
	}
	job, e := s.get(ctx, s.Table, r.PK, "JOB#"+r.JobID)
	if e != nil {
		return e
	}
	if terminal(job.State) || job.State == "running" {
		r.State = "published"
		r.DPK = ""
		r.DSK = ""
		r.Version++
		a, _ := s.put(s.Table, r, false)
		return s.transact(ctx, []dt.TransactWriteItem{s.stageGuard(stage), s.projectGuard(p, "active"), a})
	}
	if r.State == "published" || r.DeliveryDeadline > stamp(s.now()) {
		return nil
	}
	r.Version++
	r.DeliveryDeadline = stamp(s.now().Add(30 * time.Second))
	claim, _ := s.put(s.Table, r, false)
	if e = s.transact(ctx, []dt.TransactWriteItem{s.stageGuard(stage), s.projectGuard(p, "active"), claim}); e != nil {
		return safeError()
	}
	raw, _ := json.Marshal(Message{r.ProjectID, r.JobID, r.SK})
	send, cancel := context.WithTimeout(ctx, 2*time.Second)
	_, e = s.Queue.SendMessage(send, &sqs.SendMessageInput{QueueUrl: &s.QueueURL, MessageBody: aws.String(string(raw))})
	cancel()
	r.Version++
	r.DeliveryDeadline = ""
	r.Attempts++
	if e == nil {
		r.State = "published"
		r.DPK = ""
		r.DSK = ""
	} else {
		seconds := int64(1) << min(r.Attempts-1, 6)
		r.DSK = stamp(s.now().Add(time.Duration(min(seconds, 60))*time.Second)) + "#" + r.JobID
	}
	done, _ := s.put(s.Table, r, false)
	if e = s.transact(ctx, []dt.TransactWriteItem{s.stageGuard(stage), s.projectGuard(p, "active"), done}); e != nil {
		return safeError()
	}
	return nil
}
func (s *Service) Maintain(ctx context.Context) error {
	if _, e := s.stage(ctx); e != nil {
		return e
	}
	for shard := 0; shard < 8; shard++ {
		pk := "MAINTENANCE#" + s.Stage
		sk := string(rune('0'+shard)) + "#EXECUTION"
		cursor, e := s.get(ctx, s.Table, pk, sk)
		fresh := missingRow(e)
		if e != nil && !fresh {
			return e
		}
		if !fresh && cursor.Kind != "executionCursor" {
			return safeError()
		}
		var start map[string]dt.AttributeValue
		if cursor.ConfigurationJSON != "" {
			var last map[string]string
			if StrictJSON([]byte(cursor.ConfigurationJSON), &last) != nil {
				return safeError()
			}
			start = map[string]dt.AttributeValue{}
			for k, v := range last {
				start[k] = str(v)
			}
		}
		partition := "execution#" + string(rune('0'+shard))
		page, e := s.DB.Query(ctx, &dynamodb.QueryInput{TableName: &s.Table, IndexName: aws.String("GSI2"), KeyConditionExpression: aws.String("DPK = :pk AND DSK <= :now"), ExpressionAttributeValues: map[string]dt.AttributeValue{":pk": str(partition), ":now": str(stamp(s.now()) + "#~")}, ExclusiveStartKey: start, Limit: aws.Int32(20)})
		if e != nil {
			return safeError()
		}
		for _, item := range page.Items {
			var candidate row
			if attributevalue.UnmarshalMap(item, &candidate) != nil {
				return safeError()
			}
			r, e := s.get(ctx, s.Table, candidate.PK, candidate.SK)
			if missingRow(e) {
				continue
			}
			if e != nil {
				return e
			}
			switch r.Kind {
			case "executionOutbox":
				_ = s.publishOutbox(ctx, r)
			case "executionJob":
				if r.State == "running" && r.Deadline <= stamp(s.now()) || r.State == "queued" && r.SubmittedAt < stamp(s.now().Add(-10*time.Minute)) {
					_ = s.Process(ctx, Message{r.ProjectID, r.JobID, "OUT#" + r.JobID})
				} else if r.State == "claimed" && r.Deadline <= stamp(s.now()) {
					s.fastNotify(ctx, r.ProjectID, r.JobID)
				}
			case "executionUpload":
				if r.Deadline <= stamp(s.now()) {
					_ = s.cleanTicket(ctx, r, false)
				}
			case "executionResidual":
				_ = s.residualSweep(ctx, r)
			case "executionRetention":
				if !r.Pinned && r.NormalExpiresAt <= stamp(s.now()) {
					_ = s.expire(ctx, r)
				}
			}
		}
		if len(page.Items) == 0 && len(page.LastEvaluatedKey) == 0 && fresh {
			continue
		}
		last := map[string]string{}
		for k, v := range page.LastEvaluatedKey {
			value, ok := v.(*dt.AttributeValueMemberS)
			if !ok {
				return safeError()
			}
			last[k] = value.Value
		}
		raw, _ := json.Marshal(last)
		if len(last) == 0 {
			raw = nil
		}
		cursor.PK = pk
		cursor.SK = sk
		cursor.Kind = "executionCursor"
		cursor.SchemaVersion = 1
		cursor.ConfigurationJSON = string(raw)
		if !fresh {
			cursor.Version++
		}
		a, _ := s.put(s.Table, cursor, fresh)
		if e = s.transact(ctx, []dt.TransactWriteItem{a}); e != nil {
			return safeError()
		}
	}
	return nil
}
func notFoundObject(e error) bool {
	var a smithy.APIError
	return errors.As(e, &a) && (a.ErrorCode() == "NotFound" || a.ErrorCode() == "NoSuchKey" || a.ErrorCode() == "404")
}

// pending has no persisted PUT intent; writing without a durable ACK is
// uncertain and stays inaccessible/pending indefinitely for reviewed settlement.
func (s *Service) cleanTicket(ctx context.Context, r row, deleting bool) error {
	if r.Kind != "executionUpload" || r.SchemaVersion != 1 || r.ObjectKey == "" || r.Bytes < 0 {
		return safeError()
	}
	if r.State == "writing" && !r.WriteSettled {
		return failure(409, "upload_outcome_unknown")
	}
	if !deleting && r.State == "uploaded" && r.WriteSettled {
		job, e := s.get(ctx, s.Table, r.PK, "JOB#"+r.JobID)
		if e != nil && !missingRow(e) {
			return e
		}
		if e == nil && !terminal(job.State) {
			if job.State == "running" && job.Deadline <= stamp(s.now()) {
				return s.Process(ctx, Message{r.ProjectID, r.JobID, "OUT#" + r.JobID})
			}
			return failure(409, "cleanup_pending")
		}
	}
	if r.State == "published" && !deleting {
		return nil
	}
	if !deleting && r.Deadline > stamp(s.now()) {
		return failure(409, "conflict")
	}
	stage, e := s.stage(ctx)
	if e != nil {
		return e
	}
	p, e := s.get(ctx, s.Table, r.PK, "META")
	if e != nil {
		return e
	}
	if p.State != "active" && p.State != "deleting" && p.State != "deleted" {
		return safeError()
	}
	var retentionGuard *dt.TransactWriteItem
	if p.State == "active" && deleting {
		ret, e := s.get(ctx, s.Table, r.PK, "EXEC#"+r.ExecutionID+"#RET")
		if e != nil || ret.State != "deleting" || ret.Pinned {
			return failure(409, "conflict")
		}
		a := dt.TransactWriteItem{ConditionCheck: &dt.ConditionCheck{TableName: &s.Table, Key: keys(ret.PK, ret.SK), ConditionExpression: aws.String("#v = :v AND #s = :s"), ExpressionAttributeNames: map[string]string{"#v": "version", "#s": "state"}, ExpressionAttributeValues: map[string]dt.AttributeValue{":v": num(ret.Version), ":s": str("deleting")}}}
		retentionGuard = &a
	}
	if r.State != "deleting" {
		r.State = "deleting"
		r.Version++
		a, _ := s.put(s.Table, r, false)
		actions := []dt.TransactWriteItem{s.stageGuard(stage), s.projectGuard(p, p.State), a}
		if retentionGuard != nil {
			actions = append(actions, *retentionGuard)
		}
		if e = s.transact(ctx, actions); e != nil {
			return safeError()
		}
	}
	if _, e = s.Objects.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &s.Bucket, Key: &r.ObjectKey}); e != nil {
		return safeError()
	}
	if _, e = s.Objects.HeadObject(ctx, &s3.HeadObjectInput{Bucket: &s.Bucket, Key: &r.ObjectKey}); !notFoundObject(e) {
		return safeError()
	}
	p, e = s.get(ctx, s.Table, r.PK, "META")
	if e != nil {
		return e
	}
	quota, e := s.get(ctx, s.Table, "STAGE#"+s.Stage, "QUOTA")
	if e != nil || quota.StorageReservedBytes < r.Bytes || p.StorageReservedBytes < r.Bytes {
		return safeError()
	}
	quota.StorageReservedBytes -= r.Bytes
	quota.Version++
	q, _ := s.put(s.Table, quota, false)
	remove := dt.TransactWriteItem{Delete: &dt.Delete{TableName: &s.Table, Key: keys(r.PK, r.SK), ConditionExpression: aws.String("#v = :v AND #s = :s"), ExpressionAttributeNames: map[string]string{"#v": "version", "#s": "state"}, ExpressionAttributeValues: map[string]dt.AttributeValue{":v": num(r.Version), ":s": str("deleting")}}}
	return s.transact(ctx, []dt.TransactWriteItem{s.stageGuard(stage), s.projectUpdate(p, -r.Bytes, 0), q, remove})
}
func (s *Service) expire(ctx context.Context, ret row) error {
	stage, e := s.stage(ctx)
	if e != nil {
		return e
	}
	p, e := s.get(ctx, s.Table, ret.PK, "META")
	if e != nil || p.State != "active" {
		return safeError()
	}
	if ret.State == "live" {
		ret.State = "deleting"
		ret.Version++
		a, _ := s.put(s.Table, ret, false)
		if e = s.transact(ctx, []dt.TransactWriteItem{s.stageGuard(stage), s.projectGuard(p, "active"), a}); e != nil {
			return safeError()
		}
	}
	if err := s.drainExecution(ctx, p, ret.ExecutionID, true); err != nil {
		return err
	}
	p, e = s.get(ctx, s.Table, p.PK, "META")
	if e != nil {
		return e
	}
	remove := dt.TransactWriteItem{Delete: &dt.Delete{TableName: &s.Table, Key: keys(ret.PK, ret.SK), ConditionExpression: aws.String("#v = :v AND #s = :s AND (attribute_not_exists(pinned) OR pinned = :no)"), ExpressionAttributeNames: map[string]string{"#v": "version", "#s": "state"}, ExpressionAttributeValues: map[string]dt.AttributeValue{":v": num(ret.Version), ":s": str("deleting"), ":no": &dt.AttributeValueMemberBOOL{Value: false}}}}
	return s.transact(ctx, []dt.TransactWriteItem{s.stageGuard(stage), s.projectGuard(p, "active"), remove})
}
func knownExecution(item map[string]dt.AttributeValue, r row) bool {
	if r.SchemaVersion != 1 || !strings.HasPrefix(r.Kind, "execution") || r.ProjectID == "" || r.PK != "P#"+r.ProjectID {
		return false
	}
	allowed := map[string]bool{}
	t := reflect.TypeOf(row{})
	for i := 0; i < t.NumField(); i++ {
		allowed[strings.Split(t.Field(i).Tag.Get("dynamodbav"), ",")[0]] = true
	}
	for k := range item {
		if !allowed[k] {
			return false
		}
	}
	encoded, err := attributevalue.MarshalMap(r)
	if err != nil {
		return false
	}
	for k, v := range item {
		if !reflect.DeepEqual(v, encoded[k]) {
			return false
		}
	}
	switch r.Kind {
	case "execution":
		return r.SK == "EXEC#"+r.ExecutionID && r.Plan != nil
	case "executionJob":
		return r.SK == "JOB#"+r.JobID
	case "executionOutbox":
		return r.SK == "OUT#"+r.JobID
	case "executionBindings":
		return r.SK == "JOB#"+r.JobID+"#BINDINGS"
	case "executionUpload":
		return strings.HasPrefix(r.SK, "UPLOAD#") && strings.Contains(r.ObjectKey, "/projects/"+r.ProjectID+"/evidence/"+r.ExecutionID+"/")
	case "executionSnapshot":
		return r.SK == "EXEC#"+r.ExecutionID+"#SNAP"
	case "executionRetention":
		return r.SK == "EXEC#"+r.ExecutionID+"#RET"
	case "executionReceipt":
		return strings.HasPrefix(r.SK, "IDEM#execution-admission#")
	}
	return false
}
func (s *Service) drainExecution(ctx context.Context, p row, execution string, retention bool) error {
	// Tickets are settled before any job/manifest/retention record is removed.
	// Cursor lives outside the project so it cannot invalidate emptiness proofs.
	for _, phase := range []struct{ table, prefix string }{{s.Table, "UPLOAD#"}, {s.Table, ""}, {s.Protected, ""}} {
		cursorKey := "DRAIN#" + p.ProjectID + "#" + execution + "#" + phase.table + "#" + phase.prefix
		cursor, ce := s.get(ctx, s.Table, "MAINTENANCE#"+s.Stage, cursorKey)
		fresh := missingRow(ce)
		if ce != nil && !fresh {
			return ce
		}
		if !fresh && cursor.Kind != "executionCursor" {
			return safeError()
		}
		var start map[string]dt.AttributeValue
		if cursor.ConfigurationJSON != "" {
			var last map[string]string
			if StrictJSON([]byte(cursor.ConfigurationJSON), &last) != nil {
				return safeError()
			}
			start = map[string]dt.AttributeValue{}
			for k, v := range last {
				start[k] = str(v)
			}
		}
		expression := "PK = :pk"
		values := map[string]dt.AttributeValue{":pk": str(p.PK)}
		if phase.prefix != "" {
			expression += " AND begins_with(SK,:prefix)"
			values[":prefix"] = str(phase.prefix)
		}
		page, e := s.DB.Query(ctx, &dynamodb.QueryInput{TableName: &phase.table, KeyConditionExpression: &expression, ExpressionAttributeValues: values, ConsistentRead: aws.Bool(true), ExclusiveStartKey: start, Limit: aws.Int32(100)})
		if e != nil {
			return safeError()
		}
		// Leave the cursor unchanged when acting on a page: repeated strong reads
		// visit remaining children after a bounded transaction or ticket deletion.
		actions := []dt.TransactWriteItem{}
		for _, item := range page.Items {
			var r row
			if attributevalue.UnmarshalMap(item, &r) != nil {
				return safeError()
			}
			if execution != "" && r.ExecutionID != execution {
				continue
			}
			if !strings.HasPrefix(r.Kind, "execution") {
				continue
			}
			if !knownExecution(item, r) {
				return failure(409, "unknown_record")
			}
			if retention && r.Kind == "executionRetention" {
				continue
			}
			if r.Kind == "executionUpload" {
				if phase.prefix == "" {
					return failure(409, "cleanup_pending")
				}
				if e = s.cleanTicket(ctx, r, true); e != nil {
					return e
				}
				return failure(409, "cleanup_pending")
			}
			actions = append(actions, dt.TransactWriteItem{Delete: &dt.Delete{TableName: &phase.table, Key: keys(r.PK, r.SK), ConditionExpression: aws.String("#v = :v"), ExpressionAttributeNames: map[string]string{"#v": "version"}, ExpressionAttributeValues: map[string]dt.AttributeValue{":v": num(r.Version)}}})
			if len(actions) >= 20 {
				break
			}
		}
		current, e := s.get(ctx, s.Table, p.PK, "META")
		if e != nil {
			return e
		}
		stage, e := s.stage(ctx)
		if e != nil {
			return e
		}
		if len(actions) > 0 {
			if e = s.transact(ctx, append([]dt.TransactWriteItem{s.stageGuard(stage), s.projectGuard(current, current.State)}, actions...)); e != nil {
				return safeError()
			}
			return failure(409, "cleanup_pending")
		}
		last := map[string]string{}
		for k, v := range page.LastEvaluatedKey {
			value, ok := v.(*dt.AttributeValueMemberS)
			if !ok {
				return safeError()
			}
			last[k] = value.Value
		}
		raw, _ := json.Marshal(last)
		if len(last) == 0 {
			raw = nil
		}
		cursor.PK = "MAINTENANCE#" + s.Stage
		cursor.SK = cursorKey
		cursor.Kind = "executionCursor"
		cursor.SchemaVersion = 1
		cursor.ConfigurationJSON = string(raw)
		if !fresh {
			cursor.Version++
		}
		a, _ := s.put(s.Table, cursor, fresh)
		if e = s.transact(ctx, []dt.TransactWriteItem{s.stageGuard(stage), s.projectGuard(current, current.State), a}); e != nil {
			return safeError()
		}
		if len(last) > 0 {
			return failure(409, "cleanup_pending")
		}
	}
	return nil
}
func (s *Service) DrainProject(ctx context.Context, project string) error {
	p, e := s.get(ctx, s.Table, "P#"+project, "META")
	if e != nil || p.State != "deleting" {
		return safeError()
	}
	if e = s.drainExecution(ctx, p, "", false); e != nil {
		return e
	}
	prefix := s.Stage + "/projects/" + project + "/"
	objects, e := s.Objects.ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: &s.Bucket, Prefix: &prefix, MaxKeys: aws.Int32(100)})
	if e != nil {
		return safeError()
	}
	for _, o := range objects.Contents {
		if _, e = s.Objects.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &s.Bucket, Key: o.Key}); e != nil {
			return safeError()
		}
	}
	if len(objects.Contents) > 0 || aws.ToBool(objects.IsTruncated) {
		return failure(409, "cleanup_pending")
	}
	// Register durable residual work before physical completion can be claimed.
	stage, e := s.stage(ctx)
	if e != nil {
		return e
	}
	p, e = s.get(ctx, s.Table, p.PK, "META")
	if e != nil {
		return e
	}
	existing, e := s.get(ctx, s.Table, "MAINTENANCE#"+s.Stage, "RESIDUAL#"+project)
	if e == nil {
		if existing.Kind != "executionResidual" {
			return safeError()
		}
		return nil
	}
	if !missingRow(e) {
		return e
	}
	work := row{PK: "MAINTENANCE#" + s.Stage, SK: "RESIDUAL#" + project, Kind: "executionResidual", SchemaVersion: 1, ProjectID: project, DPK: dueShard(project), DSK: stamp(s.now().Add(time.Hour)) + "#" + project}
	a, _ := s.put(s.Table, work, true)
	return s.transact(ctx, []dt.TransactWriteItem{s.stageGuard(stage), s.projectGuard(p, "deleting"), a})
}

// Permanent residual-prefix work is outside the project partition so completed
// tombstones can remain minimal while low-frequency sweeps remain durable.

func (s *Service) residualSweep(ctx context.Context, r row) error {
	p, e := s.get(ctx, s.Table, "P#"+r.ProjectID, "META")
	if e != nil {
		return e
	}
	if p.State != "deleted" {
		return nil
	}
	prefix := s.Stage + "/projects/" + r.ProjectID + "/"
	page, e := s.Objects.ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: &s.Bucket, Prefix: &prefix, MaxKeys: aws.Int32(100)})
	if e != nil {
		return safeError()
	}
	for _, o := range page.Contents {
		if _, e = s.Objects.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &s.Bucket, Key: o.Key}); e != nil {
			return safeError()
		}
	}
	r.Version++
	r.DSK = stamp(s.now().Add(time.Hour)) + "#" + r.ProjectID
	if aws.ToBool(page.IsTruncated) {
		r.DSK = stamp(s.now()) + "#" + r.ProjectID
	}
	a, _ := s.put(s.Table, r, false)
	return s.transact(ctx, []dt.TransactWriteItem{a})
}
