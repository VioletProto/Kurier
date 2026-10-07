package ownership

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type database interface {
	GetItem(context.Context, *dynamodb.GetItemInput, ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	Query(context.Context, *dynamodb.QueryInput, ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error)
	TransactWriteItems(context.Context, *dynamodb.TransactWriteItemsInput, ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error)
}
type Store struct {
	db           database
	table, stage string
	now          func() time.Time
}

func NewStore(client *dynamodb.Client, table, stage string) *Store {
	return &Store{client, table, stage, time.Now}
}
func key(pk, sk string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{"PK": s(pk), "SK": s(sk)}
}
func s(v string) types.AttributeValue { return &types.AttributeValueMemberS{Value: v} }
func n(v int64) types.AttributeValue  { return &types.AttributeValueMemberN{Value: fmt.Sprint(v)} }
func encode(r record) (map[string]types.AttributeValue, error) {
	item, err := attributevalue.MarshalMap(r)
	if err == nil && r.Operation != nil {
		item["initiatingVersion"] = n(r.InitiatingVersion)
	}
	return item, err
}
func (st *Store) get(ctx context.Context, pk, sk string) (record, error) {
	result, err := st.db.GetItem(ctx, &dynamodb.GetItemInput{TableName: aws.String(st.table), Key: key(pk, sk), ConsistentRead: aws.Bool(true)})
	if err != nil {
		return record{}, unavailable()
	}
	if len(result.Item) == 0 {
		return record{}, missing()
	}
	var r record
	if attributevalue.UnmarshalMap(result.Item, &r) != nil || r.SchemaVersion != 1 {
		return record{}, unavailable()
	}
	return r, nil
}
func (st *Store) activeStage(ctx context.Context) (record, error) {
	r, err := st.get(ctx, "STAGE#"+st.stage, "META")
	if err != nil || r.State != "active" || r.RecoveryGeneration == "" || !r.LocalEmptyProjectsOnly {
		return record{}, unavailable()
	}
	return r, nil
}
func (st *Store) userRecord(ctx context.Context, id string) (record, error) {
	r, err := st.get(ctx, "U#"+id, "META")
	if err != nil {
		return record{}, unavailable()
	}
	if r.Kind != "user" || r.UserID != id {
		return record{}, unavailable()
	}
	if r.Disabled {
		return record{}, forbidden()
	}
	return r, nil
}
func identityKey(identity Identity) string {
	h := sha256.New()
	for _, part := range []string{identity.issuer, identity.subject} {
		_ = binary.Write(h, binary.BigEndian, uint32(len(part)))
		_, _ = h.Write([]byte(part))
	}
	return "IDENTITY#" + hex.EncodeToString(h.Sum(nil))
}
func (st *Store) ResolveUser(ctx context.Context, identity Identity) (User, error) {
	if identity.issuer == "" || identity.subject == "" {
		return User{}, apiError(401, "unauthenticated", "Valid access token required.")
	}
	stage, err := st.activeStage(ctx)
	if err != nil {
		return User{}, err
	}
	pk := identityKey(identity)
	for attempt := 0; attempt < 10; attempt++ {
		mapping, readErr := st.get(ctx, pk, "META")
		if readErr == nil {
			if mapping.Kind != "identity" || mapping.Issuer != identity.issuer || mapping.Subject != identity.subject {
				return User{}, unavailable()
			}
			u, e := st.userRecord(ctx, mapping.UserID)
			if e != nil {
				return User{}, e
			}
			return u.user(), nil
		}
		var ae *APIError
		if !errors.As(readErr, &ae) || ae.Status != 404 {
			return User{}, readErr
		}
		id := newID()
		u := record{PK: "U#" + id, SK: "META", Kind: "user", SchemaVersion: 1, UserID: id, Issuer: identity.issuer, Subject: identity.subject, CreatedAt: timestamp(st.now())}
		m := record{PK: pk, SK: "META", Kind: "identity", SchemaVersion: 1, UserID: id, Issuer: identity.issuer, Subject: identity.subject}
		up, e := st.put(u, "attribute_not_exists(PK)", nil)
		if e != nil {
			return User{}, unavailable()
		}
		mp, e := st.put(m, "attribute_not_exists(PK)", nil)
		if e != nil {
			return User{}, unavailable()
		}
		err = st.transact(ctx, []types.TransactWriteItem{st.stageGuard(stage), up, mp})
		if err == nil {
			return u.user(), nil
		}
		if !isConditional(err) {
			return User{}, unavailable()
		}
		if _, e = st.activeStage(ctx); e != nil {
			return User{}, e
		}
		if e = pause(ctx, attempt); e != nil {
			return User{}, unavailable()
		}
	}
	return User{}, conflict()
}
func pause(ctx context.Context, attempt int) error {
	timer := time.NewTimer(time.Duration(10+attempt*5) * time.Millisecond)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (st *Store) stageGuard(stage record) types.TransactWriteItem {
	return types.TransactWriteItem{ConditionCheck: &types.ConditionCheck{TableName: aws.String(st.table), Key: key(stage.PK, "META"), ConditionExpression: aws.String("#state = :active AND recoveryGeneration = :gen AND localEmptyProjectsOnly = :yes"), ExpressionAttributeNames: map[string]string{"#state": "state"}, ExpressionAttributeValues: map[string]types.AttributeValue{":active": s("active"), ":gen": s(stage.RecoveryGeneration), ":yes": &types.AttributeValueMemberBOOL{Value: true}}}}
}
func (st *Store) userGuard(u record) types.TransactWriteItem {
	return types.TransactWriteItem{ConditionCheck: &types.ConditionCheck{TableName: aws.String(st.table), Key: key(u.PK, "META"), ConditionExpression: aws.String("#version = :v AND (attribute_not_exists(disabled) OR disabled = :no)"), ExpressionAttributeNames: map[string]string{"#version": "version"}, ExpressionAttributeValues: map[string]types.AttributeValue{":v": n(u.Version), ":no": &types.AttributeValueMemberBOOL{Value: false}}}}
}
func (st *Store) put(r record, condition string, values map[string]types.AttributeValue) (types.TransactWriteItem, error) {
	item, err := encode(r)
	return types.TransactWriteItem{Put: &types.Put{TableName: aws.String(st.table), Item: item, ConditionExpression: aws.String(condition), ExpressionAttributeValues: values}}, err
}
func (st *Store) transact(ctx context.Context, actions []types.TransactWriteItem) error {
	// This slice has at most four small items, far below the accepted 80/2MiB
	// application budget. Later entities must implement their own byte budgets.
	if len(actions) > 4 {
		return errors.New("slice transaction budget exceeded")
	}
	_, err := st.db.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{TransactItems: actions, ClientRequestToken: aws.String(newID())})
	return err
}
func isConditional(err error) bool {
	var canceled *types.TransactionCanceledException
	if errors.As(err, &canceled) {
		found := false
		for _, reason := range canceled.CancellationReasons {
			code := aws.ToString(reason.Code)
			if code == "ConditionalCheckFailed" || code == "TransactionConflict" {
				found = true
			} else if code != "None" && code != "" {
				return false
			}
		}
		return found
	}
	var concurrent *types.TransactionConflictException
	return errors.As(err, &concurrent)
}
func (st *Store) CreateProject(ctx context.Context, userID, name string) (Project, error) {
	stage, err := st.activeStage(ctx)
	if err != nil {
		return Project{}, err
	}
	u, err := st.userRecord(ctx, userID)
	if err != nil {
		return Project{}, err
	}
	id := newID()
	now := timestamp(st.now())
	r := record{PK: "P#" + id, SK: "META", Kind: "project", SchemaVersion: 1, ProjectID: id, OwnerID: userID, Name: name, State: "active", CreatedAt: now, UpdatedAt: now, LPK: "U#" + userID + "#PROJECT", LSK: now + "#" + id}
	p, err := st.put(r, "attribute_not_exists(PK)", nil)
	if err != nil {
		return Project{}, unavailable()
	}
	err = st.transact(ctx, []types.TransactWriteItem{st.stageGuard(stage), st.userGuard(u), p})
	if err != nil {
		// No second create attempt, even after an uncertain transaction result.
		if _, e := st.userRecord(ctx, userID); e != nil {
			return Project{}, e
		}
		return Project{}, unavailable()
	}
	return r.project(), nil
}
func (st *Store) owned(ctx context.Context, userID, projectID string, includeDeleted bool) (record, error) {
	r, err := st.get(ctx, "P#"+projectID, "META")
	if err != nil {
		return record{}, err
	}
	if r.OwnerID != userID || r.ProjectID != projectID || (!includeDeleted && r.State != "active") {
		return record{}, missing()
	}
	return r, nil
}
func (st *Store) GetProject(ctx context.Context, userID, projectID string) (Project, error) {
	if _, err := st.activeStage(ctx); err != nil {
		return Project{}, err
	}
	if _, err := st.userRecord(ctx, userID); err != nil {
		return Project{}, err
	}
	r, err := st.owned(ctx, userID, projectID, false)
	if err != nil {
		return Project{}, err
	}
	return r.project(), nil
}
func (st *Store) mutationFailure(ctx context.Context, userID, projectID string, expected int64, err error) error {
	if _, e := st.activeStage(ctx); e != nil {
		return e
	}
	if _, e := st.userRecord(ctx, userID); e != nil {
		return e
	}
	r, e := st.owned(ctx, userID, projectID, false)
	if e != nil {
		return e
	}
	// An unknown transport outcome might be our own committed write, not a
	// failed client precondition. Never report 412 merely from that ambiguity.
	if !isConditional(err) {
		return unavailable()
	}
	if r.Version != expected {
		return stale()
	}
	return conflict()
}
func (st *Store) RenameProject(ctx context.Context, userID, projectID, name string, expected int64) (Project, error) {
	stage, err := st.activeStage(ctx)
	if err != nil {
		return Project{}, err
	}
	u, err := st.userRecord(ctx, userID)
	if err != nil {
		return Project{}, err
	}
	r, err := st.owned(ctx, userID, projectID, false)
	if err != nil {
		return Project{}, err
	}
	if r.Version != expected {
		return Project{}, stale()
	}
	updated := timestamp(st.now())
	action := types.TransactWriteItem{Update: &types.Update{TableName: aws.String(st.table), Key: key(r.PK, "META"), UpdateExpression: aws.String("SET #name = :name, updatedAt = :time, #version = :next"), ConditionExpression: aws.String("ownerId = :owner AND #state = :active AND #version = :v"), ExpressionAttributeNames: map[string]string{"#name": "name", "#state": "state", "#version": "version"}, ExpressionAttributeValues: map[string]types.AttributeValue{":name": s(name), ":time": s(updated), ":next": n(expected + 1), ":owner": s(userID), ":active": s("active"), ":v": n(expected)}}}
	err = st.transact(ctx, []types.TransactWriteItem{st.stageGuard(stage), st.userGuard(u), action})
	if err != nil {
		return Project{}, st.mutationFailure(ctx, userID, projectID, expected, err)
	}
	r.Name = name
	r.UpdatedAt = updated
	r.Version++
	return r.project(), nil
}
func (st *Store) DeleteProject(ctx context.Context, userID, projectID string, expected int64) (DeletionOperation, error) {
	stage, err := st.activeStage(ctx)
	if err != nil {
		return DeletionOperation{}, err
	}
	u, err := st.userRecord(ctx, userID)
	if err != nil {
		return DeletionOperation{}, err
	}
	r, err := st.owned(ctx, userID, projectID, true)
	if err != nil {
		return DeletionOperation{}, err
	}
	if r.State != "active" {
		if r.Operation != nil && r.InitiatingVersion == expected {
			return *r.Operation, nil
		}
		return DeletionOperation{}, stale()
	}
	if r.Version != expected {
		return DeletionOperation{}, stale()
	}
	op := DeletionOperation{OperationID: newID(), ProjectID: projectID, State: "accepted", StartedAt: timestamp(st.now()), RetryAfterSeconds: 2}
	opItem, err := attributevalue.Marshal(op)
	if err != nil {
		return DeletionOperation{}, unavailable()
	}
	gate := types.TransactWriteItem{Update: &types.Update{TableName: aws.String(st.table), Key: key(r.PK, "META"), UpdateExpression: aws.String("SET #state = :deleting, #version = :next, deletionEpoch = :epoch, initiatingVersion = :v, deletionOperation = :op REMOVE LPK, LSK"), ConditionExpression: aws.String("ownerId = :owner AND #state = :active AND #version = :v"), ExpressionAttributeNames: map[string]string{"#state": "state", "#version": "version"}, ExpressionAttributeValues: map[string]types.AttributeValue{":deleting": s("deleting"), ":next": n(expected + 1), ":epoch": n(r.DeletionEpoch + 1), ":v": n(expected), ":op": opItem, ":owner": s(userID), ":active": s("active")}}}
	work, err := st.put(record{PK: r.PK, SK: "WORK#delete#" + op.OperationID, Kind: "deletionWork", SchemaVersion: 1, ProjectID: projectID, DPK: cleanupShard(projectID), DSK: op.StartedAt + "#" + op.OperationID}, "attribute_not_exists(PK)", nil)
	if err != nil {
		return DeletionOperation{}, unavailable()
	}
	err = st.transact(ctx, []types.TransactWriteItem{st.stageGuard(stage), st.userGuard(u), gate, work})
	if err != nil {
		// Read back the durable operation after a racing/uncertain acceptance.
		if _, e := st.activeStage(ctx); e != nil {
			return DeletionOperation{}, e
		}
		if _, e := st.userRecord(ctx, userID); e != nil {
			return DeletionOperation{}, e
		}
		r, e := st.owned(ctx, userID, projectID, true)
		if e != nil {
			return DeletionOperation{}, e
		}
		if r.State != "active" && r.Operation != nil && r.InitiatingVersion == expected {
			return *r.Operation, nil
		}
		return DeletionOperation{}, st.mutationFailure(ctx, userID, projectID, expected, err)
	}
	return op, nil
}
func (st *Store) GetDeletion(ctx context.Context, userID, projectID, operationID string) (DeletionOperation, error) {
	if _, err := st.activeStage(ctx); err != nil {
		return DeletionOperation{}, err
	}
	if _, err := st.userRecord(ctx, userID); err != nil {
		return DeletionOperation{}, err
	}
	r, err := st.owned(ctx, userID, projectID, true)
	if err != nil {
		return DeletionOperation{}, err
	}
	if r.Operation == nil || r.Operation.OperationID != operationID {
		return DeletionOperation{}, missing()
	}
	return *r.Operation, nil
}

// markDeleting is its own durable phase: a process may stop here and resume.
func (st *Store) markDeleting(ctx context.Context, r, stage record) (record, error) {
	if r.Operation == nil || r.State != "deleting" {
		return record{}, missing()
	}
	if r.Operation.State == "deleting" {
		return r, nil
	}
	copyOp := *r.Operation
	copyOp.State = "deleting"
	r.Operation = &copyOp
	r.Version++
	item, err := encode(r)
	if err != nil {
		return record{}, unavailable()
	}
	action := types.TransactWriteItem{Put: &types.Put{TableName: aws.String(st.table), Item: item, ConditionExpression: aws.String("#version = :v AND #state = :deleting AND deletionEpoch = :epoch"), ExpressionAttributeNames: map[string]string{"#version": "version", "#state": "state"}, ExpressionAttributeValues: map[string]types.AttributeValue{":v": n(r.Version - 1), ":deleting": s("deleting"), ":epoch": n(r.DeletionEpoch)}}}
	if err = st.transact(ctx, []types.TransactWriteItem{st.stageGuard(stage), action}); err != nil {
		return record{}, conflict()
	}
	return r, nil
}

// CleanupProject is used by local maintenance and scheduled cloud maintenance,
// never exposed as an eighth API route.
// It never drains unknown entities. All participating writers must use gate CAS;
// arbitrary administrator writes outside that protocol cannot be fenced here.
func (st *Store) CleanupProject(ctx context.Context, projectID string) (DeletionOperation, error) {
	stage, err := st.activeStage(ctx)
	if err != nil {
		return DeletionOperation{}, err
	}
	r, err := st.get(ctx, "P#"+projectID, "META")
	if err != nil {
		return DeletionOperation{}, err
	}
	if r.State == "deleted" && r.Operation != nil {
		return *r.Operation, nil
	}
	r, err = st.markDeleting(ctx, r, stage)
	if err != nil {
		return DeletionOperation{}, err
	}
	workKey := "WORK#delete#" + r.Operation.OperationID
	work, err := st.get(ctx, r.PK, workKey)
	if err != nil || work.Kind != "deletionWork" {
		return DeletionOperation{}, unavailable()
	}
	result, err := st.db.Query(ctx, &dynamodb.QueryInput{TableName: aws.String(st.table), KeyConditionExpression: aws.String("PK = :pk"), ExpressionAttributeValues: map[string]types.AttributeValue{":pk": s(r.PK)}, ConsistentRead: aws.Bool(true), ProjectionExpression: aws.String("PK, SK"), Limit: aws.Int32(3)})
	if err != nil {
		return DeletionOperation{}, unavailable()
	}
	if len(result.LastEvaluatedKey) > 0 {
		return *r.Operation, conflict()
	}
	for _, item := range result.Items {
		var k struct{ SK string }
		if attributevalue.UnmarshalMap(item, &k) != nil || (k.SK != "META" && k.SK != workKey) {
			return *r.Operation, conflict()
		}
	}
	if len(result.Items) != 2 {
		return *r.Operation, conflict()
	}
	completed := timestamp(st.now())
	op := *r.Operation
	op.State = "completed"
	op.CompletedAt = &completed
	op.RetryAfterSeconds = 0
	// Replacing the whole record ensures no project name/timestamps/index data
	// accidentally survives. The retained fields are only coordination metadata.
	tombstone := record{PK: r.PK, SK: "META", Kind: "projectTombstone", SchemaVersion: 1, ProjectID: projectID, OwnerID: r.OwnerID, Version: r.Version + 1, State: "deleted", DeletionEpoch: r.DeletionEpoch, InitiatingVersion: r.InitiatingVersion, Operation: &op}
	item, err := encode(tombstone)
	if err != nil {
		return DeletionOperation{}, unavailable()
	}
	gate := types.TransactWriteItem{Put: &types.Put{TableName: aws.String(st.table), Item: item, ConditionExpression: aws.String("#version = :v AND #state = :deleting AND deletionEpoch = :epoch AND deletionOperation.operationId = :op"), ExpressionAttributeNames: map[string]string{"#version": "version", "#state": "state"}, ExpressionAttributeValues: map[string]types.AttributeValue{":v": n(r.Version), ":deleting": s("deleting"), ":epoch": n(r.DeletionEpoch), ":op": s(op.OperationID)}}}
	drain := types.TransactWriteItem{Delete: &types.Delete{TableName: aws.String(st.table), Key: key(r.PK, workKey), ConditionExpression: aws.String("attribute_exists(PK)")}}
	if err = st.transact(ctx, []types.TransactWriteItem{st.stageGuard(stage), gate, drain}); err != nil {
		current, e := st.get(ctx, r.PK, "META")
		if e == nil && current.State == "deleted" && current.Operation != nil && current.Operation.OperationID == op.OperationID {
			return *current.Operation, nil
		}
		return DeletionOperation{}, conflict()
	}
	return op, nil
}
