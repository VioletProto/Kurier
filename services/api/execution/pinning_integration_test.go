//go:build integration

package execution

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dt "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func pinFixture(t *testing.T) (*protocolFixture, string, row) {
	t.Helper()
	f := newProtocolFixture(t)
	ctx := context.Background()
	options, _ := ParseOptions([]byte(`{}`))
	v, err := f.s.Submit(ctx, f.owner, f.project, f.request, id(), 1, options, false)
	if err != nil {
		t.Fatal(err)
	}
	anchor, _ := f.s.get(ctx, f.s.Table, "P#"+f.project, "EXEC#"+v.ExecutionID)
	if err = f.s.Process(ctx, Message{f.project, anchor.JobID, "OUT#" + anchor.JobID}); err != nil {
		t.Fatal(err)
	}
	stage, _ := f.s.stage(ctx)
	stage.SavedRequestsSchemaVersion = 4
	stage.EvidencePinningSchemaVersion = 1
	stage.Version++
	f.write(t, f.s.Table, stage)
	p, _ := f.s.project(ctx, f.owner, f.project)
	p.PinnedCount = pointer(int64(0))
	p.PinnedBytes = pointer(int64(0))
	f.write(t, f.s.Table, p)
	quota, _ := f.s.get(ctx, f.s.Table, "STAGE#dev-api", "QUOTA")
	quota.PinnedCount = pointer(int64(0))
	quota.PinnedBytes = pointer(int64(0))
	f.write(t, f.s.Table, quota)
	snap, err := f.s.get(ctx, f.s.Table, p.PK, anchor.SK+"#SNAP")
	if err != nil {
		t.Fatal(err)
	}
	return f, v.ExecutionID, snap
}
func pinCode(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	if err == nil {
		return ""
	}
	return "unexpected"
}
func assertPinAccounting(t *testing.T, f *protocolFixture, count, bytes int64) {
	t.Helper()
	p, err := f.s.get(context.Background(), f.s.Table, "P#"+f.project, "META")
	if err != nil {
		t.Fatal(err)
	}
	q, err := f.s.get(context.Background(), f.s.Table, "STAGE#dev-api", "QUOTA")
	if err != nil {
		t.Fatal(err)
	}
	if !healthyPinCounters(p) || !healthyPinCounters(q) || *p.PinnedCount != count || *q.PinnedCount != count || *p.PinnedBytes != bytes || *q.PinnedBytes != bytes {
		t.Fatal("pin accounting differs from expected committed classification")
	}
}
func TestIntegrationPinRepeatsExpiryOwnershipImmutable(t *testing.T) {
	f, execution, snap := pinFixture(t)
	ctx := context.Background()
	before, err := f.s.Evidence(ctx, f.owner, f.project, execution)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.Pin(ctx, id(), f.project, execution, true); pinCode(err) != "not_found" {
		t.Fatal("foreign owner allowed")
	}
	if _, err = f.s.Pin(ctx, f.owner, id(), execution, true); pinCode(err) != "not_found" {
		t.Fatal("foreign project allowed")
	}
	first, err := f.s.Pin(ctx, f.owner, f.project, execution, true)
	if err != nil {
		t.Fatal(err)
	}
	repeat, err := f.s.Pin(ctx, f.owner, f.project, execution, true)
	if err != nil || repeat.Retention.Version != first.Retention.Version || *repeat.Retention.PinnedAt != *first.Retention.PinnedAt {
		t.Fatal("pin repeat mutated retention")
	}
	assertPinAccounting(t, f, 1, snap.Bytes)
	after, err := f.s.Evidence(ctx, f.owner, f.project, execution)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("pin changed evidence")
	}
	afterSnap, _ := f.s.get(ctx, f.s.Table, snap.PK, snap.SK)
	if afterSnap.Checksum != snap.Checksum || afterSnap.Bytes != snap.Bytes || afterSnap.Version != snap.Version {
		t.Fatal("pin changed manifest")
	}
	f.now = f.now.Add(31 * 24 * time.Hour)
	if _, err = f.s.Pin(ctx, f.owner, f.project, execution, true); err != nil {
		t.Fatal("existing pin lost after expiry", err)
	}
	ret, _ := f.s.get(ctx, f.s.Table, snap.PK, "EXEC#"+execution+"#RET")
	if err = f.s.expire(ctx, ret); pinCode(err) != "conflict" {
		t.Fatal("cleaner accepted pin")
	}
	unpin, err := f.s.Pin(ctx, f.owner, f.project, execution, false)
	if err != nil || unpin.Retention.EffectiveStatus != "expired" || unpin.Retention.NormalExpiresAt != first.Retention.NormalExpiresAt {
		t.Fatal("unpin granted grace", err)
	}
	assertPinAccounting(t, f, 0, 0)
	if _, err = f.s.Detail(ctx, f.owner, f.project, execution); pinCode(err) != "not_found" {
		t.Fatal("expired detail accessible")
	}
	if _, err = f.s.Evidence(ctx, f.owner, f.project, execution); pinCode(err) != "not_found" {
		t.Fatal("expired evidence accessible")
	}
	repeat, err = f.s.Pin(ctx, f.owner, f.project, execution, false)
	if err != nil || repeat.Retention.Version != unpin.Retention.Version {
		t.Fatal("unpin repeat mutated")
	}
	if _, err = f.s.Pin(ctx, f.owner, f.project, execution, true); pinCode(err) != "pin_window_closed" {
		t.Fatal("expired evidence resurrected")
	}
	for n := 0; n < 20; n++ {
		ret, err = f.s.get(ctx, f.s.Table, snap.PK, "EXEC#"+execution+"#RET")
		if missingRow(err) {
			break
		}
		_ = f.s.expire(ctx, ret)
	}
	if _, err = f.s.Pin(ctx, f.owner, f.project, execution, false); pinCode(err) != "not_found" {
		t.Fatal("cleaned anchor survived")
	}
	q, _ := f.s.get(ctx, f.s.Table, "STAGE#dev-api", "QUOTA")
	if q.StorageReservedBytes != 0 {
		t.Fatal("storage charge not settled")
	}
}

