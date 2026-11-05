package ownership

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	runner "github.com/VioletProto/Kurier/services/request-runtime"
	oapi "github.com/VioletProto/Kurier/services/request-runtime/openapi"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func validOpenAPIRecord(r openapiRecord) bool {
	if r.SchemaVersion != 1 || !bindingUUID.MatchString(r.ImportID) || r.PK != "P#"+r.ProjectID || r.Revision < 0 {
		return false
	}
	switch r.Kind {
	case "openapiImport":
		return r.SK == openapiKey(r.ImportID) && r.OperationCount > 0 && r.OperationCount <= 100 && r.SourceChunks > 0 && r.SourceChunks <= 44 && (r.State == "creating" || r.State == "preparing" || r.State == "publishing" || r.State == "ready" || r.State == "deleting")
	case "openapiSource":
		b, e := base64.StdEncoding.DecodeString(r.Data)
		if e != nil || len(b) > 48*1024 {
			return false
		}
		for i := 0; i < 44; i++ {
			if r.SK == fmt.Sprintf("%sSOURCE#%03d", openapiPrefix(r.ImportID), i) {
				return true
			}
		}
		return false
	case "openapiOperation":
		var op oapi.Operation
		c, choicesErr := decodeOpenAPIChoices([]byte(r.ChoicesJSON))
		if len(r.OperationJSON) > 32*1024 || len(r.ChoicesJSON) > 64*1024 || runner.StrictJSON([]byte(r.OperationJSON), &op) != nil || choicesErr != nil || op.OperationID != r.OperationID || r.SK != openapiPrefix(r.ImportID)+"OP#"+r.OperationID || !bindingUUID.MatchString(r.RequestID) {
			return false
		}
		_, e := oapi.Generate(op, c)
		var oe *oapi.Error
		return e == nil || errors.As(e, &oe) && oe.Code == "inputs_required"
	}
	return false
}
func (st *Store) openapiDeleteAction(r openapiRecord) types.TransactWriteItem {
	return types.TransactWriteItem{Delete: &types.Delete{TableName: &st.table, Key: key(r.PK, r.SK), ConditionExpression: aws.String("#k = :k AND schemaVersion = :schema AND importId = :i AND projectId = :p AND revision = :r"), ExpressionAttributeNames: map[string]string{"#k": "kind"}, ExpressionAttributeValues: map[string]types.AttributeValue{":k": s(r.Kind), ":schema": n(1), ":i": s(r.ImportID), ":p": s(r.ProjectID), ":r": n(r.Revision)}}}
}
func (st *Store) deleteOpenAPI(ctx context.Context, user, project, id string, expected int64) error {
	if _, _, _, e := st.openapiContext(ctx, user, project); e != nil {
		return e
	}
	root, e := st.openapiGet(ctx, project, id, openapiKey(id))
	if e != nil {
		return e
	}
	if root.Revision != expected {
		return requestStale()
	}
	if root.State == "ready" {
		return apiError(409, "import_in_use", "Published imports are retained with their requests; delete the project to remove them.")
	}
	if root.State != "deleting" {
		next := root
		next.State = "deleting"
		if e = st.openapiChange(ctx, user, project, root, next, nil); e != nil {
			return e
		}
		root, e = st.openapiGet(ctx, project, id, openapiKey(id))
		if e != nil {
			return e
		}
	}
	rows, e := st.openapiRows(ctx, project, id)
	if e != nil {
		return e
	}
	// Every linked staged request is still hidden while deleting. Reject unknown
	// records rather than treating this operation as a generic partition eraser.
	deletes := []types.TransactWriteItem{}
	for _, r := range rows {
		if !validOpenAPIRecord(r) {
			return unavailable()
		}
		if r.Kind == "openapiOperation" {
			out, e := st.db.GetItem(ctx, &dynamodb.GetItemInput{TableName: &st.table, Key: key(r.PK, "REQ#"+r.RequestID), ConsistentRead: aws.Bool(true)})
			if e != nil {
				return unavailable()
			}
			if len(out.Item) > 0 {
				var req record
				if attributevalue.UnmarshalMap(out.Item, &req) != nil {
					return unavailable()
				}
				canonical, e := encode(req)
				if e != nil || !sameAttributeMap(canonical, out.Item) || req.ImportID != id || req.ProjectID != project || req.SchemaVersion != 4 {
					return unavailable()
				}
				if _, e = req.savedRequest(); e != nil {
					return unavailable()
				}
				deletes = append(deletes, types.TransactWriteItem{Delete: &types.Delete{TableName: &st.table, Key: key(req.PK, req.SK), ConditionExpression: aws.String("importId = :i AND revision = :r AND #k = :k AND schemaVersion = :schema"), ExpressionAttributeNames: map[string]string{"#k": "kind"}, ExpressionAttributeValues: map[string]types.AttributeValue{":i": s(id), ":r": n(req.Revision), ":k": s("request"), ":schema": n(4)}}})
			}
		}
		if r.SK != root.SK {
			deletes = append(deletes, st.openapiDeleteAction(r))
		}
	}
	for start := 0; start < len(deletes); start += 20 {
		end := start + 20
		if end > len(deletes) {
			end = len(deletes)
		}
		stage, u, p, e := st.openapiContext(ctx, user, project)
		if e != nil {
			return e
		}
		actions := []types.TransactWriteItem{st.stageGuard(stage), st.userGuard(u), st.requestGate(p, "active"), st.openapiGuard(root)}
		if e = st.transact(ctx, append(actions, deletes[start:end]...)); e != nil {
			return e
		}
	}
	stage, u, p, e := st.openapiContext(ctx, user, project)
	if e != nil {
		return e
	}
	return st.transact(ctx, []types.TransactWriteItem{st.stageGuard(stage), st.userGuard(u), st.requestGate(p, "active"), st.openapiDeleteAction(root)})
}
func (st *Store) drainOpenAPI(ctx context.Context, p, stage, work record) (record, bool, error) {
	if stage.OpenAPISchemaVersion != 1 {
		return p, false, nil
	}
	var start map[string]types.AttributeValue
	if sk := work.MaintenanceCursor["OPENAPISK"]; sk != "" {
		start = key(p.PK, sk)
	}
	page, e := st.db.Query(ctx, &dynamodb.QueryInput{TableName: &st.table, KeyConditionExpression: aws.String("PK = :p AND begins_with(SK, :s)"), ExpressionAttributeValues: map[string]types.AttributeValue{":p": s(p.PK), ":s": s("OPENAPI#")}, ConsistentRead: aws.Bool(true), ExclusiveStartKey: start, Limit: aws.Int32(20)})
	if e != nil {
		return p, false, unavailable()
	}
	if len(page.Items) == 0 && len(start) == 0 {
		return p, false, nil
	}
	actions := []types.TransactWriteItem{st.stageGuard(stage), st.requestGate(p, "deleting")}
	for _, item := range page.Items {
		var r openapiRecord
		if attributevalue.UnmarshalMap(item, &r) != nil || r.PK != p.PK || r.ProjectID != p.ProjectID || !validOpenAPIRecord(r) {
			continue
		}
		canonical, e := attributevalue.MarshalMap(r)
		if e != nil || !sameAttributeMap(canonical, item) {
			continue
		}
		actions = append(actions, st.openapiDeleteAction(r))
	}
	work.MaintenanceCursor = nil
	if sk, ok := page.LastEvaluatedKey["SK"].(*types.AttributeValueMemberS); ok {
		work.MaintenanceCursor = map[string]string{"OPENAPISK": sk.Value}
	}
	checkpoint, e := st.put(work, "#v = :v AND #k = :k", map[string]types.AttributeValue{":v": n(work.Version), ":k": s("deletionWork")})
	if e != nil {
		return p, false, e
	}
	checkpoint.Put.ExpressionAttributeNames = map[string]string{"#v": "version", "#k": "kind"}
	checkpoint.Put.Item["version"] = n(work.Version + 1)
	actions = append(actions, checkpoint)
	if e = st.transact(ctx, actions); e != nil {
		return p, false, conflict()
	}
	p.Version++
	return p, len(page.LastEvaluatedKey) > 0, nil
}
