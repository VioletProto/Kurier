package execution

import (
	"context"
	"crypto/hmac"
	"encoding/base64"
	"encoding/json"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dt "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"strings"
	"time"
)

type historyCursor struct {
	Owner, Project, Stage, Generation, Request, Status, HighWater string
	Expires                                                       int64
	Last                                                          map[string]string
}
type History struct {
	Items      []View  `json:"items"`
	NextCursor *string `json:"nextCursor"`
}

func (s *Service) History(ctx context.Context, owner, project, request, status string, limit int, token string) (History, error) {
	out := History{Items: []View{}}
	stage, e := s.stage(ctx)
	if e != nil {
		return out, e
	}
	if _, e = s.project(ctx, owner, project); e != nil {
		return out, e
	}
	if limit < 1 || limit > 100 || request != "" && !uuidPattern.MatchString(request) || status != "" && status != "queued" && status != "claimed" && status != "running" && !terminal(status) {
		return out, failure(400, "invalid_request")
	}
	c := historyCursor{Owner: owner, Project: project, Stage: s.Stage, Generation: stage.RecoveryGeneration, Request: request, Status: status, HighWater: stamp(s.now()) + "#~", Expires: s.now().Add(24 * time.Hour).Unix()}
	var start map[string]dt.AttributeValue
	if token != "" {
		parts := strings.Split(token, ".")
		if len(parts) != 2 || len(token) > 8192 {
			return out, failure(400, "invalid_request")
		}
		raw, e := base64.RawURLEncoding.DecodeString(parts[0])
		if e != nil || !hmac.Equal([]byte(parts[1]), []byte(s.digest("execution-history-cursor-v1", string(raw)))) || StrictJSON(raw, &c) != nil || c.Owner != owner || c.Project != project || c.Stage != s.Stage || c.Generation != stage.RecoveryGeneration || c.Request != request || c.Status != status || c.Expires <= s.now().Unix() || c.Expires > s.now().Add(24*time.Hour).Unix() || c.Last["PK"] != "P#"+project || !strings.HasPrefix(c.Last["SK"], "EXEC#") {
			return out, failure(400, "invalid_request")
		}
		start = map[string]dt.AttributeValue{}
		for k, v := range c.Last {
			if k != "PK" && k != "SK" && k != "LPK" && k != "LSK" && k != "HPK" && k != "HSK" {
				return out, failure(400, "invalid_request")
			}
			start[k] = str(v)
		}
	}
	index, keyName, sortName, partition := "GSI1", "LPK", "LSK", "P#"+project+"#EXEC"
	if request != "" {
		index, keyName, sortName, partition = "GSI3", "HPK", "HSK", "P#"+project+"#REQ#"+request
	}
	for page := 0; page < 5; page++ {
		result, e := s.DB.Query(ctx, &dynamodb.QueryInput{TableName: &s.Table, IndexName: &index, KeyConditionExpression: aws.String(keyName + " = :pk AND " + sortName + " <= :hi"), ExpressionAttributeValues: map[string]dt.AttributeValue{":pk": str(partition), ":hi": str(c.HighWater)}, ScanIndexForward: aws.Bool(false), Limit: aws.Int32(int32(limit - len(out.Items))), ExclusiveStartKey: start})
		if e != nil {
			return out, safeError()
		}
		for _, item := range result.Items {
			var candidate row
			if attributevalue.UnmarshalMap(item, &candidate) != nil || candidate.PK != "P#"+project || !strings.HasPrefix(candidate.SK, "EXEC#") {
				return out, safeError()
			}
			v, e := s.Detail(ctx, owner, project, strings.TrimPrefix(candidate.SK, "EXEC#"))
			if e != nil {
				if missingRow(e) {
					continue
				}
				return out, e
			}
			if status != "" && v.Status != status {
				continue
			}
			v.Configuration = nil
			out.Items = append(out.Items, v)
		}
		start = result.LastEvaluatedKey
		if len(start) == 0 || len(out.Items) >= limit {
			break
		}
	}
	if len(start) > 0 {
		c.Last = map[string]string{}
		for k, v := range start {
			value, ok := v.(*dt.AttributeValueMemberS)
			if !ok {
				return out, safeError()
			}
			c.Last[k] = value.Value
		}
		raw, _ := json.Marshal(c)
		next := base64.RawURLEncoding.EncodeToString(raw) + "." + s.digest("execution-history-cursor-v1", string(raw))
		out.NextCursor = &next
	}
	if _, e = s.project(ctx, owner, project); e != nil {
		return History{Items: []View{}}, e
	}
	return out, nil
}
