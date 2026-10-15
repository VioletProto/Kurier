//go:build integration && aws

package ownership

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/VioletProto/Kurier/services/api/execution"
	runner "github.com/VioletProto/Kurier/services/request-runtime"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dt "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// Only this SDK-side notifier is fault-injected. No real target dispatch,
// encryption/storage operation or deployed successor worker is mocked.
type workflowLostNotification struct{ calls int }

func (q *workflowLostNotification) SendMessage(context.Context, *sqs.SendMessageInput, ...func(*sqs.Options)) (*sqs.SendMessageOutput, error) {
	q.calls++
	return nil, errors.New("injected SDK notification loss")
}

// Refresh the CLI login in child memory for long scheduled observers. Static
// environment exports can expire while the CLI login itself remains renewable.
func workflowAWSConfig(ctx context.Context) (aws.Config, error) {
	provider := aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
		cmd := exec.CommandContext(ctx, "aws", "configure", "export-credentials", "--format", "process")
		for _, entry := range os.Environ() {
			name, _, _ := strings.Cut(entry, "=")
			if name != "AWS_ACCESS_KEY_ID" && name != "AWS_SECRET_ACCESS_KEY" && name != "AWS_SESSION_TOKEN" {
				cmd.Env = append(cmd.Env, entry)
			}
		}
		raw, e := cmd.Output()
		defer clear(raw)
		if e != nil {
			return aws.Credentials{}, errors.New("renewable AWS CLI session unavailable")
		}
		var v struct {
			AccessKeyID     string `json:"AccessKeyId"`
			SecretAccessKey string
			SessionToken    string
			Expiration      string
		}
		if json.Unmarshal(raw, &v) != nil || v.AccessKeyID == "" || v.SecretAccessKey == "" {
			return aws.Credentials{}, errors.New("AWS CLI credential metadata invalid")
		}
		c := aws.Credentials{AccessKeyID: v.AccessKeyID, SecretAccessKey: v.SecretAccessKey, SessionToken: v.SessionToken, Source: "memory-only AWS CLI login"}
		if v.Expiration != "" {
			var e error
			c.Expires, e = time.Parse(time.RFC3339, v.Expiration)
			if e != nil {
				return aws.Credentials{}, errors.New("AWS CLI expiry invalid")
			}
			c.CanExpire = true
		}
		return c, nil
	})
	return config.LoadDefaultConfig(ctx, config.WithRegion("us-east-2"), config.WithCredentialsProvider(aws.NewCredentialsCache(provider, func(o *aws.CredentialsCacheOptions) { o.ExpiryWindow = time.Minute })), config.WithRetryer(func() aws.Retryer { return aws.NopRetryer{} }))
}

