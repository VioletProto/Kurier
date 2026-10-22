# Assertions PR review and functional closeout

Davian accepted scoped assertions functionality and normal cleanup from retained
A–G and H–L evidence at **`4ffdaaf4cfb569a349ebd16ca739d8deb15aa973`**, explicitly
including the nine read-only OPTIONS stop-guard failures. Functional validation
and cleanup are **complete**. This acceptance does not erase failed exchanges,
prove missing historical observations, or validate the tooling correction live.

The full assertions slice is prepared on `feat/workflow-assertions-rollout`
against freshly fetched `origin/main` **`f4d1f3b84d3bbf04c451ba927ad9b4853468fed5`**.
That exact commit is also the merge base; no main integration conflict exists.
The pre-closeout diff contains 149 files, 32,353 insertions and 55 deletions.
It includes application code/tests, controlled fixtures, contract and deployment
preparation, operator tooling and retained execution/cleanup evidence. These
historical artifacts stay in the PR so stopped attempts remain reviewable.

This closeout performs repository/local-fixture checks, GitHub CI and coordination
updates only. **No additional live validation, admissions, identities/projects,
target requests, deployment, stage writes or AWS changes are authorized or run.**
Local DynamoDB fixtures and controlled loopback HTTP are not live campaigns.

## Review findings and disposition

