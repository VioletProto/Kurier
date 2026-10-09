//go:build integration

package execution

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dt "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/smithy-go"
)

// AWS faults below are simulated around actual DynamoDB Local transactions.
// The HTTP fixture is controlled loopback, not AWS/IAM/SQS/S3 validation.
type faultDB struct {
	*dynamodb.Client
	before    func(*dynamodb.TransactWriteItemsInput) error
	after     func(*dynamodb.TransactWriteItemsInput) error
	failReads bool
}

func (d *faultDB) GetItem(ctx context.Context, in *dynamodb.GetItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	if d.failReads {
		return nil, errors.New("simulated lost intent readback")
	}
	return d.Client.GetItem(ctx, in, opts...)
}
func (d *faultDB) TransactWriteItems(ctx context.Context, in *dynamodb.TransactWriteItemsInput, opts ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error) {
	if d.before != nil {
		if e := d.before(in); e != nil {
			return nil, e
		}
	}
	out, e := d.Client.TransactWriteItems(ctx, in, opts...)
	if e == nil && d.after != nil {
		e = d.after(in)
	}
	return out, e
}

type fixtureKMS struct{}

func (fixtureKMS) GenerateDataKey(_ context.Context, in *kms.GenerateDataKeyInput, _ ...func(*kms.Options)) (*kms.GenerateDataKeyOutput, error) {
	scope, _ := json.Marshal(in.EncryptionContext)
	return &kms.GenerateDataKeyOutput{KeyId: in.KeyId, Plaintext: bytes.Repeat([]byte{7}, 32), CiphertextBlob: scope}, nil
}
func (fixtureKMS) Decrypt(_ context.Context, in *kms.DecryptInput, _ ...func(*kms.Options)) (*kms.DecryptOutput, error) {
	scope, _ := json.Marshal(in.EncryptionContext)
	if !bytes.Equal(scope, in.CiphertextBlob) {
		return nil, errors.New("fixture scope mismatch")
	}
	return &kms.DecryptOutput{KeyId: in.KeyId, Plaintext: bytes.Repeat([]byte{7}, 32)}, nil
}

type fixtureObjects struct {
	mu       sync.Mutex
	data     map[string][]byte
	putFault func()
	lostACK  bool
	puts     int
}

func (o *fixtureObjects) PutObject(_ context.Context, in *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	raw, e := io.ReadAll(in.Body)
	if e != nil {
		return nil, e
	}
	o.mu.Lock()
	o.data[*in.Key] = raw
	o.puts++
	o.mu.Unlock()
	if o.putFault != nil {
		o.putFault()
	}
	if o.lostACK {
		return nil, errors.New("simulated lost PUT acknowledgment")
	}
	return &s3.PutObjectOutput{}, nil
}
func (o *fixtureObjects) GetObject(_ context.Context, in *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	raw, ok := o.data[*in.Key]
	if !ok {
		return nil, &smithy.GenericAPIError{Code: "NoSuchKey"}
	}
	return &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(raw))}, nil
}
func (o *fixtureObjects) DeleteObject(_ context.Context, in *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	delete(o.data, *in.Key)
	return &s3.DeleteObjectOutput{}, nil
}
func (o *fixtureObjects) HeadObject(_ context.Context, in *s3.HeadObjectInput, _ ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if _, ok := o.data[*in.Key]; !ok {
		return nil, &smithy.GenericAPIError{Code: "NotFound"}
	}
	return &s3.HeadObjectOutput{}, nil
}
func (o *fixtureObjects) ListObjectsV2(_ context.Context, in *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if len(o.data) > 0 {
		return nil, errors.New("fixture requires exact-key cleanup first")
	}
	return &s3.ListObjectsV2Output{}, nil
}

type fixtureQueue struct {
	calls int
	fail  bool
}

func (q *fixtureQueue) SendMessage(_ context.Context, in *sqs.SendMessageInput, _ ...func(*sqs.Options)) (*sqs.SendMessageOutput, error) {
	q.calls++
	var m Message
	if StrictJSON([]byte(*in.MessageBody), &m) != nil {
		return nil, errors.New("unsafe notification")
	}
	if q.fail {
		return nil, errors.New("simulated lost send acknowledgment")
	}
	return &sqs.SendMessageOutput{}, nil
}

