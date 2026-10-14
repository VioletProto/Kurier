// Package execution implements the bounded cloud execution and publication protocol.
// Runtime credentials never belong to Evidence or Control records.
package execution

import (
	"crypto/rand"
	"fmt"
	"time"
)

const ConfigLimit = 64 * 1024
const BodyLimit = 2 * 1024 * 1024
const EvidenceLimit = 4 * 1024 * 1024

// LocalSanitizedBodyLimit bounds defensive parsing of already sanitized text.
// JSON escaping can expand a response beyond BodyLimit; the complete result and
// evidence envelopes must still independently fit EvidenceLimit and LambdaLimit.
const LocalSanitizedBodyLimit = EvidenceLimit
const LambdaLimit = 6 * 1024 * 1024
const MetadataLimit = 16 * 1024

type Error struct {
	Status int
	Code   string
}

func (e *Error) Error() string              { return e.Code }
func failure(status int, code string) error { return &Error{status, code} }
func safeError() error                      { return failure(503, "service_unavailable") }
func id() string {
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		panic("secure randomness unavailable")
	}
	b[6] = b[6]&15 | 64
	b[8] = b[8]&63 | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
}
func stamp(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05.000000000Z") }

type Ref struct {
	SecretID string `json:"secretId"`
}
type Field struct {
	Name      string `json:"name"`
	Value     string `json:"value,omitempty"`
	Enabled   bool   `json:"enabled"`
	Sensitive bool   `json:"sensitive"`
	BindingID string `json:"bindingId,omitempty"`
	Masked    bool   `json:"masked,omitempty"`
	SecretRef *Ref   `json:"secretRef,omitempty"`
}
type SecretField struct {
	Pointer   string `json:"pointer"`
	BindingID string `json:"bindingId"`
	Masked    bool   `json:"masked,omitempty"`
	SecretRef *Ref   `json:"secretRef,omitempty"`
}
type RequestBody struct {
	Type         string        `json:"type"`
	Text         string        `json:"text,omitempty"`
	Sensitive    bool          `json:"sensitive"`
	BindingID    string        `json:"bindingId,omitempty"`
	Masked       bool          `json:"masked,omitempty"`
	SecretRef    *Ref          `json:"secretRef,omitempty"`
	SecretFields []SecretField `json:"secretFields,omitempty"`
}
type Configuration struct {
	Name            string       `json:"name"`
	Method          string       `json:"method"`
	URL             string       `json:"url"`
	QueryParameters []Field      `json:"queryParameters"`
	Headers         []Field      `json:"headers"`
	Body            *RequestBody `json:"body"`
	OperationRef    any          `json:"operationRef"`
}
type RedactionPolicy struct {
	Headers      []string `json:"headers"`
	JSONPointers []string `json:"jsonPointers"`
	OmitBody     bool     `json:"omitBody"`
}
type Options struct {
	Target string        `json:"target,omitempty" dynamodbav:"target,omitempty"`
	Local  *LocalOptions `json:"local,omitempty" dynamodbav:"local,omitempty"`

	TimeoutSeconds       int             `json:"timeoutSeconds"`
	AllowInsecureSecrets bool            `json:"allowInsecureSecrets"`
	ResponseRedaction    RedactionPolicy `json:"responseRedaction"`
}
type Plan struct {
	Target            string       `json:"target,omitempty" dynamodbav:"target,omitempty"`
	LocalAgentID      string       `json:"localAgentId,omitempty" dynamodbav:"localAgentId,omitempty"`
	Destination       *Destination `json:"destination,omitempty" dynamodbav:"destination,omitempty"`
	CredentialVersion int64        `json:"credentialVersion,omitempty" dynamodbav:"credentialVersion,omitempty"`

	Configuration Configuration `json:"configuration"`
	Options       Options       `json:"options"`
	RequestID     string        `json:"requestId"`
	Revision      int64         `json:"revision"`
}
type Source struct {
	RequestID string `json:"requestId"`
	Revision  int64  `json:"revision"`
}
type Header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
type Body struct {
	Kind           string  `json:"kind"`
	Text           *string `json:"text"`
	OmissionReason *string `json:"omissionReason"`
}
type RequestCapture struct {
	Method  string   `json:"method"`
	URL     string   `json:"url"`
	Headers []Header `json:"headers"`
	Body    Body     `json:"body"`
}
type ResponseCapture struct {
	HTTPStatus       *int     `json:"httpStatus"`
	Headers          []Header `json:"headers"`
	Body             Body     `json:"body"`
	WireBytesRead    int64    `json:"wireBytesRead"`
	DecodedBytesRead int64    `json:"decodedBytesRead"`
}
type Timing struct {
	DurationMS        *int64 `json:"durationMs"`
	TimeToFirstByteMS *int64 `json:"timeToFirstByteMs"`
}
type Outcome struct {
	Code      *string `json:"code"`
	Certainty string  `json:"certainty"`
}
type Evidence struct {
	SchemaVersion int             `json:"schemaVersion"`
	ProjectID     string          `json:"projectId"`
	ExecutionID   string          `json:"executionId"`
	Source        Source          `json:"source"`
	Target        string          `json:"target"`
	SubmittedAt   string          `json:"submittedAt"`
	StartedAt     *string         `json:"startedAt"`
	CompletedAt   string          `json:"completedAt"`
	Status        string          `json:"status"`
	Outcome       Outcome         `json:"outcome"`
	Request       RequestCapture  `json:"request"`
	Response      ResponseCapture `json:"response"`
	Timing        Timing          `json:"timing"`
	Redaction     Redaction       `json:"redaction"`
}

func omitted(reason string) Body { return Body{Kind: "omitted", OmissionReason: &reason} }
func emptyBody() Body            { return Body{Kind: "none"} }
func pointer[T any](v T) *T      { return &v }

type Summary struct {
	HTTPStatus *int    `json:"httpStatus"`
	DurationMS *int64  `json:"durationMs"`
	Outcome    Outcome `json:"outcome"`
}
type Retention struct {
	Version              int64   `json:"version"`
	PinnedAt             *string `json:"pinnedAt"`
	State                string  `json:"state"`
	EffectiveStatus      string  `json:"effectiveStatus"`
	PinEligible          bool    `json:"pinEligible"`
	NormalExpiresAt      string  `json:"normalExpiresAt"`
	Pinned               bool    `json:"pinned"`
	EvidenceAvailability string  `json:"evidenceAvailability"`
}
type View struct {
	Target       string  `json:"target"`
	LocalAgentID *string `json:"localAgentId"`

	ExecutionID   string         `json:"executionId"`
	ProjectID     string         `json:"projectId"`
	Status        string         `json:"status"`
	Version       int64          `json:"version"`
	SubmittedAt   string         `json:"submittedAt"`
	StartedAt     *string        `json:"startedAt"`
	CompletedAt   *string        `json:"completedAt"`
	Summary       *Summary       `json:"summary"`
	Retention     *Retention     `json:"retention"`
	ServerTime    string         `json:"serverTime"`
	Source        Source         `json:"source"`
	Configuration *Configuration `json:"configuration,omitempty"`
	LiveRequestID *string        `json:"liveRequestId"`
}

func NewID() string { return id() }

type Redaction struct {
	PolicyVersion int  `json:"policyVersion"`
	Applied       bool `json:"applied"`
}
