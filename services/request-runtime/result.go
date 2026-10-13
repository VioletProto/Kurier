package execution

import (
	"encoding/base64"
	"encoding/json"
	"github.com/cyberphone/json-canonicalization/go/src/webpki.org/jsoncanonicalizer"
)

// CanonicalResult is version 1: explicit null scalars, ordered arrays and body
// text retained. Typed integer fields reject unsafe/decimal/nonfinite numbers.
func CanonicalResult(r Result) ([]byte, error) {
	nonce, e := base64.RawURLEncoding.DecodeString(r.Nonce)
	if e != nil || len(nonce) != 32 || base64.RawURLEncoding.EncodeToString(nonce) != r.Nonce || r.SchemaVersion != 1 || r.Fence < 1 || r.Fence > 9007199254740991 || r.LeaseID == "" {
		return nil, failure(400, "invalid_request")
	}
	if r.Capture.Request.Headers == nil {
		r.Capture.Request.Headers = []Header{}
	}
	if r.Capture.Response.Headers == nil {
		r.Capture.Response.Headers = []Header{}
	}
	raw, e := json.Marshal(r)
	if e != nil || len(raw) > EvidenceLimit {
		return nil, failure(413, "payload_too_large")
	}
	canonical, e := jsoncanonicalizer.Transform(raw)
	if e != nil {
		return nil, failure(400, "invalid_request")
	}
	return canonical, nil
}
func ResultFromEvidence(w Work, e *Evidence) (Result, error) {
	r := Result{1, w.LeaseID, w.Fence, w.Nonce, CaptureEvidence(e)}
	raw, _ := json.Marshal(r)
	if !resultTransportFits(raw) {
		r.Capture.Request.Body = omitted("encoded_evidence_limit")
		r.Capture.Response.Body = omitted("encoded_evidence_limit")
	}
	_, err := CanonicalResult(r)
	return r, err
}

func resultTransportFits(raw []byte) bool {
	quoted, _ := json.Marshal(string(raw))
	base64Bytes := 4 * ((len(raw) + 2) / 3)
	// Bound both possible payload-v2 ingress representations, with 32 KiB fixed
	// headers/context. Freeze omissions before canonical identity or upload.
	return len(raw) <= EvidenceLimit && len(quoted)+32*1024+1 <= LambdaLimit && base64Bytes+32*1024+1 <= LambdaLimit
}
