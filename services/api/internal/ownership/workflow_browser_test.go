//go:build integration && browser

package ownership

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"testing"
)

// Browser coverage exercises real Go ownership/CRUD/admission against DDB Local.
// It never dispatches cloud HTTP or pretends fixture KMS is AWS.
func configureBrowserWorkflows(t *testing.T, i *integration) {
	t.Helper()
	ctx := context.Background()
	stage, _ := i.store.get(ctx, "STAGE#local", "META")
	stage.SavedRequestsSchemaVersion = 6
	stage.SequentialWorkflowsSchemaVersion = 1
	stage.Version++
	item, _ := encode(stage)
	if _, e := i.client.PutItem(ctx, &dynamodb.PutItemInput{TableName: &i.store.table, Item: item}); e != nil {
		t.Fatal(e)
	}
	p, e := i.store.CreateProject(ctx, i.alice.UserID, "Workflow browser fixture")
	if e != nil {
		t.Fatal(e)
	}
	c := publicConfiguration()
	c.Name = "Workflow login"
	c.Headers = []RequestField{}
	if _, e = i.store.CreateRequest(ctx, i.alice.UserID, p.ProjectID, c); e != nil {
		t.Fatal(e)
	}
	c.Name = "Workflow consumer"
	c.Headers = []RequestField{{Name: "Authorization", Value: "", Enabled: true}}
	if _, e = i.store.CreateRequest(ctx, i.alice.UserID, p.ProjectID, c); e != nil {
		t.Fatal(e)
	}
}
