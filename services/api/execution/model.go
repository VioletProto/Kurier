package execution

import (
	"encoding/base64"
	"encoding/json"
	runner "github.com/VioletProto/Kurier/services/request-runtime"
	"github.com/aws/aws-lambda-go/events"
	"strings"
	"time"
)

const (
	ConfigLimit   = runner.ConfigLimit
	BodyLimit     = runner.BodyLimit
	EvidenceLimit = runner.EvidenceLimit
	LambdaLimit   = runner.LambdaLimit
	MetadataLimit = runner.MetadataLimit
)

type Error = runner.Error
type Ref = runner.Ref
type Field = runner.Field
type SecretField = runner.SecretField
type RequestBody = runner.RequestBody
type Configuration = runner.Configuration
type RedactionPolicy = runner.RedactionPolicy
type Options = runner.Options
type Plan = runner.Plan
type Source = runner.Source
type Header = runner.Header
type Body = runner.Body
type RequestCapture = runner.RequestCapture
type ResponseCapture = runner.ResponseCapture
type Timing = runner.Timing
type Outcome = runner.Outcome
type Evidence = runner.Evidence
type Summary = runner.Summary
type Retention = runner.Retention
type View = runner.View
type Prepared = runner.Prepared
type Resolver = runner.Resolver

var StrictJSON = runner.StrictJSON
var NonNullObject = runner.NonNullObject
var ParseOptions = runner.ParseOptions
var Prepare = runner.Prepare
var NewEvidence = runner.NewEvidence
var ValidateDestination = runner.ValidateDestination
var PublicIP = runner.PublicIP

func failure(status int, code string) error { return &Error{Status: status, Code: code} }
func safeError() error                      { return failure(503, "service_unavailable") }
func id() string                            { return runner.NewID() }
func stamp(t time.Time) string              { return t.UTC().Format("2006-01-02T15:04:05.000000000Z") }
func pointer[T any](v T) *T                 { return &v }
func emptyBody() Body                       { return Body{Kind: "none"} }
func omitted(reason string) Body            { return Body{Kind: "omitted", OmissionReason: &reason} }
func EncodeEvidence(e *Evidence) ([]byte, []byte, error) {
	for attempt := 0; attempt < 2; attempt++ {
		raw, err := json.Marshal(e)
		if err != nil {
			return nil, nil, safeError()
		}
		api, err := json.Marshal(map[string]any{"data": map[string]any{"evidence": json.RawMessage(raw)}})
		if err != nil {
			return nil, nil, safeError()
		}
		// Standard base64 contains no JSON-escaped characters. Reserve the complete
		// maximum permitted outer metadata, independently from the browser cap.
		if len(raw) <= EvidenceLimit && len(api) <= EvidenceLimit && publicationTransportFits(api) {
			return raw, api, nil
		}
		e.Request.Body = omitted("encoded_evidence_limit")
		e.Response.Body = omitted("encoded_evidence_limit")
	}
	return nil, nil, safeError()
}

// Measure the same payload-v2 type and JSON serializer as the deployed adapter.
// Fill the entire serialized metadata allowance; base64 needs no JSON escaping.
func publicationTransportFits(api []byte) bool {
	proxy := events.APIGatewayV2HTTPResponse{StatusCode: 200, Headers: map[string]string{"X-Bound": ""}, IsBase64Encoded: true}
	metadata, _ := json.Marshal(proxy)
	proxy.Headers["X-Bound"] = strings.Repeat("x", MetadataLimit-len(metadata))
	metadata, _ = json.Marshal(proxy)
	proxy.Body = base64.StdEncoding.EncodeToString(api)
	serialized, err := json.Marshal(proxy)
	return err == nil && len(metadata) <= MetadataLimit && len(serialized)+1 <= LambdaLimit
}

type LocalOptions = runner.LocalOptions