func TestIntegrationPinWindowAvailabilityQuotas(t *testing.T) {
	for _, mode := range []string{"outside", "exact", "inside", "expired", "missing", "corrupt", "unavailable", "count-project", "count-stage", "bytes-project", "bytes-stage", "missing-counter", "disabled", "closed-stage", "unpublished", "preterminal"} {
		t.Run(mode, func(t *testing.T) {
			f, execution, snap := pinFixture(t)
			ctx := context.Background()
			ret, _ := f.s.get(ctx, f.s.Table, snap.PK, "EXEC#"+execution+"#RET")
			want := "pin_window_closed"
			switch mode {
			case "outside", "exact", "inside", "expired":
				offset := 6 * time.Second
				if mode == "exact" {
					offset = 5 * time.Second
				}
				if mode == "inside" {
					offset = 4 * time.Second
				}
				if mode == "expired" {
					offset = -time.Second
				}
				ret.NormalExpiresAt = stamp(f.now.Add(offset))
				f.write(t, f.s.Table, ret)
				if mode == "outside" {
					want = ""
				}
			case "missing", "corrupt":
				f.objects.mu.Lock()
				if mode == "missing" {
					delete(f.objects.data, snap.ObjectKey)
				} else {
					f.objects.data[snap.ObjectKey] = []byte(`broken`)
				}
				f.objects.mu.Unlock()
				want = "evidence_unavailable"
			case "unavailable":
				ret.EvidenceAvailability = "unavailable"
				f.write(t, f.s.Table, ret)
				want = "evidence_unavailable"
			case "count-project", "count-stage", "bytes-project", "bytes-stage", "missing-counter":
				p, _ := f.s.get(ctx, f.s.Table, snap.PK, "META")
				q, _ := f.s.get(ctx, f.s.Table, "STAGE#dev-api", "QUOTA")
				switch mode {
				case "count-project":
					p.PinnedCount = pointer(ProjectPinLimit)
					p.PinnedBytes = pointer(int64(1))
				case "count-stage":
					q.PinnedCount = pointer(StagePinLimit)
					q.PinnedBytes = pointer(int64(1))
				case "bytes-project":
					p.PinnedCount = pointer(int64(1))
					p.PinnedBytes = pointer(ProjectPinBytes - snap.Bytes + 1)
				case "bytes-stage":
					q.PinnedCount = pointer(int64(1))
					q.PinnedBytes = pointer(StagePinBytes - snap.Bytes + 1)
				case "missing-counter":
					p.PinnedBytes = nil
				}
				f.write(t, f.s.Table, p)
				f.write(t, f.s.Table, q)
				want = "pin_quota_exceeded"
				if mode == "missing-counter" {
					want = "service_unavailable"
				}
			case "disabled":
				u, _ := f.s.get(ctx, f.s.Table, "U#"+f.owner, "META")
				u.Disabled = true
				f.write(t, f.s.Table, u)
				want = "forbidden"
			case "closed-stage":
				stage, _ := f.s.stage(ctx)
				stage.State = "recovering"
				f.write(t, f.s.Table, stage)
				want = "service_unavailable"
			case "unpublished":
				_, _ = f.db.Client.DeleteItem(ctx, &dynamodb.DeleteItemInput{TableName: &f.s.Table, Key: keys(snap.PK, snap.SK)})
				want = "conflict"
			case "preterminal":
				a, _ := f.s.get(ctx, f.s.Table, snap.PK, "EXEC#"+execution)
				a.State = "running"
				f.write(t, f.s.Table, a)
				want = "conflict"
			}
			_, err := f.s.Pin(ctx, f.owner, f.project, execution, true)
			if pinCode(err) != want {
				t.Fatalf("want %s got %s", want, pinCode(err))
			}
		})
	}
}

