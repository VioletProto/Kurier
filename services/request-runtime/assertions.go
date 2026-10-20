package execution

import (
	"bytes"
	"encoding/json"
	"math/big"
	"mime"
	"regexp"
	"strconv"
	"strings"
)

const AssertionArrayLimit = 8 * 1024
const AssertionResultLimit = 2 * 1024

// Workflow authoring permits null only as an equality expected literal.
func ValidateWorkflowJSON(raw []byte) error {
	var root any
	if StrictJSON(raw, &root) != nil {
		return workflowInvalid()
	}
	if _, ok := root.(map[string]any); !ok {
		return workflowInvalid()
	}
	var valid func(any, []string) bool
	valid = func(v any, path []string) bool {
		if v == nil {
			return len(path) == 5 && path[0] == "steps" && path[2] == "assertions" && path[4] == "expected"
		}
		switch x := v.(type) {
		case map[string]any:
			for k, c := range x {
				if !valid(c, append(append([]string{}, path...), k)) {
					return false
				}
			}
		case []any:
			for _, c := range x {
				if !valid(c, append(append([]string{}, path...), "[]")) {
					return false
				}
			}
		}
		return true
	}
	if !valid(root, nil) {
		return workflowInvalid()
	}
	return nil
}

// Expected stays raw JSON on the wire. Numeric operands are tagged strings,
// never floating point or bare JSON numbers, including after editor/rerun reads.
type Assertion struct {
	AssertionID string          `json:"assertionId"`
	Kind        string          `json:"kind"`
	Path        string          `json:"path,omitempty"`
	Expected    json.RawMessage `json:"expected,omitempty"`
}

func (a *Assertion) UnmarshalJSON(raw []byte) error {
	var fields map[string]json.RawMessage
	if StrictJSON(raw, &fields) != nil || fields == nil {
		return workflowInvalid()
	}
	for k := range fields {
		if k != "assertionId" && k != "kind" && k != "path" && k != "expected" {
			return workflowInvalid()
		}
	}
	type plain Assertion
	var v plain
	if StrictJSON(raw, &v) != nil {
		return workflowInvalid()
	}
	if v.Kind == "statusEquals" && fields["path"] != nil || v.Kind == "jsonExists" && fields["expected"] != nil {
		return workflowInvalid()
	}
	*a = Assertion(v)
	return a.validate()
}

