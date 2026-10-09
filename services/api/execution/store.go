package execution

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dt "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"strings"
	"time"
)

type Database interface {
	GetItem(context.Context, *dynamodb.GetItemInput, ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	Query(context.Context, *dynamodb.QueryInput, ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error)
	TransactWriteItems(context.Context, *dynamodb.TransactWriteItemsInput, ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error)
}
type Objects interface {
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	DeleteObject(context.Context, *s3.DeleteObjectInput, ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
	ListObjectsV2(context.Context, *s3.ListObjectsV2Input, ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	HeadObject(context.Context, *s3.HeadObjectInput, ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
}
type Queue interface {
	SendMessage(context.Context, *sqs.SendMessageInput, ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
}
type Service struct {
	DB                                                Database
	Objects                                           Objects
	Queue                                             Queue
	KMS                                               KMS
	Table, Protected, Bucket, QueueURL, KeyARN, Stage string
	SigningKey                                        []byte
	Now                                               func() time.Time
	prepare                                           func(context.Context, Plan, map[string]string) (*Prepared, error)
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

type SecretSource struct {
	SecretID string `dynamodbav:"secretId"`
	Revision int64  `dynamodbav:"revision"`
}
type row struct {
	PK                            string         `dynamodbav:"PK"`
	SK                            string         `dynamodbav:"SK"`
	Kind                          string         `dynamodbav:"kind"`
	SchemaVersion                 int            `dynamodbav:"schemaVersion"`
	ProjectID                     string         `dynamodbav:"projectId,omitempty"`
	OwnerID                       string         `dynamodbav:"ownerId,omitempty"`
	State                         string         `dynamodbav:"state,omitempty"`
	Version                       int64          `dynamodbav:"version"`
	Revision                      int64          `dynamodbav:"revision,omitempty"`
	RequestID                     string         `dynamodbav:"requestId,omitempty"`
	SecretID                      string         `dynamodbav:"secretId,omitempty"`
	ValueKind                     string         `dynamodbav:"valueKind,omitempty"`
	ConfigurationJSON             string         `dynamodbav:"configurationJSON,omitempty"`
	RecoveryGeneration            string         `dynamodbav:"recoveryGeneration,omitempty"`
	SavedRequestsSchemaVersion    int            `dynamodbav:"savedRequestsSchemaVersion,omitempty"`
	ExecutionInputsSchemaVersion  int            `dynamodbav:"executionInputsSchemaVersion,omitempty"`
	ProtectedSecretsSchemaVersion int            `dynamodbav:"protectedSecretsSchemaVersion,omitempty"`
	CloudExecutionsSchemaVersion  int            `dynamodbav:"cloudExecutionsSchemaVersion,omitempty"`
	DeletionEpoch                 int64          `dynamodbav:"deletionEpoch,omitempty"`
	Disabled                      bool           `dynamodbav:"disabled,omitempty"`
	UserID                        string         `dynamodbav:"userId,omitempty"`
	Plan                          *Plan          `dynamodbav:"plan,omitempty"`
	ExecutionID                   string         `dynamodbav:"executionId,omitempty"`
	JobID                         string         `dynamodbav:"jobId,omitempty"`
	Sources                       []SecretSource `dynamodbav:"sources,omitempty"`
	SubmittedAt                   string         `dynamodbav:"submittedAt,omitempty"`
	StartedAt                     *string        `dynamodbav:"startedAt,omitempty"`
	CompletedAt                   *string        `dynamodbav:"completedAt,omitempty"`
	Summary                       *Summary       `dynamodbav:"summary,omitempty"`
	NormalExpiresAt               string         `dynamodbav:"normalExpiresAt,omitempty"`
	Pinned                        bool           `dynamodbav:"pinned,omitempty"`
	EvidenceAvailability          string         `dynamodbav:"evidenceAvailability,omitempty"`
	Fence                         int64          `dynamodbav:"fence,omitempty"`
	Attempts                      int64          `dynamodbav:"attempts,omitempty"`
	LeaseID                       string         `dynamodbav:"leaseId,omitempty"`
	Deadline                      string         `dynamodbav:"deadline,omitempty"`
	DispatchIntentAt              string         `dynamodbav:"dispatchIntentAt,omitempty"`
	DeliveryDeadline              string         `dynamodbav:"deliveryDeadline,omitempty"`
	Envelope                      *Envelope      `dynamodbav:"envelope,omitempty"`
	InputDigest                   string         `dynamodbav:"inputDigest,omitempty"`
	ExpiresAt                     string         `dynamodbav:"expiresAt,omitempty"`
	ObjectKey                     string         `dynamodbav:"objectKey,omitempty"`
	Checksum                      string         `dynamodbav:"checksum,omitempty"`
	Bytes                         int64          `dynamodbav:"bytes,omitempty"`
	WriteSettled                  bool           `dynamodbav:"writeSettled,omitempty"`
	Month                         string         `dynamodbav:"month,omitempty"`
	Count                         int64          `dynamodbav:"count,omitempty"`
	StorageReservedBytes          int64          `dynamodbav:"storageReservedBytes,omitempty"`
	InflightCount                 int64          `dynamodbav:"inflightCount,omitempty"`
	LPK                           string         `dynamodbav:"LPK,omitempty"`
	LSK                           string         `dynamodbav:"LSK,omitempty"`
	DPK                           string         `dynamodbav:"DPK,omitempty"`
	DSK                           string         `dynamodbav:"DSK,omitempty"`
	HPK                           string         `dynamodbav:"HPK,omitempty"`
	HSK                           string         `dynamodbav:"HSK,omitempty"`
}

func str(v string) dt.AttributeValue { return &dt.AttributeValueMemberS{Value: v} }
func num(v int64) dt.AttributeValue  { return &dt.AttributeValueMemberN{Value: fmt.Sprint(v)} }
func keys(pk, sk string) map[string]dt.AttributeValue {
	return map[string]dt.AttributeValue{"PK": str(pk), "SK": str(sk)}
}
func (s *Service) get(ctx context.Context, table, pk, sk string) (row, error) {
	out, e := s.DB.GetItem(ctx, &dynamodb.GetItemInput{TableName: &table, Key: keys(pk, sk), ConsistentRead: aws.Bool(true)})
	if e != nil {
		return row{}, safeError()
	}
	if out == nil || len(out.Item) == 0 {
		return row{}, failure(404, "not_found")
	}
	var r row
	if attributevalue.UnmarshalMap(out.Item, &r) != nil || r.PK != pk || r.SK != sk {
		return row{}, safeError()
	}
	if strings.HasPrefix(r.Kind, "execution") && strings.HasPrefix(r.PK, "P#") && !knownExecution(out.Item, r) {
		return row{}, safeError()
	}
	return r, nil
}
func (s *Service) put(table string, r row, fresh bool) (dt.TransactWriteItem, error) {
	item, e := attributevalue.MarshalMap(r)
	if e != nil {
		return dt.TransactWriteItem{}, safeError()
	}
	raw, _ := json.Marshal(item)
	if len(raw) > 128*1024 {
		return dt.TransactWriteItem{}, failure(413, "payload_too_large")
	}
	p := &dt.Put{TableName: &table, Item: item}
	if fresh {
		p.ConditionExpression = aws.String("attribute_not_exists(PK)")
	} else {
		p.ConditionExpression = aws.String("#v = :v")
		p.ExpressionAttributeNames = map[string]string{"#v": "version"}
		p.ExpressionAttributeValues = map[string]dt.AttributeValue{":v": num(r.Version - 1)}
	}
	return dt.TransactWriteItem{Put: p}, nil
}
func (s *Service) transact(ctx context.Context, actions []dt.TransactWriteItem) error {
	raw, _ := json.Marshal(actions)
	if len(actions) > 80 || len(raw) > 2*1024*1024 {
		return safeError()
	}
	_, e := s.DB.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{TransactItems: actions})
	return e
}
func (s *Service) stage(ctx context.Context) (row, error) {
	r, e := s.get(ctx, s.Table, "STAGE#"+s.Stage, "META")
	if e != nil || r.State != "active" || r.RecoveryGeneration == "" || r.SavedRequestsSchemaVersion != 3 || r.CloudExecutionsSchemaVersion != 1 || r.ExecutionInputsSchemaVersion != 1 || r.ProtectedSecretsSchemaVersion != 1 {
		return row{}, safeError()
	}
	return r, nil
}
func (s *Service) stageGuard(stage row) dt.TransactWriteItem {
	return dt.TransactWriteItem{ConditionCheck: &dt.ConditionCheck{TableName: &s.Table, Key: keys(stage.PK, "META"), ConditionExpression: aws.String("#s = :active AND recoveryGeneration = :g AND savedRequestsSchemaVersion = :r AND cloudExecutionsSchemaVersion = :c AND executionInputsSchemaVersion = :c AND protectedSecretsSchemaVersion = :c"), ExpressionAttributeNames: map[string]string{"#s": "state"}, ExpressionAttributeValues: map[string]dt.AttributeValue{":active": str("active"), ":g": str(stage.RecoveryGeneration), ":r": num(3), ":c": num(1)}}}
}
func (s *Service) project(ctx context.Context, owner, project string) (row, error) {
	r, e := s.get(ctx, s.Table, "P#"+project, "META")
	if e != nil || r.Kind != "project" || r.State != "active" || r.ProjectID != project || r.OwnerID != owner {
		return row{}, failure(404, "not_found")
	}
	return r, nil
}
func (s *Service) projectGuard(p row, state string) dt.TransactWriteItem {
	return dt.TransactWriteItem{ConditionCheck: &dt.ConditionCheck{TableName: &s.Table, Key: keys(p.PK, "META"), ConditionExpression: aws.String("#v = :v AND #s = :s AND ownerId = :o AND (attribute_not_exists(deletionEpoch) OR deletionEpoch = :e)"), ExpressionAttributeNames: map[string]string{"#v": "version", "#s": "state"}, ExpressionAttributeValues: map[string]dt.AttributeValue{":v": num(p.Version), ":s": str(state), ":o": str(p.OwnerID), ":e": num(p.DeletionEpoch)}}}
}
func (s *Service) projectUpdate(p row, storage, inflight int64) dt.TransactWriteItem {
	g := s.projectGuard(p, p.State).ConditionCheck
	return dt.TransactWriteItem{Update: &dt.Update{TableName: g.TableName, Key: g.Key, ConditionExpression: g.ConditionExpression, ExpressionAttributeNames: g.ExpressionAttributeNames, ExpressionAttributeValues: mergeValues(g.ExpressionAttributeValues, map[string]dt.AttributeValue{":next": num(p.Version + 1), ":bytes": num(storage), ":flight": num(inflight), ":zero": num(0)}), UpdateExpression: aws.String("SET #v = :next, storageReservedBytes = if_not_exists(storageReservedBytes,:zero) + :bytes, inflightCount = if_not_exists(inflightCount,:zero) + :flight")}}
}
func mergeValues(a, b map[string]dt.AttributeValue) map[string]dt.AttributeValue {
	for k, v := range b {
		a[k] = v
	}
	return a
}
func (s *Service) userGuard(ctx context.Context, owner string) (dt.TransactWriteItem, error) {
	u, e := s.get(ctx, s.Table, "U#"+owner, "META")
	if e != nil || u.Kind != "user" || u.Disabled {
		return dt.TransactWriteItem{}, failure(403, "forbidden")
	}
	return dt.TransactWriteItem{ConditionCheck: &dt.ConditionCheck{TableName: &s.Table, Key: keys(u.PK, "META"), ConditionExpression: aws.String("#v = :v AND (attribute_not_exists(disabled) OR disabled = :no)"), ExpressionAttributeNames: map[string]string{"#v": "version"}, ExpressionAttributeValues: map[string]dt.AttributeValue{":v": num(u.Version), ":no": &dt.AttributeValueMemberBOOL{Value: false}}}}, nil
}
func (s *Service) digest(domain string, v any) string {
	raw, _ := json.Marshal(v)
	h := hmac.New(sha256.New, s.SigningKey)
	h.Write([]byte(domain))
	h.Write([]byte{0})
	h.Write(raw)
	return hex.EncodeToString(h.Sum(nil))
}
func missingRow(e error) bool    { var a *Error; return errors.As(e, &a) && a.Status == 404 }
func terminal(state string) bool { return state == "completed" || state == "failed" }
func (s *Service) view(ctx context.Context, r row) (View, error) {
	p, e := s.project(ctx, r.OwnerID, r.ProjectID)
	if e != nil {
		return View{}, e
	}
	_ = p
	v := View{ExecutionID: r.ExecutionID, ProjectID: r.ProjectID, Status: r.State, Version: r.Version, SubmittedAt: r.SubmittedAt, StartedAt: r.StartedAt, CompletedAt: r.CompletedAt, Summary: r.Summary, ServerTime: stamp(s.now())}
	if r.Plan == nil {
		return v, safeError()
	}
	v.Source = Source{r.Plan.RequestID, r.Plan.Revision}
	v.Configuration = &r.Plan.Configuration
	if source, e := s.get(ctx, s.Table, r.PK, "REQ#"+r.Plan.RequestID); e == nil && source.State == "active" && source.Kind == "request" {
		v.LiveRequestID = &r.Plan.RequestID
	}
	if terminal(r.State) {
		ret, e := s.get(ctx, s.Table, r.PK, r.SK+"#RET")
		if e != nil {
			return v, e
		}
		if !ret.Pinned && ret.NormalExpiresAt <= stamp(s.now()) || ret.State != "live" {
			return v, failure(404, "not_found")
		}
		v.Retention = &Retention{ret.NormalExpiresAt, ret.Pinned, ret.EvidenceAvailability}
	}
	return v, nil
}
func (s *Service) Detail(ctx context.Context, owner, project, execution string) (View, error) {
	if _, e := s.stage(ctx); e != nil {
		return View{}, e
	}
	if _, e := s.project(ctx, owner, project); e != nil {
		return View{}, e
	}
	r, e := s.get(ctx, s.Table, "P#"+project, "EXEC#"+execution)
	if e != nil || r.Kind != "execution" || r.OwnerID != owner {
		return View{}, failure(404, "not_found")
	}
	return s.view(ctx, r)
}

type Message struct {
	ProjectID string `json:"projectId"`
	JobID     string `json:"jobId"`
	OutboxID  string `json:"outboxId"`
}

func (s *Service) fastNotify(ctx context.Context, project, job string) {
	if s.Queue == nil {
		return
	}
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) < 3*time.Second {
		return
	}
	send, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	raw, _ := json.Marshal(Message{project, job, "OUT#" + job})
	_, _ = s.Queue.SendMessage(send, &sqs.SendMessageInput{QueueUrl: &s.QueueURL, MessageBody: aws.String(string(raw))})
}
func (s *Service) sourceGuard(r row, table string) dt.TransactWriteItem {
	return dt.TransactWriteItem{ConditionCheck: &dt.ConditionCheck{TableName: &table, Key: keys(r.PK, r.SK), ConditionExpression: aws.String("revision = :r AND #s = :active AND #k = :kind"), ExpressionAttributeNames: map[string]string{"#s": "state", "#k": "kind"}, ExpressionAttributeValues: map[string]dt.AttributeValue{":r": num(r.Revision), ":active": str("active"), ":kind": str(r.Kind)}}}
}
func requiredRefs(c Configuration) []string {
	seen := map[string]bool{}
	add := func(r *Ref) {
		if r != nil {
			seen[r.SecretID] = true
		}
	}
	for _, f := range append(append([]Field{}, c.Headers...), c.QueryParameters...) {
		if f.Enabled && f.Sensitive {
			add(f.SecretRef)
		}
	}
	if c.Body != nil {
		if c.Body.Sensitive {
			add(c.Body.SecretRef)
		}
		for _, f := range c.Body.SecretFields {
			add(f.SecretRef)
		}
	}
	out := []string{}
	for key := range seen {
		out = append(out, key)
	}
	return out
}
func (s *Service) Submit(ctx context.Context, owner, project, source, key string, revision int64, options Options, rerun bool) (View, error) {
	stage, e := s.stage(ctx)
	if e != nil {
		return View{}, e
	}
	p, e := s.project(ctx, owner, project)
	if e != nil {
		return View{}, e
	}
	if len(s.SigningKey) < 32 || !uuidPattern.MatchString(key) || !uuidPattern.MatchString(source) {
		return View{}, failure(400, "invalid_request")
	}
	purpose := "submit"
	var identity any = struct {
		Owner, Project, Source string
		Revision               int64
		Options                Options
	}{owner, project, source, revision, options}
	if rerun {
		purpose = "rerun"
		identity = struct {
			Owner, Project, Source string
			Allow                  bool
		}{owner, project, source, options.AllowInsecureSecrets}
	}
	receiptSK := "IDEM#execution-admission#" + s.digest("execution-admission-key-v1", []string{s.Stage, owner, project, key})
	inputDigest := s.digest("execution-admission-input-v1", []any{purpose, identity})
	receipt, e := s.get(ctx, s.Table, p.PK, receiptSK)
	if e == nil {
		if receipt.Kind != "executionReceipt" || receipt.InputDigest != inputDigest || receipt.ExpiresAt <= stamp(s.now()) {
			return View{}, failure(409, "conflict")
		}
		return s.Detail(ctx, owner, project, receipt.ExecutionID)
	}
	if !missingRow(e) {
		return View{}, e
	}
	if p.StorageReservedBytes >= 1024*1024*1024 {
		return View{}, failure(429, "rate_limited")
	}
	var plan Plan
	if rerun {
		old, e := s.get(ctx, s.Table, p.PK, "EXEC#"+source)
		if e != nil || old.Kind != "execution" || old.OwnerID != owner || old.Plan == nil {
			return View{}, failure(404, "not_found")
		}
		if _, e = s.view(ctx, old); e != nil {
			return View{}, e
		}
		plan = *old.Plan
		plan.Options.AllowInsecureSecrets = options.AllowInsecureSecrets
		source = plan.RequestID
	}
	request, e := s.get(ctx, s.Table, p.PK, "REQ#"+source)
	if e != nil || request.Kind != "request" || request.State != "active" || (request.SchemaVersion != 1 && request.SchemaVersion != 2) {
		return View{}, failure(400, "source_unavailable")
	}
	if !rerun {
		if request.Revision != revision {
			return View{}, failure(412, "precondition_failed")
		}
		if StrictJSON([]byte(request.ConfigurationJSON), &plan.Configuration) != nil {
			return View{}, safeError()
		}
		plan.RequestID = source
		plan.Revision = revision
		plan.Options = options
	}
	frozen, _ := json.Marshal(plan)
	if len(frozen) > ConfigLimit {
		return View{}, failure(413, "payload_too_large")
	}
	destination, e := ValidateDestination(plan.Configuration.URL)
	if e != nil {
		return View{}, e
	}
	refs := requiredRefs(plan.Configuration)
	if len(refs) > 16 {
		return View{}, failure(413, "payload_too_large")
	}
	if len(refs) > 0 && destination.Scheme == "http" && !options.AllowInsecureSecrets {
		return View{}, failure(400, "insecure_secret_transport")
	}
	execution, job := id(), id()
	now := s.now()
	user, e := s.userGuard(ctx, owner)
	if e != nil {
		return View{}, e
	}
	actions := []dt.TransactWriteItem{s.stageGuard(stage), user, s.projectUpdate(p, 0, 1), s.sourceGuard(request, s.Table)}
	values := map[string]string{}
	defer clear(values)
	sources := []SecretSource{}
	for _, ref := range refs {
		r, e := s.get(ctx, s.Protected, p.PK, "SECRET#"+ref)
		if e != nil || r.Kind != "savedSecret" || r.SchemaVersion != 1 || r.SecretID != ref || r.ProjectID != project || r.State != "active" {
			return View{}, failure(400, "source_unavailable")
		}
		raw, e := s.decrypt(ctx, project, ref, "saved-secret", r.Revision, r.Envelope)
		if e != nil {
			return View{}, e
		}
		values[ref] = string(raw)
		clear(raw)
		sources = append(sources, SecretSource{ref, r.Revision})
		actions = append(actions, s.sourceGuard(r, s.Protected))
	}
	envelope, e := s.encrypt(ctx, project, job, values)
	if e != nil {
		return View{}, e
	}
	r := row{PK: p.PK, SK: "EXEC#" + execution, Kind: "execution", SchemaVersion: 1, ProjectID: project, OwnerID: owner, State: "queued", ExecutionID: execution, JobID: job, Plan: &plan, SubmittedAt: stamp(now), RecoveryGeneration: stage.RecoveryGeneration, LPK: p.PK + "#EXEC", LSK: stamp(now) + "#" + execution, HPK: p.PK + "#REQ#" + source, HSK: stamp(now) + "#" + execution}
	j := row{PK: p.PK, SK: "JOB#" + job, Kind: "executionJob", SchemaVersion: 1, ProjectID: project, OwnerID: owner, State: "queued", ExecutionID: execution, JobID: job, Sources: sources, SubmittedAt: r.SubmittedAt, RecoveryGeneration: stage.RecoveryGeneration, DPK: dueShard(project), DSK: stamp(now.Add(10*time.Minute)) + "#" + job}
	receipt = row{PK: p.PK, SK: receiptSK, Kind: "executionReceipt", SchemaVersion: 1, ProjectID: project, OwnerID: owner, InputDigest: inputDigest, ExecutionID: execution, ExpiresAt: stamp(now.Add(7 * 24 * time.Hour))}
	out := row{PK: p.PK, SK: "OUT#" + job, Kind: "executionOutbox", SchemaVersion: 1, ProjectID: project, OwnerID: owner, ExecutionID: execution, JobID: job, RecoveryGeneration: stage.RecoveryGeneration, State: "pending", DPK: dueShard(project), DSK: stamp(now) + "#" + job}
	bundle := row{PK: p.PK, SK: "JOB#" + job + "#BINDINGS", Kind: "executionBindings", SchemaVersion: 1, ProjectID: project, JobID: job, Envelope: envelope}
	for _, entry := range []struct {
		table string
		r     row
	}{{s.Table, r}, {s.Table, j}, {s.Table, receipt}, {s.Table, out}, {s.Protected, bundle}} {
		a, e := s.put(entry.table, entry.r, true)
		if e != nil {
			return View{}, e
		}
		actions = append(actions, a)
	}
	quota, e := s.admissionQuota(ctx, owner, now)
	if e != nil {
		return View{}, e
	}
	actions = append(actions, quota...)
	if e = s.transact(ctx, actions); e != nil {
		existing, readErr := s.get(ctx, s.Table, p.PK, receiptSK)
		if readErr == nil {
			if existing.InputDigest != inputDigest {
				return View{}, failure(409, "conflict")
			}
			return s.Detail(ctx, owner, project, existing.ExecutionID)
		}
		if !missingRow(readErr) {
			return View{}, safeError()
		}
		var conditional *dt.TransactionCanceledException
		if errors.As(e, &conditional) {
			return View{}, failure(409, "conflict")
		}
		return View{}, safeError()
	}
	s.fastNotify(ctx, project, job)
	return s.view(ctx, r)
}
func (s *Service) admissionQuota(ctx context.Context, owner string, now time.Time) ([]dt.TransactWriteItem, error) {
	actions := []dt.TransactWriteItem{}
	for _, spec := range []struct {
		pk, sk string
		limit  int64
	}{{"STAGE#" + s.Stage, "QUOTA", 1000}, {"U#" + owner, "EXECUTION_DAY#" + now.UTC().Format("2006-01-02"), 100}} {
		r, e := s.get(ctx, s.Table, spec.pk, spec.sk)
		fresh := missingRow(e)
		if e != nil && !fresh {
			return nil, e
		}
		if fresh {
			r = row{PK: spec.pk, SK: spec.sk, Kind: "executionQuota", SchemaVersion: 1}
		}
		if r.Kind != "executionQuota" || r.SchemaVersion != 1 {
			return nil, safeError()
		}
		if r.Month != now.UTC().Format("2006-01") {
			r.Month = now.UTC().Format("2006-01")
			r.Count = 0
		}
		if r.Count >= spec.limit || (spec.pk == "STAGE#"+s.Stage && r.StorageReservedBytes >= 2*1024*1024*1024) {
			return nil, failure(429, "rate_limited")
		}
		r.Count++
		if !fresh {
			r.Version++
		}
		a, e := s.put(s.Table, r, fresh)
		if e != nil {
			return nil, e
		}
		actions = append(actions, a)
	}
	return actions, nil
}
func dueShard(project string) string {
	h := sha256.Sum256([]byte(project))
	return fmt.Sprintf("execution#%d", h[0]%8)
}
