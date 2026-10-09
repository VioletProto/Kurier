package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/VioletProto/Kurier/services/api/execution"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"os"
	"strings"
	"time"
)

func cloudMain() {
	if os.Getenv("KURIER_STAGE") != "dev-api" || os.Getenv("AWS_REGION") != "us-east-2" {
		panic("worker configuration unavailable")
	}
	if os.Getenv("KURIER_CONTROLLED_ENDPOINT") == "true" {
		lambda.Start(controlledEndpoint)
		return
	}
	_ = os.Setenv("AWS_IGNORE_CONFIGURED_ENDPOINT_URLS", "true")
	cfg, e := config.LoadDefaultConfig(context.Background(), config.WithRegion("us-east-2"), config.WithRetryer(func() aws.Retryer { return aws.NopRetryer{} }))
	if e != nil {
		panic("worker configuration unavailable")
	}
	service := &execution.Service{DB: dynamodb.NewFromConfig(cfg), Objects: s3.NewFromConfig(cfg), Queue: sqs.NewFromConfig(cfg), KMS: kms.NewFromConfig(cfg), Table: os.Getenv("KURIER_CONTROL_TABLE"), Protected: os.Getenv("KURIER_PROTECTED_TABLE"), Bucket: os.Getenv("KURIER_EVIDENCE_BUCKET"), QueueURL: os.Getenv("KURIER_EXECUTION_QUEUE_URL"), KeyARN: os.Getenv("KURIER_KMS_KEY_ARN"), Stage: "dev-api"}
	lambda.Start(func(ctx context.Context, event events.SQSEvent) (events.SQSEventResponse, error) {
		result := events.SQSEventResponse{BatchItemFailures: []events.SQSBatchItemFailure{}}
		for _, record := range event.Records {
			var message execution.Message
			if execution.StrictJSON([]byte(record.Body), &message) != nil {
				continue
			}
			if e := service.Process(ctx, message); e != nil {
				result.BatchItemFailures = append(result.BatchItemFailures, events.SQSBatchItemFailure{ItemIdentifier: record.MessageId})
			}
		}
		return result, nil
	})
}

// A development-only owned endpoint: no persistence, tracing or payload logs,
// no AWS resource permissions. Echo values exist only in request/response memory.
func controlledEndpoint(_ context.Context, event events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	mode := event.PathParameters["mode"]
	headers := map[string]string{"Content-Type": "application/json", "Cache-Control": "no-store"}
	status := 200
	var payload []byte
	switch mode {
	case "credential-names":
		// Fixed non-secret markers exercise name-based masking without protected
		// inputs. Never echo caller values in this response-header fixture.
		fields := map[string]string{"public": "keep-public"}
		for _, name := range []string{"key", "auth", "pwd"} {
			fields[name] = "unmapped-fixture-" + name
			headers[name] = fields[name]
		}
		payload, _ = json.Marshal(map[string]any{"fields": fields, "query": fields})
	case "echo":
		body := event.Body
		if event.IsBase64Encoded {
			raw, e := base64.StdEncoding.DecodeString(body)
			if e != nil {
				return events.APIGatewayV2HTTPResponse{}, errors.New("invalid controlled request")
			}
			body = string(raw)
		}
		echo := map[string]any{"body": body, "headers": event.Headers, "query": event.QueryStringParameters, "ok": true}
		payload, _ = json.Marshal(echo)
	case "delay":
		time.Sleep(5 * time.Second)
		payload = []byte(`{"delayed":true}`)
	case "http-error":
		status = 500
		payload = []byte(`{"ok":false}`)
	case "redirect":
		status = 302
		headers["Location"] = "http://127.0.0.1/metadata"
		payload = []byte(`{"redirect":true}`)
	case "oversize":
		headers["Content-Type"] = "text/plain"
		payload = []byte(strings.Repeat("x", execution.BodyLimit+1024))
	case "gzip-limit":
		headers["Content-Type"] = "text/plain"
		headers["Content-Encoding"] = "gzip"
		var b bytes.Buffer
		writer := gzip.NewWriter(&b)
		_, _ = writer.Write([]byte(strings.Repeat("x", execution.BodyLimit+1024)))
		_ = writer.Close()
		payload = b.Bytes()
	case "html":
		headers["Content-Type"] = "text/html"
		payload = []byte(`<script>alert("fixture")</script>`)
	default:
		status = 404
		payload = []byte(`{"unavailable":true}`)
	}
	return events.APIGatewayV2HTTPResponse{StatusCode: status, Headers: headers, Body: base64.StdEncoding.EncodeToString(payload), IsBase64Encoded: true}, nil
}