type protocolFixture struct {
	s                       *Service
	db                      *faultDB
	objects                 *fixtureObjects
	queue                   *fixtureQueue
	owner, project, request string
	now                     time.Time
	calls                   atomic.Int32
}

func newProtocolFixture(t *testing.T) *protocolFixture {
	t.Helper()
	endpoint := os.Getenv("KURIER_DYNAMODB_TEST_ENDPOINT")
	if endpoint != "http://127.0.0.1:8000" {
		t.Fatal("integration requires explicit controlled DynamoDB Local endpoint")
	}
	client := dynamodb.New(dynamodb.Options{Region: "us-east-2", BaseEndpoint: &endpoint, Credentials: credentials.NewStaticCredentialsProvider("local", "local", ""), Retryer: aws.NopRetryer{}})
	f := &protocolFixture{db: &faultDB{Client: client}, objects: &fixtureObjects{data: map[string][]byte{}}, queue: &fixtureQueue{}, owner: id(), project: id(), request: id(), now: time.Now().UTC()}
	f.s = &Service{DB: f.db, Objects: f.objects, Queue: f.queue, KMS: fixtureKMS{}, Table: "execution-test-" + id(), Protected: "execution-protected-" + id(), Bucket: "fixture", QueueURL: "fixture", KeyARN: "fixture-key", Stage: "dev-api", SigningKey: bytes.Repeat([]byte{8}, 32), Now: func() time.Time { return f.now }}
	for _, table := range []string{f.s.Table, f.s.Protected} {
		definitions := []dt.AttributeDefinition{{AttributeName: aws.String("PK"), AttributeType: dt.ScalarAttributeTypeS}, {AttributeName: aws.String("SK"), AttributeType: dt.ScalarAttributeTypeS}}
		var indexes []dt.GlobalSecondaryIndex
		if table == f.s.Table {
			for i, names := range [][2]string{{"LPK", "LSK"}, {"DPK", "DSK"}, {"HPK", "HSK"}} {
				for _, name := range names {
					definitions = append(definitions, dt.AttributeDefinition{AttributeName: aws.String(name), AttributeType: dt.ScalarAttributeTypeS})
				}
				indexes = append(indexes, dt.GlobalSecondaryIndex{IndexName: aws.String([]string{"GSI1", "GSI2", "GSI3"}[i]), KeySchema: []dt.KeySchemaElement{{AttributeName: aws.String(names[0]), KeyType: dt.KeyTypeHash}, {AttributeName: aws.String(names[1]), KeyType: dt.KeyTypeRange}}, Projection: &dt.Projection{ProjectionType: dt.ProjectionTypeKeysOnly}})
			}
		}
		_, e := client.CreateTable(context.Background(), &dynamodb.CreateTableInput{TableName: &table, BillingMode: dt.BillingModePayPerRequest, AttributeDefinitions: definitions, KeySchema: []dt.KeySchemaElement{{AttributeName: aws.String("PK"), KeyType: dt.KeyTypeHash}, {AttributeName: aws.String("SK"), KeyType: dt.KeyTypeRange}}, GlobalSecondaryIndexes: indexes})
		if e != nil {
			t.Fatal(e)
		}
		t.Cleanup(func() {
			_, _ = client.DeleteTable(context.Background(), &dynamodb.DeleteTableInput{TableName: aws.String(table)})
		})
	}
	f.write(t, f.s.Table, row{PK: "STAGE#dev-api", SK: "META", Kind: "stage", SchemaVersion: 1, State: "active", RecoveryGeneration: "fixture", SavedRequestsSchemaVersion: 3, CloudExecutionsSchemaVersion: 1, ExecutionInputsSchemaVersion: 1, ProtectedSecretsSchemaVersion: 1})
	f.write(t, f.s.Table, row{PK: "U#" + f.owner, SK: "META", Kind: "user", SchemaVersion: 1, UserID: f.owner})
	f.write(t, f.s.Table, row{PK: "P#" + f.project, SK: "META", Kind: "project", SchemaVersion: 1, ProjectID: f.project, OwnerID: f.owner, State: "active"})
	plan := fixturePlan()
	config, _ := json.Marshal(plan.Configuration)
	f.write(t, f.s.Table, row{PK: "P#" + f.project, SK: "REQ#" + f.request, Kind: "request", SchemaVersion: 2, ProjectID: f.project, RequestID: f.request, Revision: 1, State: "active", ConfigurationJSON: string(config)})
	f.s.prepare = func(ctx context.Context, plan Plan, values map[string]string) (*Prepared, error) {
		return fixturePrepared(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			f.calls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true}`))
		}), plan, values), nil
	}
	return f
}
func (f *protocolFixture) write(t *testing.T, table string, r row) {
	t.Helper()
	item, e := attributevalue.MarshalMap(r)
	if e != nil {
		t.Fatal(e)
	}
	_, e = f.db.Client.PutItem(context.Background(), &dynamodb.PutItemInput{TableName: &table, Item: item})
	if e != nil {
		t.Fatal(e)
	}
}
func (f *protocolFixture) admit(t *testing.T, key string) (View, Message) {
	t.Helper()
	options, _ := ParseOptions([]byte(`{}`))
	v, e := f.s.Submit(context.Background(), f.owner, f.project, f.request, key, 1, options, false)
	if e != nil {
		t.Fatal(e)
	}
	r, e := f.s.get(context.Background(), f.s.Table, "P#"+f.project, "EXEC#"+v.ExecutionID)
	if e != nil {
		t.Fatal(e)
	}
	return v, Message{f.project, r.JobID, "OUT#" + r.JobID}
}

func TestIntegrationCredentialNamesMaskedBeforePublication(t *testing.T) {
	f := newProtocolFixture(t)
	ctx := context.Background()
	plan := fixturePlan()
	query := url.Values{"public": {"keep-query"}}
	for _, name := range []string{"key", "auth", "pwd"} {
		query.Add(name, "unmapped-query-"+name)
	}
	// Exercise the execution boundary independently of authoring validation and
	// protected-value reflection matching, using only disposable fixture values.
	plan.Configuration.URL += "?" + query.Encode()
	request, err := f.s.get(ctx, f.s.Table, "P#"+f.project, "REQ#"+f.request)
	if err != nil {
		t.Fatal(err)
	}
	configuration, err := json.Marshal(plan.Configuration)
	if err != nil {
		t.Fatal(err)
	}
	request.ConfigurationJSON = string(configuration)
	f.write(t, f.s.Table, request)
	f.s.prepare = func(ctx context.Context, plan Plan, values map[string]string) (*Prepared, error) {
		if len(values) != 0 {
			t.Fatal("regression must not depend on protected-input values")
		}
		return fixturePrepared(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			f.calls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			body := map[string]string{"public": "keep-json"}
			for _, name := range []string{"key", "auth", "pwd"} {
				w.Header().Set(name, "unmapped-header-"+name)
				body[name] = "unmapped-response-" + name
			}
			_ = json.NewEncoder(w).Encode(body)
		}), plan, values), nil
	}
	v, message := f.admit(t, id())
	assertSanitized := func(raw []byte) {
		t.Helper()
		if bytes.Contains(raw, []byte("unmapped-")) {
			t.Fatal("unmapped credential reached evidence storage or delivery")
		}
		var evidence Evidence
		if err := json.Unmarshal(raw, &evidence); err != nil {
			t.Fatal(err)
		}
		if evidence.Response.Body.Text == nil || evidence.Status != "completed" {
			t.Fatal("expected completed JSON evidence")
		}
		var body map[string]string
		if err := json.Unmarshal([]byte(*evidence.Response.Body.Text), &body); err != nil {
			t.Fatal(err)
		}
		capturedURL, err := url.Parse(evidence.Request.URL)
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"key", "auth", "pwd"} {
			if body[name] != "[REDACTED]" || capturedURL.Query().Get(name) != "[REDACTED]" {
				t.Fatal("credential JSON field or query parameter was not masked")
			}
			found := false
			for _, h := range evidence.Response.Headers {
				if strings.EqualFold(h.Name, name) {
					found = true
					if h.Value != "[REDACTED]" {
						t.Fatal("credential response header was not masked")
					}
				}
			}
			if !found {
				t.Fatal("credential response header disappeared instead of being masked")
			}
		}
		if body["public"] != "keep-json" || capturedURL.Query().Get("public") != "keep-query" {
			t.Fatal("public evidence changed")
		}
	}
	var uploaded []byte
	f.objects.putFault = func() {
		f.objects.mu.Lock()
		for _, raw := range f.objects.data {
			uploaded = append([]byte(nil), raw...)
		}
		f.objects.mu.Unlock()
		assertSanitized(uploaded)
		if _, err := f.s.get(ctx, f.s.Table, "P#"+f.project, "EXEC#"+v.ExecutionID+"#SNAP"); !missingRow(err) {
			t.Fatal("publication preceded sanitized upload verification")
		}
	}
	if err := f.s.Process(ctx, message); err != nil {
		t.Fatal(err)
	}
	if f.calls.Load() != 1 || f.objects.puts != 1 {
		t.Fatal("expected one outbound request and one evidence upload")
	}
	api, err := f.s.Evidence(ctx, f.owner, f.project, v.ExecutionID)
	if err != nil {
		t.Fatal(err)
	}
	var response struct {
		Data struct {
			Evidence json.RawMessage `json:"evidence"`
		} `json:"data"`
	}
	if err := json.Unmarshal(api, &response); err != nil {
		t.Fatal(err)
	}
	assertSanitized(response.Data.Evidence)
	if !bytes.Equal(uploaded, response.Data.Evidence) {
		t.Fatal("published evidence differs from sanitized upload")
	}
}

// Invocation throttling is simulated by withholding worker delivery. No AWS
// account concurrency or production trigger is changed by this test.
func TestIntegrationThrottledDeliveryRecovery(t *testing.T) {
	for _, delay := range []time.Duration{2 * time.Minute, 11 * time.Minute} {
		t.Run(delay.String(), func(t *testing.T) {
			f := newProtocolFixture(t)
			v, message := f.admit(t, id())
			f.now = f.now.Add(delay)
			if f.calls.Load() != 0 {
				t.Fatal("queue admission dispatched before worker delivery")
			}
			for range 3 {
				if e := f.s.Process(context.Background(), message); e != nil {
					t.Fatal(e)
				}
			}
			view, e := f.s.Detail(context.Background(), f.owner, f.project, v.ExecutionID)
			if e != nil {
				t.Fatal(e)
			}
			if delay < 10*time.Minute {
				if f.calls.Load() != 1 || view.Status != "completed" {
					t.Fatal("delayed duplicate delivery failed single-dispatch recovery")
				}
			} else if f.calls.Load() != 0 || view.Status != "failed" || view.Summary.Outcome.Certainty != "not_dispatched" {
				t.Fatal("expired throttled admission performed outbound HTTP")
			}
		})
	}
}
func writesState(in *dynamodb.TransactWriteItemsInput, kind, state string) bool {
	for _, a := range in.TransactItems {
		if a.Put != nil {
			var r row
			_ = attributevalue.UnmarshalMap(a.Put.Item, &r)
			if r.Kind == kind && r.State == state {
				return true
			}
		}
	}
	return false
}

func TestIntegrationAdmissionIdentityAndDuplicateDelivery(t *testing.T) {
	f := newProtocolFixture(t)
	ctx := context.Background()
	key := id()
	v, m := f.admit(t, key)
	again, _ := f.admit(t, key)
	if again.ExecutionID != v.ExecutionID {
		t.Fatal("retry admitted new execution")
	}
	options, _ := ParseOptions([]byte(`{}`))
	if _, e := f.s.Submit(ctx, f.owner, f.project, id(), key, 1, options, false); e == nil {
		t.Fatal("cross-source key reuse accepted")
	}
	if _, e := f.s.Submit(ctx, f.owner, f.project, v.ExecutionID, key, 0, options, true); e == nil {
		t.Fatal("cross-operation key reuse accepted")
	}
	if e := f.s.Process(ctx, m); e != nil {
		t.Fatal(e)
	}
	for range 3 {
		if e := f.s.Process(ctx, m); e != nil {
			t.Fatal(e)
		}
	}
	if f.calls.Load() != 1 {
		t.Fatal("duplicate delivery repeated HTTP")
	}
	first, e := f.s.Evidence(ctx, f.owner, f.project, v.ExecutionID)
	if e != nil {
		t.Fatal(e)
	}
	second, e := f.s.Evidence(ctx, f.owner, f.project, v.ExecutionID)
	if e != nil || !bytes.Equal(first, second) {
		t.Fatal("immutable evidence changed")
	}
	if _, e := f.s.get(ctx, f.s.Protected, "P#"+f.project, "JOB#"+m.JobID+"#BINDINGS"); !missingRow(e) {
		t.Fatal("terminal inputs were retained")
	}
}
func TestIntegrationDispatchAndAcknowledgmentFaults(t *testing.T) {
	for _, fault := range []string{"admission-ack", "claim-ack", "intent-ack", "before-intent", "after-intent", "publication", "queue-ack", "put-ack"} {
		t.Run(fault, func(t *testing.T) {
			f := newProtocolFixture(t)
			ctx := context.Background()
			fired := false
			if fault == "queue-ack" {
				f.queue.fail = true
			}
			if fault == "admission-ack" {
				f.db.after = func(in *dynamodb.TransactWriteItemsInput) error {
					if !fired && writesState(in, "executionJob", "queued") {
						fired = true
						return errors.New("lost acknowledgment")
					}
					return nil
				}
			}
			v, m := f.admit(t, id())
			f.db.after = nil
			switch fault {
			case "claim-ack", "intent-ack":
				state := "claimed"
				if fault == "intent-ack" {
					state = "running"
				}
				f.db.after = func(in *dynamodb.TransactWriteItemsInput) error {
					if !fired && writesState(in, "executionJob", state) {
						fired = true
						return errors.New("lost acknowledgment")
					}
					return nil
				}
			case "before-intent":
				f.db.before = func(in *dynamodb.TransactWriteItemsInput) error {
					if !fired && writesState(in, "executionJob", "running") {
						fired = true
						return errors.New("crash before intent")
					}
					return nil
				}
			case "after-intent":
				f.db.after = func(in *dynamodb.TransactWriteItemsInput) error {
					if !fired && writesState(in, "executionJob", "running") {
						fired = true
						f.now = f.now.Add(121 * time.Second)
						f.db.failReads = true
						return errors.New("crash after intent")
					}
					return nil
				}
				f.db.before = func(in *dynamodb.TransactWriteItemsInput) error {
					if fired {
						return errors.New("crashed writer")
					}
					return nil
				}
			case "publication":
				f.db.before = func(in *dynamodb.TransactWriteItemsInput) error {
					if writesState(in, "executionJob", "completed") {
						return errors.New("publication unavailable")
					}
					return nil
				}
			case "put-ack":
				f.objects.lostACK = true
			}
			err := f.s.Process(ctx, m)
			if (fault == "before-intent" || fault == "after-intent" || fault == "publication" || fault == "put-ack") && err == nil {
				t.Fatal("fault did not interrupt protocol")
			}
			f.db.before = nil
			f.db.after = nil
			f.db.failReads = false
			f.objects.lostACK = false
			if fault == "before-intent" || fault == "after-intent" || fault == "publication" || fault == "put-ack" {
				f.now = f.now.Add(121 * time.Second)
				if e := f.s.Process(ctx, m); e != nil {
					t.Fatal(e)
				}
			}
			if e := f.s.Process(ctx, m); e != nil {
				t.Fatal(e)
			}
			want := int32(1)
			if fault == "after-intent" {
				want = 0
			}
			if f.calls.Load() != want {
				t.Fatalf("HTTP attempt count %d expected %d", f.calls.Load(), want)
			}
			view, e := f.s.Detail(ctx, f.owner, f.project, v.ExecutionID)
			if e != nil {
				t.Fatal(e)
			}
			if fault == "after-intent" || fault == "put-ack" {
				if view.Summary.Outcome.Certainty != "unknown" {
					t.Fatal("ambiguous result replayed or fabricated")
				}
			}
		})
	}
}
func TestIntegrationRetentionAndLateDeletion(t *testing.T) {
	for _, mode := range []string{"retention", "pinned", "delete-during-put", "unknown-put"} {
		t.Run(mode, func(t *testing.T) {
			f := newProtocolFixture(t)
			ctx := context.Background()
			v, m := f.admit(t, id())
			if mode == "delete-during-put" {
				f.objects.putFault = func() {
					p, e := f.s.get(ctx, f.s.Table, "P#"+f.project, "META")
					if e != nil {
						t.Fatal(e)
					}
					p.State = "deleting"
					p.DeletionEpoch++
					p.Version++
					f.write(t, f.s.Table, p)
				}
			}
			if mode == "unknown-put" {
				f.objects.lostACK = true
			}
			err := f.s.Process(ctx, m)
			if mode == "delete-during-put" || mode == "unknown-put" {
				if err == nil {
					t.Fatal("unsafe publication succeeded")
				}
				if _, e := f.s.Evidence(ctx, f.owner, f.project, v.ExecutionID); e == nil {
					t.Fatal("unpublished evidence delivered")
				}
			}
			if mode == "retention" || mode == "pinned" {
				ret, e := f.s.get(ctx, f.s.Table, "P#"+f.project, "EXEC#"+v.ExecutionID+"#RET")
				if e != nil {
					t.Fatal(e)
				}
				f.now = f.now.Add(31 * 24 * time.Hour)
				if mode == "pinned" {
					ret.Pinned = true
					ret.Version++
					f.write(t, f.s.Table, ret)
					if e := f.s.Maintain(ctx); e != nil {
						t.Fatal(e)
					}
					if _, e := f.s.Evidence(ctx, f.owner, f.project, v.ExecutionID); e != nil {
						t.Fatal("pinned evidence expired")
					}
					return
				}
				for range 10 {
					ret, e = f.s.get(ctx, f.s.Table, ret.PK, ret.SK)
					if missingRow(e) {
						break
					}
					_ = f.s.expire(ctx, ret)
				}
				if _, e := f.s.get(ctx, f.s.Table, "P#"+f.project, "EXEC#"+v.ExecutionID); !missingRow(e) {
					t.Fatal("retention did not drain execution")
				}
				if len(f.objects.data) != 0 {
					t.Fatal("retention leaked object")
				}
			}
			if mode == "delete-during-put" {
				for range 10 {
					if f.s.DrainProject(ctx, f.project) == nil {
						break
					}
				}
				if len(f.objects.data) != 0 {
					t.Fatal("late upload not cleaned")
				}
			}
			if mode == "unknown-put" {
				p, e := f.s.get(ctx, f.s.Table, "P#"+f.project, "META")
				if e != nil {
					t.Fatal(e)
				}
				p.State = "deleting"
				p.Version++
				p.DeletionEpoch++
				f.write(t, f.s.Table, p)
				for range 3 {
					if f.s.DrainProject(ctx, f.project) == nil {
						t.Fatal("uncertain upload incorrectly completed deletion")
					}
				}
			}
		})
	}
}

func TestUnknownExecutionFieldsArePreserved(t *testing.T) {
	r := row{PK: "P#fixture", SK: "EXEC#fixture", Kind: "execution", SchemaVersion: 1, ProjectID: "fixture", ExecutionID: "fixture", Plan: &Plan{Configuration: Configuration{Headers: []Field{}, QueryParameters: []Field{}}}}
	item, _ := attributevalue.MarshalMap(r)
	item["futureField"] = str("future")
	if knownExecution(item, r) {
		t.Fatal("unknown field accepted for deletion")
	}
	delete(item, "futureField")
	item["plan"].(*dt.AttributeValueMemberM).Value["futureNested"] = str("future")
	if knownExecution(item, r) {
		t.Fatal("unknown nested field accepted for deletion")
	}
	if strings.Contains(safeError().Error(), "fixture") {
		t.Fatal("unsafe diagnostics")
	}
}

func fixtureSecretEnvelope(t *testing.T, s *Service, project, secret, value string, revision int64) *Envelope {
	t.Helper()
	scope := s.cryptoScope(project, secret, "saved-secret", revision)
	aad, _ := json.Marshal(scope)
	block, _ := aes.NewCipher(bytes.Repeat([]byte{7}, 32))
	gcm, _ := cipher.NewGCM(block)
	nonce := make([]byte, 12)
	_, _ = rand.Read(nonce)
	sealed := gcm.Seal(nil, nonce, []byte(value), aad)
	return &Envelope{1, "AES-256-GCM", s.KeyARN, aad, nonce, sealed[:len(sealed)-16], sealed[len(sealed)-16:]}
}
func TestIntegrationFrozenSecretReplacementAndRevocationRaces(t *testing.T) {
	for _, mode := range []string{"replace-after-submit", "revoke-before-intent", "replace-at-intent"} {
		t.Run(mode, func(t *testing.T) {
			f := newProtocolFixture(t)
			ctx := context.Background()
			secret := id()
			oldValue := "disposable-old-fixture-credential"
			newValue := "disposable-new-fixture-credential"
			saved := row{PK: "P#" + f.project, SK: "SECRET#" + secret, Kind: "savedSecret", SchemaVersion: 1, ProjectID: f.project, SecretID: secret, Revision: 1, State: "active", ValueKind: "string", Envelope: fixtureSecretEnvelope(t, f.s, f.project, secret, oldValue, 1)}
			f.write(t, f.s.Protected, saved)
			request, e := f.s.get(ctx, f.s.Table, saved.PK, "REQ#"+f.request)
			if e != nil {
				t.Fatal(e)
			}
			plan := fixturePlan()
			plan.Configuration.Headers = []Field{{Name: "X-Protected", Enabled: true, Sensitive: true, BindingID: id(), Masked: true, SecretRef: &Ref{secret}}}
			config, _ := json.Marshal(plan.Configuration)
			request.ConfigurationJSON = string(config)
			f.write(t, f.s.Table, request)
			options, _ := ParseOptions([]byte(`{"allowInsecureSecrets":true}`))
			v, e := f.s.Submit(ctx, f.owner, f.project, f.request, id(), 1, options, false)
			if e != nil {
				t.Fatal(e)
			}
			anchor, _ := f.s.get(ctx, f.s.Table, saved.PK, "EXEC#"+v.ExecutionID)
			m := Message{f.project, anchor.JobID, "OUT#" + anchor.JobID}
			var oldSent atomic.Bool
			f.s.prepare = func(ctx context.Context, plan Plan, values map[string]string) (*Prepared, error) {
				return fixturePrepared(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					f.calls.Add(1)
					oldSent.Store(r.Header.Get("X-Protected") == oldValue)
					w.Header().Set("Content-Type", "application/json")
					raw, _ := json.Marshal(map[string]string{"reflection": r.Header.Get("X-Protected")})
					_, _ = w.Write(raw)
				}), plan, values), nil
			}
			replace := func() {
				saved.Revision++
				saved.Envelope = fixtureSecretEnvelope(t, f.s, f.project, secret, newValue, saved.Revision)
				if mode == "revoke-before-intent" {
					saved.State = "revoked"
					saved.Envelope = nil
				}
				f.write(t, f.s.Protected, saved)
			}
			if mode == "replace-after-submit" {
				replace()
			} else {
				fired := false
				f.db.before = func(in *dynamodb.TransactWriteItemsInput) error {
					if !fired && writesState(in, "executionJob", "running") {
						fired = true
						replace()
					}
					return nil
				}
			}
			e = f.s.Process(ctx, m)
			if mode != "replace-after-submit" {
				if e == nil {
					t.Fatal("source race not fenced")
				}
				if f.calls.Load() != 0 {
					t.Fatal("source race permitted HTTP")
				}
				f.db.before = nil
				f.now = f.now.Add(121 * time.Second)
				e = f.s.Process(ctx, m)
			}
			if e != nil {
				t.Fatal(e)
			}
			view, e := f.s.Detail(ctx, f.owner, f.project, v.ExecutionID)
			if e != nil {
				t.Fatal(e)
			}
			if mode == "revoke-before-intent" {
				if f.calls.Load() != 0 || view.Summary.Outcome.Certainty != "not_dispatched" {
					t.Fatal("revocation dispatched")
				}
			} else {
				if f.calls.Load() != 1 || !oldSent.Load() {
					t.Fatal("replacement changed accepted protected inputs")
				}
				raw, e := f.s.Evidence(ctx, f.owner, f.project, v.ExecutionID)
				if e != nil || bytes.Contains(raw, []byte(oldValue)) || bytes.Contains(raw, []byte(newValue)) {
					t.Fatal("protected value escaped evidence redaction")
				}
			}
		})
	}
}
