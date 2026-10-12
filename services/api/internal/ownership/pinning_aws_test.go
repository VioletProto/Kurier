//go:build integration && aws

package ownership

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/VioletProto/Kurier/services/api/execution"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dt "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// Real AWS services and deployed worker/scheduled cleanup. Identity is a direct
// SDK fixture, not Cognito. Only the unauthenticated route check uses HTTP here.
// Expiry is moved into the past for this disposable RET only, under CAS guards;
// this is a timing fixture, not a claim that thirty wall-clock days elapsed.
func TestAWSEvidencePinning(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-east-2"), config.WithRetryer(func() aws.Retryer { return aws.NopRetryer{} }))
	if err != nil {
		t.Fatal("AWS configuration unavailable")
	}
	identity, err := sts.NewFromConfig(cfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil || aws.ToString(identity.Account) != "747336059622" || strings.HasSuffix(aws.ToString(identity.Arn), ":root") {
		t.Fatal("intended non-root identity required")
	}
	db := dynamodb.NewFromConfig(cfg)
	st := NewStore(db, os.Getenv("KURIER_AWS_TEST_TABLE"), "dev-api")
	if !strings.HasPrefix(st.table, "kurier-dev-api-ControlTable-") {
		t.Fatal("scoped table required")
	}
	st.ConfigureProtected(os.Getenv("KURIER_AWS_TEST_PROTECTED"), nil)
	configureAWSExecutionFixture(st, cfg)
	svc := st.executions
	parameter, err := ssm.NewFromConfig(cfg).GetParameter(ctx, &ssm.GetParameterInput{Name: aws.String("/kurier/dev-api/cursor-key"), WithDecryption: aws.Bool(true)})
	if err != nil {
		t.Fatal("signing key unavailable")
	}
	svc.SigningKey, err = base64.StdEncoding.DecodeString(aws.ToString(parameter.Parameter.Value))
	if err != nil {
		t.Fatal("signing key invalid")
	}
	defer clear(svc.SigningKey)
	if owner := os.Getenv("KURIER_AWS_PIN_BROWSER_OWNER"); owner != "" {
		prepareAWSBrowserPinExpiry(t, ctx, st, owner)
		return
	}
	issuer := "https://disposable-pinning-sdk.invalid/" + newID()
	subject := newID()
	owner, err := st.ResolveUser(ctx, Identity{issuer: issuer, subject: subject})
	if err != nil {
		t.Fatal("fixture user unavailable")
	}
	project, err := st.CreateProject(ctx, owner.UserID, "Disposable AWS pinning fixture")
	if err != nil {
		t.Fatal("fixture project unavailable")
	}
	t.Logf("Disposable SDK fixture project %s owner %s", project.ProjectID, owner.UserID)
	t.Cleanup(func() {
		c, done := context.WithTimeout(context.Background(), 2*time.Minute)
		defer done()
		p, e := st.get(c, "P#"+project.ProjectID, "META")
		if e == nil && p.State != "deleted" {
			if p.State == "active" {
				_, e = st.DeleteProject(c, owner.UserID, project.ProjectID, p.Version)
			}
			for n := 0; e == nil && n < 50; n++ {
				op, x := st.CleanupProject(c, project.ProjectID)
				if x == nil && op.State == "completed" {
					break
				}
				if n == 49 {
					t.Error("fixture emergency cleanup pending; retain safe IDs")
				}
			}
		}
		current, readErr := st.get(c, "P#"+project.ProjectID, "META")
		if readErr != nil || current.State != "deleted" {
			t.Error("fixture cleanup incomplete; identity retained for guarded resume")
			return
		}
		for _, k := range []map[string]dt.AttributeValue{key("U#"+owner.UserID, "META"), key(identityKey(Identity{issuer: issuer, subject: subject}), "META")} {
			if _, e = db.DeleteItem(c, &dynamodb.DeleteItemInput{TableName: &st.table, Key: k}); e != nil {
				t.Error("fixture identity cleanup pending")
			}
		}
	})
	saved, err := st.CreateRequest(ctx, owner.UserID, project.ProjectID, RequestConfiguration{Name: "Public controlled echo", Method: "GET", URL: os.Getenv("KURIER_AWS_TEST_ENDPOINT"), Headers: []RequestField{}, QueryParameters: []RequestField{}})
	if err != nil {
		t.Fatal("fixture request unavailable")
	}
	read := func(pk, sk string) map[string]dt.AttributeValue {
		t.Helper()
		r, e := db.GetItem(ctx, &dynamodb.GetItemInput{TableName: &st.table, Key: key(pk, sk), ConsistentRead: aws.Bool(true)})
		if e != nil {
			t.Fatal("strong metadata read unavailable")
		}
		return r.Item
	}
	number := func(r map[string]dt.AttributeValue, k string) string {
		if n, ok := r[k].(*dt.AttributeValueMemberN); ok {
			return n.Value
		}
		return "missing"
	}
	stageBefore := read("STAGE#dev-api", "QUOTA")
	options, _ := execution.ParseOptions([]byte(`{}`))
	var runs []execution.View
	for n := 0; n < 2; n++ {
		v, e := svc.Submit(ctx, owner.UserID, project.ProjectID, saved.RequestID, newID(), saved.Revision, options, false)
		if e != nil {
			t.Fatal("AWS admission unavailable")
		}
		for deadline := time.Now().Add(90 * time.Second); time.Now().Before(deadline); {
			v, e = svc.Detail(ctx, owner.UserID, project.ProjectID, v.ExecutionID)
			if e == nil && v.Status == "completed" {
				break
			}
			time.Sleep(time.Second)
		}
		if e != nil || v.Status != "completed" {
			t.Fatal("deployed worker completion pending")
		}
		runs = append(runs, v)
	}
	var total int64
	objects := s3.NewFromConfig(cfg)
	capture := func(id string) ([]byte, map[string]dt.AttributeValue) {
		t.Helper()
		m := read("P#"+project.ProjectID, "EXEC#"+id+"#SNAP")
		k, ok := m["objectKey"].(*dt.AttributeValueMemberS)
		if !ok {
			t.Fatal("snapshot missing")
		}
		out, e := objects.GetObject(ctx, &s3.GetObjectInput{Bucket: &svc.Bucket, Key: &k.Value})
		if e != nil {
			t.Fatal("private object unavailable")
		}
		b, e := io.ReadAll(io.LimitReader(out.Body, execution.EvidenceLimit+1))
		out.Body.Close()
		h := sha256.Sum256(b)
		if e != nil || hex.EncodeToString(h[:]) != m["checksum"].(*dt.AttributeValueMemberS).Value {
			t.Fatal("stored checksum mismatch")
		}
		return b, m
	}
	for _, v := range runs {
		before, m := capture(v.ExecutionID)
		total += int64(len(before))
		if _, e := svc.Pin(ctx, newID(), project.ProjectID, v.ExecutionID, true); e == nil {
			t.Fatal("foreign pin accepted")
		}
		first, e := svc.Pin(ctx, owner.UserID, project.ProjectID, v.ExecutionID, true)
		if e != nil {
			t.Fatal("actual pin failed")
		}
		repeat, e := svc.Pin(ctx, owner.UserID, project.ProjectID, v.ExecutionID, true)
		if e != nil || repeat.Retention.Version != first.Retention.Version || *repeat.Retention.PinnedAt != *first.Retention.PinnedAt {
			t.Fatal("repeat changed retention")
		}
		after, m2 := capture(v.ExecutionID)
		if !bytes.Equal(before, after) || number(m, "version") != number(m2, "version") {
			t.Fatal("pin changed immutable evidence")
		}
	}
	p, e := st.get(ctx, "P#"+project.ProjectID, "META")
	if e != nil || p.PinnedCount == nil || *p.PinnedCount != 2 || *p.PinnedBytes != total {
		t.Fatal("project classification mismatch")
	}
	q := read("STAGE#dev-api", "QUOTA")
	var baseCount, baseBytes int64
	// Decode only bounded numeric accounting; never print metadata or payloads.
	for k, target := range map[string]*int64{"pinnedCount": &baseCount, "pinnedBytes": &baseBytes} {
		value, parseErr := strconv.ParseInt(number(stageBefore, k), 10, 64)
		if parseErr != nil {
			t.Fatal("initial pin accounting unavailable")
		}
		*target = value
	}
	if number(q, "pinnedCount") != intString(baseCount+2) || number(q, "pinnedBytes") != intString(baseBytes+total) {
		t.Fatal("stage classification mismatch")
	}
	t.Log("Real DynamoDB pin/repeat/foreign-owner checks, S3 bytes/checksums and project/stage classification passed")
	id := runs[0].ExecutionID
	ret := read(p.PK, "EXEC#"+id+"#RET")
	expired := timestamp(time.Now().Add(-time.Minute))
	marker, e := st.activeStage(ctx)
	if e != nil {
		t.Fatal("stage unavailable")
	}
	_, err = db.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{TransactItems: []dt.TransactWriteItem{st.stageGuard(marker), dt.TransactWriteItem{ConditionCheck: &dt.ConditionCheck{TableName: &st.table, Key: key(p.PK, "META"), ConditionExpression: aws.String("#v = :v AND #s = :active AND ownerId = :owner"), ExpressionAttributeNames: map[string]string{"#v": "version", "#s": "state"}, ExpressionAttributeValues: map[string]dt.AttributeValue{":v": n(p.Version), ":active": s("active"), ":owner": s(owner.UserID)}}}, {Update: &dt.Update{TableName: &st.table, Key: key(p.PK, "EXEC#"+id+"#RET"), ConditionExpression: aws.String("#v = :v AND pinned = :yes AND #s = :live"), UpdateExpression: aws.String("SET normalExpiresAt = :expiry, #v = #v + :one"), ExpressionAttributeNames: map[string]string{"#v": "version", "#s": "state"}, ExpressionAttributeValues: map[string]dt.AttributeValue{":v": ret["version"], ":yes": &dt.AttributeValueMemberBOOL{Value: true}, ":live": s("live"), ":expiry": s(expired), ":one": n(1)}}}}})
	if err != nil {
		t.Fatal("fixture-only expiry CAS failed")
	}
	// Wait across a minute schedule without manually invoking cleanup.
	time.Sleep(75 * time.Second)
	if _, err = svc.Evidence(ctx, owner.UserID, project.ProjectID, id); err != nil {
		t.Fatal("scheduled cleanup removed pinned fixture")
	}
	unpin, err := svc.Pin(ctx, owner.UserID, project.ProjectID, id, false)
	if err != nil || unpin.Retention.EffectiveStatus != "expired" || unpin.Retention.NormalExpiresAt != expired {
		t.Fatal("expired unpin granted grace")
	}
	if _, err = svc.Evidence(ctx, owner.UserID, project.ProjectID, id); err == nil {
		t.Fatal("expired evidence readable")
	}
	if _, err = svc.Pin(ctx, owner.UserID, project.ProjectID, id, true); err == nil {
		t.Fatal("expired pin resurrected")
	}
	for deadline := time.Now().Add(8 * time.Minute); time.Now().Before(deadline); {
		if len(read(p.PK, "EXEC#"+id)) == 0 {
			break
		}
		time.Sleep(3 * time.Second)
	}
	if len(read(p.PK, "EXEC#"+id)) != 0 {
		t.Fatal("scheduled expiry cleanup pending")
	}
	t.Log("Actual scheduled cleanup skipped pinned fixture and drained it after expired unpin; expiry timestamp was a guarded fixture mutation")
	p, e = st.get(ctx, p.PK, "META")
	if e != nil || *p.PinnedCount != 1 {
		t.Fatal("remaining pin accounting mismatch")
	}
	op, e := st.DeleteProject(ctx, owner.UserID, project.ProjectID, p.Version)
	if e != nil {
		t.Fatal("deletion acceptance failed")
	}
	if _, err = svc.Pin(ctx, owner.UserID, project.ProjectID, runs[1].ExecutionID, true); err == nil {
		t.Fatal("project deletion allowed pin")
	}
	for deadline := time.Now().Add(8 * time.Minute); time.Now().Before(deadline); {
		op, e = st.GetDeletion(ctx, owner.UserID, project.ProjectID, op.OperationID)
		if e == nil && op.State == "completed" {
			break
		}
		time.Sleep(3 * time.Second)
	}
	if e != nil || op.State != "completed" {
		t.Fatal("scheduled pinned project deletion pending")
	}
	list, e := objects.ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: &svc.Bucket, Prefix: aws.String("dev-api/projects/" + project.ProjectID + "/")})
	if e != nil || list.IsTruncated != nil && *list.IsTruncated || len(list.Contents) != 0 {
		t.Fatal("fixture S3 cleanup incomplete")
	}
	for _, table := range []string{st.table, svc.Protected} {
		out, e := db.Query(ctx, &dynamodb.QueryInput{TableName: &table, KeyConditionExpression: aws.String("PK = :pk"), ExpressionAttributeValues: map[string]dt.AttributeValue{":pk": s(p.PK)}, ConsistentRead: aws.Bool(true)})
		wanted := 0
		if table == st.table {
			wanted = 1
		}
		if e != nil || len(out.Items) != wanted || len(out.LastEvaluatedKey) > 0 {
			t.Fatal("fixture partition cleanup incomplete")
		}
	}
	// Other authorized browser fixtures may change global accounting concurrently.
	// Reconcile all pins against a stable quota version; never reset a counter.
	verified := false
	for attempt := 0; attempt < 3; attempt++ {
		before := read("STAGE#dev-api", "QUOTA")
		var cursor map[string]dt.AttributeValue
		var count, encodedBytes int64
		for {
			page, err := db.Scan(ctx, &dynamodb.ScanInput{TableName: &st.table, ConsistentRead: aws.Bool(true), ExclusiveStartKey: cursor})
			if err != nil {
				t.Fatal("bounded classification reconciliation unavailable")
			}
			for _, row := range page.Items {
				pin, ok := row["pinned"].(*dt.AttributeValueMemberBOOL)
				if !ok || !pin.Value {
					continue
				}
				kind, ok := row["kind"].(*dt.AttributeValueMemberS)
				if !ok || kind.Value != "executionRetention" {
					t.Fatal("unknown pinned accounting requires review")
				}
				manifest := read(row["PK"].(*dt.AttributeValueMemberS).Value, "EXEC#"+row["executionId"].(*dt.AttributeValueMemberS).Value+"#SNAP")
				size, err := strconv.ParseInt(number(manifest, "bytes"), 10, 64)
				if err != nil || size < 1 {
					t.Fatal("pin manifest accounting unavailable")
				}
				count++
				encodedBytes += size
			}
			cursor = page.LastEvaluatedKey
			if len(cursor) == 0 {
				break
			}
		}
		after := read("STAGE#dev-api", "QUOTA")
		if number(before, "version") != number(after, "version") {
			continue
		}
		if number(after, "pinnedCount") != intString(count) || number(after, "pinnedBytes") != intString(encodedBytes) {
			t.Fatal("stage pin classification reconciliation mismatch")
		}
		verified = true
		break
	}
	if !verified {
		t.Fatal("concurrent accounting reconciliation pending")
	}
	t.Logf("Scheduled project deletion overrode remaining pin; operation %s completed; Control tombstone only, Protected/S3 empty, pin accounting reconciled with concurrent disposable browser activity", op.OperationID)
	for _, method := range []string{"PUT", "DELETE"} {
		req, _ := http.NewRequestWithContext(ctx, method, strings.TrimSuffix(os.Getenv("KURIER_AWS_TEST_ENDPOINT"), "/execution-fixture/echo")+"/api/v1/projects/"+project.ProjectID+"/executions/"+id+"/pin", nil)
		response, e := http.DefaultClient.Do(req)
		if e != nil {
			t.Fatal("actual route unavailable")
		}
		response.Body.Close()
		if response.StatusCode != 401 {
			t.Fatal("unauthenticated route not denied")
		}
	}
	t.Log("Actual HTTPS PUT/DELETE routes deny missing Cognito tokens with 401; authenticated HTTP/browser checks remain separate")
}