var decimalToken = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`)
var integerToken = regexp.MustCompile(`^(0|[1-9][0-9]*)$`)
var decimalMin, _ = new(big.Rat).SetString("1e-308")
var decimalMax = new(big.Rat).SetInt64(9007199254740991)

// ParseAssertionDecimal allocates only after token/exponent bounds are known.
// The returned rational is transient; the authored spelling is never replaced.
func ParseAssertionDecimal(token string) (*big.Rat, error) {
	if len(token) == 0 || len(token) > 128 || !decimalToken.MatchString(token) {
		return nil, workflowInvalid()
	}
	if i := strings.IndexAny(token, "eE"); i >= 0 {
		exponent, e := strconv.Atoi(token[i+1:])
		if e != nil || exponent < -308 || exponent > 308 {
			return nil, workflowInvalid()
		}
	}
	n, ok := new(big.Rat).SetString(token)
	if !ok {
		return nil, workflowInvalid()
	}
	abs := new(big.Rat).Abs(n)
	if abs.Cmp(decimalMax) > 0 || abs.Sign() != 0 && abs.Cmp(decimalMin) < 0 {
		return nil, workflowInvalid()
	}
	return n, nil
}

func (a Assertion) operand() (any, error) {
	var v any
	if len(a.Expected) == 0 || StrictJSON(a.Expected, &v) != nil {
		return nil, workflowInvalid()
	}
	switch x := v.(type) {
	case nil, string, bool:
		return v, nil
	case map[string]any:
		if len(x) != 2 || x["type"] != "number" {
			return nil, workflowInvalid()
		}
		token, ok := x["decimal"].(string)
		if !ok {
			return nil, workflowInvalid()
		}
		if _, e := ParseAssertionDecimal(token); e != nil {
			return nil, e
		}
		return json.Number(token), nil
	}
	return nil, workflowInvalid()
}

func (a Assertion) validate() error {
	if !workflowUUID.MatchString(a.AssertionID) {
		return workflowInvalid()
	}
	serialized, err := json.Marshal(a.Expected)
	if err != nil {
		return workflowInvalid()
	}
	if len(serialized) > 2048 {
		return failure(413, "payload_too_large")
	}
	switch a.Kind {
	case "statusEquals":
		token := string(a.Expected)
		n, e := strconv.Atoi(token)
		if a.Path != "" || !integerToken.MatchString(token) || e != nil || n < 100 || n > 599 {
			return workflowInvalid()
		}
	case "jsonExists", "jsonEquals":
		if _, e := ParseExtractionPath(a.Path); e != nil {
			return e
		}
		if a.Kind == "jsonExists" {
			if a.Expected != nil {
				return workflowInvalid()
			}
		} else if _, e := a.operand(); e != nil {
			return e
		}
	default:
		return workflowInvalid()
	}
	return nil
}

func assertionPointer(path string) string {
	parts, _ := ParseExtractionPath(path)
	out := ""
	for _, p := range parts {
		part := p.Property
		if p.Array {
			part = strconv.Itoa(p.Index)
		}
		out += "/" + strings.ReplaceAll(strings.ReplaceAll(part, "~", "~0"), "/", "~1")
	}
	return out
}

func assertionSensitivePath(a Assertion, step WorkflowStep, policy RedactionPolicy) bool {
	parts, _ := ParseExtractionPath(a.Path)
	for _, p := range parts {
		if !p.Array && credentialName(p.Property) {
			return true
		}
	}
	pointer := assertionPointer(a.Path)
	if policy.OmitBody {
		return true
	}
	for _, p := range policy.JSONPointers {
		if overlaps(pointer, p) {
			return true
		}
	}
	for _, x := range step.Extract {
		if overlaps(pointer, assertionPointer(x.Path)) {
			return true
		}
	}
	return false
}

func ValidateAssertions(step WorkflowStep, policy RedactionPolicy) error {
	if len(step.Assertions) > 8 {
		return workflowInvalid()
	}
	raw, e := json.Marshal(step.Assertions)
	if e != nil || len(raw) > AssertionArrayLimit {
		return failure(413, "payload_too_large")
	}
	ids := map[string]bool{}
	status := false
	for _, a := range step.Assertions {
		if e := a.validate(); e != nil {
			return e
		}
		if ids[a.AssertionID] || a.Kind == "statusEquals" && status {
			return workflowInvalid()
		}
		ids[a.AssertionID] = true
		status = status || a.Kind == "statusEquals"
		if a.Kind == "jsonEquals" && assertionSensitivePath(a, step, policy) {
			return workflowInvalid()
		}
	}
	return nil
}

type AssertionResult struct {
	AssertionID string `json:"assertionId"`
	Kind        string `json:"kind"`
	Outcome     string `json:"outcome"`
	Reason      string `json:"reason"`
}
type AssertionEvaluation struct {
	SchemaVersion int               `json:"schemaVersion"`
	State         string            `json:"state"`
	Results       []AssertionResult `json:"results"`
}

func NewAssertionEvaluation(step WorkflowStep, reason string) *AssertionEvaluation {
	out := &AssertionEvaluation{SchemaVersion: 1, State: "not_configured", Results: []AssertionResult{}}
	for _, a := range step.Assertions {
		out.State = "evaluated"
		out.Results = append(out.Results, AssertionResult{a.AssertionID, a.Kind, "not_evaluated", reason})
	}
	return out
}
func (a *AssertionEvaluation) Passed() bool {
	for _, r := range a.Results {
		if r.Outcome != "passed" {
			return false
		}
	}
	return true
}

func assertionJSON(raw []byte, contentType string) (any, string) {
	if len(raw) == 0 {
		return nil, "body_absent"
	}
	if len(raw) > BodyLimit {
		return nil, "body_limit_exceeded"
	}
	media, params, e := mime.ParseMediaType(contentType)
	if e != nil || params["charset"] != "" && !strings.EqualFold(params["charset"], "utf-8") || !(media == "application/json" || strings.HasPrefix(media, "application/") && strings.HasSuffix(media, "+json")) {
		return nil, "body_unsupported"
	}
	var root any
	if StrictJSON(raw, &root) != nil {
		return nil, "json_invalid"
	}
	var numbersValid func(any) bool
	numbersValid = func(v any) bool {
		switch x := v.(type) {
		case json.Number:
			_, e := ParseAssertionDecimal(x.String())
			return e == nil
		case map[string]any:
			for _, v := range x {
				if !numbersValid(v) {
					return false
				}
			}
		case []any:
			for _, v := range x {
				if !numbersValid(v) {
					return false
				}
			}
		}
		return true
	}
	if !numbersValid(root) {
		return nil, "json_number_unsupported"
	}
	return root, ""
}

func assertionType(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case string:
		return "string"
	case json.Number:
		return "number"
	}
	return "container"
}

// Only fixed outcomes leave this function. Taint discovery uses raw worker
// memory after extraction, and never substitutes sanitized text as an operand.
func EvaluateAssertions(step WorkflowStep, status *int, raw []byte, contentType string, policy RedactionPolicy, values map[string]string, unavailable string) *AssertionEvaluation {
	out := NewAssertionEvaluation(step, unavailable)
	root, reason := any(nil), unavailable
	if unavailable == "" {
		root, reason = assertionJSON(raw, contentType)
	}
	san := newSanitizer(values)
	sensitivePaths := []string{}
	var discover func(any, string)
	discover = func(v any, path string) {
		switch x := v.(type) {
		case map[string]any:
			for k, child := range x {
				p := path + "/" + strings.ReplaceAll(strings.ReplaceAll(k, "~", "~0"), "/", "~1")
				if credentialName(k) {
					sensitivePaths = append(sensitivePaths, p)
				}
				discover(child, p)
			}
		case []any:
			for i, child := range x {
				discover(child, path+"/"+strconv.Itoa(i))
			}
		default:
			encoded, _ := json.Marshal(v)
			if san.text(string(encoded)) != string(encoded) {
				sensitivePaths = append(sensitivePaths, path)
			}
		}
	}
	if reason == "" {
		discover(root, "")
	}
	for i, a := range step.Assertions {
		r := &out.Results[i]
		fail := func(code string) { r.Outcome, r.Reason = "failed", code }
		if a.Kind == "statusEquals" {
			if status == nil {
				r.Reason = "status_unavailable"
				continue
			}
			expected, _ := strconv.Atoi(string(a.Expected))
			if expected == *status {
				r.Outcome, r.Reason = "passed", "matched"
			} else {
				fail("status_mismatch")
			}
			continue
		}
		if reason != "" {
			r.Reason = reason
			continue
		}
		parts, _ := ParseExtractionPath(a.Path)
		v, _, e := selected(root, parts)
		if a.Kind == "jsonEquals" {
			unsafe := assertionSensitivePath(a, step, policy)
			for _, p := range sensitivePaths {
				unsafe = unsafe || overlaps(p, assertionPointer(a.Path))
			}
			expected, _ := a.operand()
			encoded, _ := json.Marshal(expected)
			unsafe = unsafe || san.text(string(encoded)) != string(encoded)
			if unsafe {
				r.Reason = "sensitive_input"
				continue
			}
		}
		if e != nil {
			fail("path_missing")
			continue
		}
		if a.Kind == "jsonExists" {
			r.Outcome, r.Reason = "passed", "exists"
			continue
		}
		expected, _ := a.operand()
		if assertionType(v) == "container" || assertionType(v) != assertionType(expected) {
			fail("type_mismatch")
			continue
		}
		equal := false
		if n, ok := v.(json.Number); ok {
			x, _ := ParseAssertionDecimal(n.String())
			y, _ := ParseAssertionDecimal(expected.(json.Number).String())
			equal = x != nil && y != nil && x.Cmp(y) == 0
		} else {
			x, _ := json.Marshal(v)
			y, _ := json.Marshal(expected)
			equal = bytes.Equal(x, y)
		}
		if equal {
			r.Outcome, r.Reason = "passed", "matched"
		} else {
			fail("value_mismatch")
		}
	}
	return out
}

// CompleteAssertionAttempt covers early exits without granting another send.
func CompleteAssertionAttempt(e *Evidence, step WorkflowStep) {
	if e.Assertions == nil {
		return
	}
	if len(e.Assertions.Results) > 0 && e.Assertions.Results[0].Reason == "not_dispatched" && e.Outcome.Certainty != "not_dispatched" {
		reason := "response_unavailable"
		if e.Response.Body.OmissionReason != nil {
			switch *e.Response.Body.OmissionReason {
			case "response_limit_exceeded":
				reason = "body_limit_exceeded"
			case "unsupported_encoding", "invalid_encoding":
				reason = "body_unsupported"
			}
		}
		e.Assertions = EvaluateAssertions(step, e.Response.HTTPStatus, nil, "", RedactionPolicy{}, nil, reason)
	}
	if e.Status == "completed" && !e.Assertions.Passed() {
		e.Fail("assertion_failed", "known")
	}
}
