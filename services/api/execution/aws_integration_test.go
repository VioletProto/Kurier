//go:build integration && aws

package execution

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dt "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kt "github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// These tests use real AWS KMS, DynamoDB, SQS, deployed worker/IAM, HTTPS and S3.
// Admission is direct SDK/service, not a claim of browser/Cognito HTTP validation.
func TestAWSCloudExecution(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	cfg, e := config.LoadDefaultConfig(ctx, config.WithRegion("us-east-2"), config.WithRetryer(func() aws.Retryer { return aws.NopRetryer{} }))
	if e != nil {
		t.Fatal("AWS configuration unavailable")
	}
	identity, e := sts.NewFromConfig(cfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if e != nil || aws.ToString(identity.Account) != "747336059622" || strings.HasSuffix(aws.ToString(identity.Arn), ":root") {
		t.Fatal("intended non-root AWS development account required")
	}
	s := &Service{DB: dynamodb.NewFromConfig(cfg), Objects: s3.NewFromConfig(cfg), Queue: sqs.NewFromConfig(cfg), KMS: kms.NewFromConfig(cfg), Table: os.Getenv("KURIER_AWS_TEST_TABLE"), Protected: os.Getenv("KURIER_AWS_TEST_PROTECTED"), Bucket: os.Getenv("KURIER_AWS_TEST_BUCKET"), QueueURL: os.Getenv("KURIER_AWS_TEST_QUEUE"), KeyARN: os.Getenv("KURIER_AWS_TEST_KEY"), Stage: "dev-api"}
	endpoint := os.Getenv("KURIER_AWS_TEST_ENDPOINT")
	if !strings.HasPrefix(s.Table, "kurier-dev-api-ControlTable-") || !strings.HasPrefix(s.Protected, "kurier-dev-api-ProtectedTable-") || s.Bucket == "" || s.QueueURL == "" || !strings.HasPrefix(endpoint, "https://8dnymkcoa0.execute-api.us-east-2.amazonaws.com/execution-fixture/") {
		t.Fatal("scoped deployed outputs required")
	}
	parameter, e := ssm.NewFromConfig(cfg).GetParameter(ctx, &ssm.GetParameterInput{Name: aws.String("/kurier/dev-api/cursor-key"), WithDecryption: aws.Bool(true)})
	if e != nil || parameter.Parameter == nil {
		t.Fatal("private signing key unavailable")
	}
	s.SigningKey, e = base64.StdEncoding.DecodeString(aws.ToString(parameter.Parameter.Value))
	if e != nil || len(s.SigningKey) < 32 {
		t.Fatal("private signing key invalid")
	}
	defer clear(s.SigningKey)
	if project := os.Getenv("KURIER_AWS_AUDIT_PROJECT"); project != "" {
		auditAWSBrowserEvidence(t, ctx, s, project, strings.Split(os.Getenv("KURIER_AWS_AUDIT_EXECUTIONS"), ","))
		return
	}
	client := s.DB.(*dynamodb.Client)
	if project := os.Getenv("KURIER_AWS_TEST_CLEANUP_PROJECT"); project != "" {
		owner := os.Getenv("KURIER_AWS_TEST_CLEANUP_OWNER")
		if !uuidPattern.MatchString(project) || !uuidPattern.MatchString(owner) {
			t.Fatal("explicit disposable fixture IDs required")
		}
		cleanupAWSExecutionFixture(t, s, owner, project)
		t.Log("Interrupted disposable AWS fixture cleanup resumed and verified")
		return
	}
	owner, project, request, secret := id(), id(), id(), id()
	t.Logf("Disposable fixture project %s; owner %s (public identifiers only)", project, owner)
	started := time.Now()
	write := func(table string, r row) {
		t.Helper()
		item, err := attributevalue.MarshalMap(r)
		if err != nil {
			t.Fatal("fixture encoding unavailable")
		}
		_, err = client.PutItem(ctx, &dynamodb.PutItemInput{TableName: &table, Item: item, ConditionExpression: aws.String("attribute_not_exists(PK)")})
		if err != nil {
			t.Fatal("AWS fixture creation unavailable")
		}
	}
	write(s.Table, row{PK: "U#" + owner, SK: "META", Kind: "user", SchemaVersion: 1, UserID: owner})
	write(s.Table, row{PK: "P#" + project, SK: "META", Kind: "project", SchemaVersion: 1, ProjectID: project, OwnerID: owner, State: "active"})
	value := "Bearer " + id() + "-disposable-aws-execution-fixture"
	component := strings.TrimPrefix(value, "Bearer ")
	scope := s.cryptoScope(project, secret, "saved-secret", 1)
	dataKey, e := s.KMS.GenerateDataKey(ctx, &kms.GenerateDataKeyInput{KeyId: &s.KeyARN, KeySpec: kt.DataKeySpecAes256, EncryptionContext: scope})
	if e != nil {
		t.Fatal("actual KMS source preparation unavailable")
	}
	block, _ := aes.NewCipher(dataKey.Plaintext)
	gcm, _ := cipher.NewGCM(block)
	clear(dataKey.Plaintext)
	nonce := make([]byte, 12)
	_, _ = rand.Read(nonce)
	aad, _ := json.Marshal(scope)
	sealed := gcm.Seal(nil, nonce, []byte(value), aad)
	envelope := &Envelope{1, "AES-256-GCM", s.KeyARN, dataKey.CiphertextBlob, nonce, sealed[:len(sealed)-16], sealed[len(sealed)-16:]}
	write(s.Protected, row{PK: "P#" + project, SK: "SECRET#" + secret, Kind: "savedSecret", SchemaVersion: 1, ProjectID: project, SecretID: secret, Revision: 1, State: "active", ValueKind: "string", Envelope: envelope})
	base := strings.TrimSuffix(endpoint, "echo")
	plan := fixturePlan()
	plan.Configuration.URL = endpoint
	plan.Configuration.Headers = []Field{{Name: "Authorization", Enabled: true, Sensitive: true, BindingID: id(), Masked: true, SecretRef: &Ref{secret}}, {Name: "X-Designated", Enabled: true, Sensitive: true, BindingID: id(), Masked: true, SecretRef: &Ref{secret}}}
	plan.Configuration.QueryParameters = []Field{{Name: "designated", Enabled: true, Sensitive: true, BindingID: id(), Masked: true, SecretRef: &Ref{secret}}}
	raw, _ := json.Marshal(plan.Configuration)
	write(s.Table, row{PK: "P#" + project, SK: "REQ#" + request, Kind: "request", SchemaVersion: 2, ProjectID: project, RequestID: request, Revision: 1, State: "active", ConfigurationJSON: string(raw)})
	options, _ := ParseOptions([]byte(`{}`))
	v, e := s.Submit(ctx, owner, project, request, id(), 1, options, false)
	if e != nil {
		t.Fatal("actual cloud admission unavailable")
	}
	await := func(execution string) View {
		t.Helper()
		deadline := time.Now().Add(90 * time.Second)
		for time.Now().Before(deadline) {
			view, err := s.Detail(ctx, owner, project, execution)
			if err == nil && terminal(view.Status) {
				return view
			}
			time.Sleep(time.Second)
		}
		t.Fatal("actual SQS worker terminal status pending")
		return View{}
	}
	v = await(v.ExecutionID)
	if v.Status != "completed" || v.Summary.HTTPStatus == nil || *v.Summary.HTTPStatus != 200 {
		t.Fatal("actual worker execution failed")
	}
	first, e := s.Evidence(ctx, owner, project, v.ExecutionID)
	if e != nil || bytes.Contains(first, []byte(value)) || bytes.Contains(first, []byte(component)) {
		t.Fatal("actual evidence containment check failed")
	}
	anchor, e := s.get(ctx, s.Table, "P#"+project, "EXEC#"+v.ExecutionID)
	if e != nil {
		t.Fatal("actual execution metadata unavailable")
	}
	for range 3 {
		s.fastNotify(ctx, project, anchor.JobID)
	}
	time.Sleep(3 * time.Second)
	second, e := s.Evidence(ctx, owner, project, v.ExecutionID)
	if e != nil || !bytes.Equal(first, second) {
		t.Fatal("duplicate SQS delivery changed published evidence")
	}
	for _, test := range []struct {
		mode, status, code, body string
		timeout                  int
	}{{"http-error", "failed", "upstream_http_error", "json", 30}, {"redirect", "completed", "", "json", 30}, {"oversize", "failed", "response_limit_exceeded", "omitted", 30}, {"gzip-limit", "failed", "response_limit_exceeded", "omitted", 30}, {"html", "completed", "", "omitted", 30}, {"delay", "failed", "execution_timeout", "none", 1}} {
		configuration := plan.Configuration
		configuration.URL = base + test.mode
		raw, _ := json.Marshal(configuration)
		nextRequest := id()
		write(s.Table, row{PK: "P#" + project, SK: "REQ#" + nextRequest, Kind: "request", SchemaVersion: 2, ProjectID: project, RequestID: nextRequest, Revision: 1, State: "active", ConfigurationJSON: string(raw)})
		o := options
		o.TimeoutSeconds = test.timeout
		run, err := s.Submit(ctx, owner, project, nextRequest, id(), 1, o, false)
		if err != nil {
			t.Fatal("actual mode admission unavailable")
		}
		run = await(run.ExecutionID)
		capture, err := s.Evidence(ctx, owner, project, run.ExecutionID)
		if err != nil || bytes.Contains(capture, []byte(component)) {
			t.Fatal("actual mode evidence unavailable or unsafe")
		}
		var response struct {
			Data struct {
				Evidence Evidence `json:"evidence"`
			} `json:"data"`
		}
		if json.Unmarshal(capture, &response) != nil {
			t.Fatal("actual evidence delivery invalid")
		}
		got := response.Data.Evidence
		if run.Status != test.status || got.Response.Body.Kind != test.body || (test.code != "" && (got.Outcome.Code == nil || *got.Outcome.Code != test.code)) {
			t.Fatalf("actual controlled mode %s failed contract", test.mode)
		}
	}
	// Access logs and runtime logs are fetched privately, never filtered by a secret.
	for _, name := range strings.Split(os.Getenv("KURIER_AWS_TEST_LOG_FUNCTIONS"), ",") {
		if !strings.HasPrefix(name, "kurier-dev-api-") {
			t.Fatal("scoped log functions required")
		}
		configuration, err := exec.CommandContext(ctx, "aws", "lambda", "get-function-configuration", "--region", "us-east-2", "--function-name", name, "--query", "LoggingConfig.LogGroup", "--output", "json").Output()
		var group string
		if err != nil || json.Unmarshal(configuration, &group) != nil || !strings.HasPrefix(group, "/aws/lambda/kurier-dev-api-") {
			t.Fatal("actual configured log group unavailable")
		}
		output, err := exec.CommandContext(ctx, "aws", "logs", "filter-log-events", "--region", "us-east-2", "--log-group-name", group, "--start-time", strconv.FormatInt(started.Add(-time.Minute).UnixMilli(), 10), "--output", "json").Output()
		if err != nil {
			t.Fatal("actual log containment audit pending")
		}
		if bytes.Contains(output, []byte(value)) || bytes.Contains(output, []byte(component)) {
			t.Fatal("actual log containment audit failed")
		}
		var events struct {
			Events    []json.RawMessage `json:"events"`
			NextToken string            `json:"nextToken"`
		}
		if json.Unmarshal(output, &events) != nil || events.NextToken != "" {
			t.Fatal("bounded log audit incomplete")
		}
		if (strings.Contains(name, "CloudExecutionWorker") || strings.Contains(name, "ExecutionControlledEndpoint")) && len(events.Events) == 0 {
			t.Fatal("worker/endpoint log ingestion pending; no empty audit accepted")
		}
		t.Logf("Log audit %s: %d events, disposable value absent", name, len(events.Events))
		clear(output)
	}
	cleanupAWSExecutionFixture(t, s, owner, project)
	// Public unauthenticated calls must still be rejected by the API.
	response, e := http.Get(strings.TrimSuffix(base, "/execution-fixture/") + "/api/v1/projects/" + project + "/executions")
	if e != nil {
		t.Fatal("actual HTTP authorization check pending")
	}
	_ = response.Body.Close()
	if response.StatusCode != 401 {
		t.Fatal("actual API anonymous read allowed")
	}
	t.Log("Real AWS KMS/DynamoDB/SQS/Lambda/HTTPS/S3, immutable duplicate handling, controlled modes, boolean-only log/evidence audit and execution deletion passed. Cognito/browser admission remains separate.")
}

// Read-only containment audit for public IDs explicitly supplied by the owner.
// Protected test values are decrypted only in this authorized process's memory.
// Never print them or use them as CLI arguments, log filters or persisted files.
func auditAWSBrowserEvidence(t *testing.T, ctx context.Context, s *Service, project string, executions []string) {
	t.Helper()
	if !uuidPattern.MatchString(project) || len(executions) == 0 || len(executions) > 8 {
		t.Fatal("explicit scoped browser probe IDs required")
	}
	values := map[string]string{}
	var captures [][]byte
	var start time.Time
	for _, execution := range executions {
		if !uuidPattern.MatchString(execution) {
			t.Fatal("invalid browser execution ID")
		}
		anchor, err := s.get(ctx, s.Table, "P#"+project, "EXEC#"+execution)
		if err != nil {
			t.Fatal("browser execution audit unavailable")
		}
		job, err := s.get(ctx, s.Table, anchor.PK, "JOB#"+anchor.JobID)
		if err != nil || len(job.Sources) == 0 {
			t.Fatal("protected browser inputs required")
		}
		for _, source := range job.Sources {
			secret, err := s.get(ctx, s.Protected, anchor.PK, "SECRET#"+source.SecretID)
			if err != nil || secret.State != "active" || secret.Revision != source.Revision {
				t.Fatal("browser source changed or revoked; original-value audit unavailable")
			}
			raw, err := s.decrypt(ctx, project, source.SecretID, "saved-secret", source.Revision, secret.Envelope)
			if err != nil {
				t.Fatal("authorized browser test-input audit unavailable")
			}
			values[source.SecretID] = string(raw)
			clear(raw)
		}
		capture, err := s.Evidence(ctx, anchor.OwnerID, project, execution)
		if err != nil {
			t.Fatal("browser immutable evidence unavailable")
		}
		captures = append(captures, capture)
		date, err := time.Parse("2006-01-02T15:04:05.000000000Z", anchor.SubmittedAt)
		if err != nil {
			t.Fatal("browser audit time unavailable")
		}
		if start.IsZero() || date.Before(start) {
			start = date
		}
	}
	sanitizer := newSanitizer(values)
	for _, capture := range captures {
		if sanitizer.text(string(capture)) != string(capture) {
			t.Fatal("browser evidence containment failed; no values returned")
		}
		clear(capture)
	}
	eventsChecked := 0
	for _, name := range strings.Split(os.Getenv("KURIER_AWS_TEST_LOG_FUNCTIONS"), ",") {
		if !strings.HasPrefix(name, "kurier-dev-api-") {
			t.Fatal("scoped browser log group required")
		}
		configuration, err := exec.CommandContext(ctx, "aws", "lambda", "get-function-configuration", "--region", "us-east-2", "--function-name", name, "--query", "LoggingConfig.LogGroup", "--output", "json").Output()
		var group string
		if err != nil || json.Unmarshal(configuration, &group) != nil || !strings.HasPrefix(group, "/aws/lambda/kurier-dev-api-") {
			t.Fatal("browser log configuration unavailable")
		}
		output, err := exec.CommandContext(ctx, "aws", "logs", "filter-log-events", "--region", "us-east-2", "--log-group-name", group, "--start-time", strconv.FormatInt(start.Add(-time.Minute).UnixMilli(), 10), "--output", "json").Output()
		if err != nil || sanitizer.text(string(output)) != string(output) {
			t.Fatal("browser log containment failed or unavailable; no values returned")
		}
		var events struct {
			Events    []json.RawMessage `json:"events"`
			NextToken string            `json:"nextToken"`
		}
		if json.Unmarshal(output, &events) != nil || events.NextToken != "" {
			t.Fatal("browser log audit incomplete")
		}
		if strings.Contains(name, "CloudExecutionWorker") && len(events.Events) == 0 {
			t.Fatal("worker log ingestion pending")
		}
		eventsChecked += len(events.Events)
		clear(output)
	}
	t.Logf("Browser evidence/log containment passed: %d executions, %d protected sources, %d log events; no values returned", len(executions), len(values), eventsChecked)
}

// The explicit cleanup path only targets a disposable fixture partition.
func cleanupAWSExecutionFixture(t *testing.T, s *Service, owner, project string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	client := s.DB.(*dynamodb.Client)
	p, e := s.get(ctx, s.Table, "P#"+project, "META")
	if e != nil {
		t.Fatal("AWS deletion gate unavailable")
	}
	if p.OwnerID != owner || p.Kind != "project" {
		t.Fatal("disposable fixture owner mismatch")
	}
	p.State = "deleting"
	p.DeletionEpoch++
	p.Version++
	a, _ := s.put(s.Table, p, false)
	stage, e := s.stage(ctx)
	if e != nil || s.transact(ctx, []dt.TransactWriteItem{s.stageGuard(stage), a}) != nil {
		t.Fatal("AWS deletion gate unavailable")
	}
	for attempt := 0; attempt < 40; attempt++ {
		err := s.DrainProject(ctx, project)
		if err == nil {
			break
		}
		if attempt == 39 {
			t.Fatal("actual AWS execution deletion pending")
		}
	}
	list, e := s.Objects.ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: &s.Bucket, Prefix: aws.String("dev-api/projects/" + project + "/")})
	if e != nil || len(list.Contents) != 0 {
		t.Fatal("actual AWS evidence cleanup pending")
	}
	for _, partition := range []struct{ table, pk string }{{s.Table, "P#" + project}, {s.Protected, "P#" + project}, {s.Table, "U#" + owner}} {
		page, err := client.Query(ctx, &dynamodb.QueryInput{TableName: &partition.table, KeyConditionExpression: aws.String("PK = :pk"), ExpressionAttributeValues: map[string]dt.AttributeValue{":pk": str(partition.pk)}, ConsistentRead: aws.Bool(true)})
		if err != nil || len(page.LastEvaluatedKey) != 0 {
			t.Fatal("actual fixture cleanup proof pending")
		}
		for _, item := range page.Items {
			if partition.pk == "P#"+project && partition.table == s.Table && item["SK"].(*dt.AttributeValueMemberS).Value == "META" {
				continue
			}
			_, err = client.DeleteItem(ctx, &dynamodb.DeleteItemInput{TableName: &partition.table, Key: map[string]dt.AttributeValue{"PK": item["PK"], "SK": item["SK"]}})
			if err != nil {
				t.Fatal("actual fixture cleanup pending")
			}
		}
	}
	p, e = s.get(ctx, s.Table, "P#"+project, "META")
	if e != nil {
		t.Fatal("actual tombstone unavailable")
	}
	tombstone := row{PK: p.PK, SK: "META", Kind: "projectTombstone", SchemaVersion: 1, ProjectID: project, OwnerID: owner, Version: p.Version + 1, State: "deleted", DeletionEpoch: p.DeletionEpoch}
	a, _ = s.put(s.Table, tombstone, false)
	if s.transact(ctx, []dt.TransactWriteItem{s.stageGuard(stage), a}) != nil {
		t.Fatal("actual tombstone publication pending")
	}
}