func TestIntegrationPinCleanupTimingRaces(t *testing.T) {
	for _, mode := range []string{"pin-first", "cleaner-first", "fresh-time", "slow-commit", "project-delete"} {
		t.Run(mode, func(t *testing.T) {
			f, execution, snap := pinFixture(t)
			ctx := context.Background()
			ret, _ := f.s.get(ctx, f.s.Table, snap.PK, "EXEC#"+execution+"#RET")
			if mode == "pin-first" {
				if _, err := f.s.Pin(ctx, f.owner, f.project, execution, true); err != nil {
					t.Fatal(err)
				}
				f.now = f.now.Add(31 * 24 * time.Hour)
				if err := f.s.expire(ctx, ret); err == nil {
					t.Fatal("stale cleaner won over pin")
				}
				return
			}
			if mode == "cleaner-first" {
				f.now = f.now.Add(31 * 24 * time.Hour)
				_ = f.s.expire(ctx, ret)
				if _, err := f.s.Pin(ctx, f.owner, f.project, execution, true); pinCode(err) != "conflict" && pinCode(err) != "not_found" {
					t.Fatal("cleanup reversed")
				}
				return
			}
			var fired atomic.Bool
			f.db.before = func(in *dynamodb.TransactWriteItemsInput) error {
				if !writesPin(in, true) || fired.Swap(true) {
					return nil
				}
				if mode == "project-delete" {
					p, _ := f.s.project(ctx, f.owner, f.project)
					p.State = "deleting"
					p.DeletionEpoch++
					p.Version++
					f.write(t, f.s.Table, p)
					return nil
				}
				expiry, _ := time.Parse(time.RFC3339Nano, ret.NormalExpiresAt)
				f.now = expiry.Add(-4 * time.Second)
				if mode == "fresh-time" {
					return &dt.TransactionCanceledException{CancellationReasons: []dt.CancellationReason{{Code: aws.String("ConditionalCheckFailed")}}}
				}
				return nil
			}
			result, err := f.s.Pin(ctx, f.owner, f.project, execution, true)
			if mode == "fresh-time" && pinCode(err) != "pin_window_closed" {
				t.Fatal("reused old decision", err)
			}
			if mode == "slow-commit" && (err != nil || !result.Retention.Pinned) {
				t.Fatal("did not preserve documented decision-time boundary", err)
			}
			if mode == "project-delete" && pinCode(err) != "not_found" {
				t.Fatal("deletion gate bypassed", err)
			}
		})
	}
}
func writesPin(in *dynamodb.TransactWriteItemsInput, pinned bool) bool {
	for _, a := range in.TransactItems {
		if a.Put != nil && aws.ToString(func() *string {
			v, ok := a.Put.Item["kind"].(*dt.AttributeValueMemberS)
			if !ok {
				return nil
			}
			return &v.Value
		}()) == "executionRetention" {
			v, ok := a.Put.Item["pinned"].(*dt.AttributeValueMemberBOOL)
			return (ok && v.Value) == pinned
		}
	}
	return false
}
func TestIntegrationPinConcurrentAndLostACK(t *testing.T) {
	f, execution, snap := pinFixture(t)
	ctx := context.Background()
	for _, pin := range []bool{true, false, true} {
		var wg sync.WaitGroup
		errs := make(chan error, 8)
		for n := 0; n < 8; n++ {
			wg.Add(1)
			go func() { defer wg.Done(); _, err := f.s.Pin(ctx, f.owner, f.project, execution, pin); errs <- err }()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil && pinCode(err) != "conflict" && pinCode(err) != "service_unavailable" {
				t.Fatal(err)
			}
		}
		count := int64(0)
		if pin {
			count = 1
		}
		assertPinAccounting(t, f, count, count*snap.Bytes)
	}
	f.db.after = func(in *dynamodb.TransactWriteItemsInput) error {
		if writesPin(in, false) {
			return errors.New("simulated lost acknowledgement")
		}
		return nil
	}
	if _, err := f.s.Pin(ctx, f.owner, f.project, execution, false); err != nil {
		t.Fatal("same-token readback did not reconcile", err)
	}
	assertPinAccounting(t, f, 0, 0)
	f.db.after = nil
	var wg sync.WaitGroup
	for n := 0; n < 8; n++ {
		wg.Add(1)
		go func(pin bool) { defer wg.Done(); _, _ = f.s.Pin(ctx, f.owner, f.project, execution, pin) }(n%2 == 0)
	}
	wg.Wait()
	ret, _ := f.s.get(ctx, f.s.Table, snap.PK, "EXEC#"+execution+"#RET")
	count := int64(0)
	if ret.Pinned {
		count = 1
	}
	assertPinAccounting(t, f, count, count*snap.Bytes)
}

