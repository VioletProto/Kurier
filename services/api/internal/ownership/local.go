package ownership

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// LocalClient deliberately cannot target AWS or load AWS credentials. This task
// preserves the local executable boundary; cloud uses a separate adapter.
func LocalClient(endpoint string) (*dynamodb.Client, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "http" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" || u.Port() == "" {
		return nil, errors.New("explicit loopback DynamoDB Local URL required")
	}
	host := u.Hostname()
	if host != "localhost" && !net.ParseIP(host).IsLoopback() {
		return nil, errors.New("DynamoDB endpoint must be loopback")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return dynamodb.New(dynamodb.Options{
		Region: "us-east-2", BaseEndpoint: aws.String(endpoint),
		Credentials: credentials.NewStaticCredentialsProvider("local", "local", ""),
		Retryer:     aws.NopRetryer{},
		HTTPClient:  &http.Client{Timeout: 5 * time.Second, Transport: transport, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return errors.New("local DynamoDB redirects disabled") }},
	}), nil
}

// InitLocal creates the local Control subset with list/maintenance indexes.
// Existing tables/stages are never overwritten or silently reactivated.
func InitLocal(ctx context.Context, client *dynamodb.Client, table, stage string) error {
	_, err := client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String(table), BillingMode: types.BillingModePayPerRequest,
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("PK"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: aws.String("SK"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: aws.String("LPK"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: aws.String("LSK"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: aws.String("DPK"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: aws.String("DSK"), AttributeType: types.ScalarAttributeTypeS},
		}, KeySchema: []types.KeySchemaElement{{AttributeName: aws.String("PK"), KeyType: types.KeyTypeHash}, {AttributeName: aws.String("SK"), KeyType: types.KeyTypeRange}},
		GlobalSecondaryIndexes: []types.GlobalSecondaryIndex{
			{IndexName: aws.String("GSI1"), KeySchema: []types.KeySchemaElement{{AttributeName: aws.String("LPK"), KeyType: types.KeyTypeHash}, {AttributeName: aws.String("LSK"), KeyType: types.KeyTypeRange}}, Projection: &types.Projection{ProjectionType: types.ProjectionTypeKeysOnly}},
			{IndexName: aws.String("GSI2"), KeySchema: []types.KeySchemaElement{{AttributeName: aws.String("DPK"), KeyType: types.KeyTypeHash}, {AttributeName: aws.String("DSK"), KeyType: types.KeyTypeRange}}, Projection: &types.Projection{ProjectionType: types.ProjectionTypeKeysOnly}},
		},
	})
	var exists *types.ResourceInUseException
	if err != nil && !errors.As(err, &exists) {
		return err
	}
	if err = dynamodb.NewTableExistsWaiter(client).Wait(ctx, &dynamodb.DescribeTableInput{TableName: aws.String(table)}, 30*time.Second); err != nil {
		return err
	}
	item, err := encode(record{PK: "STAGE#" + stage, SK: "META", Kind: "stage", SchemaVersion: 1, State: "active", RecoveryGeneration: newID(), SavedRequestsSchemaVersion: 1})
	if err != nil {
		return err
	}
	_, err = client.PutItem(ctx, &dynamodb.PutItemInput{TableName: aws.String(table), Item: item, ConditionExpression: aws.String("attribute_not_exists(PK)")})
	var conditional *types.ConditionalCheckFailedException
	if errors.As(err, &conditional) {
		return nil
	}
	return err
}
