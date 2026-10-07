package ownership

import (
	"context"
	"encoding/json"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"strings"
	"time"
)

type RequestCollection struct {
	Items      []SavedRequest `json:"items"`
	NextCursor *string        `json:"nextCursor"`
}

func (st *Store) ListRequests(ctx context.Context, userID, projectID string, limit int, raw string, secret []byte) (RequestCollection, error) {
	out := RequestCollection{Items: []SavedRequest{}}
	if limit < 1 || limit > 100 || len(secret) < 32 {
		return out, apiError(400, "invalid_request", "Invalid pagination parameters.")
	}
	stage, _, _, err := st.requestContext(ctx, userID, projectID)
	if err != nil {
		return out, err
	}
	if _, err = st.userRecord(ctx, userID); err != nil {
		return out, err
	}
	now := st.now()
	c := projectCursor{Owner: userID, Stage: st.stage, Generation: stage.RecoveryGeneration, Type: "requests:" + projectID, Index: "GSI1", HighWater: timestamp(now) + "#~", IssuedAt: now.Unix(), ExpiresAt: now.Add(24 * time.Hour).Unix()}
	var start map[string]types.AttributeValue
	if raw != "" {
		c, err = decodeCursor(raw, secret)
		if err != nil {
			return out, err
		}
		if c.Owner != userID || c.Stage != st.stage || c.Generation != stage.RecoveryGeneration || c.Type != "requests:"+projectID || c.Index != "GSI1" || c.ExpiresAt <= now.Unix() || c.IssuedAt > now.Unix() || c.ExpiresAt-c.IssuedAt != 86400 || c.HighWater == "" || c.Last.PK != "P#"+projectID || !strings.HasPrefix(c.Last.SK, "REQ#") || c.Last.LPK != "P#"+projectID+"#REQUEST" || c.Last.LSK == "" || c.Last.LSK > c.HighWater {
			return out, apiError(400, "invalid_request", "Invalid or expired cursor.")
		}
		start = map[string]types.AttributeValue{"PK": s(c.Last.PK), "SK": s(c.Last.SK), "LPK": s(c.Last.LPK), "LSK": s(c.Last.LSK)}
	}
	responseBytes := 0
pagesLoop:
	for pages := 0; pages < 5; pages++ {
		result, e := st.db.Query(ctx, &dynamodb.QueryInput{TableName: aws.String(st.table), IndexName: aws.String("GSI1"), KeyConditionExpression: aws.String("LPK = :owner AND LSK <= :high"), ExpressionAttributeValues: map[string]types.AttributeValue{":owner": s("P#" + projectID + "#REQUEST"), ":high": s(c.HighWater)}, ScanIndexForward: aws.Bool(false), Limit: aws.Int32(int32(limit - len(out.Items))), ExclusiveStartKey: start})
		if e != nil {
			return out, unavailable()
		}
		previousKey := start
		for _, item := range result.Items {
			var k cursorKey
			if attributevalue.UnmarshalMap(item, &k) != nil || k.PK != "P#"+projectID || !strings.HasPrefix(k.SK, "REQ#") {
				return out, unavailable()
			}
			r, e := st.GetRequest(ctx, userID, projectID, strings.TrimPrefix(k.SK, "REQ#"))
			if e != nil {
				if ae, ok := e.(*APIError); ok && ae.Status == 404 {
					previousKey = item
					continue
				}
				return out, e
			}
			encoded, err := json.Marshal(r)
			if err != nil {
				return out, unavailable()
			}
			if responseBytes+len(encoded) > 2*1024*1024 {
				start = previousKey
				break pagesLoop
			}
			responseBytes += len(encoded)
			out.Items = append(out.Items, r)
			previousKey = item
		}
		start = result.LastEvaluatedKey
		if len(start) == 0 {
			if _, _, _, err := st.requestContext(ctx, userID, projectID); err != nil {
				return RequestCollection{Items: []SavedRequest{}}, err
			}
			return out, nil
		}
		if len(out.Items) == limit {
			break
		}
	}
	if attributevalue.UnmarshalMap(start, &c.Last) != nil {
		return out, unavailable()
	}
	next, err := encodeCursor(c, secret)
	if err != nil {
		return out, unavailable()
	}
	out.NextCursor = &next
	if _, _, _, err := st.requestContext(ctx, userID, projectID); err != nil {
		return RequestCollection{Items: []SavedRequest{}}, err
	}
	return out, nil
}