// Admission uses a disposable SDK identity. Runtime/crypto/publication/cleanup
// use real AWS and deployed consumers. This does not prove Cognito/API admission.
func TestAWSSequentialWorkflows(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Minute)
	defer cancel()
	cfg, e := workflowAWSConfig(ctx)
	if e != nil {
		t.Fatal("AWS configuration unavailable")
	}
	who, e := sts.NewFromConfig(cfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if e != nil || aws.ToString(who.Account) != "747336059622" || strings.HasSuffix(aws.ToString(who.Arn), ":root") {
		t.Fatal("intended non-root identity required")
	}
	db := dynamodb.NewFromConfig(cfg)
	st := NewStore(db, os.Getenv("KURIER_AWS_TEST_TABLE"), "dev-api")
	configureAWSExecutionFixture(st, cfg)
	st.ConfigureProtected(os.Getenv("KURIER_AWS_TEST_PROTECTED"), nil)
	svc := st.executions
	endpoint := os.Getenv("KURIER_AWS_TEST_ENDPOINT")
	if !strings.HasPrefix(st.table, "kurier-dev-api-ControlTable-") || !strings.HasPrefix(svc.Protected, "kurier-dev-api-ProtectedTable-") || endpoint != "https://8dnymkcoa0.execute-api.us-east-2.amazonaws.com/execution-fixture/echo" {
		t.Fatal("scoped outputs required")
	}
	param, e := ssm.NewFromConfig(cfg).GetParameter(ctx, &ssm.GetParameterInput{Name: aws.String("/kurier/dev-api/cursor-key"), WithDecryption: aws.Bool(true)})
	if e != nil {
		t.Fatal("signing key unavailable")
	}
	svc.SigningKey, e = base64.StdEncoding.DecodeString(aws.ToString(param.Parameter.Value))
	if e != nil {
		t.Fatal("signing key invalid")
	}
	defer clear(svc.SigningKey)
	read := func(table, pk, sk string) map[string]dt.AttributeValue {
		t.Helper()
		r, e := db.GetItem(ctx, &dynamodb.GetItemInput{TableName: &table, Key: key(pk, sk), ConsistentRead: aws.Bool(true)})
		if e != nil {
			t.Fatal("strong read unavailable")
		}
		return r.Item
	}
	number := func(r map[string]dt.AttributeValue, k string) int64 {
		if x, ok := r[k].(*dt.AttributeValueMemberN); ok {
			n, e := strconv.ParseInt(x.Value, 10, 64)
			if e == nil {
				return n
			}
		}
		return 0
	}
	textValue := func(r map[string]dt.AttributeValue, k string) string {
		if x, ok := r[k].(*dt.AttributeValueMemberS); ok {
			return x.Value
		}
		return ""
	}
	stageBefore := read(st.table, "STAGE#dev-api", "META")
	quotaBefore := read(st.table, "STAGE#dev-api", "QUOTA")
	ident := Identity{issuer: "https://disposable-workflow-sdk.invalid/" + newID(), subject: newID()}
	owner, e := st.ResolveUser(ctx, ident)
	if e != nil {
		t.Fatal("fixture identity unavailable")
	}
	p, e := st.CreateProject(ctx, owner.UserID, "Disposable AWS sequential workflow fixture")
	if e != nil {
		t.Fatal("fixture project unavailable")
	}
	t.Logf("Disposable SDK fixture project=%s owner=%s", p.ProjectID, owner.UserID)
	// Cleanup is scheduled-only. On failure preserve IDs and obligations for
	// inspection; never hide a schedule failure by directly draining the project.
	cleanupVerified := false
	t.Cleanup(func() {
		if !cleanupVerified {
			t.Errorf("fixture requires guarded inspection: project=%s owner=%s", p.ProjectID, owner.UserID)
		}
	})
	start := time.Now()
	nonces := []string{}
	tokenValues := []string{}
	runs := []execution.WorkflowRun{}
	makeDefinition := func(mode string, bindings bool) (execution.Workflow, string) {
		t.Helper()
		nonce := newID()
		nonces = append(nonces, nonce)
		sum := sha256.Sum256([]byte("kurier-workflow-fixture-v1/" + nonce))
		tokenValues = append(tokenValues, hex.EncodeToString(sum[:]))
		create := func(name, mode string, header bool) SavedRequest {
			t.Helper()
			headers := []RequestField{}
			if header {
				headers = append(headers, RequestField{Name: "Authorization", Value: "", Enabled: true})
			}
			r, e := st.CreateRequest(ctx, owner.UserID, p.ProjectID, RequestConfiguration{Name: name, Method: "GET", URL: strings.TrimSuffix(endpoint, "echo") + mode + "?fixture=" + nonce, Headers: headers, QueryParameters: []RequestField{}})
			if e != nil {
				t.Fatal("fixture request creation failed")
			}
			return r
		}
		first := create("Workflow producer "+mode, mode, false)
		secondMode := "workflow-consume"
		if !bindings {
			secondMode = "echo"
		}
		second := create("Workflow consumer "+mode, secondMode, bindings)
		a, b := newID(), newID()
		steps := []runner.WorkflowStep{{StepID: a, RequestID: first.RequestID, RequestRevision: first.Revision, Extract: []runner.Extraction{}, Bindings: []runner.Binding{}}, {StepID: b, RequestID: second.RequestID, RequestRevision: second.Revision, Extract: []runner.Extraction{}, Bindings: []runner.Binding{}}}
		if bindings {
			index := 0
			prefix := "Bearer "
			steps[0].Extract = []runner.Extraction{{Name: "token", Path: "$.token", Type: "string"}}
			steps[1].Bindings = []runner.Binding{{From: runner.Producer{StepID: a, Name: "token"}, To: runner.BindingTarget{Kind: "header", Index: &index, Prefix: &prefix}}}
		}
		wf, e := svc.SaveWorkflow(ctx, owner.UserID, p.ProjectID, "", 0, execution.WorkflowDefinition{Name: "AWS " + mode, Steps: steps}, true)
		if e != nil {
			t.Fatal("workflow authoring failed")
		}
		return wf, nonce
	}
	// Reconcile uncertain admission with the exact original key. Workers may
	// advance while the postcommit RunDetail hydration performs optimistic CAS.
	admit := func(source string, revision int64, rerun bool) execution.WorkflowRun {
		t.Helper()
		key := newID()
		for until := time.Now().Add(time.Minute); time.Now().Before(until); {
			r, e := svc.SubmitWorkflow(ctx, owner.UserID, p.ProjectID, source, key, revision, execution.RunOptions{Target: "cloud"}, rerun)
			if e == nil {
				return r
			}
			time.Sleep(300 * time.Millisecond)
		}
		t.Fatal("admission reconciliation pending; never issue a fresh key")
		return execution.WorkflowRun{}
	}
	submit := func(w execution.Workflow) execution.WorkflowRun {
		t.Helper()
		r := admit(w.WorkflowID, w.Revision, false)
		runs = append(runs, r)
		return r
	}
	wait := func(r execution.WorkflowRun, stateWanted string) execution.WorkflowRun {
		t.Helper()
		for until := time.Now().Add(4 * time.Minute); time.Now().Before(until); {
			// Poll with ordinary strong reads; transactional RunDetail hydration can
			// contend with the live writer. Only hydrate the immutable terminal result.
			state := textValue(read(st.table, "P#"+p.ProjectID, "RUN#"+r.WorkflowRunID), "state")
			if state == "completed" || state == "failed" {
				v, e := svc.RunDetail(ctx, owner.UserID, p.ProjectID, r.WorkflowRunID)
				if e == nil {
					if v.State != state || v.State != stateWanted {
						t.Fatalf("unexpected run state=%s run=%s", v.State, v.WorkflowRunID)
					}
					return v
				}
			}
			time.Sleep(time.Second)
		}
		t.Fatal("deployed workflow did not terminalize")
		return r
	}
	noCiphertext := func(r execution.WorkflowRun) {
		t.Helper()
		q, e := db.Query(ctx, &dynamodb.QueryInput{TableName: &svc.Protected, ConsistentRead: aws.Bool(true), KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK,:prefix)"), ExpressionAttributeValues: map[string]dt.AttributeValue{":pk": s("P#" + p.ProjectID), ":prefix": s("RUN#" + r.WorkflowRunID)}})
		if e != nil || len(q.Items) != 0 || len(q.LastEvaluatedKey) != 0 {
			t.Fatal("terminal runtime ciphertext remains")
		}
	}
	checkEvidence := func(r execution.WorkflowRun) {
		t.Helper()
		for _, step := range r.Steps {
			if step.ExecutionID == nil {
				continue
			}
			raw, e := svc.Evidence(ctx, owner.UserID, p.ProjectID, *step.ExecutionID)
			if e != nil {
				t.Fatal("evidence delivery failed")
			}
			for _, v := range tokenValues {
				if bytes.Contains(raw, []byte(v)) {
					t.Fatal("raw/rendered runtime value escaped evidence containment")
				}
			}
		}
	}
	originalWF, successNonce := makeDefinition("workflow-token", true)
	original := wait(submit(originalWF), "completed")
	checkEvidence(original)
	noCiphertext(original)
	snapshot := read(st.table, "P#"+p.ProjectID, "EXEC#"+*original.Steps[1].ExecutionID+"#SNAP")
	objectKey := textValue(snapshot, "objectKey")
	objects := s3.NewFromConfig(cfg)
	capture := func() []byte {
		t.Helper()
		o, e := objects.GetObject(ctx, &s3.GetObjectInput{Bucket: &svc.Bucket, Key: &objectKey})
		if e != nil {
			t.Fatal("immutable object unavailable")
		}
		defer o.Body.Close()
		b, e := io.ReadAll(io.LimitReader(o.Body, execution.EvidenceLimit+1))
		h := sha256.Sum256(b)
		if e != nil || hex.EncodeToString(h[:]) != textValue(snapshot, "checksum") {
			t.Fatal("immutable checksum mismatch")
		}
		return b
	}
	immutable := capture()
	// Real duplicate SQS notifications of already finalized steps, no HTTP retry.
	queue := sqs.NewFromConfig(cfg)
	for _, step := range original.Steps {
		anchor := read(st.table, "P#"+p.ProjectID, "EXEC#"+*step.ExecutionID)
		job := textValue(anchor, "jobId")
		out := read(st.table, "P#"+p.ProjectID, "OUT#"+job)
		if textValue(out, "runId") != original.WorkflowRunID || textValue(out, "jobId") != job {
			t.Fatal("durable successor/outbox missing")
		}
		message, _ := json.Marshal(execution.Message{ProjectID: p.ProjectID, JobID: job, OutboxID: "OUT#" + job})
		for i := 0; i < 2; i++ {
			if _, e := queue.SendMessage(ctx, &sqs.SendMessageInput{QueueUrl: &svc.QueueURL, MessageBody: aws.String(string(message))}); e != nil {
				t.Fatal("duplicate notification failed")
			}
		}
	}
	if e = svc.DeleteWorkflow(ctx, owner.UserID, p.ProjectID, originalWF.WorkflowID, originalWF.Revision); e != nil {
		t.Fatal("definition deletion failed")
	}
	rerun := admit(original.WorkflowRunID, 0, true)
	runs = append(runs, rerun)
	rerun = wait(rerun, "completed")
	if rerun.LiveWorkflowID != nil {
		t.Fatal("deleted definition appeared live")
	}
	checkEvidence(rerun)
	noCiphertext(rerun)
	failureWF, failureNonce := makeDefinition("workflow-failure", false)
	failed := wait(submit(failureWF), "failed")
	if failed.Steps[1].State != "skipped" || failed.Steps[0].Summary.Outcome.Certainty != "known" {
		t.Fatal("HTTP failure did not stop")
	}
	noCiphertext(failed)
	unknownWF, unknownNonce := makeDefinition("workflow-delay", false)
	unknown := wait(submit(unknownWF), "failed")
	if unknown.Steps[1].State != "skipped" || unknown.Steps[0].Summary.Outcome.Certainty != "unknown" {
		t.Fatal("timeout did not stop with unknown certainty")
	}
	noCiphertext(unknown)
	// Ask the real SQS worker to redeliver the unknown job. It must not send HTTP.
	anchor := read(st.table, "P#"+p.ProjectID, "EXEC#"+*unknown.Steps[0].ExecutionID)
	job := textValue(anchor, "jobId")
	msg, _ := json.Marshal(execution.Message{ProjectID: p.ProjectID, JobID: job, OutboxID: "OUT#" + job})
	if _, e = queue.SendMessage(ctx, &sqs.SendMessageInput{QueueUrl: &svc.QueueURL, MessageBody: aws.String(string(msg))}); e != nil {
		t.Fatal("unknown duplicate notification failed")
	}
	// The real scheduled consumer must settle the persisted successor outbox.
	successor := read(st.table, "P#"+p.ProjectID, "EXEC#"+*original.Steps[1].ExecutionID)
	outKey := "OUT#" + textValue(successor, "jobId")
	for until := time.Now().Add(3 * time.Minute); time.Now().Before(until); {
		if textValue(read(st.table, "P#"+p.ProjectID, outKey), "state") == "published" {
			break
		}
		time.Sleep(time.Second)
	}
	if textValue(read(st.table, "P#"+p.ProjectID, outKey), "state") != "published" {
		t.Fatal("scheduled durable successor settlement pending")
	}
	if !bytes.Equal(immutable, capture()) {
		t.Fatal("duplicate changed immutable evidence")
	}
	t.Log("Real KMS/DDB/SQS/Lambda/HTTPS/S3: raw extraction/Bearer success, terminal ciphertext removal, immutable redaction, durable successor and deleted-definition whole-run rerun passed")
	// Exercise durable successor wakeup with SDK-only notification suppression.
	// The first HTTP/crypto/finalization is real but SDK-driven; the second HTTP
	// must come from the deployed worker after real scheduled OUT delivery.
	lost := &workflowLostNotification{}
	svc.Queue = lost
	durable := admit(original.WorkflowRunID, 0, true)
	first := read(st.table, "P#"+p.ProjectID, "EXEC#"+*durable.Steps[0].ExecutionID)
	firstJob := textValue(first, "jobId")
	e = svc.Process(ctx, execution.Message{ProjectID: p.ProjectID, JobID: firstJob, OutboxID: "OUT#" + firstJob})
	svc.Queue = queue
	if e != nil || lost.calls != 2 {
		t.Fatal("SDK notification-loss injection did not run admission/finalization exactly once")
	}
	durable = wait(durable, "completed")
	checkEvidence(durable)
	noCiphertext(durable)
	t.Log("Injected SDK first-step fast-notification loss; real durable successor OUT, scheduled delivery and deployed second-step worker completed without manual next-job delivery")

	// Age only disposable RET/run timestamps under stage/project/version guards.
	// This is fixture-controlled time, not thirty elapsed wall-clock days.
	pinned := *rerun.Steps[0].ExecutionID
	if _, e = svc.Pin(ctx, owner.UserID, p.ProjectID, pinned, true); e != nil {
		t.Fatal("workflow step pin failed")
	}
	beforePin := read(st.table, "STAGE#dev-api", "QUOTA")
	if number(beforePin, "pinnedCount") != number(quotaBefore, "pinnedCount")+1 {
		t.Fatal("pin accounting mismatch")
	}
	expired := timestamp(time.Now().Add(-time.Minute))
	age := func(sk string, ret bool) {
		t.Helper()
		r := read(st.table, "P#"+p.ProjectID, sk)
		stage, e := st.activeStage(ctx)
		if e != nil {
			t.Fatal("stage unavailable")
		}
		project, e := st.get(ctx, "P#"+p.ProjectID, "META")
		if e != nil {
			t.Fatal("project unavailable")
		}
		expr := "SET normalExpiresAt = :expiry, #v = #v + :one"
		values := map[string]dt.AttributeValue{":expiry": s(expired), ":one": n(1), ":v": r["version"]}
		// Keep the existing shard and exact suffix, advancing only due time.
		if dsk := textValue(r, "DSK"); dsk != "" {
			expr += ", DSK = :due"
			values[":due"] = s(expired + dsk[strings.Index(dsk, "#"):])
		}
		_, e = db.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{TransactItems: []dt.TransactWriteItem{st.stageGuard(stage), {ConditionCheck: &dt.ConditionCheck{TableName: &st.table, Key: key(project.PK, "META"), ConditionExpression: aws.String("#v = :v AND #s = :active AND ownerId = :owner"), ExpressionAttributeNames: map[string]string{"#v": "version", "#s": "state"}, ExpressionAttributeValues: map[string]dt.AttributeValue{":v": n(project.Version), ":active": s("active"), ":owner": s(owner.UserID)}}}, {Update: &dt.Update{TableName: &st.table, Key: key(project.PK, sk), UpdateExpression: &expr, ConditionExpression: aws.String("#v = :v"), ExpressionAttributeNames: map[string]string{"#v": "version"}, ExpressionAttributeValues: values}}}})
		if e != nil {
			t.Fatal("guarded fixture expiry mutation failed")
		}
	}
	age("RUN#"+rerun.WorkflowRunID, false)
	for _, step := range rerun.Steps {
		age("EXEC#"+*step.ExecutionID+"#RET", true)
	}
	for until := time.Now().Add(4 * time.Minute); time.Now().Before(until); {
		step := read(st.table, "P#"+p.ProjectID, "RUN#"+rerun.WorkflowRunID+"#STEP#01")
		expiredFlag, _ := step["evidenceExpired"].(*dt.AttributeValueMemberBOOL)
		if expiredFlag != nil && expiredFlag.Value {
			v, e := svc.RunDetail(ctx, owner.UserID, p.ProjectID, rerun.WorkflowRunID)
			if e != nil {
				time.Sleep(time.Second)
				continue
			}
			rerun = v
			break
		}
		time.Sleep(time.Second)
	}
	if !rerun.Steps[1].EvidenceExpired {
		t.Fatal("scheduled sibling expiry pending")
	}
	if _, e = svc.Evidence(ctx, owner.UserID, p.ProjectID, pinned); e != nil {
		t.Fatal("pin lost evidence/context during scheduled expiry")
	}
	if _, e = svc.Evidence(ctx, owner.UserID, p.ProjectID, *rerun.Steps[1].ExecutionID); e == nil {
		t.Fatal("expired sibling evidence readable")
	}
	noCiphertext(rerun)
	t.Log("Real scheduled sibling expiry preserved pinned evidence and retained frozen run context; timestamps were deliberately aged")
	// Audit raw logs privately; never use a secret as a filter or print a payload.
	counts := map[string]int{}
	for until := time.Now().Add(2 * time.Minute); time.Now().Before(until); {
		counts = map[string]int{}
		for _, name := range strings.Split(os.Getenv("KURIER_AWS_TEST_LOG_FUNCTIONS"), ",") {
			b, e := exec.CommandContext(ctx, "aws", "lambda", "get-function-configuration", "--function-name", name, "--region", "us-east-2", "--query", "LoggingConfig.LogGroup", "--output", "json").Output()
			var group string
			if e != nil || json.Unmarshal(b, &group) != nil || !strings.HasPrefix(group, "/aws/lambda/kurier-dev-api-") {
				t.Fatal("scoped log lookup failed")
			}
			b, e = exec.CommandContext(ctx, "aws", "logs", "filter-log-events", "--log-group-name", group, "--region", "us-east-2", "--start-time", strconv.FormatInt(start.Add(-time.Minute).UnixMilli(), 10), "--output", "json").Output()
			if e != nil || len(b) > 16*1024*1024 {
				t.Fatal("bounded log audit unavailable")
			}
			for _, v := range tokenValues {
				if bytes.Contains(b, []byte(v)) {
					t.Fatal("secret escaped log containment")
				}
			}
			var events struct {
				Events    []struct{ Message string }
				NextToken string
			}
			if json.Unmarshal(b, &events) != nil || events.NextToken != "" {
				t.Fatal("incomplete log audit")
			}
			for _, event := range events.Events {
				i := strings.Index(event.Message, `{"authorized"`)
				if i < 0 {
					continue
				}
				var record struct {
					FixtureID  string `json:"fixtureId"`
					Mode       string `json:"mode"`
					Authorized bool   `json:"authorized"`
				}
				if json.Unmarshal([]byte(strings.TrimSpace(event.Message[i:])), &record) != nil {
					continue
				}
				for _, nonce := range nonces {
					if record.FixtureID == nonce {
						counts[nonce+"/"+record.Mode]++
						if record.Mode == "workflow-consume" && !record.Authorized {
							t.Fatal("target rejected Bearer binding")
						}
					}
				}
			}
			clear(b)
		}
		if counts[successNonce+"/workflow-token"] >= 3 && counts[successNonce+"/workflow-consume"] >= 3 && counts[failureNonce+"/workflow-failure"] >= 1 && counts[unknownNonce+"/workflow-delay"] >= 1 {
			break
		}
		time.Sleep(3 * time.Second)
	}
	if counts[successNonce+"/workflow-token"] != 3 || counts[successNonce+"/workflow-consume"] != 3 || counts[failureNonce+"/workflow-failure"] != 1 || counts[unknownNonce+"/workflow-delay"] != 1 || len(counts) != 4 {
		t.Fatalf("controlled target counts mismatch: token=%d consumer=%d failure=%d unknown=%d", counts[successNonce+"/workflow-token"], counts[successNonce+"/workflow-consume"], counts[failureNonce+"/workflow-failure"], counts[unknownNonce+"/workflow-delay"])
	}
	q := read(st.table, "STAGE#dev-api", "QUOTA")
	project, e := st.get(ctx, "P#"+p.ProjectID, "META")
	if e != nil || project.InflightCount != 0 || number(q, "count") != number(quotaBefore, "count")+10 {
		t.Fatal("N-step or inflight accounting mismatch")
	}
	t.Log("Controlled target counts: token=3 consumer=3 failure=1 timeout=1; duplicate finalized/unknown SQS deliveries caused no HTTP replay; ten admitted step slots, no skipped-step refund")
	op, e := st.DeleteProject(ctx, owner.UserID, p.ProjectID, project.Version)
	if e != nil {
		t.Fatal("project deletion acceptance failed")
	}
	if _, e = svc.RunDetail(ctx, owner.UserID, p.ProjectID, original.WorkflowRunID); e == nil {
		t.Fatal("deleting project exposed run")
	}
	for until := time.Now().Add(20 * time.Minute); time.Now().Before(until); {
		v, e := st.GetDeletion(ctx, owner.UserID, p.ProjectID, op.OperationID)
		if e == nil && v.State == "completed" {
			op = v
			break
		}
		time.Sleep(2 * time.Second)
	}
	if op.State != "completed" {
		t.Fatal("scheduled project cleanup pending")
	}
	for _, table := range []string{st.table, svc.Protected} {
		page, e := db.Query(ctx, &dynamodb.QueryInput{TableName: &table, ConsistentRead: aws.Bool(true), KeyConditionExpression: aws.String("PK = :pk"), ExpressionAttributeValues: map[string]dt.AttributeValue{":pk": s("P#" + p.ProjectID)}})
		want := 0
		if table == st.table {
			want = 1
		}
		if e != nil || len(page.LastEvaluatedKey) != 0 || len(page.Items) != want {
			t.Fatal("fixture partition cleanup incomplete")
		}
		if want == 1 && textValue(page.Items[0], "kind") != "projectTombstone" {
			t.Fatal("fixture tombstone proof missing")
		}
	}
	listing, e := objects.ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: &svc.Bucket, Prefix: aws.String("dev-api/projects/" + p.ProjectID + "/")})
	if e != nil || aws.ToBool(listing.IsTruncated) || len(listing.Contents) != 0 {
		t.Fatal("fixture S3 cleanup incomplete")
	}
	finalQuota := read(st.table, "STAGE#dev-api", "QUOTA")
	for _, k := range []string{"pinnedCount", "pinnedBytes"} {
		if number(finalQuota, k) != number(quotaBefore, k) {
			t.Fatal("cleanup classification/storage accounting mismatch")
		}
	}
	if number(finalQuota, "count") != number(quotaBefore, "count")+10 {
		t.Fatal("cleanup refunded admission accounting")
	}
	// Reconcile storage against a stable global quota version, allowing the
	// separately identified interrupted disposable project to drain concurrently.
	storageVerified := false
	for attempt := 0; attempt < 5; attempt++ {
		before := read(st.table, "STAGE#dev-api", "QUOTA")
		var cursor map[string]dt.AttributeValue
		var uploads, projects int64
		for page := 0; page < 1000; page++ {
			r, e := db.Scan(ctx, &dynamodb.ScanInput{TableName: &st.table, ConsistentRead: aws.Bool(true), ExclusiveStartKey: cursor, ProjectionExpression: aws.String("#k,#b,storageReservedBytes"), ExpressionAttributeNames: map[string]string{"#k": "kind", "#b": "bytes"}})
			if e != nil {
				t.Fatal("storage inventory failed")
			}
			for _, item := range r.Items {
				switch textValue(item, "kind") {
				case "executionUpload":
					uploads += number(item, "bytes")
				case "project":
					projects += number(item, "storageReservedBytes")
				}
			}
			cursor = r.LastEvaluatedKey
			if len(cursor) == 0 {
				break
			}
			if page == 999 {
				t.Fatal("storage inventory incomplete")
			}
		}
		after := read(st.table, "STAGE#dev-api", "QUOTA")
		if number(before, "version") != number(after, "version") {
			continue
		}
		if number(after, "storageReservedBytes") != uploads || uploads != projects {
			t.Fatal("stable stage/project/upload storage reconciliation mismatch")
		}
		storageVerified = true
		break
	}
	if !storageVerified {
		t.Fatal("storage reconciliation remained concurrent")
	}
	stageAfter := read(st.table, "STAGE#dev-api", "META")
	beforeJSON, _ := json.Marshal(stageBefore)
	afterJSON, _ := json.Marshal(stageAfter)
	if !bytes.Equal(beforeJSON, afterJSON) {
		t.Fatal("validation changed capability/generation marker")
	}
	for _, k := range []map[string]dt.AttributeValue{key("U#"+owner.UserID, "META"), key(identityKey(ident), "META")} {
		if _, e = db.DeleteItem(ctx, &dynamodb.DeleteItemInput{TableName: &st.table, Key: k}); e != nil {
			t.Fatal("disposable identity removal pending")
		}
	}
	cleanupVerified = true
	t.Logf("Scheduled deletion operation=%s completed; project tombstone only, Protected/S3 empty, identity META/mapping removed; numeric quota usage retained; generation/capabilities/accounting preserved", op.OperationID)
}

