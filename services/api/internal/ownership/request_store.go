package ownership

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func (st *Store) requestContext(ctx context.Context, userID, projectID string) (record, record, record, error) {
	stage, err := st.activeStage(ctx)
	if err != nil {
		return record{}, record{}, record{}, err
	}
	if stage.SavedRequestsSchemaVersion != 1 {
		return record{}, record{}, record{}, unavailable()
	}
	user, err := st.userRecord(ctx, userID)
	if err != nil {
		return record{}, record{}, record{}, err
	}
	project, err := st.owned(ctx, userID, projectID, false)
	return stage, user, project, err
}
func (st *Store) requestGate(p record, state string) types.TransactWriteItem {
	return types.TransactWriteItem{Update: &types.Update{TableName: aws.String(st.table), Key: key(p.PK, "META"),
		UpdateExpression: aws.String("SET #v = :next"), ConditionExpression: aws.String("ownerId = :owner AND #state = :state AND #v = :v AND (attribute_not_exists(deletionEpoch) OR deletionEpoch = :epoch)"),
		ExpressionAttributeNames: map[string]string{"#v": "version", "#state": "state"}, ExpressionAttributeValues: map[string]types.AttributeValue{":next": n(p.Version + 1), ":owner": s(p.OwnerID), ":state": s(state), ":v": n(p.Version), ":epoch": n(p.DeletionEpoch)}}}
}
func (st *Store) GetRequest(ctx context.Context, userID, projectID, requestID string) (SavedRequest, error) {
	if _, _, _, err := st.requestContext(ctx, userID, projectID); err != nil {
		return SavedRequest{}, err
	}
	r, err := st.get(ctx, "P#"+projectID, "REQ#"+requestID)
	if err != nil {
		return SavedRequest{}, err
	}
	result, err := r.savedRequest()
	if err != nil {
		return SavedRequest{}, err
	}
	// Recheck after hydration so deletion during the read cannot disclose a child.
	if _, _, _, err = st.requestContext(ctx, userID, projectID); err != nil {
		return SavedRequest{}, err
	}
	return result, nil
}
func (st *Store) CreateRequest(ctx context.Context, userID, projectID string, c RequestConfiguration) (SavedRequest, error) {
	stage, user, p, err := st.requestContext(ctx, userID, projectID)
	if err != nil {
		return SavedRequest{}, err
	}
	if err := validateConfiguration(&c); err != nil {
		return SavedRequest{}, err
	}
	raw, err := configurationJSON(c)
	if err != nil {
		return SavedRequest{}, err
	}
	id := newID()
	now := timestamp(st.now())
	r := record{PK: p.PK, SK: "REQ#" + id, Kind: "request", SchemaVersion: 1, ProjectID: projectID, RequestID: id, State: "active", ConfigurationJSON: string(raw), CreatedAt: now, UpdatedAt: now, LPK: p.PK + "#REQUEST", LSK: now + "#" + id}
	action, err := st.put(r, "attribute_not_exists(PK)", nil)
	if err != nil {
		return SavedRequest{}, unavailable()
	}
	if err = st.transact(ctx, []types.TransactWriteItem{st.stageGuard(stage), st.userGuard(user), st.requestGate(p, "active"), action}); err != nil {
		return SavedRequest{}, st.requestFailure(ctx, userID, p, id, nil, err)
	}
	return r.savedRequest()
}
func (st *Store) PatchRequest(ctx context.Context, userID, projectID, requestID string, raw []byte, expected int64) (SavedRequest, error) {
	stage, user, p, err := st.requestContext(ctx, userID, projectID)
	if err != nil {
		return SavedRequest{}, err
	}
	r, err := st.get(ctx, p.PK, "REQ#"+requestID)
	if err != nil {
		return SavedRequest{}, err
	}
	previous, err := r.savedRequest()
	if err != nil {
		return SavedRequest{}, err
	}
	if r.Revision != expected {
		return SavedRequest{}, requestStale()
	}
	c, err := mergeConfiguration(raw, &previous.RequestConfiguration)
	if err != nil {
		return SavedRequest{}, err
	}
	encoded, err := configurationJSON(c)
	if err != nil {
		return SavedRequest{}, err
	}
	r.ConfigurationJSON = string(encoded)
	r.Revision++
	r.UpdatedAt = timestamp(st.now())
	action, err := st.put(r, "#r = :r AND #state = :active AND #kind = :kind", map[string]types.AttributeValue{":r": n(expected), ":active": s("active"), ":kind": s("request")})
	if err != nil {
		return SavedRequest{}, unavailable()
	}
	action.Put.ExpressionAttributeNames = map[string]string{"#r": "revision", "#state": "state", "#kind": "kind"}
	if err = st.transact(ctx, []types.TransactWriteItem{st.stageGuard(stage), st.userGuard(user), st.requestGate(p, "active"), action}); err != nil {
		return SavedRequest{}, st.requestFailure(ctx, userID, p, requestID, &expected, err)
	}
	return r.savedRequest()
}
func requestStale() error {
	return apiError(412, "precondition_failed", "Request changed; refresh before retrying.")
}
func (st *Store) requestFailure(ctx context.Context, userID string, p record, id string, expected *int64, original error) error {
	if _, _, _, err := st.requestContext(ctx, userID, p.ProjectID); err != nil {
		return err
	}
	if !isConditional(original) {
		return unavailable()
	}
	if expected != nil {
		r, err := st.get(ctx, p.PK, "REQ#"+id)
		if err != nil {
			return err
		}
		if _, err = r.savedRequest(); err != nil {
			return err
		}
		if r.Revision != *expected {
			return requestStale()
		}
	}
	return conflict()
}
func (st *Store) DeleteRequest(ctx context.Context, userID, projectID, requestID string, expected int64) error {
	stage, user, p, err := st.requestContext(ctx, userID, projectID)
	if err != nil {
		return err
	}
	r, err := st.get(ctx, p.PK, "REQ#"+requestID)
	if err != nil {
		return err
	}
	if _, err = r.savedRequest(); err != nil {
		return err
	}
	if r.Revision != expected {
		return requestStale()
	}
	tombstone := record{PK: r.PK, SK: r.SK, Kind: "requestTombstone", SchemaVersion: 1, ProjectID: projectID, RequestID: requestID, Revision: expected + 1, State: "deleted"}
	action, err := st.put(tombstone, "#r = :r AND #state = :active AND #kind = :kind", map[string]types.AttributeValue{":r": n(expected), ":active": s("active"), ":kind": s("request")})
	if err != nil {
		return unavailable()
	}
	action.Put.ExpressionAttributeNames = map[string]string{"#r": "revision", "#state": "state", "#kind": "kind"}
	if err = st.transact(ctx, []types.TransactWriteItem{st.stageGuard(stage), st.userGuard(user), st.requestGate(p, "active"), action}); err != nil {
		return st.requestFailure(ctx, userID, p, requestID, &expected, err)
	}
	return nil
}

