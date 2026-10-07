package ownership

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type CleanupSummary struct {
	Discovered int `json:"discovered"`
	Completed  int `json:"completed"`
	Pending    int `json:"pending"`
}

func cleanupShard(projectID string) string {
	sum := sha256.Sum256([]byte(projectID))
	return fmt.Sprintf("delete#%d", sum[0]%8)
}

// One bounded page per shard per tick. Durable cursors rotate through blocked
// work so unknown children cannot starve later empty projects. Discovery is
// eventual; only strong base reads and the existing gate CAS authorize cleanup.
func (st *Store) CleanupPending(ctx context.Context) (CleanupSummary, error) {
	var summary CleanupSummary
	stage, err := st.activeStage(ctx)
	if err != nil {
		return summary, err
	}
	for shard := 0; shard < 8; shard++ {
		pk := "MAINTENANCE#" + st.stage
		sk := fmt.Sprintf("DELETE#%d", shard)
		cursor, err := st.get(ctx, pk, sk)
		var ae *APIError
		absent := errors.As(err, &ae) && ae.Status == 404
		if err != nil && !absent {
			return summary, err
		}
		if !absent && cursor.Kind != "cleanupCursor" {
			return summary, unavailable()
		}
		start := map[string]types.AttributeValue{}
		for k, v := range cursor.MaintenanceCursor {
			start[k] = s(v)
		}
		if len(start) == 0 {
			start = nil
		}
		page, err := st.db.Query(ctx, &dynamodb.QueryInput{TableName: aws.String(st.table), IndexName: aws.String("GSI2"), KeyConditionExpression: aws.String("DPK = :pk AND DSK <= :now"), ExpressionAttributeValues: map[string]types.AttributeValue{":pk": s(fmt.Sprintf("delete#%d", shard)), ":now": s(timestamp(st.now()) + "#~")}, ExclusiveStartKey: start, Limit: aws.Int32(25)})
		if err != nil {
			return summary, unavailable()
		}
		if len(page.Items) == 0 && len(page.LastEvaluatedKey) == 0 && len(cursor.MaintenanceCursor) == 0 {
			continue
		}
		for _, item := range page.Items {
			var candidate record
			if attributevalue.UnmarshalMap(item, &candidate) != nil {
				return summary, unavailable()
			}
			work, err := st.get(ctx, candidate.PK, candidate.SK)
			if errors.As(err, &ae) && ae.Status == 404 {
				continue
			}
			if err != nil {
				return summary, err
			}
			if work.Kind != "deletionWork" || work.DPK != fmt.Sprintf("delete#%d", shard) || work.ProjectID == "" {
				continue
			}
			summary.Discovered++
			op, err := st.CleanupProject(ctx, work.ProjectID)
			if err == nil && op.State == "completed" {
				summary.Completed++
			} else {
				summary.Pending++
			}
		}
		next := map[string]string{}
		for k, v := range page.LastEvaluatedKey {
			value, ok := v.(*types.AttributeValueMemberS)
			if !ok {
				return summary, unavailable()
			}
			next[k] = value.Value
		}
		updated := record{PK: pk, SK: sk, Kind: "cleanupCursor", SchemaVersion: 1, Version: cursor.Version + 1, MaintenanceCursor: next}
		condition := "attribute_not_exists(PK)"
		var values map[string]types.AttributeValue
		if !absent {
			condition = "version = :v"
			values = map[string]types.AttributeValue{":v": n(cursor.Version)}
		}
		// version is a reserved word; use the encoded numeric field via an alias.
		action, err := st.put(updated, condition, values)
		if !absent {
			action.Put.ConditionExpression = aws.String("#v = :v")
			action.Put.ExpressionAttributeNames = map[string]string{"#v": "version"}
		}
		if err != nil {
			return summary, unavailable()
		}
		if err = st.transact(ctx, []types.TransactWriteItem{st.stageGuard(stage), action}); err != nil {
			return summary, conflict()
		}
	}
	return summary, nil
}
