package browsersession

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dt "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type DynamoAPI interface {
	GetItem(context.Context, *dynamodb.GetItemInput, ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	TransactWriteItems(context.Context, *dynamodb.TransactWriteItemsInput, ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error)
	UpdateItem(context.Context, *dynamodb.UpdateItemInput, ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
	Scan(context.Context, *dynamodb.ScanInput, ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error)
}
type DynamoStore struct {
	Client                DynamoAPI
	Table, Control, Stage string
}

func avs(v string) dt.AttributeValue { return &dt.AttributeValueMemberS{Value: v} }
func avn(v int64) dt.AttributeValue {
	return &dt.AttributeValueMemberN{Value: strconv.FormatInt(v, 10)}
}
func (s *DynamoStore) Generation(ctx context.Context) (string, error) {
	o, e := s.Client.GetItem(ctx, &dynamodb.GetItemInput{TableName: aws.String(s.Control), Key: map[string]dt.AttributeValue{"PK": avs("STAGE#" + s.Stage), "SK": avs("META")}, ConsistentRead: aws.Bool(true)})
	if e != nil {
		return "", ErrUnavailable
	}
	var v struct {
		State      string `dynamodbav:"state"`
		Generation string `dynamodbav:"recoveryGeneration"`
	}
	if attributevalue.UnmarshalMap(o.Item, &v) != nil || v.State != "active" || v.Generation == "" {
		return "", ErrUnavailable
	}
	return v.Generation, nil
}
func (s *DynamoStore) Get(ctx context.Context, key string) (*Record, error) {
	o, e := s.Client.GetItem(ctx, &dynamodb.GetItemInput{TableName: aws.String(s.Table), Key: map[string]dt.AttributeValue{"PK": avs(key)}, ConsistentRead: aws.Bool(true)})
	if e != nil {
		return nil, ErrUnavailable
	}
	if len(o.Item) == 0 {
		return nil, nil
	}
	var v Record
	if attributevalue.UnmarshalMap(o.Item, &v) != nil || v.PK != key {
		return nil, ErrUnavailable
	}
	return &v, nil
}
func (s *DynamoStore) Commit(ctx context.Context, gen string, before, after map[string]*Record) error {
	actions := []dt.TransactWriteItem{{ConditionCheck: &dt.ConditionCheck{TableName: aws.String(s.Control), Key: map[string]dt.AttributeValue{"PK": avs("STAGE#" + s.Stage), "SK": avs("META")}, ConditionExpression: aws.String("#state = :active AND recoveryGeneration = :gen"), ExpressionAttributeNames: map[string]string{"#state": "state"}, ExpressionAttributeValues: map[string]dt.AttributeValue{":active": avs("active"), ":gen": avs(gen)}}}}
	for key, v := range after {
		old, ok := before[key]
		if !ok || v == nil || v.PK != key {
			return ErrUnavailable
		}
		item, e := attributevalue.MarshalMap(v)
		if e != nil {
			return ErrUnavailable
		}
		put := &dt.Put{TableName: aws.String(s.Table), Item: item, ConditionExpression: aws.String("attribute_not_exists(PK)")}
		if old != nil {
			put.ConditionExpression = aws.String("#version = :version")
			put.ExpressionAttributeNames = map[string]string{"#version": "version"}
			put.ExpressionAttributeValues = map[string]dt.AttributeValue{":version": avn(old.Version)}
		}
		actions = append(actions, dt.TransactWriteItem{Put: put})
	}
	_, e := s.Client.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{TransactItems: actions})
	if e != nil {
		var canceled *dt.TransactionCanceledException
		if errors.As(e, &canceled) {
			for i, r := range canceled.CancellationReasons {
				if aws.ToString(r.Code) == "ConditionalCheckFailed" {
					if i == 0 {
						return ErrUnavailable
					}
					return ErrConflict
				}
			}
		}
		return ErrUnavailable
	}
	return nil
}
func (s *DynamoStore) Rate(ctx context.Context, key string, window int64, limit int) error {
	_, e := s.Client.UpdateItem(ctx, &dynamodb.UpdateItemInput{TableName: aws.String(s.Table), Key: map[string]dt.AttributeValue{"PK": avs("R#" + key + "#" + strconv.FormatInt(window, 10))}, UpdateExpression: aws.String("SET #ttl = :ttl ADD #count :one"), ConditionExpression: aws.String("attribute_not_exists(#count) OR #count < :limit"), ExpressionAttributeNames: map[string]string{"#ttl": "ttl", "#count": "count"}, ExpressionAttributeValues: map[string]dt.AttributeValue{":ttl": avn((window + 2) * 60), ":one": avn(1), ":limit": avn(int64(limit))}})
	if e != nil {
		var conditional *dt.ConditionalCheckFailedException
		if errors.As(e, &conditional) {
			return ErrRate
		}
		return ErrUnavailable
	}
	return nil
}
func (s *DynamoStore) Scan(ctx context.Context, cursor string) ([]Record, string, error) {
	in := &dynamodb.ScanInput{TableName: aws.String(s.Table), Limit: aws.Int32(100), ConsistentRead: aws.Bool(true)}
	if cursor != "" {
		in.ExclusiveStartKey = map[string]dt.AttributeValue{"PK": avs(cursor)}
	}
	out, e := s.Client.Scan(ctx, in)
	if e != nil {
		return nil, "", ErrUnavailable
	}
	var rows []Record
	if attributevalue.UnmarshalListOfMaps(out.Items, &rows) != nil {
		return nil, "", ErrUnavailable
	}
	next := ""
	if k, ok := out.LastEvaluatedKey["PK"].(*dt.AttributeValueMemberS); ok {
		next = k.Value
	}
	return rows, next, nil
}

