package execution

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPublicationTraceContainsOnlySafeIdentifiersAndCodes(t *testing.T) {
	const sensitive = "sensitive diagnostics must not escape"
	var events []PublicationTrace
	s := &Service{PublicationTrace: func(v PublicationTrace) { events = append(events, v) }}
	job := row{ProjectID: id(), RunID: id(), JobID: id(), ExecutionID: id(), LeaseID: id(), Fence: 4, ConfigurationJSON: sensitive}
	ticket := row{SK: "UPLOAD#" + id(), ObjectKey: sensitive, Checksum: sensitive}
	s.tracePublication(job, ticket, "settle", canceledCodes("TransactionConflict"))
	if len(events) != 1 || events[0].Code != "transaction_conflict" || events[0].Fence != 4 {
		t.Fatal("missing safe trace")
	}
	raw, _ := json.Marshal(events)
	if strings.Contains(string(raw), sensitive) {
		t.Fatal("trace leaked SDK diagnostics or row data")
	}
	s.tracePublication(job, ticket, sensitive, nil)
	ticket.SK = sensitive
	s.tracePublication(job, ticket, "settle", nil)
	job.LeaseID = sensitive
	s.tracePublication(job, row{}, "finalize", nil)
	if len(events) != 1 {
		t.Fatal("unsafe phase or identifiers logged")
	}
}