func TestIntegrationPinnedProjectDrain(t *testing.T) {
	f, execution, snap := pinFixture(t)
	ctx := context.Background()
	if _, err := f.s.Pin(ctx, f.owner, f.project, execution, true); err != nil {
		t.Fatal(err)
	}
	p, _ := f.s.project(ctx, f.owner, f.project)
	p.State = "deleting"
	p.Version++
	p.DeletionEpoch++
	f.write(t, f.s.Table, p)
	if _, err := f.s.Pin(ctx, f.owner, f.project, execution, false); pinCode(err) != "not_found" {
		t.Fatal("deleting project accessible")
	}
	for n := 0; n < 30; n++ {
		err := f.s.DrainProject(ctx, f.project)
		if err == nil {
			break
		}
		if pinCode(err) != "cleanup_pending" {
			t.Fatal(err)
		}
	}
	assertPinAccounting(t, f, 0, 0)
	if _, err := f.s.get(ctx, f.s.Table, snap.PK, snap.SK); !missingRow(err) {
		t.Fatal("pinned manifest survived project drain")
	}
	if len(f.objects.data) != 0 {
		t.Fatal("pinned objects survived")
	}
	q, _ := f.s.get(ctx, f.s.Table, "STAGE#dev-api", "QUOTA")
	if q.StorageReservedBytes != 0 {
		t.Fatal("pinned project bytes not settled")
	}
}

func TestIntegrationPinUnavailableRepeatAndUnpin(t *testing.T) {
	f, execution, snap := pinFixture(t)
	ctx := context.Background()
	if _, err := f.s.Pin(ctx, f.owner, f.project, execution, true); err != nil {
		t.Fatal(err)
	}
	f.objects.mu.Lock()
	delete(f.objects.data, snap.ObjectKey)
	f.objects.mu.Unlock()
	if _, err := f.s.Evidence(ctx, f.owner, f.project, execution); pinCode(err) != "evidence_unavailable" {
		t.Fatal("lost evidence was fabricated")
	}
	result, err := f.s.Pin(ctx, f.owner, f.project, execution, true)
	if err != nil || result.Retention.EvidenceAvailability != "unavailable" {
		t.Fatal("repeat concealed unavailable evidence", err)
	}
	if _, err = f.s.Pin(ctx, f.owner, f.project, execution, false); err != nil {
		t.Fatal(err)
	}
	assertPinAccounting(t, f, 0, 0)
	if _, err = f.s.Pin(ctx, f.owner, f.project, execution, true); pinCode(err) != "evidence_unavailable" {
		t.Fatal("lost evidence re-pinned")
	}
}

