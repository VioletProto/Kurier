package execution

import (
	"context"
	"encoding/json"

	runner "github.com/VioletProto/Kurier/services/request-runtime"
	contract "github.com/VioletProto/Kurier/services/request-runtime/openapi"
	"github.com/aws/aws-sdk-go-v2/aws"
	dt "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func (s *Service) freezeContract(ctx context.Context, stage row, project string, plan *Plan, rerun bool) ([]dt.TransactWriteItem, error) {
	ref := plan.Configuration.OperationRef
	if ref == nil {
		if plan.FrozenContract != nil {
			return nil, failure(400, "openapi_reference_invalid")
		}
		return nil, nil
	}
	if stage.OpenAPISchemaVersion != 1 || ref.Validate() != nil {
		return nil, failure(400, "openapi_reference_invalid")
	}
	if plan.Target == "local" || plan.Options.Target == "local" {
		return nil, failure(409, "openapi_local_validation_unavailable")
	}
	root, e := s.get(ctx, s.Table, "P#"+project, "OPENAPI#"+ref.ImportID+"#META")
	if e != nil || root.Kind != "openapiImport" || root.SchemaVersion != 1 || root.State != "ready" || root.ProjectID != project || root.ImportID != ref.ImportID || root.Revision < 0 {
		return nil, failure(400, "source_unavailable")
	}
	op, e := s.get(ctx, s.Table, root.PK, "OPENAPI#"+ref.ImportID+"#OP#"+ref.OperationID)
	var operation contract.Operation
	if e != nil || op.Kind != "openapiOperation" || op.SchemaVersion != 1 || op.ProjectID != project || op.ImportID != ref.ImportID || op.OperationID != ref.OperationID || StrictJSON([]byte(op.OperationJSON), &operation) != nil || operation.OperationID != ref.OperationID || operation.Method != plan.Configuration.Method || contract.ValidateOperation(operation) != nil {
		return nil, failure(400, "source_unavailable")
	}
	if rerun {
		if plan.FrozenContract.Validate(ref) != nil || plan.FrozenContract.Revision != root.Revision {
			return nil, failure(400, "source_unavailable")
		}
		frozen, _ := json.Marshal(plan.FrozenContract.Operation)
		current, _ := json.Marshal(operation)
		if string(frozen) != string(current) {
			return nil, failure(400, "source_unavailable")
		}
	} else {
		plan.FrozenContract = &runner.FrozenContract{SchemaVersion: 1, ImportID: ref.ImportID, OperationID: ref.OperationID, Revision: root.Revision, Operation: operation}
		if plan.FrozenContract.Validate(ref) != nil {
			return nil, failure(400, "source_unavailable")
		}
	}
	rootGuard := s.openAPIImportGuard(root)
	opGuard := dt.TransactWriteItem{ConditionCheck: &dt.ConditionCheck{TableName: &s.Table, Key: keys(op.PK, op.SK), ConditionExpression: aws.String("#k = :k AND schemaVersion = :schema AND importId = :i AND projectId = :p AND operationId = :o AND operationJSON = :json"), ExpressionAttributeNames: map[string]string{"#k": "kind"}, ExpressionAttributeValues: map[string]dt.AttributeValue{":k": str(op.Kind), ":schema": num(1), ":i": str(ref.ImportID), ":p": str(project), ":o": str(ref.OperationID), ":json": str(op.OperationJSON)}}}
	return []dt.TransactWriteItem{rootGuard, opGuard}, nil
}

func (s *Service) openAPIImportGuard(root row) dt.TransactWriteItem {
	return dt.TransactWriteItem{ConditionCheck: &dt.ConditionCheck{TableName: &s.Table, Key: keys(root.PK, root.SK), ConditionExpression: aws.String("#k = :k AND schemaVersion = :schema AND #s = :ready AND revision = :r AND importId = :i AND projectId = :p"), ExpressionAttributeNames: map[string]string{"#k": "kind", "#s": "state"}, ExpressionAttributeValues: map[string]dt.AttributeValue{":k": str(root.Kind), ":schema": num(1), ":ready": str("ready"), ":r": num(root.Revision), ":i": str(root.ImportID), ":p": str(root.ProjectID)}}}
}

// Publication belongs to the generated request, independently of whether its
// owner keeps response validation attached. A hidden staged request cannot be
// made executable by omitting its public OperationRef.
func (s *Service) importPublicationGuards(ctx context.Context, stage row, project string, request row) ([]dt.TransactWriteItem, error) {
	if request.ImportID == "" {
		return nil, nil
	}
	if stage.OpenAPISchemaVersion != 1 || !uuidPattern.MatchString(request.ImportID) {
		return nil, failure(400, "source_unavailable")
	}
	root, e := s.get(ctx, s.Table, "P#"+project, "OPENAPI#"+request.ImportID+"#META")
	if e != nil || root.Kind != "openapiImport" || root.SchemaVersion != 1 || root.State != "ready" || root.ProjectID != project || root.ImportID != request.ImportID || root.Revision < 0 {
		return nil, failure(400, "source_unavailable")
	}
	return []dt.TransactWriteItem{s.openAPIImportGuard(root)}, nil
}

func mergeContractGuards(existing, additions []dt.TransactWriteItem) ([]dt.TransactWriteItem, error) {
	for _, candidate := range additions {
		found := false
		for _, old := range existing {
			if old.ConditionCheck == nil || candidate.ConditionCheck == nil {
				continue
			}
			a, b := old.ConditionCheck, candidate.ConditionCheck
			if aws.ToString(a.TableName) == aws.ToString(b.TableName) && a.Key["PK"].(*dt.AttributeValueMemberS).Value == b.Key["PK"].(*dt.AttributeValueMemberS).Value && a.Key["SK"].(*dt.AttributeValueMemberS).Value == b.Key["SK"].(*dt.AttributeValueMemberS).Value {
				left, _ := json.Marshal(a)
				right, _ := json.Marshal(b)
				if string(left) != string(right) {
					return nil, failure(409, "conflict")
				}
				found = true
				break
			}
		}
		if !found {
			existing = append(existing, candidate)
		}
	}
	return existing, nil
}
