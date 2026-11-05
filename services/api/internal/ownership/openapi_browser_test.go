//go:build integration && browser

package ownership

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

func configureBrowserOpenAPI(t *testing.T, i *integration) {
	t.Helper()
	stage, err := i.store.get(context.Background(), "STAGE#local", "META")
	if err != nil {
		t.Fatal(err)
	}
	stage.SavedRequestsSchemaVersion = 10
	stage.OpenAPISchemaVersion = 1
	stage.Version++
	item, err := encode(stage)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = i.client.PutItem(context.Background(), &dynamodb.PutItemInput{TableName: &i.store.table, Item: item}); err != nil {
		t.Fatal(err)
	}
}