// Explicit cleanup/audit of the first interrupted SDK validation fixture.
// Never submits a request, changes a run, or directly invokes cleanup.
func TestAWSSequentialWorkflowCleanup(t *testing.T) {
	project := os.Getenv("KURIER_AWS_WORKFLOW_CLEANUP_PROJECT")
	owner := os.Getenv("KURIER_AWS_WORKFLOW_CLEANUP_OWNER")
	if !((project == "d9c8b5ca-6341-4074-88b2-c1eca4d59567" && owner == "39cc13b4-648b-4cd1-9420-65dc53c48ac1") || (project == "4e72644b-f6ba-4717-b5a3-d36e02a8768c" && owner == "ac8ce164-78d0-4252-a648-19944cf68115") || (project == "c55b7542-3d30-47b1-bb6d-35aec5ee9dfb" && owner == "37714cd7-3a91-4ed5-ae10-32785d5bebc6")) {
		t.Skip("explicit interrupted disposable fixture only")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel()
	cfg, e := workflowAWSConfig(ctx)
	if e != nil {
		t.Fatal("AWS configuration unavailable")
	}
	identity, e := sts.NewFromConfig(cfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if e != nil || aws.ToString(identity.Account) != "747336059622" || strings.HasSuffix(aws.ToString(identity.Arn), ":root") {
		t.Fatal("intended identity required")
	}
	db := dynamodb.NewFromConfig(cfg)
	st := NewStore(db, os.Getenv("KURIER_AWS_TEST_TABLE"), "dev-api")
	st.ConfigureProtected(os.Getenv("KURIER_AWS_TEST_PROTECTED"), nil)
	configureAWSExecutionFixture(st, cfg)
	u, e := st.userRecord(ctx, owner)
	if e != nil || !strings.HasPrefix(u.Issuer, "https://disposable-workflow-sdk.invalid/") {
		t.Fatal("disposable owner guard failed")
	}
	svc := st.executions
	param, e := ssm.NewFromConfig(cfg).GetParameter(ctx, &ssm.GetParameterInput{Name: aws.String("/kurier/dev-api/cursor-key"), WithDecryption: aws.Bool(true)})
	if e != nil {
		t.Fatal("signing root unavailable")
	}
	svc.SigningKey, e = base64.StdEncoding.DecodeString(aws.ToString(param.Parameter.Value))
	if e != nil {
		t.Fatal("signing root invalid")
	}
	defer clear(svc.SigningKey)
	p, e := st.get(ctx, "P#"+project, "META")
	if e != nil {
		t.Fatal("disposable project unavailable")
	}
	if p.State == "active" && project == "d9c8b5ca-6341-4074-88b2-c1eca4d59567" {
		for _, id := range []string{"e7a34fd2-176a-4170-98e9-600e55de78c6", "a77dfffe-fa6a-445e-9e04-e48f6229cd6d"} {
			r, e := svc.RunDetail(ctx, owner, project, id)
			if e != nil || r.State != "completed" {
				t.Fatal("durable admitted run inspection failed")
			}
			if id == "a77dfffe-fa6a-445e-9e04-e48f6229cd6d" && r.LiveWorkflowID != nil {
				t.Fatal("rerun still has live definition")
			}
			for _, step := range r.Steps {
				if _, e = svc.Evidence(ctx, owner, project, *step.ExecutionID); e != nil {
					t.Fatal("completed immutable evidence unavailable")
				}
			}
		}
	}
	p, e = st.get(ctx, "P#"+project, "META")
	if e != nil {
		t.Fatal("interrupted fixture unavailable")
	}
	q, e := db.Query(ctx, &dynamodb.QueryInput{TableName: &svc.Protected, ConsistentRead: aws.Bool(true), KeyConditionExpression: aws.String("PK = :pk"), ExpressionAttributeValues: map[string]dt.AttributeValue{":pk": s(p.PK)}})
	if e != nil || len(q.LastEvaluatedKey) != 0 {
		t.Fatal("ciphertext inventory unavailable")
	}
	var op DeletionOperation
	if p.State != "active" && p.Operation != nil {
		op = *p.Operation
	} else {
		op, e = st.DeleteProject(ctx, owner, project, p.Version)
		if e != nil {
			t.Fatal("deletion acceptance failed")
		}
	}
	for until := time.Now().Add(20 * time.Minute); time.Now().Before(until); {
		v, e := st.GetDeletion(ctx, owner, project, op.OperationID)
		if e == nil && v.State == "completed" {
			op = v
			break
		}
		time.Sleep(2 * time.Second)
	}
	if op.State != "completed" {
		t.Fatal("scheduled cleanup pending")
	}
	for _, table := range []string{st.table, svc.Protected} {
		r, e := db.Query(ctx, &dynamodb.QueryInput{TableName: &table, ConsistentRead: aws.Bool(true), KeyConditionExpression: aws.String("PK = :pk"), ExpressionAttributeValues: map[string]dt.AttributeValue{":pk": s(p.PK)}})
		want := 0
		if table == st.table {
			want = 1
		}
		if e != nil || len(r.Items) != want || len(r.LastEvaluatedKey) != 0 {
			t.Fatal("partition cleanup incomplete")
		}
	}
	objects, e := s3.NewFromConfig(cfg).ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: &svc.Bucket, Prefix: aws.String("dev-api/projects/" + project + "/")})
	if e != nil || aws.ToBool(objects.IsTruncated) || len(objects.Contents) != 0 {
		t.Fatal("S3 cleanup incomplete")
	}
	for _, k := range []map[string]dt.AttributeValue{key("U#"+owner, "META"), key(identityKey(Identity{issuer: u.Issuer, subject: u.Subject}), "META")} {
		if _, e = db.DeleteItem(ctx, &dynamodb.DeleteItemInput{TableName: &st.table, Key: k}); e != nil {
			t.Fatal("identity cleanup pending")
		}
	}
	t.Logf("Interrupted fixture scheduled cleanup operation=%s completed; Control tombstone/Protected0/S30; disposable identity removed; numeric usage preserved", op.OperationID)
}

