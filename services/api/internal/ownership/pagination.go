package ownership

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type Collection struct {
	Items      []Project `json:"items"`
	NextCursor *string   `json:"nextCursor"`
}
type cursorKey struct{ PK, SK, LPK, LSK string }
type projectCursor struct {
	Owner, Stage, Generation, Type, Index, HighWater string
	IssuedAt, ExpiresAt                              int64
	Last                                             cursorKey
}

func encodeCursor(c projectCursor, secret []byte) (string, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(b)
	return base64.RawURLEncoding.EncodeToString(b) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}
func decodeCursor(raw string, secret []byte) (projectCursor, error) {
	bad := apiError(400, "invalid_request", "Invalid or expired cursor.")
	if len(raw) > 4096 {
		return projectCursor{}, bad
	}
	parts := strings.Split(raw, ".")
	if len(parts) != 2 {
		return projectCursor{}, bad
	}
	b, e1 := base64.RawURLEncoding.DecodeString(parts[0])
	sig, e2 := base64.RawURLEncoding.DecodeString(parts[1])
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(b)
	if e1 != nil || e2 != nil || !hmac.Equal(sig, mac.Sum(nil)) {
		return projectCursor{}, bad
	}
	var c projectCursor
	if json.Unmarshal(b, &c) != nil {
		return projectCursor{}, bad
	}
	return c, nil
}
func (st *Store) ListProjects(ctx context.Context, userID string, limit int, raw string, secret []byte) (Collection, error) {
	out := Collection{Items: []Project{}}
	if limit < 1 || limit > 100 || len(secret) < 32 {
		return out, apiError(400, "invalid_request", "Invalid pagination parameters.")
	}
	stage, err := st.activeStage(ctx)
	if err != nil {
		return out, err
	}
	if _, err = st.userRecord(ctx, userID); err != nil {
		return out, err
	}
	now := st.now()
	c := projectCursor{Owner: userID, Stage: st.stage, Generation: stage.RecoveryGeneration, Type: "projects", Index: "GSI1", HighWater: timestamp(now) + "#~", IssuedAt: now.Unix(), ExpiresAt: now.Add(24 * time.Hour).Unix()}
	var start map[string]types.AttributeValue
	if raw != "" {
		c, err = decodeCursor(raw, secret)
		if err != nil {
			return out, err
		}
		if c.Owner != userID || c.Stage != st.stage || c.Generation != stage.RecoveryGeneration || c.Type != "projects" || c.Index != "GSI1" || c.ExpiresAt <= now.Unix() || c.IssuedAt > now.Unix() || c.ExpiresAt-c.IssuedAt != 86400 || c.HighWater == "" || !strings.HasPrefix(c.Last.PK, "P#") || c.Last.SK != "META" || c.Last.LPK != "U#"+userID+"#PROJECT" || c.Last.LSK == "" || c.Last.LSK > c.HighWater {
			return out, apiError(400, "invalid_request", "Invalid or expired cursor.")
		}
		start = map[string]types.AttributeValue{"PK": s(c.Last.PK), "SK": s(c.Last.SK), "LPK": s(c.Last.LPK), "LSK": s(c.Last.LSK)}
	}
	for pages := 0; pages < 5; pages++ {
		result, e := st.db.Query(ctx, &dynamodb.QueryInput{TableName: aws.String(st.table), IndexName: aws.String("GSI1"), KeyConditionExpression: aws.String("LPK = :owner AND LSK <= :high"), ExpressionAttributeValues: map[string]types.AttributeValue{":owner": s("U#" + userID + "#PROJECT"), ":high": s(c.HighWater)}, ScanIndexForward: aws.Bool(false), Limit: aws.Int32(int32(limit - len(out.Items))), ExclusiveStartKey: start})
		if e != nil {
			return out, unavailable()
		}
		for _, item := range result.Items {
			var k cursorKey
			if attributevalue.UnmarshalMap(item, &k) != nil || !strings.HasPrefix(k.PK, "P#") || k.SK != "META" {
				return out, unavailable()
			}
			r, e := st.owned(ctx, userID, strings.TrimPrefix(k.PK, "P#"), false)
			if e != nil {
				if ae, ok := e.(*APIError); ok && ae.Status == 404 {
					continue
				}
				return out, e
			}
			out.Items = append(out.Items, r.project())
		}
		start = result.LastEvaluatedKey
		if len(start) == 0 {
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
	return out, nil
}
