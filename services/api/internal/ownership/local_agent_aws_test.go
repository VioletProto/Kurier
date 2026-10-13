//go:build integration && aws

package ownership

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VioletProto/Kurier/services/api/execution"
	runner "github.com/VioletProto/Kurier/services/request-runtime"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dt "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// SDK-created disposable identity/admission is deliberately separate from genuine
// browser/Cognito pairing. Agent protocol uses deployed HTTPS/Lambda IAM/KMS/S3;
// target HTTP is controlled loopback and is sent once, never retried on ambiguity.
func TestAWSLocalAgent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-east-2"), config.WithRetryer(func() aws.Retryer { return aws.NopRetryer{} }))
	if err != nil {
		t.Fatal("AWS configuration unavailable")
	}
	identity, err := sts.NewFromConfig(cfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil || aws.ToString(identity.Account) != "747336059622" || strings.HasSuffix(aws.ToString(identity.Arn), ":root") {
		t.Fatal("intended non-root account required")
	}
	db := dynamodb.NewFromConfig(cfg)
	st := NewStore(db, os.Getenv("KURIER_AWS_TEST_TABLE"), "dev-api")
	if !strings.HasPrefix(st.table, "kurier-dev-api-ControlTable-") {
		t.Fatal("scoped development table required")
	}
	configureAWSExecutionFixture(st, cfg)
	st.ConfigureProtected(os.Getenv("KURIER_AWS_TEST_PROTECTED"), NewEnvelopeCipher(kms.NewFromConfig(cfg), os.Getenv("KURIER_AWS_TEST_KEY"), "dev-api"))
	svc := st.executions
	parameter, err := ssm.NewFromConfig(cfg).GetParameter(ctx, &ssm.GetParameterInput{Name: aws.String("/kurier/dev-api/cursor-key"), WithDecryption: aws.Bool(true)})
	if err != nil || parameter.Parameter == nil {
		t.Fatal("signing root unavailable")
	}
	svc.SigningKey, err = base64.StdEncoding.DecodeString(aws.ToString(parameter.Parameter.Value))
	if err != nil {
		t.Fatal("signing root invalid")
	}
	defer clear(svc.SigningKey)
	who := Identity{issuer: "https://disposable-agent-sdk.invalid/" + newID(), subject: newID()}
	owner, err := st.ResolveUser(ctx, who)
	if err != nil {
		t.Fatal("fixture identity unavailable")
	}
	p, err := st.CreateProject(ctx, owner.UserID, "Disposable AWS local-agent fixture")
	if err != nil {
		t.Fatal("fixture project unavailable")
	}
	t.Logf("Disposable fixture project %s owner %s", p.ProjectID, owner.UserID)
	var credential string
	var agentID string
	t.Cleanup(func() {
		c, done := context.WithTimeout(context.Background(), 3*time.Minute)
		defer done()
		current, e := st.get(c, "P#"+p.ProjectID, "META")
		if e == nil && current.State == "active" {
			_, e = st.DeleteProject(c, owner.UserID, p.ProjectID, current.Version)
		}
		for n := 0; e == nil && n < 50; n++ {
			current, e = st.get(c, "P#"+p.ProjectID, "META")
			if e != nil || current.State == "deleted" {
				break
			}
			_, e = st.CleanupProject(c, p.ProjectID)
		}
		if e != nil || current.State != "deleted" {
			t.Error("fixture cleanup pending; retain public IDs for guarded resume")
			return
		}
		// Disposable credential records have no automatic TTL; remove only after
		// scheduled project drain proves no receipt/work/scope obligations survive.
		rows, e := db.Query(c, &dynamodb.QueryInput{TableName: &st.table, KeyConditionExpression: aws.String("PK = :pk"), ExpressionAttributeValues: map[string]dt.AttributeValue{":pk": s("U#" + owner.UserID)}, ConsistentRead: aws.Bool(true)})
		if e != nil || len(rows.LastEvaluatedKey) != 0 {
			t.Error("fixture identity inventory incomplete")
			return
		}
		for _, r := range rows.Items {
			if r["SK"].(*dt.AttributeValueMemberS).Value == "CRED#"+agentID {
				if r["state"].(*dt.AttributeValueMemberS).Value != "revoked" || r["activeJob"] != nil || r["activeProject"] != nil {
					t.Error("credential obligations remain")
					return
				}
			}
		}
		for _, r := range rows.Items {
			if _, e = db.DeleteItem(c, &dynamodb.DeleteItemInput{TableName: &st.table, Key: map[string]dt.AttributeValue{"PK": r["PK"], "SK": r["SK"]}}); e != nil {
				t.Error("disposable identity removal failed")
			}
		}
		hash := sha256.Sum256([]byte(credential))
		for _, k := range []map[string]dt.AttributeValue{key(identityKey(who), "META"), key("AGENTTOKEN#"+hex.EncodeToString(hash[:]), "META")} {
			if _, e = db.DeleteItem(c, &dynamodb.DeleteItemInput{TableName: &st.table, Key: k}); e != nil {
				t.Error("disposable lookup removal failed")
			}
			read, x := db.GetItem(c, &dynamodb.GetItemInput{TableName: &st.table, Key: k, ConsistentRead: aws.Bool(true)})
			if x != nil || len(read.Item) != 0 {
				t.Error("lookup cleanup readback failed")
			}
		}
		remaining, e := db.Query(c, &dynamodb.QueryInput{TableName: &st.table, KeyConditionExpression: aws.String("PK = :pk"), ExpressionAttributeValues: map[string]dt.AttributeValue{":pk": s("U#" + owner.UserID)}, ConsistentRead: aws.Bool(true)})
		if e != nil || len(remaining.Items) != 0 {
			t.Error("fixture owner cleanup readback failed")
		}
	})
	value := "Bearer " + newID() + "-disposable-local"
	var sends atomic.Int64
	var protectedDelivered atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sends.Add(1)
		protectedDelivered.Store(r.Header.Get("Authorization") == value)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Set-Cookie", "disposable-aws-local-cookie")
		// Valid literal JSON expands beyond 2 MiB after shared sanitization.
		_, _ = io.WriteString(w, `{"value":"`+strings.Repeat("<", 360*1024)+`","authorization":"`+value+`"}`)
	}))
	defer target.Close()
	conf := protectedConfig(value)
	conf.Name = "Controlled local expansion"
	conf.URL = target.URL
	saved, err := st.CreateRequest(ctx, owner.UserID, p.ProjectID, conf)
	if err != nil {
		t.Fatal("protected source creation unavailable")
	}
	pair, _, err := svc.CreateAgent(ctx, owner.UserID, newID(), execution.PairInput{Name: "AWS protocol fixture", Scopes: []execution.Scope{{ProjectID: p.ProjectID, Destinations: []execution.Destination{{Origin: target.URL, AddressClass: "loopback"}}}}})
	if err != nil || pair.Token == nil {
		t.Fatal("SDK pairing fixture unavailable")
	}
	credential = *pair.Token
	agentID = pair.Agent.LocalAgentID
	origin := strings.TrimSuffix(os.Getenv("KURIER_AWS_TEST_ENDPOINT"), "/execution-fixture/echo")
	if origin != "https://8dnymkcoa0.execute-api.us-east-2.amazonaws.com" {
		t.Fatal("scoped API required")
	}
	client := &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }, Transport: &http.Transport{Proxy: nil}}
	defer client.CloseIdleConnections()
	call := func(method, path, token string, body []byte, expected int) []byte {
		t.Helper()
		req, e := http.NewRequestWithContext(ctx, method, origin+path, bytes.NewReader(body))
		if e != nil {
			t.Fatal("HTTP construction failed")
		}
		req.Header.Set("Authorization", "Bearer "+token)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, e := client.Do(req)
		if e != nil {
			t.Fatal("HTTP outcome uncertain; no automatic work replay")
		}
		defer resp.Body.Close()
		raw, e := io.ReadAll(io.LimitReader(resp.Body, execution.EvidenceLimit+1))
		if e != nil || len(raw) > execution.EvidenceLimit || resp.StatusCode != expected {
			t.Fatalf("protocol response mismatch: status %d expected %d", resp.StatusCode, expected)
		}
		return raw
	}
	call("GET", "/api/v1/local-agent/session", "invalid", nil, 401)
	call("GET", "/api/v1/local-agent/session", credential, nil, 200)
	call("GET", "/api/v1/projects", credential, nil, 401)
	options, _ := execution.ParseOptions([]byte(`{}`))
	options.Target = "local"
	options.Local = &execution.LocalOptions{LocalAgentID: agentID, AllowPrivateNetwork: true}
	options.AllowInsecureSecrets = true
	view, err := svc.Submit(ctx, owner.UserID, p.ProjectID, saved.RequestID, newID(), saved.Revision, options, false)
	if err != nil {
		t.Fatal("local admission unavailable")
	}
	poll := "/api/v1/projects/" + p.ProjectID + "/local-agent/poll"
	var work *runner.Work
	for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); {
		raw := call("POST", poll, credential, nil, 200)
		var packet struct {
			Data runner.PollResponse `json:"data"`
		}
		if runner.StrictJSON(raw, &packet) != nil {
			t.Fatal("invalid poll response")
		}
		if packet.Data.Work != nil {
			work = packet.Data.Work
			break
		}
		time.Sleep(time.Second)
	}
	if work == nil {
		t.Fatal("work discovery timed out")
	}
	defer clear(work.ProtectedInputs.Values)
	if work.ProtectedInputs.Values[saved.Headers[0].SecretRef.SecretID] != value {
		t.Fatal("frozen protected input not delivered")
	}
	prepared, err := runner.PrepareLocal(ctx, work.Plan, work.ProtectedInputs.Values, work.Destination)
	if err != nil {
		t.Fatal("local preparation failed")
	}
	evidence := runner.NewEvidence(p.ProjectID, view.ExecutionID, work.DispatchIntentAt, work.Plan)
	prepared.Run(ctx, evidence)
	if evidence.Status != "completed" || evidence.Response.Body.Text == nil || len(*evidence.Response.Body.Text) <= runner.BodyLimit || sends.Load() != 1 || !protectedDelivered.Load() {
		t.Fatal("single protected target capture failed")
	}
	result, err := runner.ResultFromEvidence(*work, evidence)
	if err != nil {
		t.Fatal("result construction failed")
	}
	raw, _ := json.Marshal(result)
	defer clear(raw)
	path := "/api/v1/projects/" + p.ProjectID + "/local-agent/jobs/" + work.JobID + "/result"
	ack := call("POST", path, credential, raw, 200)
	if repeat := call("POST", path, credential, raw, 200); !bytes.Equal(ack, repeat) || sends.Load() != 1 {
		t.Fatal("receipt retry mismatch or target replay")
	}
	stored, err := svc.Evidence(ctx, owner.UserID, p.ProjectID, view.ExecutionID)
	if err != nil || bytes.Contains(stored, []byte(value)) || bytes.Contains(stored, []byte("disposable-aws-local-cookie")) {
		t.Fatal("published evidence unavailable or unsafe")
	}
	var publication struct {
		Data struct {
			Evidence runner.Evidence `json:"evidence"`
		} `json:"data"`
	}
	if runner.StrictJSON(stored, &publication) != nil || publication.Data.Evidence.Response.Body.Text == nil || *publication.Data.Evidence.Response.Body.Text != *evidence.Response.Body.Text {
		t.Fatal("expanded evidence not published intact")
	}
	call("DELETE", "/api/v1/local-agent/session", credential, nil, 204)
	call("GET", "/api/v1/local-agent/session", credential, nil, 401)
	call("POST", poll, credential, nil, 401)
	call("POST", path, credential, raw, 401)
	t.Log("Real deployed agent auth/isolation, KMS protected delivery, expanded result/S3 publication, identical ACK and revoked session/poll/upload passed; one target HTTP send")
	current, err := st.get(ctx, "P#"+p.ProjectID, "META")
	if err != nil {
		t.Fatal("fixture read unavailable")
	}
	op, err := st.DeleteProject(ctx, owner.UserID, p.ProjectID, current.Version)
	if err != nil {
		t.Fatal("deletion admission unavailable")
	}
	for deadline := time.Now().Add(8 * time.Minute); time.Now().Before(deadline); {
		op, err = st.GetDeletion(ctx, owner.UserID, p.ProjectID, op.OperationID)
		if err == nil && op.State == "completed" {
			break
		}
		time.Sleep(3 * time.Second)
	}
	if err != nil || op.State != "completed" {
		t.Fatal("scheduled local project cleanup pending")
	}
	for _, table := range []string{st.table, svc.Protected} {
		rows, e := db.Query(ctx, &dynamodb.QueryInput{TableName: &table, KeyConditionExpression: aws.String("PK = :pk"), ExpressionAttributeValues: map[string]dt.AttributeValue{":pk": s("P#" + p.ProjectID)}, ConsistentRead: aws.Bool(true)})
		expected := 0
		if table == st.table {
			expected = 1
		}
		if e != nil || len(rows.Items) != expected || len(rows.LastEvaluatedKey) != 0 {
			t.Fatal("disposable partition cleanup incomplete")
		}
	}
	objects, err := s3.NewFromConfig(cfg).ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: &svc.Bucket, Prefix: aws.String("dev-api/projects/" + p.ProjectID + "/")})
	if err != nil || aws.ToBool(objects.IsTruncated) || len(objects.Contents) != 0 {
		t.Fatal("disposable evidence object cleanup incomplete")
	}
	t.Log("Actual scheduled project cleanup drained local job/receipt/protected binding/scope and S3 evidence; partitions verified")
}
