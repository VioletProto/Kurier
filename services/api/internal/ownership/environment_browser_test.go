//go:build integration && browser

package ownership

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"testing"
)

func configureBrowserEnvironments(t *testing.T, i *integration) {
	t.Helper()
	ctx := context.Background()
	stage, e := i.store.get(ctx, "STAGE#local", "META")
	if e != nil {
		t.Fatal(e)
	}
	stage.SavedRequestsSchemaVersion = 9
	stage.ProjectEnvironmentsSchemaVersion = 1
	stage.ProtectedSecretsSchemaVersion = 2
	stage.LocalAgentExecutionsSchemaVersion = 2
	stage.WorkflowAssertionsSchemaVersion = 1
	stage.MCPReadOnlySchemaVersion = 1
	stage.Version++
	item, _ := encode(stage)
	if _, e = i.client.PutItem(ctx, &dynamodb.PutItemInput{TableName: &i.store.table, Item: item}); e != nil {
		t.Fatal(e)
	}
	i.store.cipher.kms = workflowFixtureKMS{}
}