// Explicit operator-authorized cleanup of the reported browser deletion only.
// Calls the existing fenced maintenance path; never sends HTTP or deletes users.
func TestAWSSequentialWorkflowBrowserCleanup(t *testing.T) {
	const project = "31365f35-9c01-46a7-a70d-62f822ffbc4a"
	const operation = "efe8008a-ce58-447d-b9c7-440ae9bb4f7d"
	if os.Getenv("KURIER_AWS_BROWSER_CLEANUP_OPERATION") != operation {
		t.Skip("explicit browser deletion authorization required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cfg, e := workflowAWSConfig(ctx)
	if e != nil {
		t.Fatal("AWS configuration unavailable")
	}
	who, e := sts.NewFromConfig(cfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if e != nil || aws.ToString(who.Account) != "747336059622" || strings.HasSuffix(aws.ToString(who.Arn), ":root") {
		t.Fatal("intended non-root identity required")
	}
	db := dynamodb.NewFromConfig(cfg)
	st := NewStore(db, os.Getenv("KURIER_AWS_TEST_TABLE"), "dev-api")
	if st.table != "kurier-dev-api-ControlTable-bdxbaoxb" || os.Getenv("KURIER_AWS_TEST_PROTECTED") != "kurier-dev-api-ProtectedTable-szbvcchc" || os.Getenv("KURIER_AWS_TEST_BUCKET") != "executionevidence-71a0f7a" {
		t.Fatal("development resources required")
	}
	st.ConfigureProtected(os.Getenv("KURIER_AWS_TEST_PROTECTED"), nil)
	configureAWSExecutionFixture(st, cfg)
	read := func(pk, sk string) map[string]dt.AttributeValue {
		r, err := db.GetItem(ctx, &dynamodb.GetItemInput{TableName: &st.table, Key: key(pk, sk), ConsistentRead: aws.Bool(true)})
		if err != nil {
			t.Fatal("strong read failed")
		}
		return r.Item
	}
	marker, _ := json.Marshal(read("STAGE#dev-api", "META"))
	p, e := st.get(ctx, "P#"+project, "META")
	if e != nil || p.Operation == nil || p.Operation.OperationID != operation || (p.State != "deleting" && p.State != "deleted") {
		t.Fatal("exact accepted deletion required")
	}
	user := read("U#"+p.OwnerID, "META")
	if len(user) == 0 {
		t.Fatal("real owner must remain present")
	}
	userBefore, _ := json.Marshal(user)
	quotaBefore, _ := json.Marshal(read("STAGE#dev-api", "QUOTA"))
	for pass := 0; pass < 40 && p.State != "deleted"; pass++ {
		_, err := st.CleanupProject(ctx, project)
		next, getErr := st.get(ctx, p.PK, "META")
		if getErr != nil || next.Operation == nil || next.Operation.OperationID != operation {
			t.Fatal("deletion readback failed")
		}
		t.Logf("guarded pass=%d state=%s version=%d pending=%t", pass+1, next.State, next.Version, err != nil)
		p = next
	}
	if p.State != "deleted" || p.Operation.State != "completed" {
		t.Fatal("bounded cleanup incomplete")
	}
	for _, table := range []string{st.table, st.executions.Protected} {
		r, err := db.Query(ctx, &dynamodb.QueryInput{TableName: &table, ConsistentRead: aws.Bool(true), KeyConditionExpression: aws.String("PK = :pk"), ExpressionAttributeValues: map[string]dt.AttributeValue{":pk": s(p.PK)}})
		want := 0
		if table == st.table {
			want = 1
		}
		if err != nil || len(r.LastEvaluatedKey) != 0 || len(r.Items) != want {
			t.Fatal("physical partition cleanup incomplete")
		}
	}
	objects, e := s3.NewFromConfig(cfg).ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: &st.executions.Bucket, Prefix: aws.String("dev-api/projects/" + project + "/")})
	if e != nil || aws.ToBool(objects.IsTruncated) || len(objects.Contents) != 0 {
		t.Fatal("S3 cleanup incomplete")
	}
	markerAfter, _ := json.Marshal(read("STAGE#dev-api", "META"))
	userAfter, _ := json.Marshal(read("U#"+p.OwnerID, "META"))
	quotaAfter, _ := json.Marshal(read("STAGE#dev-api", "QUOTA"))
	if !bytes.Equal(marker, markerAfter) || !bytes.Equal(userBefore, userAfter) || !bytes.Equal(quotaBefore, quotaAfter) {
		t.Fatal("capability, owner or admission quota changed")
	}
	t.Logf("operation=%s completed started=%s completed=%s; Control tombstone only, Protected0/S30; owner, stage marker and admission quota preserved; no HTTP dispatched", operation, p.Operation.StartedAt, *p.Operation.CompletedAt)
}

// Prepare a disposable project for this operator's genuine signed-in browser.
// No HTTP is submitted here; the operator deliberately submits the workflow.
func TestAWSWorkflowPublicationBrowserSetup(t *testing.T) {
	if os.Getenv("KURIER_AWS_PUBLICATION_BROWSER_SETUP") != "prepare" {
		t.Skip("explicit browser fixture setup required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cfg, e := workflowAWSConfig(ctx)
	if e != nil {
		t.Fatal("AWS session unavailable")
	}
	who, e := sts.NewFromConfig(cfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if e != nil || aws.ToString(who.Account) != "747336059622" || strings.HasSuffix(aws.ToString(who.Arn), ":root") {
		t.Fatal("intended identity required")
	}
	db := dynamodb.NewFromConfig(cfg)
	st := NewStore(db, os.Getenv("KURIER_AWS_TEST_TABLE"), "dev-api")
	st.ConfigureProtected(os.Getenv("KURIER_AWS_TEST_PROTECTED"), nil)
	configureAWSExecutionFixture(st, cfg)
	// Reuse only the real owner of Davian's independently verified browser
	// tombstone; never create a fake Cognito identity or change its credentials.
	original, e := st.get(ctx, "P#31365f35-9c01-46a7-a70d-62f822ffbc4a", "META")
	if e != nil || original.State != "deleted" || original.Operation == nil || original.Operation.OperationID != "efe8008a-ce58-447d-b9c7-440ae9bb4f7d" {
		t.Fatal("verified operator owner reference required")
	}
	owner, e := st.userRecord(ctx, original.OwnerID)
	if e != nil || owner.Issuer != "https://cognito-idp.us-east-2.amazonaws.com/us-east-2_qTDZQT1FE" {
		t.Fatal("genuine Cognito owner required")
	}
	if _, err := st.get(ctx, "P#ace3b7a2-6499-4a5e-a5dd-281c46a3dbac", "META"); err == nil {
		t.Skip("this investigation fixture was already prepared; do not create duplicates")
	} else {
		var api *APIError
		if !errors.As(err, &api) || api.Status != 404 {
			t.Fatal("fixture absence readback unavailable")
		}
	}
	p, e := st.CreateProject(ctx, original.OwnerID, "Workflow publication polling investigation")
	if e != nil {
		t.Fatal("project preparation failed")
	}
	t.Logf("prepared project=%s; fixture cleanup required", p.ProjectID)
	req, e := st.CreateRequest(ctx, original.OwnerID, p.ProjectID, RequestConfiguration{Name: "Controlled five-second delay", Method: "GET", URL: "https://8dnymkcoa0.execute-api.us-east-2.amazonaws.com/execution-fixture/delay", Headers: []RequestField{}, QueryParameters: []RequestField{}})
	if e != nil {
		t.Fatal("request preparation failed")
	}
	steps := []runner.WorkflowStep{}
	for i := 0; i < 6; i++ {
		steps = append(steps, runner.WorkflowStep{StepID: newID(), RequestID: req.RequestID, RequestRevision: req.Revision, Extract: []runner.Extraction{}, Bindings: []runner.Binding{}})
	}
	wf, e := st.executions.SaveWorkflow(ctx, original.OwnerID, p.ProjectID, "", 0, execution.WorkflowDefinition{Name: "Six sequential delays — normal browser polling", Steps: steps}, true)
	if e != nil {
		t.Fatal("workflow preparation failed")
	}
	t.Logf("prepared project=%s workflow=%s revision=%d; no run submitted, no HTTP dispatched, owner unchanged", p.ProjectID, wf.WorkflowID, wf.Revision)
}

func TestAWSWorkflowPublicationBrowserReadback(t *testing.T) {
	const project = "ace3b7a2-6499-4a5e-a5dd-281c46a3dbac"
	const runID = "c59a8ba2-8342-4522-9bbd-fcad1996290b"
	mode := os.Getenv("KURIER_AWS_PUBLICATION_BROWSER_READBACK")
	if mode != "audit" && mode != "cleanup" {
		t.Skip("explicit browser publication fixture only")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cfg, e := workflowAWSConfig(ctx)
	if e != nil {
		t.Fatal("AWS session unavailable")
	}
	who, e := sts.NewFromConfig(cfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if e != nil || aws.ToString(who.Account) != "747336059622" || strings.HasSuffix(aws.ToString(who.Arn), ":root") {
		t.Fatal("intended identity required")
	}
	db := dynamodb.NewFromConfig(cfg)
	st := NewStore(db, os.Getenv("KURIER_AWS_TEST_TABLE"), "dev-api")
	st.ConfigureProtected(os.Getenv("KURIER_AWS_TEST_PROTECTED"), nil)
	configureAWSExecutionFixture(st, cfg)
	read := func(pk, sk string) map[string]dt.AttributeValue {
		r, err := db.GetItem(ctx, &dynamodb.GetItemInput{TableName: &st.table, Key: key(pk, sk), ConsistentRead: aws.Bool(true)})
		if err != nil {
			t.Fatal("strong read unavailable")
		}
		return r.Item
	}
	stageBefore, _ := json.Marshal(read("STAGE#dev-api", "META"))
	quotaBefore := read("STAGE#dev-api", "QUOTA")
	p, e := st.get(ctx, "P#"+project, "META")
	reference, refErr := st.get(ctx, "P#31365f35-9c01-46a7-a70d-62f822ffbc4a", "META")
	if e != nil || refErr != nil || p.OwnerID != reference.OwnerID {
		t.Fatal("operator fixture ownership guard failed")
	}
	userBefore, _ := json.Marshal(read("U#"+p.OwnerID, "META"))
	query := func(table string) []map[string]dt.AttributeValue {
		r, err := db.Query(ctx, &dynamodb.QueryInput{TableName: &table, ConsistentRead: aws.Bool(true), KeyConditionExpression: aws.String("PK = :pk"), ExpressionAttributeValues: map[string]dt.AttributeValue{":pk": s(p.PK)}})
		if err != nil || len(r.LastEvaluatedKey) > 0 {
			t.Fatal("complete partition read unavailable")
		}
		return r.Items
	}
	if p.State == "active" {
		run, err := st.executions.RunDetail(ctx, p.OwnerID, project, runID)
		if err != nil || run.State != "completed" || len(run.Steps) != 6 {
			t.Fatal("genuine browser run not completed")
		}
		for _, step := range run.Steps {
			if step.State != "completed" || step.ExecutionID == nil {
				t.Fatal("step finalization missing")
			}
			if _, err = st.executions.Evidence(ctx, p.OwnerID, project, *step.ExecutionID); err != nil {
				t.Fatal("immutable evidence verification failed")
			}
		}
		jobs, tickets, manifests := 0, 0, 0
		var stored int64
		for _, item := range query(st.table) {
			switch item["kind"].(*dt.AttributeValueMemberS).Value {
			case "executionJob":
				jobs++
				if item["state"].(*dt.AttributeValueMemberS).Value != "completed" || item["fence"].(*dt.AttributeValueMemberN).Value != "1" || item["attempts"].(*dt.AttributeValueMemberN).Value != "1" {
					t.Fatal("unexpected retry/lease history")
				}
			case "executionUpload":
				tickets++
				if item["state"].(*dt.AttributeValueMemberS).Value != "published" || !item["writeSettled"].(*dt.AttributeValueMemberBOOL).Value {
					t.Fatal("unsettled publication")
				}
				n, err := strconv.ParseInt(item["bytes"].(*dt.AttributeValueMemberN).Value, 10, 64)
				if err != nil {
					t.Fatal("invalid accounting")
				}
				stored += n
			case "executionSnapshot":
				manifests++
			}
		}
		if jobs != 6 || tickets != 6 || manifests != 6 || p.StorageReservedBytes != stored || p.InflightCount != 0 || len(query(st.executions.Protected)) != 0 {
			t.Fatal("terminal ciphertext/accounting/publication mismatch")
		}
		t.Logf("Genuine deployed browser run: six completed steps, six verified immutable manifests, six settled published tickets, fence1/attempt1 each, Protected0, storage%d/inflight0; sampled trace is not a complete transition log", stored)
	}
	if mode == "cleanup" && p.State == "active" {
		op, err := st.DeleteProject(ctx, p.OwnerID, project, p.Version)
		if err != nil {
			t.Fatal("fixture deletion acceptance uncertain; reconcile existing operation")
		}
		t.Logf("accepted fixture deletion operation=%s", op.OperationID)
		p, e = st.get(ctx, p.PK, "META")
		if e != nil {
			t.Fatal("deletion readback unavailable")
		}
	}
	if mode == "cleanup" {
		if p.Operation == nil || (p.State != "deleting" && p.State != "deleted") {
			t.Fatal("accepted deletion required")
		}
		operation := p.Operation.OperationID
		for pass := 0; pass < 40 && p.State != "deleted"; pass++ {
			_, _ = st.CleanupProject(ctx, project)
			p, e = st.get(ctx, p.PK, "META")
			if e != nil || p.Operation == nil || p.Operation.OperationID != operation {
				t.Fatal("cleanup operation readback failed")
			}
		}
		if p.State != "deleted" || p.Operation.State != "completed" || len(query(st.table)) != 1 || len(query(st.executions.Protected)) != 0 {
			t.Fatal("bounded guarded cleanup incomplete")
		}
		objects, err := s3.NewFromConfig(cfg).ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: &st.executions.Bucket, Prefix: aws.String("dev-api/projects/" + project + "/")})
		if err != nil || aws.ToBool(objects.IsTruncated) || len(objects.Contents) != 0 {
			t.Fatal("complete S3 emptiness proof failed")
		}
		t.Logf("Manually assisted guarded fixture deletion=%s completed: Control tombstone only/Protected0/S30; no HTTP replay or user deletion", operation)
	}
	stageAfter, _ := json.Marshal(read("STAGE#dev-api", "META"))
	userAfter, _ := json.Marshal(read("U#"+p.OwnerID, "META"))
	quotaAfter := read("STAGE#dev-api", "QUOTA")
	if !bytes.Equal(stageBefore, stageAfter) || !bytes.Equal(userBefore, userAfter) || !reflect.DeepEqual(quotaBefore["count"], quotaAfter["count"]) || !reflect.DeepEqual(quotaBefore["month"], quotaAfter["month"]) {
		t.Fatal("stage/user/admission usage changed")
	}
}
