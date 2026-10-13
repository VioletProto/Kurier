package execution

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestEvidenceWorstCaseEscapingAndEnvelopeBoundary(t *testing.T) {
	for _, text := range []string{strings.Repeat("\"", BodyLimit), strings.Repeat("\\", BodyLimit), strings.Repeat("\x00", BodyLimit), strings.Repeat("<", BodyLimit), strings.Repeat("é", BodyLimit/2), strings.Repeat(`\u0000`, BodyLimit/6)} {
		plan := fixturePlan()
		e := NewEvidence(id(), id(), stampTime(), plan)
		e.Fail("execution_outcome_unknown", "unknown")
		e.Response.Body = Body{Kind: "text", Text: &text}
		raw, api, err := EncodeEvidence(e)
		if err != nil {
			t.Fatal(err)
		}
		if len(raw) > EvidenceLimit || len(api) > EvidenceLimit || base64.StdEncoding.EncodedLen(len(api))+MetadataLimit > LambdaLimit {
			t.Fatal("encoded publication bounds violated")
		}
		var envelope struct {
			Data struct {
				Evidence Evidence `json:"evidence"`
			} `json:"data"`
		}
		decoded, _ := base64.StdEncoding.DecodeString(base64.StdEncoding.EncodeToString(api))
		if json.Unmarshal(decoded, &envelope) != nil {
			t.Fatal("delivery roundtrip failed")
		}
		published, _ := json.Marshal(envelope.Data.Evidence)
		if !bytes.Equal(raw, published) {
			t.Fatal("delivery changed immutable capture")
		}
	}
	// Construct a capture that fits S3 but its wrapper crosses the browser cap.
	e := NewEvidence(id(), id(), stampTime(), fixturePlan())
	e.Fail("execution_outcome_unknown", "unknown")
	text := ""
	e.Response.Body = Body{Kind: "text", Text: &text}
	minimal, _ := json.Marshal(e)
	text = strings.Repeat("x", EvidenceLimit-len(minimal))
	candidate, _ := json.Marshal(e)
	if len(candidate) != EvidenceLimit {
		t.Fatal("boundary fixture incorrect")
	}
	_, _, err := EncodeEvidence(e)
	if err != nil || e.Response.Body.Kind != "omitted" || *e.Response.Body.OmissionReason != "encoded_evidence_limit" {
		t.Fatal("API-envelope overflow was not omitted before publication")
	}
}
