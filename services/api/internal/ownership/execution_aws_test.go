//go:build integration && aws

package ownership

import (
	"os"

	"github.com/VioletProto/Kurier/services/api/execution"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

// Historical AWS fixtures must configure the accepted execution consumer after
// capability activation, just like deployed API and maintenance binaries.
func configureAWSExecutionFixture(store *Store, cfg aws.Config) {
	store.ConfigureExecutions(&execution.Service{
		DB: store.db, Objects: s3.NewFromConfig(cfg), Queue: sqs.NewFromConfig(cfg),
		KMS: kms.NewFromConfig(cfg), Table: store.table, Protected: os.Getenv("KURIER_AWS_TEST_PROTECTED"),
		Bucket: os.Getenv("KURIER_AWS_TEST_BUCKET"), QueueURL: os.Getenv("KURIER_AWS_TEST_QUEUE"),
		KeyARN: os.Getenv("KURIER_AWS_TEST_KEY"), Stage: "dev-api",
	})
}
