package ownership

import (
	"context"
	runner "github.com/VioletProto/Kurier/services/request-runtime"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func (st *Store) drainEnvironments(ctx context.Context, p, stage, work record) (record, bool, error) {
	if stage.ProjectEnvironmentsSchemaVersion != 1 {
		return p, false, nil
	}
	var start map[string]types.AttributeValue
	if sk := work.MaintenanceCursor["ENVSK"]; sk != "" {
		start = key(p.PK, sk)
	}
	page, err := st.db.Query(ctx, &dynamodb.QueryInput{TableName: aws.String(st.table), KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :prefix)"), ExpressionAttributeValues: map[string]types.AttributeValue{":pk": s(p.PK), ":prefix": s("ENV#")}, ConsistentRead: aws.Bool(true), ExclusiveStartKey: start, Limit: aws.Int32(20)})
	if err != nil {
		return p, false, unavailable()
	}
	if len(page.Items) == 0 && len(start) == 0 {
		return p, false, nil
	}
	actions := []types.TransactWriteItem{st.stageGuard(stage), st.requestGate(p, "deleting")}
	allowed := map[string]bool{}
	for _, name := range []string{"PK", "SK", "kind", "schemaVersion", "projectId", "environmentId", "revision", "state", "configurationJSON", "createdAt", "updatedAt"} {
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
		if !recognized || attributevalue.UnmarshalMap(item, &r) != nil || r.SchemaVersion != 1 || r.PK != p.PK || r.ProjectID != p.ProjectID || !bindingUUID.MatchString(r.EnvironmentID) || r.SK != "ENV#"+r.EnvironmentID || r.Revision < 0 {
			continue
		}
		switch r.Kind {
		case "environment":
			var env runner.Environment
			if r.State != "active" || runner.StrictJSON([]byte(r.ConfigurationJSON), &env) != nil || env.EnvironmentID != r.EnvironmentID || env.ProjectID != r.ProjectID || env.Revision != r.Revision || env.Validate() != nil {
				continue
			}
		case "environmentTombstone":
			if r.State != "deleted" || r.ConfigurationJSON != "" {
				continue
			}
		default:
			continue
		}
		actions = append(actions, types.TransactWriteItem{Delete: &types.Delete{TableName: aws.String(st.table), Key: key(r.PK, r.SK), ConditionExpression: aws.String("schemaVersion = :schema AND #kind = :kind AND revision = :revision AND projectId = :project AND environmentId = :id"), ExpressionAttributeNames: map[string]string{"#kind": "kind"}, ExpressionAttributeValues: map[string]types.AttributeValue{":schema": n(int64(r.SchemaVersion)), ":kind": s(r.Kind), ":revision": n(r.Revision), ":project": s(p.ProjectID), ":id": s(r.EnvironmentID)}}})
	}
	work.MaintenanceCursor = nil
	if v, ok := page.LastEvaluatedKey["SK"].(*types.AttributeValueMemberS); ok {
		work.MaintenanceCursor = map[string]string{"ENVSK": v.Value}
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
