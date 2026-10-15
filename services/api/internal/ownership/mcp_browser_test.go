//go:build integration && browser

package ownership

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"testing"
)

func configureBrowserMCP(t *testing.T, i *integration) {
	t.Helper()
	stage, e := i.store.get(context.Background(), "STAGE#local", "META")
	if e != nil {
		t.Fatal(e)
	}
	stage.SavedRequestsSchemaVersion = 7
	stage.MCPReadOnlySchemaVersion = 1
	stage.Version++
	item, _ := encode(stage)
	if _, e = i.client.PutItem(context.Background(), &dynamodb.PutItemInput{TableName: &i.store.table, Item: item}); e != nil {
		t.Fatal(e)
	}
}