func intString(value int64) string { return strconv.FormatInt(value, 10) }

// Explicitly authorized disposable preparation for a genuine browser owner.
// Retain safe IDs on failure; never mutate any preexisting user project.
func prepareAWSBrowserPinExpiry(t *testing.T, ctx context.Context, st *Store, owner string) {
	t.Helper()
	if owner != "ca677adc-9be3-43e2-8578-46b0464be718" {
		t.Fatal("explicit participating disposable-probe owner required")
	}
	if _, err := st.userRecord(ctx, owner); err != nil {
		t.Fatal("participating owner unavailable")
	}
	project, err := st.CreateProject(ctx, owner, "Disposable browser expired-unpin validation")
	if err != nil {
		t.Fatal("browser expiry fixture creation failed")
	}
	t.Logf("Browser expiry disposable project %s owner %s; retain IDs for cleanup", project.ProjectID, owner)
	saved, err := st.CreateRequest(ctx, owner, project.ProjectID, RequestConfiguration{Name: "Public expiry echo", Method: "GET", URL: os.Getenv("KURIER_AWS_TEST_ENDPOINT"), Headers: []RequestField{}, QueryParameters: []RequestField{}})
	if err != nil {
		t.Fatal("browser expiry request preparation failed")
	}
	options, _ := execution.ParseOptions([]byte(`{}`))
	run, err := st.executions.Submit(ctx, owner, project.ProjectID, saved.RequestID, newID(), saved.Revision, options, false)
	if err != nil {
		t.Fatal("browser expiry admission failed")
	}
	for deadline := time.Now().Add(90 * time.Second); time.Now().Before(deadline); {
		run, err = st.executions.Detail(ctx, owner, project.ProjectID, run.ExecutionID)
		if err == nil && run.Status == "completed" {
			break
		}
		time.Sleep(time.Second)
	}
	if err != nil || run.Status != "completed" {
		t.Fatal("browser expiry worker completion pending")
	}
	pinned, err := st.executions.Pin(ctx, owner, project.ProjectID, run.ExecutionID, true)
	if err != nil {
		t.Fatal("browser expiry pin preparation failed")
	}
	p, err := st.get(ctx, "P#"+project.ProjectID, "META")
	if err != nil {
		t.Fatal("browser expiry gate unavailable")
	}
	stage, err := st.activeStage(ctx)
	if err != nil {
		t.Fatal("browser expiry stage unavailable")
	}
	_, err = st.db.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{TransactItems: []dt.TransactWriteItem{st.stageGuard(stage), {ConditionCheck: &dt.ConditionCheck{TableName: &st.table, Key: key(p.PK, "META"), ConditionExpression: aws.String("#v = :v AND #s = :active AND ownerId = :owner"), ExpressionAttributeNames: map[string]string{"#v": "version", "#s": "state"}, ExpressionAttributeValues: map[string]dt.AttributeValue{":v": n(p.Version), ":active": s("active"), ":owner": s(owner)}}}, {Update: &dt.Update{TableName: &st.table, Key: key(p.PK, "EXEC#"+run.ExecutionID+"#RET"), ConditionExpression: aws.String("#v = :v AND pinned = :yes AND #s = :live"), UpdateExpression: aws.String("SET normalExpiresAt = :expiry, #v = #v + :one"), ExpressionAttributeNames: map[string]string{"#v": "version", "#s": "state"}, ExpressionAttributeValues: map[string]dt.AttributeValue{":v": n(pinned.Retention.Version), ":yes": &dt.AttributeValueMemberBOOL{Value: true}, ":live": s("live"), ":expiry": s(timestamp(time.Now().Add(-time.Minute))), ":one": n(1)}}}}})
	if err != nil {
		t.Fatal("browser fixture-only expiry mutation failed")
	}
	t.Logf("Prepared genuine expired-unpin probe projectId=%s executionId=%s ownerId=%s; immutable evidence retained, fixture-only past expiry, cleanup pending", project.ProjectID, run.ExecutionID, owner)
}