func TestIntegrationPinExactLimitsAndObscuredACK(t *testing.T) {
	for _, kind := range []string{"count", "bytes", "obscured-ack", "bad-generation"} {
		t.Run(kind, func(t *testing.T) {
			f, execution, snap := pinFixture(t)
			ctx := context.Background()
			if kind == "count" || kind == "bytes" {
				p, _ := f.s.project(ctx, f.owner, f.project)
				q, _ := f.s.get(ctx, f.s.Table, "STAGE#dev-api", "QUOTA")
				p.PinnedCount = pointer(ProjectPinLimit - 1)
				q.PinnedCount = pointer(StagePinLimit - 1)
				p.PinnedBytes = pointer(int64(1))
				q.PinnedBytes = pointer(int64(1))
				if kind == "bytes" {
					p.PinnedBytes = pointer(ProjectPinBytes - snap.Bytes)
					q.PinnedBytes = pointer(StagePinBytes - snap.Bytes)
				}
				f.write(t, f.s.Table, p)
				f.write(t, f.s.Table, q)
				result, err := f.s.Pin(ctx, f.owner, f.project, execution, true)
				if err != nil {
					t.Fatal("exact limit rejected", err)
				}
				repeated, err := f.s.Pin(ctx, f.owner, f.project, execution, true)
				if err != nil || result.Retention.Version != repeated.Retention.Version {
					t.Fatal("repeat at cap rejected", err)
				}
				return
			}
			var fired atomic.Bool
			if kind == "obscured-ack" {
				f.db.after = func(in *dynamodb.TransactWriteItemsInput) error {
					if !writesPin(in, true) {
						return nil
					}
					if !fired.Swap(true) {
						ret, _ := f.s.get(ctx, f.s.Table, snap.PK, "EXEC#"+execution+"#RET")
						ret.Version++
						ret.Pinned = false
						f.write(t, f.s.Table, ret)
					}
					return errors.New("simulated acknowledgement obscured by later transition")
				}
			} else {
				f.db.before = func(in *dynamodb.TransactWriteItemsInput) error {
					if writesPin(in, true) && !fired.Swap(true) {
						stage, _ := f.s.stage(ctx)
						stage.RecoveryGeneration = "changed"
						stage.Version++
						f.write(t, f.s.Table, stage)
					}
					return nil
				}
			}
			_, err := f.s.Pin(ctx, f.owner, f.project, execution, true)
			if kind == "obscured-ack" && pinCode(err) != "service_unavailable" {
				t.Fatal("obscured outcome guessed", err)
			}
			// A known cancelled attempt may re-authorize under the new generation,
			// but must use it in every newly prepared transaction.
			if kind == "bad-generation" && err != nil && pinCode(err) != "conflict" {
				t.Fatal(err)
			}
		})
	}
}

func TestIntegrationPinRetentionDrainCannotAdoptProjectDeletion(t *testing.T) {
	f, execution, snap := pinFixture(t)
	ctx := context.Background()
	f.now = f.now.Add(31 * 24 * time.Hour)
	ret, _ := f.s.get(ctx, f.s.Table, snap.PK, "EXEC#"+execution+"#RET")
	_ = f.s.expire(ctx, ret)
	p, _ := f.s.project(ctx, f.owner, f.project)
	p.State = "deleting"
	p.Version++
	p.DeletionEpoch++
	f.write(t, f.s.Table, p)
	if err := f.s.drainExecution(ctx, p, execution, true); pinCode(err) != "conflict" {
		t.Fatal("retention drain adopted deletion state", err)
	}
	for n := 0; n < 30; n++ {
		err := f.s.DrainProject(ctx, f.project)
		if err == nil {
			return
		}
		if pinCode(err) != "cleanup_pending" {
			t.Fatal(err)
		}
	}
	t.Fatal("explicit project drain did not settle")
}

