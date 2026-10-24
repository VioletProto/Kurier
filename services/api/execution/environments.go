package execution

import (
	"context"
	"encoding/json"
	"fmt"
	runner "github.com/VioletProto/Kurier/services/request-runtime"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"sort"
)

func (s *Service) environmentStageGuard(stage row) types.TransactWriteItem {
	names := map[string]string{"#s": "state"}
	values := map[string]types.AttributeValue{":active": str("active"), ":g": str(stage.RecoveryGeneration)}
	expr := "#s = :active AND recoveryGeneration = :g"
	keys := []string{}
	versions := runner.EnvironmentStageVersions()
	for k := range versions {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for i, k := range keys {
		tag := fmt.Sprintf(":cap%d", i)
		expr += " AND " + k + " = " + tag
		values[tag] = num(versions[k])
	}
	return types.TransactWriteItem{ConditionCheck: &types.ConditionCheck{TableName: &s.Table, Key: keysMap(stage.PK, "META"), ConditionExpression: &expr, ExpressionAttributeNames: names, ExpressionAttributeValues: values}}
}
func keysMap(pk, sk string) map[string]types.AttributeValue { return keys(pk, sk) }

// Freeze reads only selected, used values. Source revision conditions are included
// in the existing atomic admission, never in a separate publication transaction.
func (s *Service) freezeEnvironment(ctx context.Context, project string, p *Plan, selection *runner.EnvironmentContext, rerun bool) ([]types.TransactWriteItem, error) {
	if p.Configuration.TemplateSemanticsVersion == 0 {
		if selection != nil || p.TemplateSnapshot != nil {
			return nil, failure(400, "template_invalid")
		}
		return nil, nil
	}
	if rerun {
		return s.environmentGuards(ctx, project, *p)
	}
	snapshot := &runner.TemplateSnapshot{SchemaVersion: 1, EnvironmentContext: selection, Variables: []runner.FrozenVariable{}}
	used, e := runner.UsedTemplateNames(p.Configuration)
	if e != nil {
		return nil, e
	}
	guards := []types.TransactWriteItem{}
	if selection != nil {
		record, e := s.get(ctx, s.Table, "P#"+project, "ENV#"+selection.EnvironmentID)
		var env runner.Environment
		if e != nil || record.Kind != "environment" || record.State != "active" || record.EnvironmentID != selection.EnvironmentID || record.ProjectID != project || StrictJSON([]byte(record.ConfigurationJSON), &env) != nil || env.Validate() != nil || env.EnvironmentID != record.EnvironmentID || env.ProjectID != project || env.Revision != record.Revision {
			return nil, failure(400, "source_unavailable")
		}
		if record.Revision != selection.Revision {
			return nil, failure(409, "environment_revision_conflict")
		}
		byName := map[string]runner.EnvironmentVariable{}
		for _, v := range env.Variables {
			if v.Enabled {
				byName[v.Name] = v
			}
		}
		for _, name := range used {
			v, ok := byName[name]
			if !ok {
				return nil, failure(400, "variable_missing")
			}
			typ := v.ValueType
			if !v.Sensitive {
				if v.Value == nil {
					return nil, safeError()
				}
				typ = v.Value.Type
			}
			snapshot.Variables = append(snapshot.Variables, runner.FrozenVariable{VariableID: v.VariableID, Name: v.Name, ValueType: typ, Sensitive: v.Sensitive, Value: v.Value, SecretRef: v.SecretRef})
		}
		guards = append(guards, s.sourceGuard(record, s.Table))
	} else if len(used) > 0 {
		return nil, failure(400, "variable_missing")
	}
	runner.SortFrozenVariables(snapshot.Variables)
	p.TemplateSnapshot = snapshot
	if e = snapshot.Validate(); e != nil {
		return nil, e
	}
	return guards, nil
}
func (s *Service) environmentGuards(ctx context.Context, project string, p Plan) ([]types.TransactWriteItem, error) {
	if p.Configuration.TemplateSemanticsVersion == 0 {
		if p.TemplateSnapshot != nil {
			return nil, failure(400, "source_unavailable")
		}
		return nil, nil
	}
	if e := p.TemplateSnapshot.Validate(); e != nil {
		return nil, e
	}
	snapshot := p.TemplateSnapshot
	if snapshot.EnvironmentContext == nil {
		return nil, nil
	}
	record, e := s.get(ctx, s.Table, "P#"+project, "ENV#"+snapshot.EnvironmentContext.EnvironmentID)
	var env runner.Environment
	if e != nil || record.Kind != "environment" || record.State != "active" || StrictJSON([]byte(record.ConfigurationJSON), &env) != nil || env.Validate() != nil || env.EnvironmentID != record.EnvironmentID || env.ProjectID != project || env.Revision != record.Revision {
		return nil, failure(400, "source_unavailable")
	}
	variables := map[string]runner.EnvironmentVariable{}
	for _, v := range env.Variables {
		variables[v.VariableID] = v
	}
	for _, frozen := range snapshot.Variables {
		v, ok := variables[frozen.VariableID]
		typ := v.ValueType
		if !v.Sensitive && v.Value != nil {
			typ = v.Value.Type
		}
		if !ok || !v.Enabled || v.Name != frozen.Name || v.Sensitive != frozen.Sensitive || typ != frozen.ValueType {
			return nil, failure(400, "source_unavailable")
		}
	}
	return []types.TransactWriteItem{s.sourceGuard(record, s.Table)}, nil
}
func requiredPlanRefs(p Plan) []string {
	out := requiredRefs(p.Configuration)
	seen := map[string]bool{}
	for _, id := range out {
		seen[id] = true
	}
	if p.TemplateSnapshot != nil {
		for _, v := range p.TemplateSnapshot.Variables {
			if v.Sensitive && v.SecretRef != nil && !seen[v.SecretRef.SecretID] {
				seen[v.SecretRef.SecretID] = true
				out = append(out, v.SecretRef.SecretID)
			}
		}
	}
	sort.Strings(out)
	return out
}
func validateEnvironmentSecret(p Plan, r row) bool {
	if r.Kind != "savedSecret" || r.State != "active" || r.SchemaVersion != 1 && r.SchemaVersion != 2 || r.SchemaVersion == 1 && r.ScalarType != "" {
		return false
	}
	if p.TemplateSnapshot != nil {
		for _, v := range p.TemplateSnapshot.Variables {
			if v.Sensitive && v.SecretRef != nil && v.SecretRef.SecretID == r.SecretID {
				typ := r.ScalarType
				if r.SchemaVersion == 1 && r.ValueKind == "string" {
					typ = "string"
				}
				if typ != v.ValueType || r.ValueKind == "jsonBody" {
					return false
				}
			}
		}
	}
	return true
}
func (s *Service) preflightEnvironment(ctx context.Context, p Plan, values map[string]string) error {
	if p.Configuration.TemplateSemanticsVersion == 0 {
		return nil
	}
	e := runner.PreflightTemplatePlan(ctx, p, values)
	if e != nil {
		return e
	}
	raw, _ := json.Marshal(p)
	if len(raw) > ConfigLimit {
		return failure(413, "payload_too_large")
	}
	return nil
}