// drainRequests reads a bounded page of the REQ key range. The durable cursor
// lives on WORK; recognized records alone are removed. A full cycle resets the
// cursor so unknown entries and uncertain commits are revisited after restart.
func (st *Store) drainRequests(ctx context.Context, p, stage, work record) (record, bool, error) {
	if stage.SavedRequestsSchemaVersion != 1 {
		return p, false, nil
	}
	var start map[string]types.AttributeValue
	if sk := work.MaintenanceCursor["SK"]; sk != "" {
		start = key(p.PK, sk)
	}
	page, err := st.db.Query(ctx, &dynamodb.QueryInput{TableName: aws.String(st.table), KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :prefix)"), ExpressionAttributeValues: map[string]types.AttributeValue{":pk": s(p.PK), ":prefix": s("REQ#")}, ConsistentRead: aws.Bool(true), ExclusiveStartKey: start, Limit: aws.Int32(20)})
	if err != nil {
		return p, false, unavailable()
	}
	if len(page.Items) == 0 && len(start) == 0 {
		return p, false, nil
	}
	actions := []types.TransactWriteItem{st.stageGuard(stage), st.requestGate(p, "deleting")}
	allowed := map[string]bool{}
	for _, name := range []string{"PK", "SK", "kind", "schemaVersion", "version", "projectId", "requestId", "revision", "state", "configurationJSON", "createdAt", "updatedAt", "LPK", "LSK"} {
		allowed[name] = true
	}
	for _, item := range page.Items {
		recognized := true
		for name := range item {
			if !allowed[name] {
				recognized = false
			}
		}
		var r record
		if !recognized || attributevalue.UnmarshalMap(item, &r) != nil || r.SchemaVersion != 1 || r.PK != p.PK || r.ProjectID != p.ProjectID || r.RequestID == "" || r.SK != "REQ#"+r.RequestID || r.Revision < 0 {
			continue
		}
		switch r.Kind {
		case "request":
			if _, err := r.savedRequest(); err != nil {
				continue
			}
		case "requestTombstone":
			if r.State != "deleted" || r.ConfigurationJSON != "" {
				continue
			}
		default:
			continue
		}
		actions = append(actions, types.TransactWriteItem{Delete: &types.Delete{TableName: aws.String(st.table), Key: key(r.PK, r.SK), ConditionExpression: aws.String("schemaVersion = :schema AND #kind = :kind AND revision = :revision AND projectId = :project AND requestId = :id"), ExpressionAttributeNames: map[string]string{"#kind": "kind"}, ExpressionAttributeValues: map[string]types.AttributeValue{":schema": n(1), ":kind": s(r.Kind), ":revision": n(r.Revision), ":project": s(p.ProjectID), ":id": s(r.RequestID)}}})
	}
	work.MaintenanceCursor = nil
	if v, ok := page.LastEvaluatedKey["SK"].(*types.AttributeValueMemberS); ok {
		work.MaintenanceCursor = map[string]string{"SK": v.Value}
	}
	checkpoint, err := st.put(work, "#v = :v AND #kind = :kind", map[string]types.AttributeValue{":v": n(work.Version), ":kind": s("deletionWork")})
	if err != nil {
		return p, false, unavailable()
	}
	checkpoint.Put.ExpressionAttributeNames = map[string]string{"#v": "version", "#kind": "kind"}
	// Increment WORK as well as the gate so overlapping cleanup cannot skip pages.
	checkpoint.Put.Item["version"] = n(work.Version + 1)
	actions = append(actions, checkpoint)
	if err = st.transact(ctx, actions); err != nil {
		return p, false, conflict()
	}
	p.Version++
	return p, len(page.LastEvaluatedKey) > 0, nil
}