func TestIntegrationPinLastStageSlotAcrossProjects(t *testing.T) {
	f, first, _ := pinFixture(t)
	ctx := context.Background()
	other := id()
	p, _ := f.s.project(ctx, f.owner, f.project)
	p.PK = "P#" + other
	p.ProjectID = other
	p.Version = 0
	p.StorageReservedBytes = 0
	f.write(t, f.s.Table, p)
	request, _ := f.s.get(ctx, f.s.Table, "P#"+f.project, "REQ#"+f.request)
	request.PK = p.PK
	request.ProjectID = other
	f.write(t, f.s.Table, request)
	options, _ := ParseOptions([]byte(`{}`))
	v, err := f.s.Submit(ctx, f.owner, other, f.request, id(), 1, options, false)
	if err != nil {
		t.Fatal(err)
	}
	anchor, _ := f.s.get(ctx, f.s.Table, p.PK, "EXEC#"+v.ExecutionID)
	if err = f.s.Process(ctx, Message{other, anchor.JobID, "OUT#" + anchor.JobID}); err != nil {
		t.Fatal(err)
	}
	quota, _ := f.s.get(ctx, f.s.Table, "STAGE#dev-api", "QUOTA")
	quota.PinnedCount = pointer(StagePinLimit - 1)
	quota.PinnedBytes = pointer(int64(199))
	f.write(t, f.s.Table, quota)
	var wg sync.WaitGroup
	for _, pair := range [][2]string{{f.project, first}, {other, v.ExecutionID}} {
		wg.Add(1)
		go func(pair [2]string) { defer wg.Done(); _, _ = f.s.Pin(ctx, f.owner, pair[0], pair[1], true) }(pair)
	}
	wg.Wait()
	q, _ := f.s.get(ctx, f.s.Table, "STAGE#dev-api", "QUOTA")
	if *q.PinnedCount != StagePinLimit {
		t.Fatal("concurrent project pins overfilled stage")
	}
	a, _ := f.s.get(ctx, f.s.Table, "P#"+f.project, "EXEC#"+first+"#RET")
	b, _ := f.s.get(ctx, f.s.Table, p.PK, "EXEC#"+v.ExecutionID+"#RET")
	if a.Pinned == b.Pinned {
		t.Fatal("last stage slot did not have one winner")
	}
}

func TestIntegrationPinnedMultichunkProjectAndUnknownRecord(t *testing.T) {
	f, first, _ := pinFixture(t)
	ctx := context.Background()
	ids := []string{first}
	options, _ := ParseOptions([]byte(`{}`))
	for n := 0; n < 12; n++ {
		v, err := f.s.Submit(ctx, f.owner, f.project, f.request, id(), 1, options, false)
		if err != nil {
			t.Fatal(err)
		}
		a, _ := f.s.get(ctx, f.s.Table, "P#"+f.project, "EXEC#"+v.ExecutionID)
		if err = f.s.Process(ctx, Message{f.project, a.JobID, "OUT#" + a.JobID}); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, v.ExecutionID)
	}
	for _, execution := range ids {
		if _, err := f.s.Pin(ctx, f.owner, f.project, execution, true); err != nil {
			t.Fatal(err)
		}
	}
	item := map[string]dt.AttributeValue{"PK": str("P#" + f.project), "SK": str("EXEC#future"), "kind": str("executionFuture"), "schemaVersion": num(99), "projectId": str(f.project)}
	_, err := f.db.Client.PutItem(ctx, &dynamodb.PutItemInput{TableName: &f.s.Table, Item: item})
	if err != nil {
		t.Fatal(err)
	}
	p, _ := f.s.project(ctx, f.owner, f.project)
	p.State = "deleting"
	p.Version++
	p.DeletionEpoch++
	f.write(t, f.s.Table, p)
	blocked := false
	for n := 0; n < 60; n++ {
		err = f.s.DrainProject(ctx, f.project)
		if pinCode(err) == "unknown_record" {
			blocked = true
			break
		}
		if err == nil {
			t.Fatal("unknown record permitted completion")
		}
	}
	if !blocked {
		t.Fatal("unknown record was not preserved")
	}
	// Test-only reviewed removal of the unknown fixture, not product cleanup.
	_, _ = f.db.Client.DeleteItem(ctx, &dynamodb.DeleteItemInput{TableName: &f.s.Table, Key: keys(p.PK, "EXEC#future")})
	complete := false
	for n := 0; n < 100; n++ {
		err = f.s.DrainProject(ctx, f.project)
		if err == nil {
			complete = true
			break
		}
		if pinCode(err) != "cleanup_pending" {
			t.Fatal(err)
		}
	}
	if !complete {
		t.Fatal("multichunk drain did not complete")
	}
	assertPinAccounting(t, f, 0, 0)
	q, _ := f.s.get(ctx, f.s.Table, "STAGE#dev-api", "QUOTA")
	if q.StorageReservedBytes != 0 || len(f.objects.data) != 0 {
		t.Fatal("multichunk pinned objects/accounting survived")
	}
}