// Maintain processes one bounded page, persisting the continuation in the same
// disposable table so new/expired rate buckets cannot starve revoke retries.
func (s *Service) Maintain(ctx context.Context) error {
	scanner, ok := s.Store.(interface {
		Scan(context.Context, string) ([]Record, string, error)
	})
	if !ok {
		return ErrUnavailable
	}
	gen, e := s.Store.Generation(ctx)
	if e != nil {
		return e
	}
	cursor, e := s.Store.Get(ctx, "M#sweep")
	if e != nil {
		return e
	}
	start := ""
	if cursor != nil {
		start = cursor.SessionKey
	}
	rows, next, e := scanner.Scan(ctx, start)
	if e != nil {
		return e
	}
	last, work := start, 0
	for _, row := range rows {
		if deadline, ok := ctx.Deadline(); work >= 3 || ok && time.Until(deadline) < 4*time.Second {
			next = last
			break
		}
		r := clone(&row)
		last = r.PK
		if r.Kind != "session" || r.Generation != gen || len(r.Bundle) == 0 {
			continue
		}
		if r.Expires <= s.now().Unix() {
			work++
			nr := clone(r)
			nr.Version++
			nr.Status = "revoked"
			nr.Bundle = nil
			_ = s.Store.Commit(ctx, gen, map[string]*Record{r.PK: r}, map[string]*Record{r.PK: nr})
			continue
		}
		if r.Status == "revoked" {
			work++
			_, _ = s.revoke(ctx, r)
			continue
		}
		if r.Status == "refreshing" && r.LeaseUntil <= s.now().Unix() {
			work++
			nr := clone(r)
			nr.Version++
			nr.Status = "revoked"
			if s.Store.Commit(ctx, gen, map[string]*Record{r.PK: r}, map[string]*Record{r.PK: nr}) == nil {
				_, _ = s.revoke(ctx, nr)
			}
		}
	}
	v := &Record{PK: "M#sweep", Kind: "maintenance", Version: 1, Generation: gen, SessionKey: next, TTL: s.now().Unix() + 86400}
	if cursor != nil {
		v.Version = cursor.Version + 1
	}
	return s.Store.Commit(ctx, gen, map[string]*Record{v.PK: cursor}, map[string]*Record{v.PK: v})
}
