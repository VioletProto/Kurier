package execution

import (
	"context"
	"encoding/json"
	wire "github.com/VioletProto/Kurier/services/request-runtime/mcpwire"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dt "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"strings"
)

func (s *Service) mcpOwnedProjects(ctx context.Context, c row, in wire.Input, cur mcpCursor, tap *mcpReadSet) (*wire.Data, error) {
	out := &wire.Data{Items: []json.RawMessage{}}
	var start map[string]dt.AttributeValue
	if cur.LastID != "" {
		if !validProjectPosition(cur.LastTime, cur.LastID) || cur.LastTime > cur.High {
			return nil, failure(400, "invalid_request")
		}
		start = map[string]dt.AttributeValue{"PK": str("P#" + cur.LastID), "SK": str("META"), "LPK": str("U#" + c.OwnerID + "#PROJECT"), "LSK": str(cur.LastTime)}
	}
	finish := func() (*wire.Data, error) { var e error; out.NextCursor, e = s.mcpCursorEncode(cur); return out, e }
	for page := 0; page < 5; page++ {
		q, e := s.DB.Query(ctx, &dynamodb.QueryInput{TableName: &s.Table, IndexName: aws.String("GSI1"), KeyConditionExpression: aws.String("LPK = :p AND LSK <= :hi"), ExpressionAttributeValues: map[string]dt.AttributeValue{":p": str("U#" + c.OwnerID + "#PROJECT"), ":hi": str(cur.High)}, ScanIndexForward: aws.Bool(false), Limit: aws.Int32(25), ExclusiveStartKey: start})
		if e != nil {
			return nil, safeError()
		}
		for _, item := range q.Items {
			// Leave room for credential/stage/user/audit and size accounting. Stop
			// before reading an unprocessed candidate; cursor never skips that row.
			if len(out.Items) >= in.Limit || len(tap.items) >= 60 {
				return finish()
			}
			var candidate row
			if attributevalue.UnmarshalMap(item, &candidate) != nil || !strings.HasPrefix(candidate.PK, "P#") || candidate.SK != "META" {
				return nil, safeError()
			}
			pid := strings.TrimPrefix(candidate.PK, "P#")
			if !validProjectPosition(candidate.LSK, pid) || candidate.LSK > cur.High {
				return nil, safeError()
			}
			p, e := s.project(ctx, c.OwnerID, pid)
			if e != nil && !missingRow(e) {
				return nil, e
			}
			if e == nil {
				out.Items = append(out.Items, mcpRaw(wire.Project{ProjectID: pid, Name: p.Name, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt}))
			}
			cur.LastID = pid
			cur.LastTime = candidate.LSK
		}
		if len(q.LastEvaluatedKey) == 0 {
			return out, nil
		}
		start = q.LastEvaluatedKey
	}
	return finish()
}
