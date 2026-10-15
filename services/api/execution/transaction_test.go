package execution

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dt "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type transactionProbe struct {
	Database
	calls      int
	first      []byte
	firstToken string
	fault      func(int) error
	t          *testing.T
}

func (d *transactionProbe) TransactWriteItems(_ context.Context, in *dynamodb.TransactWriteItemsInput, _ ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error) {
	d.calls++
	raw, err := json.Marshal(in)
	if err != nil {
		d.t.Fatal("transaction encoding failed")
	}
	if d.first == nil {
		d.first = raw
		d.firstToken = aws.ToString(in.ClientRequestToken)
	} else if !bytes.Equal(d.first, raw) || d.firstToken != aws.ToString(in.ClientRequestToken) {
		d.t.Fatal("retry changed transaction/token")
	}
	return &dynamodb.TransactWriteItemsOutput{}, d.fault(d.calls)
}
func canceledCodes(codes ...string) error {
	reasons := []dt.CancellationReason{}
	for _, code := range codes {
		reasons = append(reasons, dt.CancellationReason{Code: aws.String(code), Message: aws.String("sensitive diagnostics must not escape")})
	}
	return &dt.TransactionCanceledException{CancellationReasons: reasons}
}
func TestTransactionConflictRetryPreservesGuardsAndToken(t *testing.T) {
	for _, test := range []struct {
		name  string
		err   error
		calls int
	}{
		{"transient", canceledCodes("None", "TransactionConflict"), 3},
		{"direct", &dt.TransactionConflictException{}, 3},
		{"conditional", canceledCodes("ConditionalCheckFailed"), 1},
		{"mixed", canceledCodes("TransactionConflict", "ConditionalCheckFailed"), 1},
		{"validation", canceledCodes("TransactionConflict", "ValidationError"), 1},
		{"missing", canceledCodes(), 1},
		{"missing-code", canceledCodes("TransactionConflict", ""), 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			db := &transactionProbe{t: t, fault: func(n int) error {
				if n < 3 {
					return test.err
				}
				return nil
			}}
			s := &Service{DB: db}
			action := dt.TransactWriteItem{ConditionCheck: &dt.ConditionCheck{TableName: aws.String("guarded"), Key: keys("project", "META"), ConditionExpression: aws.String("version = :v"), ExpressionAttributeValues: map[string]dt.AttributeValue{":v": num(7)}}}
			err := s.transact(context.Background(), []dt.TransactWriteItem{action})
			if db.calls != test.calls || (err == nil) != (test.calls == 3) {
				t.Fatalf("calls=%d success=%t", db.calls, err == nil)
			}
		})
	}
}
func TestTransactionConflictRetryBoundAndCancellation(t *testing.T) {
	db := &transactionProbe{t: t, fault: func(int) error { return canceledCodes("TransactionConflict") }}
	s := &Service{DB: db}
	actions := []dt.TransactWriteItem{{Put: &dt.Put{TableName: aws.String("workflow"), Item: map[string]dt.AttributeValue{"runId": str(id())}}}}
	if s.transact(context.Background(), actions) == nil || db.calls != 6 {
		t.Fatalf("unbounded retries: %d", db.calls)
	}
	db.calls = 0
	db.first = nil
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	if !errors.Is(s.transact(ctx, actions), context.DeadlineExceeded) || db.calls != 1 {
		t.Fatal("retry ignored cancellation")
	}
}
