//go:build integration

package execution

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dt "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// This bridge tests the actual writer/maintenance vocabulary and retained bytes,
// rather than manufacturing a metadata fixture whose states match the guard.
func TestIntegrationLegacyMaintenanceClosureInspection(t *testing.T) {
	f, wf := workflowProtocolFixture(t)
	ctx := context.Background()
	stage, _ := f.s.get(ctx, f.s.Table, "STAGE#dev-api", "META")
	stage.SavedRequestsSchemaVersion, stage.MCPReadOnlySchemaVersion, stage.Version = 7, 1, 13
	f.write(t, f.s.Table, stage)
	run := workflowRun(t, f, wf, id())
	finishWorkflowFixture(t, f, run)
	assertWorkflowAccounting(t, f, run, "completed", 2, 2)
	bridge, err := filepath.Abs("../../../scripts/fixtures/workflow-assertions-settlement.mjs")
	if err != nil {
		t.Fatal(err)
	}
	fieldsRaw, err := exec.Command("node", bridge, "--fields").Output()
	if err != nil {
		t.Fatal("local metadata bridge unavailable", err)
	}
	var fields []string
	if json.Unmarshal(fieldsRaw, &fields) != nil {
		t.Fatal("invalid projected fields")
	}
	scalar := func(item map[string]dt.AttributeValue) map[string]any {
		out := map[string]any{}
		for k, v := range item {
			switch x := v.(type) {
			case *dt.AttributeValueMemberS:
				out[k] = map[string]any{"S": x.Value}
			case *dt.AttributeValueMemberN:
				out[k] = map[string]any{"N": x.Value}
			case *dt.AttributeValueMemberBOOL:
				out[k] = map[string]any{"BOOL": x.Value}
			default:
				t.Fatalf("unexpected non-metadata projected field %s", k)
			}
		}
		return out
	}
	inventory := func() []map[string]any {
		rows := []map[string]any{}
		for _, table := range []string{f.s.Table, f.s.Protected} {
			page, e := f.db.Scan(ctx, &dynamodb.ScanInput{TableName: &table, ConsistentRead: pointer(true)})
			if e != nil || len(page.LastEvaluatedKey) > 0 {
				t.Fatal("local scan incomplete", e)
			}
			for _, item := range page.Items {
				projected := map[string]dt.AttributeValue{}
				for _, key := range fields {
					if v, ok := item[key]; ok {
						projected[key] = v
					}
				}
				rows = append(rows, map[string]any{"table": table, "item": scalar(projected)})
			}
		}
		return rows
	}
	markerMap, e := attributevalue.MarshalMap(stage)
	if e != nil {
		t.Fatal(e)
	}
	inspect := func(want bool, mutate func([]map[string]any) []map[string]any) {
		rows := inventory()
		if mutate != nil {
			rows = mutate(rows) // Corrupt only the inspection copy, never durable rows.
		}
		payload, _ := json.Marshal(map[string]any{"marker": scalar(markerMap), "rows": rows})
		command := exec.Command("node", bridge)
		command.Stdin = bytes.NewReader(payload)
		result, e := command.Output()
		var v struct {
			Accepted bool `json:"accepted"`
			Writes   int  `json:"writes"`
		}
		if json.Unmarshal(result, &v) != nil || v.Accepted != want || v.Writes != 0 || (want && e != nil) || (!want && e == nil) {
			t.Fatalf("unexpected local inspection: %s, %v", result, e)
		}
	}
	// Actual workers publish evidence but leave fast-notification outboxes pending.
	inspect(false, nil)
	beforeEvidence := map[string][]byte{}
	for pos := range run.Steps {
		execution := workflowIdentity(run.WorkflowRunID, pos, "execution")
		raw, e := f.s.Evidence(ctx, f.owner, f.project, execution)
		if e != nil {
			t.Fatal(e)
		}
		beforeEvidence[execution] = raw
	}
	settled := false
	for attempt := 0; attempt < 30; attempt++ {
		if e := f.s.Maintain(ctx); e != nil {
			t.Fatal(e)
		}
		settled = true
		for pos := range run.Steps {
			out, e := f.s.get(ctx, f.s.Table, "P#"+f.project, workflowMessage(f, run, pos).OutboxID)
			if e != nil || out.State != "published" {
				settled = false
			}
		}
		if settled {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !settled {
		t.Fatal("actual local maintenance did not settle both outboxes")
	}
	before := inventory()
	inspect(true, nil)
	for _, kind := range []string{"executionJob", "executionSnapshot", "executionUpload"} {
		inspect(false, func(rows []map[string]any) []map[string]any {
			for i, r := range rows {
				v := r["item"].(map[string]any)
				if v["kind"].(map[string]any)["S"] == kind {
					return append(rows[:i], rows[i+1:]...)
				}
			}
			t.Fatal("missing actual publication fixture row", kind)
			return rows
		})
	}
	inspect(false, func(rows []map[string]any) []map[string]any {
		for _, r := range rows {
			v := r["item"].(map[string]any)
			if v["kind"].(map[string]any)["S"] == "executionJob" {
				v["state"] = map[string]any{"S": "running"}
				break
			}
		}
		return rows
	})
	after := inventory()
	// Scan order is not guaranteed; compare each table/key rather than array order.
	keyed := func(rows []map[string]any) map[string]any {
		m := map[string]any{}
		for _, r := range rows {
			v := r["item"].(map[string]any)
			m[r["table"].(string)+"/"+v["PK"].(map[string]any)["S"].(string)+"/"+v["SK"].(map[string]any)["S"].(string)] = v
		}
		return m
	}
	if !reflect.DeepEqual(keyed(before), keyed(after)) {
		t.Fatal("inspection changed durable metadata")
	}
	for execution, want := range beforeEvidence {
		got, e := f.s.Evidence(ctx, f.owner, f.project, execution)
		if e != nil || !bytes.Equal(got, want) {
			t.Fatal("inspection/maintenance changed retained evidence", e)
		}
		snap, e := f.s.get(ctx, f.s.Table, "P#"+f.project, "EXEC#"+execution+"#SNAP")
		if e != nil {
			t.Fatal(e)
		}
		// Verify the actual immutable fixture object, not the owner response envelope.
		stored := f.objects.data[snap.ObjectKey]
		sum := sha256.Sum256(stored)
		var evidence Evidence
		if int64(len(stored)) != snap.Bytes || hex.EncodeToString(sum[:]) != snap.Checksum || json.Unmarshal(stored, &evidence) != nil || evidence.SchemaVersion != 1 || evidence.Assertions != nil {
			t.Fatal("legacy immutable publication changed")
		}
	}
	if f.calls.Load() != 2 {
		t.Fatal("maintenance/inspection replayed HTTP")
	}
	durableStage, e := f.s.get(ctx, f.s.Table, "STAGE#dev-api", "META")
	if e != nil || !reflect.DeepEqual(stage, durableStage) {
		t.Fatal("inspection changed stage marker")
	}
	t.Log("actual local legacy2-step/2-sends -> maintenance published -> closure inspection writes0; schema1 bytes/checksums/marker/accounting retained")
}
