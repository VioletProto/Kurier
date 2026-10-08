package ownership

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type SecretCollection struct {
	Items      []SecretMetadata `json:"items"`
	NextCursor *string          `json:"nextCursor"`
}

func (st *Store) ListSecrets(ctx context.Context, user, project string, limit int, raw string, signing []byte) (SecretCollection, error) {
	out := SecretCollection{Items: []SecretMetadata{}}
	stage, _, _, err := st.secretContext(ctx, user, project)
	if err != nil {
		return out, err
	}
	if limit < 1 || limit > 100 || len(signing) < 32 {
		return out, invalidConfiguration()
	}
	now := st.now()
	c := projectCursor{Owner: user, Stage: st.stage, Generation: stage.RecoveryGeneration, Type: "secrets:" + project, Index: "Protected", IssuedAt: now.Unix(), ExpiresAt: now.Add(24 * time.Hour).Unix()}
	var start map[string]types.AttributeValue
	if raw != "" {
		c, err = decodeCursor(raw, signing)
		if err != nil {
			return out, err
		}
		if c.Owner != user || c.Stage != st.stage || c.Generation != stage.RecoveryGeneration || c.Type != "secrets:"+project || c.Index != "Protected" || c.ExpiresAt <= now.Unix() || c.IssuedAt > now.Unix() || c.ExpiresAt-c.IssuedAt != 86400 || c.Last.PK != "P#"+project || !strings.HasPrefix(c.Last.SK, "SECRET#") {
			return out, apiError(400, "invalid_request", "Invalid or expired cursor.")
		}
		start = key(c.Last.PK, c.Last.SK)
	}
	for i := 0; i < 5; i++ {
		page, e := st.db.Query(ctx, &dynamodb.QueryInput{TableName: aws.String(st.protectedTable), KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :prefix)"), ExpressionAttributeValues: map[string]types.AttributeValue{":pk": s("P#" + project), ":prefix": s("SECRET#")}, ConsistentRead: aws.Bool(true), ExclusiveStartKey: start, Limit: aws.Int32(int32(limit - len(out.Items)))})
		if e != nil {
			return out, unavailable()
		}
		for _, item := range page.Items {
			r, ok := knownProtected(item, project)
			if ok {
				out.Items = append(out.Items, r.metadata())
			}
		}
		start = page.LastEvaluatedKey
		if len(start) == 0 || len(out.Items) == limit {
			break
		}
	}
	if len(start) > 0 {
		c.Last = cursorKey{PK: "P#" + project, SK: start["SK"].(*types.AttributeValueMemberS).Value}
		next, e := encodeCursor(c, signing)
		if e != nil {
			return out, unavailable()
		}
		out.NextCursor = &next
	}
	if _, _, _, err = st.secretContext(ctx, user, project); err != nil {
		return SecretCollection{}, err
	}
	return out, nil
}
func (a *Server) listSecrets(w http.ResponseWriter, r *http.Request, u User) error {
	p := r.PathValue("projectId")
	if _, _, _, err := a.store.secretContext(r.Context(), u.UserID, p); err != nil {
		return err
	}
	q, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return invalidConfiguration()
	}
	limit := 25
	for k, v := range q {
		if len(v) != 1 || (k != "limit" && k != "cursor") {
			return invalidConfiguration()
		}
	}
	if q.Has("limit") {
		limit, err = strconv.Atoi(q.Get("limit"))
		if err != nil {
			return invalidConfiguration()
		}
	}
	if q.Has("cursor") && q.Get("cursor") == "" {
		return invalidConfiguration()
	}
	page, err := a.store.ListSecrets(r.Context(), u.UserID, p, limit, q.Get("cursor"), a.cursorSecret)
	if err != nil {
		return err
	}
	success(w, 200, page)
	return nil
}
func (a *Server) mutateSecret(w http.ResponseWriter, r *http.Request, u User) error {
	p, id := r.PathValue("projectId"), r.PathValue("secretId")
	if _, _, _, err := a.store.secretContext(r.Context(), u.UserID, p); err != nil {
		return err
	}
	current, err := a.store.secret(r.Context(), p, id)
	if err != nil || current.State != "active" {
		if err == nil {
			err = missing()
		}
		return err
	}
	expected, err := precondition(r)
	if err != nil {
		return err
	}
	if r.Method == "DELETE" {
		if r.ContentLength != 0 || r.TransferEncoding != nil {
			return invalidConfiguration()
		}
		if err = a.store.RevokeSecret(r.Context(), u.UserID, p, id, expected); err != nil {
			return err
		}
		w.WriteHeader(204)
		return nil
	}
	raw, err := readRequestJSON(w, r)
	if err != nil {
		return err
	}
	var input struct {
		Value *string `json:"value"`
	}
	if uniqueJSON(raw) != nil || strictDecode(raw, &input) != nil || input.Value == nil {
		return invalidConfiguration()
	}
	result, err := a.store.ReplaceSecret(r.Context(), u.UserID, p, id, *input.Value, expected)
	if err != nil {
		return err
	}
	w.Header().Set("ETag", strconv.Quote(strconv.FormatInt(result.Revision, 10)))
	success(w, 200, map[string]any{"secret": result})
	return nil
}
