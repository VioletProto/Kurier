//go:build integration && aws

package ownership

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/VioletProto/Kurier/services/api/execution"
	runner "github.com/VioletProto/Kurier/services/request-runtime"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dt "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// Public identifiers only. No token, secret, owner profile or raw evidence is
// written to this local receipt. Reuse it after uncertain setup/observation.
type publicationRolloutFixture struct {
	ProjectID         string
	SuccessWorkflowID string
	StopWorkflowID    string
	SuccessFixtureID  string
	StopFixtureID     string
	CreatedAt         string
}

func TestAWSWorkflowPublicationRollout(t *testing.T) {
	mode := os.Getenv("KURIER_AWS_PUBLICATION_ROLLOUT")
	if mode != "prepare" && mode != "audit" && mode != "cleanup" {
		t.Skip("explicit development publication rollout fixture only")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	cfg, e := workflowAWSConfig(ctx)
	if e != nil {
		t.Fatal("renewable AWS session unavailable")
	}
	who, e := sts.NewFromConfig(cfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if e != nil || aws.ToString(who.Account) != "747336059622" || strings.HasSuffix(aws.ToString(who.Arn), ":root") {
		t.Fatal("intended non-root identity required")
	}
	if os.Getenv("KURIER_AWS_TEST_TABLE") != "kurier-dev-api-ControlTable-bdxbaoxb" || os.Getenv("KURIER_AWS_TEST_PROTECTED") != "kurier-dev-api-ProtectedTable-szbvcchc" || os.Getenv("KURIER_AWS_TEST_BUCKET") != "executionevidence-71a0f7a" {
		t.Fatal("exact development resources required")
	}
	db := dynamodb.NewFromConfig(cfg)
	st := NewStore(db, os.Getenv("KURIER_AWS_TEST_TABLE"), "dev-api")
	st.ConfigureProtected(os.Getenv("KURIER_AWS_TEST_PROTECTED"), nil)
	configureAWSExecutionFixture(st, cfg)
	reference, e := st.get(ctx, "P#31365f35-9c01-46a7-a70d-62f822ffbc4a", "META")
	if e != nil || reference.State != "deleted" || reference.Operation == nil || reference.Operation.OperationID != "efe8008a-ce58-447d-b9c7-440ae9bb4f7d" {
		t.Fatal("verified operator owner reference required")
	}
	owner, e := st.userRecord(ctx, reference.OwnerID)
	if e != nil || owner.Issuer != "https://cognito-idp.us-east-2.amazonaws.com/us-east-2_qTDZQT1FE" {
		t.Fatal("real Cognito owner required")
	}
	receiptPath := filepath.Join(os.TempDir(), "kurier-publication-rollout-fixture.json")
	var fixture publicationRolloutFixture
	persist := func() {
		raw, _ := json.Marshal(fixture)
		if os.WriteFile(receiptPath, raw, 0600) != nil {
			t.Fatal("public fixture receipt write failed")
		}
	}
	if mode == "prepare" {
		f, err := os.OpenFile(receiptPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			t.Fatal("fixture receipt exists; inspect it rather than creating duplicate projects")
		}
		_ = f.Close()
		p, err := st.CreateProject(ctx, reference.OwnerID, "Publication fix deployed validation")
		if err != nil {
			t.Fatal("fixture creation uncertain; inspect before retry")
		}
		fixture = publicationRolloutFixture{ProjectID: p.ProjectID, SuccessFixtureID: newID(), StopFixtureID: newID(), CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
		persist()
		t.Logf("prepared project=%s; receipt retained for cleanup", p.ProjectID)
		makeWorkflow := func(nonce string, stop bool) execution.Workflow {
			create := func(name, mode string, header bool) SavedRequest {
				headers := []RequestField{}
				if header {
					headers = append(headers, RequestField{Name: "Authorization", Value: "", Enabled: true})
				}
				r, err := st.CreateRequest(ctx, reference.OwnerID, p.ProjectID, RequestConfiguration{Name: name, Method: "GET", URL: "https://8dnymkcoa0.execute-api.us-east-2.amazonaws.com/execution-fixture/" + mode + "?fixture=" + nonce, Headers: headers, QueryParameters: []RequestField{}})
				if err != nil {
					t.Fatal("request preparation failed")
				}
				return r
			}
			producer := create("Controlled token", "workflow-token", false)
			consumer := create("Controlled Bearer consumer", "workflow-consume", true)
			first, second := newID(), newID()
			index := 0
			prefix := "Bearer "
			binding := runner.Binding{From: runner.Producer{StepID: first, Name: "token"}, To: runner.BindingTarget{Kind: "header", Index: &index, Prefix: &prefix}}
			steps := []runner.WorkflowStep{{StepID: first, RequestID: producer.RequestID, RequestRevision: producer.Revision, Extract: []runner.Extraction{{Name: "token", Path: "$.token", Type: "string"}}, Bindings: []runner.Binding{}}, {StepID: second, RequestID: consumer.RequestID, RequestRevision: consumer.Revision, Extract: []runner.Extraction{}, Bindings: []runner.Binding{binding}}}
			name := "Deployed publication — token and Bearer success"
			if stop {
				delay := create("Controlled unknown timeout", "workflow-delay", false)
				steps = append(steps, runner.WorkflowStep{StepID: newID(), RequestID: delay.RequestID, RequestRevision: delay.Revision, Extract: []runner.Extraction{}, Bindings: []runner.Binding{}}, runner.WorkflowStep{StepID: newID(), RequestID: consumer.RequestID, RequestRevision: consumer.Revision, Extract: []runner.Extraction{}, Bindings: []runner.Binding{binding}})
				name = "Deployed publication — timeout stops successor"
			}
			wf, err := st.executions.SaveWorkflow(ctx, reference.OwnerID, p.ProjectID, "", 0, execution.WorkflowDefinition{Name: name, Steps: steps}, true)
			if err != nil {
				t.Fatal("workflow preparation failed")
			}
			return wf
		}
		fixture.SuccessWorkflowID = makeWorkflow(fixture.SuccessFixtureID, false).WorkflowID
		persist()
		fixture.StopWorkflowID = makeWorkflow(fixture.StopFixtureID, true).WorkflowID
		persist()
		t.Logf("prepared project=%s successWorkflow=%s stopWorkflow=%s; no HTTP submitted", p.ProjectID, fixture.SuccessWorkflowID, fixture.StopWorkflowID)
		return
	}
	raw, e := os.ReadFile(receiptPath)
	if e != nil || json.Unmarshal(raw, &fixture) != nil || fixture.ProjectID == "" {
		t.Fatal("existing fixture receipt required")
	}
	p, e := st.get(ctx, "P#"+fixture.ProjectID, "META")
	if e != nil || p.OwnerID != reference.OwnerID || p.State == "active" && p.Name != "Publication fix deployed validation" {
		t.Fatal("fixture ownership/scope mismatch")
	}
	query := func(table string) []map[string]dt.AttributeValue {
		r, err := db.Query(ctx, &dynamodb.QueryInput{TableName: &table, ConsistentRead: aws.Bool(true), KeyConditionExpression: aws.String("PK = :pk"), ExpressionAttributeValues: map[string]dt.AttributeValue{":pk": s(p.PK)}})
		if err != nil || len(r.LastEvaluatedKey) > 0 {
			t.Fatal("complete strong partition read failed")
		}
		return r.Items
	}
	text := func(row map[string]dt.AttributeValue, k string) string {
		v, _ := row[k].(*dt.AttributeValueMemberS)
		if v == nil {
			return ""
		}
		return v.Value
	}
	number := func(row map[string]dt.AttributeValue, k string) int64 {
		v, _ := row[k].(*dt.AttributeValueMemberN)
		if v == nil {
			return 0
		}
		n, _ := strconv.ParseInt(v.Value, 10, 64)
		return n
	}
	if p.State == "active" {
		items := query(st.table)
		runs := map[string]string{}
		for _, r := range items {
			if text(r, "kind") == "executionWorkflowRun" {
				wf := text(r, "workflowId")
				if wf != fixture.SuccessWorkflowID && wf != fixture.StopWorkflowID {
					t.Fatal("unknown run in disposable project")
				}
				if runs[wf] != "" {
					t.Fatal("duplicate workflow admissions require inspection")
				}
				runs[wf] = text(r, "runId")
			}
		}
		if len(runs) != 2 {
			t.Fatal("both deliberate browser submissions must exist; no automatic admission retry")
		}
		tokens := []string{}
		for _, nonce := range []string{fixture.SuccessFixtureID, fixture.StopFixtureID} {
			h := sha256.Sum256([]byte("kurier-workflow-fixture-v1/" + nonce))
			tokens = append(tokens, hex.EncodeToString(h[:]))
		}
		controlBytes, _ := json.Marshal(items)
		protectedBytes, _ := json.Marshal(query(st.executions.Protected))
		for _, token := range tokens {
			if bytes.Contains(controlBytes, []byte(token)) || bytes.Contains(protectedBytes, []byte(token)) {
				t.Fatal("runtime value escaped into Control/Protected plaintext")
			}
		}
		var storage int64
		jobs, tickets, manifests := 0, 0, 0
		for _, wf := range []string{fixture.SuccessWorkflowID, fixture.StopWorkflowID} {
			runID := runs[wf]
			run, err := st.executions.RunDetail(ctx, reference.OwnerID, p.ProjectID, runID)
			if err != nil {
				t.Fatal("terminal run inspection unavailable")
			}
			if wf == fixture.SuccessWorkflowID && (run.State != "completed" || len(run.Steps) != 2) {
				t.Fatal("success workflow not completed")
			}
			if wf == fixture.StopWorkflowID && (run.State != "failed" || len(run.Steps) != 4 || run.Steps[2].Summary == nil || run.Steps[2].Summary.Outcome.Certainty != "unknown" || run.Steps[3].State != "skipped") {
				t.Fatal("unknown outcome did not stop successor")
			}
			for _, step := range run.Steps {
				if step.State == "skipped" {
					continue
				}
				if step.ExecutionID == nil {
					t.Fatal("missing finalized execution")
				}
				evidence, err := st.executions.Evidence(ctx, reference.OwnerID, p.ProjectID, *step.ExecutionID)
				if err != nil {
					t.Fatal("immutable evidence read failed")
				}
				again, err := st.executions.Evidence(ctx, reference.OwnerID, p.ProjectID, *step.ExecutionID)
				if err != nil || !bytes.Equal(evidence, again) {
					t.Fatal("immutable evidence changed")
				}
				for _, token := range tokens {
					if bytes.Contains(evidence, []byte(token)) {
						t.Fatal("runtime source/rendered value escaped redaction")
					}
				}
			}
			t.Logf("terminal run=%s workflow=%s state=%s; immutable evidence reread unchanged/redacted", runID, wf, run.State)
		}
		for _, r := range items {
			switch text(r, "kind") {
			case "executionJob":
				jobs++
				if text(r, "state") != "completed" && text(r, "state") != "failed" || number(r, "attempts") != 1 || number(r, "fence") != 1 {
					t.Fatal("unexpected job attempts/fence/state")
				}
			case "executionUpload":
				tickets++
				settled, _ := r["writeSettled"].(*dt.AttributeValueMemberBOOL)
				if text(r, "state") != "published" || settled == nil || !settled.Value {
					t.Fatal("unsettled evidence ticket")
				}
				storage += number(r, "bytes")
			case "executionSnapshot":
				manifests++
			}
		}
		if jobs != 5 || tickets != 5 || manifests != 5 || storage != p.StorageReservedBytes || p.InflightCount != 0 || len(query(st.executions.Protected)) != 0 {
			t.Fatal("terminal accounting/ciphertext mismatch")
		}
		t.Logf("readback jobs5/tickets5/manifests5/fence1-attempt1/Protected0/storage%d/inflight0; six admitted slots, one skipped", storage)
	}
	if mode != "cleanup" {
		if p.State != "active" {
			t.Fatal("runtime fixture already deleted; use cleanup mode for physical readback")
		}
		return
	}
	stage, e := st.get(ctx, "STAGE#dev-api", "META")
	if e != nil {
		t.Fatal("stage read failed")
	}
	stageBefore, _ := json.Marshal(stage)
	userBefore, _ := json.Marshal(owner)
	quotaBefore, e := db.GetItem(ctx, &dynamodb.GetItemInput{TableName: &st.table, Key: key("STAGE#dev-api", "QUOTA"), ConsistentRead: aws.Bool(true)})
	if e != nil {
		t.Fatal("quota read failed")
	}
	if p.State == "active" {
		op, err := st.DeleteProject(ctx, reference.OwnerID, p.ProjectID, p.Version)
		if err != nil {
			t.Fatal("deletion uncertain; inspect original operation")
		}
		t.Logf("accepted cleanup operation=%s", op.OperationID)
		p, e = st.get(ctx, p.PK, "META")
		if e != nil {
			t.Fatal("deletion readback failed")
		}
	}
	if p.Operation == nil || (p.State != "deleting" && p.State != "deleted") {
		t.Fatal("accepted deletion required")
	}
	operation := p.Operation.OperationID
	for pass := 0; pass < 40 && p.State != "deleted"; pass++ {
		_, _ = st.CleanupProject(ctx, p.ProjectID)
		p, e = st.get(ctx, p.PK, "META")
		if e != nil || p.Operation == nil || p.Operation.OperationID != operation {
			t.Fatal("cleanup readback failed")
		}
	}
	if p.State != "deleted" || p.Operation.State != "completed" || len(query(st.table)) != 1 || len(query(st.executions.Protected)) != 0 {
		t.Fatal("bounded guarded cleanup pending")
	}
	objects, err := s3.NewFromConfig(cfg).ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: &st.executions.Bucket, Prefix: aws.String("dev-api/projects/" + p.ProjectID + "/")})
	if err != nil || aws.ToBool(objects.IsTruncated) || len(objects.Contents) != 0 {
		t.Fatal("S3 emptiness proof failed")
	}
	stage, e = st.get(ctx, "STAGE#dev-api", "META")
	if e != nil {
		t.Fatal("stage read failed")
	}
	stageAfter, _ := json.Marshal(stage)
	user, e := st.userRecord(ctx, reference.OwnerID)
	if e != nil {
		t.Fatal("real user read failed")
	}
	userAfter, _ := json.Marshal(user)
	quotaAfter, e := db.GetItem(ctx, &dynamodb.GetItemInput{TableName: &st.table, Key: key("STAGE#dev-api", "QUOTA"), ConsistentRead: aws.Bool(true)})
	if e != nil || !bytes.Equal(stageBefore, stageAfter) || !bytes.Equal(userBefore, userAfter) || number(quotaBefore.Item, "count") != number(quotaAfter.Item, "count") || text(quotaBefore.Item, "month") != text(quotaAfter.Item, "month") {
		t.Fatal("cleanup changed stage/user/admission usage")
	}
	t.Logf("cleanup operation=%s completed; Control tombstone-only/Protected0/S30, real user and usage retained; manually assisted existing guarded maintenance", operation)
	// An uncertain PUT is intentionally never settled by this helper.
}
