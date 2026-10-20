# Kurier workflow assertions — proposed next slice

Status: review draft, 2026-10-19 (America/Denver). These are proposed choices,
not an accepted repository contract or evidence of implementation. Prepare the
repository review packet against current main before implementation. Deployment
and capability activation are outside this planning task.

## Sources and goal

- [Assertions Trello card](https://trello.com/c/8RUxoJyt)
- [Proposal](https://docs.google.com/document/d/1tatrrhqytTxAZxlrOnbSQRymmqjL2MmGq7Tdj0-1T60/edit)
- [API contracts](https://docs.google.com/document/d/1KSfRYOb2UxmrIl8VoFjc3YP38-PMu7k_fM9wrVkEenM/edit)
- [Accepted workflow baseline](https://github.com/VioletProto/Kurier/blob/main/docs/architecture/sequential-workflows-contract.md)
- [Accepted execution baseline](https://github.com/VioletProto/Kurier/blob/main/docs/architecture/cloud-execution-evidence-contract.md)
- [Repository workflow](https://github.com/VioletProto/Kurier/blob/main/AGENTS.md)

Demonstrate that HTTP 200 with the wrong response data fails a workflow, records
safe immutable assertion results, and prevents successor requests from being sent.

## Proposed scope

Add assertions to cloud-workflow steps only. Include revisioned authoring,
submission-time freezing, bounded evaluation, immutable results, stop/skipped
successors, and browser inspection. Keep standalone requests and local-agent
protocols unchanged in this first slice; broader target support remains explicit
backlog. No OpenAPI, environments, scripts, regex, coercion, or custom expressions.

## Proposed definition

Each step gains an optional `assertions` array; omission normalizes to an empty
array. Existing workflows retain their behavior. Replacing steps replaces their
assertion arrays under the existing workflow revision/If-Match rules.

```json
[
  {
    "assertionId": "client-generated-uuid",
    "kind": "statusEquals",
    "expected": 200
  },
  {
    "assertionId": "client-generated-uuid",
    "kind": "jsonExists",
    "path": "$.user.id"
  },
  {
    "assertionId": "client-generated-uuid",
    "kind": "jsonEquals",
    "path": "$.user.active",
    "expected": true
  }
]
```

Use distinct valid UUIDs in actual definitions, unique within each step and stable
across reorder. Strict discriminated DTOs reject unknown/duplicate fields and
invalid kind-specific combinations. UUIDs identify checks; they grant no authority.

Proposed limits: eight assertions per step, one status assertion per step,
2 KiB serialized expected primitive, existing 512-byte/32-selector extraction-path
limits and strict JSON depth/number rules. Include all rule/result overhead in
existing workflow, frozen-plan, item, transaction and evidence delivery bounds.
The repository packet must prove maximum-size fit or lower these limits.

## Proposed semantics

| Rule                   | Behavior                                                                                                                                                                                                                                                                         |
| ---------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Status equality        | Expected integer 100–599; compare exact observed final status. Interim responses are not the target response. Missing status is not a match.                                                                                                                                     |
| JSON existence         | Reuse the accepted extraction path syntax, with assertion-specific semantics: a located property exists even if its value is null, an object, or an array. Root `$` exists for any valid JSON document.                                                                          |
| JSON equality          | Located value must be a primitive of the same JSON type as the expected value. No string/number/boolean coercion. Null equals present null; missing never equals null. Strings compare exactly, without normalization.                                                           |
| Number equality        | Use the accepted numeric domain and deterministic parsing; compare numeric value within that domain, so 1 and 1.0 agree. Reject unsafe integers/nonfinite values and unsupported representations rather than silently rounding. Verify implementation feasibility in the packet. |
| Invalid or absent JSON | JSON checks cannot pass on malformed, unsupported, oversized, absent, or unavailable bodies. Emit fixed safe failure/not-evaluated reasons; preserve any stronger network/capture failure.                                                                                       |
| No rules               | Report no assertions configured, not a fabricated passing evaluation. Historical records lacking results report results unavailable, not pass.                                                                                                                                   |

Evaluate against the bounded raw response in worker memory before sanitization
and disposal, sharing strict parsing with extraction where practical. Never
evaluate redacted evidence: `[REDACTED]` must not become an assertion input.
Later omission for evidence-delivery size does not invalidate an evaluation
already completed against an otherwise valid bounded response. Record that body
omission separately; do not reconstruct values from stored evidence.

For this slice, rule outcomes do not override baseline HTTP failures. An expected
404 may match its status assertion, while HTTP 404 still fails the execution/run.
If successful negative tests are desired, review that as an explicit change to
HTTP failure semantics before implementation; do not introduce it implicitly.

All checks are ANDed. Evaluate the bounded configured set when response data is
trustworthy; represent checks that could not execute explicitly. Preserve exact
HTTP status, outcome certainty and higher-priority network/capture failures.
Otherwise an assertion failure becomes a fixed `assertion_failed` known outcome.
Specify deterministic precedence with extraction/runtime failures in the packet.

## Secret and result policy

Expected literals are public configuration, never a credential-entry mechanism.
No secret references, protected expected values or extracted-variable operands
in this first slice. Document the limitation in the editor. Reject equality
rules overlapping recognized or configured sensitive paths in either direction;
determine dynamic workflow-tainted paths before publication and fail closed if
such an equality becomes unsafe. Do not claim arbitrary secret detection.

Existence checks may return only a safe boolean result; do not expose the value
at a sensitive path. Validate this small disclosure against current MCP policy.

Proposed immutable result fields: assertionId, kind, outcome
(`passed`, `failed`, `not_evaluated`) and fixed reason code. Use the frozen rule
for browser context. Persist no actual values, body excerpts, value hashes,
dependency errors or user-defined error messages. Do not echo expected values
or paths in API errors, logs, queues, status polling or MCP results.

The repository packet must choose an explicit versioned evidence/read DTO and
compatibility plan. Existing immutable objects must remain byte-identical.
Audit strict API, worker, cleanup, browser and MCP readers before changing
schemas. Keep MCP read access to new records working safely; extending MCP
assertion-reporting behavior is separate from this slice.

## Atomic advancement, retention and reruns

Freeze rules for every step at run admission; edits cannot affect accepted runs.
Whole-run reruns use their frozen assertion rules, fresh execution identities
and current eligible stable secret inputs under existing rerun behavior.

An assertion failure publishes the terminal execution/step/run and skipped
successors through existing atomic finalization. Do not enqueue a successor or
retain runtime outputs for a failed producer. Preserve no-replay, outbox,
dispatch-fence, uncertain-write, source-revocation and recovery rules.
No second HTTP send to repair a lost assertion buffer/result.

Results follow the execution's existing immutable publication, pinning, expiry,
run-context and project-deletion lifecycle. Do not create independently retained
assertion history or revise old evidence during cleanup.

## Acceptance and verification

1. Editor creates/edits/removes bounded rules and handles stale revisions safely.
2. HTTP 200 + correct data passes; HTTP 200 + incorrect data fails; the next
   controlled endpoint records zero sends after failure.
3. Cover missing/present-null, nested objects/arrays, root scalar, false/zero/empty
   string, wrong primitive type, numeric equivalence, unsafe number and Unicode.
4. Cover invalid/non-JSON/empty/oversized bodies, omitted stored bodies,
   absent status, HTTP errors and timeout/unknown outcomes with honest precedence.
5. Expected values cannot become an alternate secret store; sensitive/tainted
   path cases expose no values through any returned or persisted channel.
6. Submission retries/duplicate delivery/finalization uncertainty do not send
   again, double-advance, or change a terminal result.
7. Editing definitions preserves old evidence/results; deliberate whole-run
   reruns create new records with frozen rules.
8. Pin/expiry/project deletion preserve existing counters, fences and cleanup.
9. Existing workflows without assertions, cloud/local standalone execution and
   MCP historical reads remain compatible. Legacy absence is never reported pass.
10. Relevant formatting/lint/build/unit, DynamoDB Local race and browser checks
    pass. Document fixtures separately from genuine AWS/Cognito/runtime evidence.

## Codex repository handoff

Prepare the review packet on an appropriate feature branch, following AGENTS.md
and the target card/dependencies/checklist. Suggested branch:
`docs/workflow-assertions-contract`; suggested packet:
`docs/architecture/workflow-assertions-contract.md`.

Inspect current runtime parsing/evaluation, workflow freezing/finalization,
evidence serializers, strict readers, cleanup and MCP before finalizing DTOs,
limits, error precedence and compatibility. Identify conflicts with this draft
and propose corrections. Update the architecture index and Trello preparation
evidence, run applicable checks, commit/push per AGENTS.md and report the exact
review commit. Leave implementation and runtime validation items open.

The packet should identify the few decisions needing acceptance: cloud-workflow
target boundary, HTTP-error policy, public-only equality expectations, exact
path/type/number behavior, result schema/limits, and coordinated compatibility.
Do not treat this planning draft as acceptance or implement/deploy dependent code.
