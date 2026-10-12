//go:build integration

package ownership

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/VioletProto/Kurier/services/api/execution"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dt "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	st "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

// Real DDB Local/HTTP/ownership, memory-only sanitized S3 fixture. No AWS trust.
type pinObjects struct {
	mu   sync.Mutex
	data map[string][]byte
}

func (o *pinObjects) GetObject(_ context.Context, in *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	raw, ok := o.data[*in.Key]
	if !ok {
		return nil, &smithy.GenericAPIError{Code: "NoSuchKey"}
	}
	return &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(raw))}, nil
}
func (o *pinObjects) PutObject(_ context.Context, in *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	raw, err := io.ReadAll(in.Body)
	o.mu.Lock()
	defer o.mu.Unlock()
	o.data[*in.Key] = raw
	return &s3.PutObjectOutput{}, err
}
func (o *pinObjects) DeleteObject(_ context.Context, in *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	delete(o.data, *in.Key)
	return &s3.DeleteObjectOutput{}, nil
}
func (o *pinObjects) HeadObject(_ context.Context, in *s3.HeadObjectInput, _ ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if _, ok := o.data[*in.Key]; !ok {
		return nil, &smithy.GenericAPIError{Code: "NotFound"}
	}
	return &s3.HeadObjectOutput{}, nil
}
func (o *pinObjects) ListObjectsV2(_ context.Context, in *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := &s3.ListObjectsV2Output{}
	for k := range o.data {
		if strings.HasPrefix(k, aws.ToString(in.Prefix)) {
			out.Contents = append(out.Contents, st.Object{Key: aws.String(k)})
		}
	}
	return out, nil
}
func pinLocal(t *testing.T) (*integration, Project, string, *pinObjects) {
	t.Helper()
	i := localIntegration(t)
	enableProtectedLocal(t, i)
	ctx := context.Background()
	objects := &pinObjects{data: map[string][]byte{}}
	service := &execution.Service{DB: i.client, Objects: objects, Table: i.store.table, Protected: i.store.protectedTable, Stage: "local", Bucket: "fixture", SigningKey: bytes.Repeat([]byte{8}, 32)}
	i.store.ConfigureExecutions(service)
	stage, _ := i.store.get(ctx, "STAGE#local", "META")
	stage.SavedRequestsSchemaVersion = 4
	stage.CloudExecutionsSchemaVersion = 1
	stage.ExecutionInputsSchemaVersion = 1
	stage.EvidencePinningSchemaVersion = 1
	stage.Version++
	item, _ := encode(stage)
	if _, err := i.client.PutItem(ctx, &dynamodb.PutItemInput{TableName: &i.store.table, Item: item}); err != nil {
		t.Fatal(err)
	}
	p, err := i.store.CreateProject(ctx, i.alice.UserID, "Pinned browser evidence")
	if err != nil {
		t.Fatal(err)
	}
	executionID, jobID, ticketID, requestID := newID(), newID(), newID(), newID()
	now := timestamp(time.Now())
	expiry := timestamp(time.Now().Add(30 * 24 * time.Hour))
	pk := "P#" + p.ProjectID
	key := "local/projects/" + p.ProjectID + "/evidence/" + executionID + "/" + ticketID + ".json"
	evidence := execution.Evidence{SchemaVersion: 1, ProjectID: p.ProjectID, ExecutionID: executionID, Source: execution.Source{RequestID: requestID, Revision: 1}, Target: "cloud", SubmittedAt: now, CompletedAt: now, Status: "completed", Outcome: execution.Outcome{Certainty: "known"}, Request: execution.RequestCapture{Method: "GET", URL: "https://fixture.example/", Headers: []execution.Header{}, Body: execution.Body{Kind: "none"}}, Response: execution.ResponseCapture{HTTPStatus: func() *int { n := 200; return &n }(), Headers: []execution.Header{}, Body: execution.Body{Kind: "json", Text: func() *string { s := `{"safe":true}`; return &s }()}}}
	raw, _, err := execution.EncodeEvidence(&evidence)
	if err != nil {
		t.Fatal(err)
	}
	objects.data[key] = raw
	hash := sha256.Sum256(raw)
	write := func(sk, kind string, extra map[string]any) {
		t.Helper()
		row := map[string]any{"PK": pk, "SK": sk, "kind": kind, "schemaVersion": 1, "version": 0, "projectId": p.ProjectID, "executionId": executionID}
		for k, v := range extra {
			row[k] = v
		}
		item, err := attributevalue.MarshalMap(row)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = i.client.PutItem(ctx, &dynamodb.PutItemInput{TableName: &i.store.table, Item: item}); err != nil {
			t.Fatal(err)
		}
	}
	config := execution.Configuration{Name: "Frozen fixture", Method: "GET", URL: "https://fixture.example/", Headers: []execution.Field{}, QueryParameters: []execution.Field{}}
	write("EXEC#"+executionID, "execution", map[string]any{"ownerId": i.alice.UserID, "state": "completed", "submittedAt": now, "completedAt": now, "jobId": jobID, "LPK": pk + "#EXEC", "LSK": now + "#" + executionID, "plan": execution.Plan{Configuration: config, RequestID: requestID, Revision: 1}, "summary": execution.Summary{HTTPStatus: evidence.Response.HTTPStatus, Outcome: evidence.Outcome}})
	write("JOB#"+jobID, "executionJob", map[string]any{"state": "completed", "jobId": jobID})
	write("EXEC#"+executionID+"#RET", "executionRetention", map[string]any{"state": "live", "normalExpiresAt": expiry, "evidenceAvailability": "available"})
	write("EXEC#"+executionID+"#SNAP", "executionSnapshot", map[string]any{"objectKey": key, "checksum": hex.EncodeToString(hash[:]), "bytes": len(raw)})
	write("UPLOAD#"+ticketID, "executionUpload", map[string]any{"state": "published", "writeSettled": true, "jobId": jobID, "objectKey": key, "checksum": hex.EncodeToString(hash[:]), "bytes": len(raw)})
	_, err = i.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{TableName: &i.store.table, Key: map[string]dt.AttributeValue{"PK": s(pk), "SK": s("META")}, UpdateExpression: aws.String("SET storageReservedBytes = :bytes"), ExpressionAttributeValues: map[string]dt.AttributeValue{":bytes": n(int64(len(raw)))}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = i.client.PutItem(ctx, &dynamodb.PutItemInput{TableName: &i.store.table, Item: map[string]dt.AttributeValue{"PK": s("STAGE#local"), "SK": s("QUOTA"), "kind": s("executionQuota"), "schemaVersion": n(1), "version": n(0), "pinnedCount": n(0), "pinnedBytes": n(0), "storageReservedBytes": n(int64(len(raw)))}})
	if err != nil {
		t.Fatal(err)
	}
	return i, p, executionID, objects
}
func TestIntegrationOwnerPinHTTPAndProjectDeletion(t *testing.T) {
	i, p, id, _ := pinLocal(t)
	path := "/api/v1/projects/" + p.ProjectID + "/executions/" + id + "/pin"
	for _, sub := range []string{"", "bob"} {
		r := i.request(t, sub, "PUT", path, "", "")
		expected := 404
		if sub == "" {
			expected = 401
		}
		if r.Code != expected {
			t.Fatal("authorization", r.Code)
		}
	}
	if r := i.request(t, "alice", "PUT", path, `{}`, ""); r.Code != 400 {
		t.Fatal("nonempty body accepted")
	}
	for _, method := range []string{"PUT", "PUT", "DELETE", "DELETE", "PUT"} {
		if r := i.request(t, "alice", method, path, "", ""); r.Code != 200 || r.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("pin route", method, r.Code)
		}
	}
	current, err := i.store.GetProject(context.Background(), i.alice.UserID, p.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = i.store.RenameProject(context.Background(), i.alice.UserID, p.ProjectID, "Renamed pinned", current.Version); err != nil {
		t.Fatal(err)
	}
	row, _ := i.store.get(context.Background(), "P#"+p.ProjectID, "META")
	if row.PinnedCount == nil || *row.PinnedCount != 1 {
		t.Fatal("rename lost counters")
	}
	_, err = i.store.DeleteProject(context.Background(), i.alice.UserID, p.ProjectID, row.Version)
	if err != nil {
		t.Fatal(err)
	}
	if r := i.request(t, "alice", "DELETE", path, "", ""); r.Code != 404 {
		t.Fatal("deleted project accessible")
	}
	for n := 0; n < 30; n++ {
		op, err := i.store.CleanupProject(context.Background(), p.ProjectID)
		if err == nil && op.State == "completed" {
			return
		}
	}
	t.Fatal("pinned project deletion did not complete")
}