| Finding                                                                                       | Disposition                                                       | Evidence / consequence                                                                                                                                                        |
| --------------------------------------------------------------------------------------------- | ----------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Normal verification/CI omitted activator and release-pin tests and Python meter unit tests    | Resolved for PR                                                   | `test:assertions-tooling` is now included in `npm test` and therefore `npm run verify`/CI. The web CI checkout fetches full history for accepted-manifest substitution tests. |
| Current documentation and implementation card still lead with superseded deployment/H–L holds | Resolved for PR                                                   | Current acceptance is explicit in architecture, contract and implementation indexes and Trello. Original receipts/ledgers retain historical failure/hold wording.             |
| Nine OPTIONS read transport failures did not latch the live stop guard                        | Accepted functional exception; separate tooling work remains OPEN | [Follow-up](https://trello.com/c/F47iWJY8), exact failures below. Existing teardown correction is locally verified only. No claim that every live guard held.                 |
| Missing historical intermediate readbacks, earlier 503 attribution and actual billing         | Retained limitations                                              | Current consistency does not reconstruct them. No additional investigation/live work is inferred.                                                                             |

No blocking application-code finding remains from this review. Closeout changes
verification wiring and current status documentation, with no application binary,
runtime contract, IAM or infrastructure change. Existing operational validation
backlogs remain separate from the accepted bounded functionality.

## Application and compatibility review

- **Browser/API numeric round trips:** numeric expectations use
  `{type:"number",decimal:string}` throughout authoring, native JSON transport,
  API RawMessage/string decoding, saved/frozen storage, reopening, unrelated
  edits/reorder and whole-run reruns. Browser validation uses bounded BigInt;
  worker evaluation uses bounded rational arithmetic on original JSON number
  tokens. The reviewed cases distinguish `0.10000000000000001` from `0.1`, preserve
  exponent spelling and signed zero, and enforce token/exponent/domain bounds.
- **Validation/security:** strict unions and duplicate/unknown-field rejection;
  bounded rule/result/configuration/item/transport sizes; no actuals, paths,
  expected values or arbitrary diagnostic text in immutable result DTOs. Public
  literals are not a secret store. Static redaction/extraction overlap and runtime
  taint discovery fail equality closed, with existence disclosing only presence.
  This does not promise detection of arbitrary unmarked secrets.
- **Runtime/publication:** frozen semantics and source rules govern each run and
  deliberate rerun. Existing capture, extraction, storage and HTTP failure
  precedence is retained; failed/unavailable checks stop successors atomically.
  Publication recovery preserves honest unknown/evaluation-unavailable outcomes
  without recreating results by resending HTTP. Local lost-ACK/lost-buffer/PUT
  tests are fixtures, not broad live fault injection.
- **Readers and lifecycle:** compatible stage8/assertions1 gates cover API,
  ownership/protected inputs, worker/recovery, both cleanup readers and MCP.
  Legacy runs keep their semantics when rerun after activation. Owner evidence
  preserves original v1/v2 bytes/checksums; MCP uses a dedicated legacy projection
  and explicit `assertions_not_exposed` omission metadata without expanding its
  seven tools or output schema. Retention/pinning/deletion retain existing fences.
- **Operator tooling:** reviewed default read-only activation, exact release and
  marker/accounting CAS, matching terminal-job/publication/settled-upload proofs,
  single-attempt writes, GET-only bounded 503 backoff/Retry-After, transport budgets,
  consistent accounting projections and integer subprocess deadlines. Historical
  launchers/approval records are evidence, not reusable execution authorization.

Full changed-file inventory, syntax/structured-artifact checks and hashes are
retained in [the review audit](workflow-assertions-pr-audit.json). The audit is
an offline inspection, not an AWS readback. Complete application diffs were
reviewed with their tests and accepted contract; evidence records were checked
as structured historical artifacts, not promoted to newly observed live state.

## Deployment and accepted genuine evidence

Application source remains `68d9f218225c7092a1e4fca0b66a21421d862a22`; release
manifest SHA256 remains
`920667777eb69cabfb7800910451062a18e6ff06ce22ad4fb543f96c78321dd6`.
API/cleanup ZIP SHA256 is
`f4be7af179049c70f479a45dd26e4ae24a19c3a9e8016a865318fa7a96965c32`;
worker/fixture ZIP SHA256 is
`15e7e6db438de3c19fe962c95be0d3c9d52d17f81b21a0b58cd4603974021379`.
Hosted frontend release is
`bd511e3f9979d2ef7dd6b73d183dcf7620a423b62659ac89c64e4a1a08981cde`.
All retained archives pass exact local release pins. No replacement application
artifact is proposed by closeout. Compared with that pinned source, subsequent
`apps`/`services`/`infra` changes are three integration-test files only.

The [actual deployment/A–G packet](workflow-assertions-final-rollout.md),
[focused cleanup verification](workflow-assertions-cleanup-verification.md) and
[H–L packet](workflow-assertions-hl-replacement.md) together establish the accepted
bounded slice. Deployment used compatible readers before guarded activation;
retained final readbacks show active stage15, savedRequests8/assertions1,
`dev-api-initial-v1`, and preserved IAM/platform/quotas/concurrency. These are
retained observations, not fresh AWS inventory in this task. Once assertion data
exists, old-binary rollback is incompatible; use the reviewed limitations rather
than downgrade/reencode immutable evidence.

Cumulative campaign admissions/sends are **28/24**, eight disposable
identities/projects created and normally cleaned, and zero reserve submissions.
Final monthly usage **32** includes four separate instructor charges. Latest
replacement cleanup proves tombstone-only Control, no Protected children/uploads,
empty S3 prefix, residual registration retained and provider identity absence.
Reserved storage/pins/inflight are zero; quota version75 follows eight legitimate
ticket releases while preserving charged usage. Current consistency does not
establish unobserved historical preservation or immediate offline JWT revocation.
Operator in-memory checks remain distinguished from retained durable readbacks
in the machine receipts; no raw credentials or execution bodies are added here.

## Preserved tooling exception

The [frozen ledger](workflow-assertions-hl-replacement-attempts.jsonl) SHA256 stays
`66050c4e8d224556dd65b5a96401039a2d4b8af1f79a59778b1ffbb5be97652f`.
Attempts **200–202, 258–260 and 344–346** are nine charged read-only API OPTIONS
transport failures with unknown HTTP status. They were discovered after live
processes stopped. No submission, mutation or target outcome is uncertain.
The [exact driver used live](workflow-assertions-hl-context-close-driver.mjs.txt)
and failures remain unchanged.

The correction at4ffdaaf waits for post-save refresh reads/network idle and checks
request failures before/after closure. Deterministic local delay reproduces nine
failures with the old driver; the corrected three-context author/reopen/edit
passes with zero transport errors. This supports cancellation as an explanation,
not conclusive historical attribution. **No live test of the correction occurred.**
The separate [tooling follow-up](https://trello.com/c/F47iWJY8) must define
cancellation versus genuine failure, stop propagation before later work, handling
of in-flight requests and bounded normal cleanup. It must preserve metering,
uncertain-write stops and no automatic reopen/replay. It is not completed here.

## Local and CI verification

Fresh local checks passed:

- `npm run verify`: Prettier/gofmt, ESLint, 64 frontend tests, 15 infrastructure
  tests, 27 coordinator/poll/collector tests, 88 activator/release cases
  (87 pass; optional local-DynamoDB case separately run below), 20 Python unit
  cases, TypeScript/Vite build and all seven Go module tests/vet.
- `KURIER_DYNAMODB_TEST_ENDPOINT=http://127.0.0.1:8000 npm run test:ownership:integration`:
  full race-enabled API/execution/ownership suite, including actual legacy
  maintenance settlement→zero-write closure inspection, publication uncertainty,
  maximum-shape cleanup and transition-during-RunDetail→fresh GET regressions.
- `KURIER_DYNAMODB_TEST_ENDPOINT=http://127.0.0.1:8000 npm run test:browser:integration`:
  Chromium→Go→DynamoDB Local, including exact decimal author/reopen/edit/rerun.
- `KURIER_ACTIVATOR_DDB_ENDPOINT=http://127.0.0.1:8000 node --test scripts/workflow-assertions-activation.test.mjs`:
  **82/82**, including actual conditional marker/accounting transactions.
- Python formatting/core lint, complete changed MJS/Python syntax and JSON/JSONL
  parsing, unchanged release pin tests and diff whitespace checks.

The PR runs the normal GitHub Actions web, seven Go matrix and ownership/browser
integration jobs. Its checks and final review comment supply the exact CI run/head
results; this local record does not predeclare remote CI success. CI lacks the
ignored release ZIP directory and explicitly skips that one local-archive case;
retained source/manifest/fixture/tampering checks still run there. The optional
activator CLI CAS is independently verified locally above.

Remaining exclusions are broad live fault injection, elapsed retention/late-upload
windows, PITR/recovery/skew, cloud maximum-size/performance, actual billing,
general signup/email and arbitrary external targets. MCP assertion-result
exposure, environments and OpenAPI remain separate product scope. Preparing or
merging this PR does not authorize any new deployment or capability activation.
