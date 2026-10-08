package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"time"

	"github.com/DrKaelum/Kurier/services/api/internal/ownership"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// Cloud configuration is deliberately separate from the loopback-only CLI.
// No endpoint override, fixture trust, or automatic uncertain-write retries.
func cloudConfig(ctx context.Context) (aws.Config, error) {
	if os.Getenv("KURIER_STAGE") != "dev-api" || os.Getenv("AWS_REGION") != "us-east-2" || os.Getenv("KURIER_CONTROL_TABLE") == "" || os.Getenv("KURIER_DYNAMODB_ENDPOINT") != "" {
		return aws.Config{}, errors.New("invalid development cloud configuration")
	}
	if err := os.Setenv("AWS_IGNORE_CONFIGURED_ENDPOINT_URLS", "true"); err != nil {
		return aws.Config{}, err
	}
	return config.LoadDefaultConfig(ctx, config.WithRegion("us-east-2"), config.WithRetryer(func() aws.Retryer { return aws.NopRetryer{} }))
}
func cloudMain() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cfg, err := cloudConfig(ctx)
	if err != nil {
		log.Fatal("cloud configuration unavailable")
	}
	store := ownership.NewStore(dynamodb.NewFromConfig(cfg), os.Getenv("KURIER_CONTROL_TABLE"), "dev-api")
	if os.Getenv("KURIER_PROTECTED_TABLE") != "" {
		store.ConfigureProtected(os.Getenv("KURIER_PROTECTED_TABLE"), ownership.NewEnvelopeCipher(kms.NewFromConfig(cfg), os.Getenv("KURIER_KMS_KEY_ARN"), "dev-api"))
	}
	if os.Getenv("KURIER_MAINTENANCE") == "empty-projects" {
		lambda.Start(func(ctx context.Context, _ json.RawMessage) (ownership.CleanupSummary, error) {
			result, err := store.CleanupPending(ctx)
			if err != nil {
				return result, errors.New("empty-project maintenance unavailable; durable work retained")
			}
			return result, nil
		})
		return
	}
	parameter, err := ssm.NewFromConfig(cfg).GetParameter(ctx, &ssm.GetParameterInput{Name: aws.String(os.Getenv("KURIER_CURSOR_KEY_PARAMETER")), WithDecryption: aws.Bool(true)})
	if err != nil || parameter.Parameter == nil {
		log.Fatal("cursor key unavailable")
	}
	getenv := func(name string) string {
		if name == "KURIER_CURSOR_KEY_BASE64" {
			return aws.ToString(parameter.Parameter.Value)
		}
		return os.Getenv(name)
	}
	handler, err := runtimeHandler(store, getenv)
	if err != nil {
		log.Fatal("cloud handler configuration invalid")
	}
	lambda.Start(proxyHTTPV2(handler))
}
