package execution

import (
	"encoding/json"
	runner "github.com/VioletProto/Kurier/services/request-runtime"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"strings"
	"testing"
)

func TestAssertionResultSurvivesEncodedBodyOmission(t *testing.T) {
	p := fixturePlan()
	e := NewEvidence(id(), id(), stampTime(), p)
	step := runner.WorkflowStep{Assertions: []runner.Assertion{{AssertionID: id(), Kind: "jsonEquals", Path: "$.ok", Expected: json.RawMessage(`true`)}}}
	e.SchemaVersion = 2
	e.Assertions = runner.NewAssertionEvaluation(step, "evaluation_unavailable")
	e.Assertions.Results[0].Outcome = "passed"
	e.Assertions.Results[0].Reason = "matched"
	e.Status = "completed"
	e.Outcome = Outcome{Certainty: "known"}
	e.Response.Body = Body{Kind: "text", Text: pointer(strings.Repeat("x", EvidenceLimit))}
	raw, api, err := EncodeEvidence(e)
	if err != nil || len(raw) > EvidenceLimit || len(api) > EvidenceLimit || !publicationTransportFits(api) {
		t.Fatal("encoded evidence did not fit", err)
	}
	var v Evidence
	if StrictJSON(raw, &v) != nil || v.Response.Body.OmissionReason == nil || *v.Response.Body.OmissionReason != "encoded_evidence_limit" || !v.Assertions.Passed() {
		t.Fatal("body omission invalidated immutable evaluation")
	}
}

func TestAssertionAdmissionMetadataHeadroomAndRunDTO(t *testing.T) {
	r := row{PK: "P#" + id(), SK: "RUN#" + id(), Kind: "executionWorkflowRun", ConfigurationJSON: "x"}
	item, _ := attributevalue.MarshalMap(r)
	raw, _ := json.Marshal(item)
	r.ConfigurationJSON = strings.Repeat("x", 128*1024-4096-len(raw)+1)
	if workflowAdmissionHeadroom(r) != nil {
		t.Fatal("exact item headroom rejected")
	}
	r.ConfigurationJSON += "x"
	if workflowAdmissionHeadroom(r) == nil {
		t.Fatal("terminal headroom not reserved")
	}
	s := RunStep{WorkflowStep: runner.WorkflowStep{StepID: id(), Extract: []runner.Extraction{}, Bindings: []runner.Binding{}}, Position: 1, State: "pending"}
	b, _ := json.Marshal(s)
	var decoded RunStep
	if StrictJSON(b, &decoded) != nil || decoded.Position != 1 || decoded.State != "pending" || decoded.Assertions == nil {
		t.Fatal("embedded authoring DTO hid run state")
	}
}
