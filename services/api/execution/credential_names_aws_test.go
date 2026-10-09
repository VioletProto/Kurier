//go:build integration && aws

package execution

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// Real deployed worker, SQS, HTTPS, Control and S3. Delivery uses the API's
// Evidence service/envelope; authenticated HTTP/browser delivery is separate.
func TestAWSCloudExecutionCredentialNames(t *testing.T) {
	if os.Getenv("KURIER_AWS_AUDIT_PROJECT") != "" || os.Getenv("KURIER_AWS_TEST_CLEANUP_PROJECT") != "" {
		t.Skip("Standalone credential fixture excluded from read-only audit and resumed cleanup")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-east-2"), config.WithRetryer(func() aws.Retryer { return aws.NopRetryer{} }))
	if err != nil {
		t.Fatal("AWS configuration unavailable")
	}
	identity, err := sts.NewFromConfig(cfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil || aws.ToString(identity.Account) != "747336059622" || strings.HasSuffix(aws.ToString(identity.Arn), ":root") {
		t.Fatal("intended non-root development identity required")
	}
	s := &Service{DB: dynamodb.NewFromConfig(cfg), Objects: s3.NewFromConfig(cfg), Queue: sqs.NewFromConfig(cfg), KMS: kms.NewFromConfig(cfg), Table: os.Getenv("KURIER_AWS_TEST_TABLE"), Protected: os.Getenv("KURIER_AWS_TEST_PROTECTED"), Bucket: os.Getenv("KURIER_AWS_TEST_BUCKET"), QueueURL: os.Getenv("KURIER_AWS_TEST_QUEUE"), KeyARN: os.Getenv("KURIER_AWS_TEST_KEY"), Stage: "dev-api"}
	endpoint := os.Getenv("KURIER_AWS_TEST_ENDPOINT")
	if !strings.HasPrefix(s.Table, "kurier-dev-api-ControlTable-") || !strings.HasPrefix(s.Protected, "kurier-dev-api-ProtectedTable-") || s.Bucket == "" || s.QueueURL == "" || endpoint != "https://8dnymkcoa0.execute-api.us-east-2.amazonaws.com/execution-fixture/echo" {
		t.Fatal("scoped development outputs required")
	}
	parameter, err := ssm.NewFromConfig(cfg).GetParameter(ctx, &ssm.GetParameterInput{Name: aws.String("/kurier/dev-api/cursor-key"), WithDecryption: aws.Bool(true)})
	if err != nil || parameter.Parameter == nil {
		t.Fatal("private signing key unavailable")
	}
	s.SigningKey, err = base64.StdEncoding.DecodeString(aws.ToString(parameter.Parameter.Value))
	if err != nil || len(s.SigningKey) < 32 {
		t.Fatal("private signing key invalid")
	}
	defer clear(s.SigningKey)
	owner, project, request := id(), id(), id()
	write := func(r row) {
		t.Helper()
		item, e := attributevalue.MarshalMap(r)
		if e != nil {
			t.Fatal("fixture encoding unavailable")
		}
		_, e = s.DB.(*dynamodb.Client).PutItem(ctx, &dynamodb.PutItemInput{TableName: &s.Table, Item: item, ConditionExpression: aws.String("attribute_not_exists(PK)")})
		if e != nil {
			t.Fatal("fixture creation unavailable")
		}
	}
	write(row{PK: "U#" + owner, SK: "META", Kind: "user", SchemaVersion: 1, UserID: owner})
	write(row{PK: "P#" + project, SK: "META", Kind: "project", SchemaVersion: 1, ProjectID: project, OwnerID: owner, State: "active"})
	t.Logf("Disposable unmapped-name fixture project %s; owner %s", project, owner)
	plan := fixturePlan()
	query := url.Values{"public": {"keep-public"}}
	for _, name := range []string{"key", "auth", "pwd"} {
		query.Set(name, "unmapped-fixture-"+name)
	}
	// Deliberately bypass authoring validation with fixed non-secret markers to
	// exercise query-name defense at the execution boundary, with zero secrets.
	plan.Configuration.URL = strings.TrimSuffix(endpoint, "echo") + "credential-names?" + query.Encode()
	raw, _ := json.Marshal(plan.Configuration)
	write(row{PK: "P#" + project, SK: "REQ#" + request, Kind: "request", SchemaVersion: 2, ProjectID: project, RequestID: request, Revision: 1, State: "active", ConfigurationJSON: string(raw)})
	options, _ := ParseOptions([]byte(`{}`))
	run, err := s.Submit(ctx, owner, project, request, id(), 1, options, false)
	if err != nil {
		t.Fatal("AWS admission unavailable")
	}
	anchor, err := s.get(ctx, s.Table, "P#"+project, "EXEC#"+run.ExecutionID)
	if err != nil {
		t.Fatal("execution metadata unavailable")
	}
	job, err := s.get(ctx, s.Table, "P#"+project, "JOB#"+anchor.JobID)
	if err != nil || len(job.Sources) != 0 {
		t.Fatal("regression must have zero protected sources")
	}
	for deadline := time.Now().Add(90 * time.Second); time.Now().Before(deadline); {
		run, err = s.Detail(ctx, owner, project, run.ExecutionID)
		if err == nil && terminal(run.Status) {
			break
		}
		time.Sleep(time.Second)
	}
	if err != nil || run.Status != "completed" {
		t.Fatal("deployed worker did not complete regression")
	}
	manifest, err := s.get(ctx, s.Table, "P#"+project, "EXEC#"+run.ExecutionID+"#SNAP")
	if err != nil {
		t.Fatal("immutable snapshot unavailable")
	}
	object, err := s.Objects.GetObject(ctx, &s3.GetObjectInput{Bucket: &s.Bucket, Key: &manifest.ObjectKey})
	if err != nil {
		t.Fatal("private evidence object unavailable")
	}
	stored, err := io.ReadAll(io.LimitReader(object.Body, EvidenceLimit+1))
	_ = object.Body.Close()
	checksum := sha256.Sum256(stored)
	if err != nil || int64(len(stored)) != manifest.Bytes || hex.EncodeToString(checksum[:]) != manifest.Checksum {
		t.Fatal("immutable stored checksum or size mismatch")
	}
	var capture Evidence
	if json.Unmarshal(stored, &capture) != nil {
		t.Fatal("stored evidence invalid")
	}
	assertUnmappedCredentialCapture(t, capture)
	delivered, err := s.Evidence(ctx, owner, project, run.ExecutionID)
	var envelope struct {
		Data struct {
			Evidence json.RawMessage `json:"evidence"`
		} `json:"data"`
	}
	if err != nil || json.Unmarshal(delivered, &envelope) != nil || !bytes.Equal(envelope.Data.Evidence, stored) || bytes.Contains(delivered, []byte("unmapped-fixture-")) {
		t.Fatal("API envelope differed from sanitized stored evidence")
	}
	t.Logf("Execution %s: stored JSON fields, response headers and request URL query names key/auth/pwd masked; zero protected sources; immutable checksum and API envelope equality passed", run.ExecutionID)
	cleanupAWSExecutionFixture(t, s, owner, project)
	t.Log("Disposable fixture physical cleanup verified")
}

func assertUnmappedCredentialCapture(t *testing.T, capture Evidence) {
	t.Helper()
	encoded, _ := json.Marshal(capture)
	if bytes.Contains(encoded, []byte("unmapped-fixture-")) || capture.Response.Body.Text == nil || capture.Response.HTTPStatus == nil || *capture.Response.HTTPStatus != 200 {
		t.Fatal("unmapped-name evidence unsafe or incomplete")
	}
	var body map[string]map[string]string
	u, err := url.Parse(capture.Request.URL)
	if err != nil || json.Unmarshal([]byte(*capture.Response.Body.Text), &body) != nil {
		t.Fatal("sanitized JSON or request URL invalid")
	}
	for _, name := range []string{"key", "auth", "pwd"} {
		if body["fields"][name] != "[REDACTED]" || body["query"][name] != "[REDACTED]" || u.Query().Get(name) != "[REDACTED]" {
			t.Fatal("unmapped JSON field or URL query name was not masked")
		}
		found := false
		for _, h := range capture.Response.Headers {
			if strings.EqualFold(h.Name, name) {
				found = true
				if h.Value != "[REDACTED]" {
					t.Fatal("unmapped response header not masked")
				}
			}
		}
		if !found {
			t.Fatal("credential response header missing")
		}
	}
	if body["fields"]["public"] != "keep-public" || u.Query().Get("public") != "keep-public" {
		t.Fatal("public control evidence changed")
	}
}
