# Sequential workflows: development deployment and AWS validation

Davian approved the scoped dev-api deployment and guarded activation including
527dd09. Region us-east-2, account 747336059622, non-root davian-admin.
This record separates genuine AWS runtime checks from SDK admission, injected
faults and pending operator Cognito/browser acceptance.

## Deployment and activation

Rebuilt/reviewed the scoped SST diff after adding the existing owned controlled
endpoint's synthetic-token modes and safe send-count metadata. No new standing
resources, bootstrap, data-resource permissions for the endpoint, Cognito
changes or frontend hosting. Scope: nine workflow route/integration/invocation
permission groups, two existing purpose-scoped IAM role updates, four existing
function packages, and compatible consumer capability environment markers.
Postdeployment SST diff reported **No changes**.

All four deployed ZIP SHA-256 values matched their local deployment archives,
with Active/Successful state and ARM64 provided.al2023:

| Consumer                       | Deployed Lambda CodeSha256 (base64)          |
| ------------------------------ | -------------------------------------------- |
| API and cleanup                | FteKPg4QOQtmMyZoAd2YpZWH483fd24xz63TYTZZ7qI= |
| Worker and controlled endpoint | 0CcO+2/TutwZcIHY0BZ3A4Lc/Rdoa3h/efaa45r3QLI= |

These identify deployed bytes, not the later documentation commit.
Memory/timeouts remain API 256 MiB/15s, cleanup 256 MiB/60s,
worker 256 MiB/90s; controlled endpoint 128 MiB/65s. Existing pinning/local/
protected/cloud/input capabilities were preserved. All nine routes
target the API with payload format 2.0 and application JWT/ownership middleware;
genuine unauthenticated HTTPS calls on every method/path returned 401.

Verified inline IAM restricts all KMS grants to the existing stage key and
app=kurier, stage=dev-api, exact purpose. API: GenerateDataKey for
saved-secret/job-bindings/workflow-inputs, Decrypt saved-secret/job-bindings.
Worker: Decrypt job-bindings/workflow-inputs/workflow-output and GenerateDataKey
workflow-output; output PutItem targets the existing Protected table, fast
notification SendMessage the existing queue. Cleanup has no KMS grant.
The verifier initially rejected the preexisting managed
AWSLambdaBasicExecutionRole; inspection found only that attachment on all
three roles. The verifier now fetches its current version, requires only
the three basic logging actions, and rejects unexpected attachments.
No IAM change was made to address the verifier's assumption.

Only after all package/config/route/IAM proofs passed, activation executed:

| Readback                                                                              | Value                                                            |
| ------------------------------------------------------------------------------------- | ---------------------------------------------------------------- |
| State/version                                                                         | active / **9**                                                   |
| savedRequestsSchemaVersion                                                            | **6**                                                            |
| sequentialWorkflowsSchemaVersion                                                      | **1**                                                            |
| localAgentExecutions/evidencePinning/cloudExecutions/executionInputs/protectedSecrets | **1**, all preserved                                             |
| recoveryGeneration                                                                    | dev-api-initial-v1, preserved                                    |
| Transition                                                                            | active/v7 → workflow-migration/v8 → active/v9                    |
| Existing accounting hash before/closed/after                                          | 77b478cb583b4c268ef97c00c7cef848c743cf7cf59c4b6004479ff5bc665348 |
| Existing accounting/capability resets                                                 | None                                                             |

Activation succeeded first attempt. No uncertain/failed capability mutation
was retried. The verifier's refusal happened **before** any marker
mutation; a strong read confirmed active/v7 intact. The activation helper has
no automatic resume/reopen path and prints only safe durable status on failure.

Free plan **ACTIVE**, account/unreserved concurrency **10**, worker SQS standard
batch **1** / max **2**, no reserved/provisioned concurrency or pollers. Minute
cleanup remains enabled and targets the existing cleanup function.
No quota/concurrency/plan settings changed. The planning forecast remains
$0.50–$1/month incremental, not measured spend or a cap.

## Genuine AWS runtime checks

Runtime validation is in services/api/internal/ownership/workflow_aws_test.go,
run through node scripts/aws-integration.mjs TestAWSSequentialWorkflows.
The test creates a disposable SDK identity/project; it does not authenticate
through Cognito or prove deployed API write-handler IAM by using admin SDK
admission. Normal steps run through the deployed SQS worker using its actual
IAM/KMS/DynamoDB/S3 roles and public HTTPS controlled target.

The latest genuine runtime observations passed:

| Check                                                | Observed result                                                                                                                                                                       |
| ---------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Raw string extraction → Authorization Bearer binding | Producer and consumer HTTP 200; controlled consumer authorization true                                                                                                                |
| Source/rendered containment                          | Token and Bearer reflection absent from sanitized evidence and bounded private runtime logs                                                                                           |
| Evidence immutability                                | Private S3 object bytes and checksum unchanged after duplicate SQS delivery                                                                                                           |
| Runtime material                                     | Initial-input/output ciphertext absent after each terminal run, including failure/unknown                                                                                             |
| Deleted definition                                   | Original completed → definition deleted → deliberate fresh run completed from frozen context                                                                                          |
| Durable successor recovery                           | SDK notifier suppressed both initial and successor fast wakeups; first step executed through real SDK services; minute scheduler delivered persisted successor to the deployed worker |
| HTTP 500 stop                                        | Known failed first step, second skipped; one controlled failure call                                                                                                                  |
| Response-header timeout stop                         | Unknown certainty, second skipped; one controlled timeout call despite duplicate SQS notification                                                                                     |
| Pin/context retention                                | Guardedly aged disposable timestamps; real schedule expired sibling while pinned evidence and frozen run context stayed available                                                     |
| Admission accounting                                 | Ten step slots retained, including two skipped steps; inflight returned to zero                                                                                                       |
| Controlled target counts                             | Producer 3, authorized consumer 3, HTTP failure 1, timeout 1; no extra dispatch from duplicates                                                                                       |

Latest disposable project c55b7542-3d30-47b1-bb6d-35aec5ee9dfb,
owner 37714cd7-3a91-4ed5-ae10-32785d5bebc6. Its runtime assertions above passed.
A separate private bounded Control/Protected audit found no plaintext runtime
tokens and Protected count zero after terminalization.

The long Go observer's static exported credentials expired before its final
cleanup observation, although the CLI login remained renewable. A child-memory
STS diagnostic confirmed ExpiredToken; only that read-only observer was
terminated. Its complete test-process result is **FAIL/interrupted**, not a
full suite pass. No target request was repeated to repair observation.
The SDK test now uses an in-memory credential provider that retrieves CLI
credentials through a private child pipe and refreshes before expiry. Neither
credentials nor exported JSON reach terminal output/files/logs. The corrected
provider passed genuine cleanup-only reconciliation in 2.561s. A complete new
runtime suite crossing another credential-expiry cycle was not rerun.

Cleanup-only reconciliation verified operation
cc1f6e9c-83fc-41e2-bf87-0727a7003902 **completed** through the real scheduler:
Control tombstone only, Protected/S3 empty, disposable identity removed and
numeric usage retained. No local/fixture test substitutes for a failed AWS check.

The first suite attempt omitted the SDK Store's protected-table configuration
and stopped before any fixture creation or HTTP. The second attempt's rerun
returned a post-admission error; inspection found both original and frozen
rerun fully completed. No fresh-key retry was issued for that admission.
A bounded private target-log audit confirmed token **2**, authorized consumer
**2**, with the disposable token absent from logs. The failed harness run is
not retrospectively labeled a suite pass. The harness now retains each key
and reconciles uncertain admission with that exact key. Rapid worker
advancement can race optimistic postcommit run hydration; durable admission
receipt reconciliation remains required.

Interrupted fixture:
project d9c8b5ca-6341-4074-88b2-c1eca4d59567,
owner 39cc13b4-648b-4cd1-9420-65dc53c48ac1,
original e7a34fd2-176a-4170-98e9-600e55de78c6,
frozen rerun a77dfffe-fa6a-445e-9e04-e48f6229cd6d.
Its deletion is observed through the real schedule, with no direct cleanup
invocation or HTTP submission by the explicit reconciliation helper.

The first cleanup observer's five-minute deadline expired while the scheduler
was still draining tickets. Extended read-only observation of the already
accepted operation 39a3f5a2-edf1-4784-b4ef-a9cc33ec1961 passed: Control tombstone
only, Protected/S3 empty, disposable identity META/mapping removed. Numeric
execution-day quota records are deliberately retained, with no refund/reset.

A subsequent SDK suite, project 4e72644b-f6ba-4717-b5a3-d36e02a8768c /
owner ac8ce164-78d0-4252-a648-19944cf68115, passed normal runtime checks but
timed out observing publication of its durable-recovery successor. The target
had already received that request; no replay was issued. A separate bounded
target audit recorded producer 3 / authorized consumer 3 / failure 1 / timeout 1,
with the token absent from logs. This failed suite is not called a pass.
Its project was submitted for scheduled deletion separately. The final harness
polls using ordinary strong GetItem reads and hydrates only terminal results;
earlier transactional RunDetail polling could contend with live writers, but
the exact cause of the publication interruption was not conclusively observed.
No production guard was weakened to obtain a pass. The AWS wrapper allows a
40-minute test deadline because ticket draining spans multiple minute ticks.

That project's 12-minute observer also expired before its final metadata
passes. Extended observation of the same accepted operation
48ff9eeb-0bde-45d9-ab89-431d07594ee7 passed in 2.293s after scheduled completion:
Control tombstone only, Protected/S3 empty and disposable identity removed.
No direct cleanup invocation or new HTTP was used. Cleanup latency for these
multi-step fixtures can exceed twelve minutes because bounded ticket and
metadata work spans multiple minute ticks; throughput optimization is not part
of this rollout.

## Final cleanup and accounting readback

Independent strong partition reads and complete S3 prefix listings verify all
three disposable projects above: each Control partition has exactly one minimal
deleted project tombstone, each Protected partition and S3 prefix is empty.
All three user META records and identity mappings are absent. External durable
drain cursors/residual-prefix sweep work remains by design for coordination and
late-write safety, along with numeric quota records; no unrelated records were
deleted or counters reset.

The three fixture owners retain **4 / 10 / 10** admitted slots, respectively:
**24** total, with no skipped-step refund. Stage usage is cumulative **monthly**,
not the current UTC day's sum: final count **73**, including **49** preexisting
and **24** fixture slots. Existing reserved storage is **10,390 bytes**,
reconciled equally across stage, existing project counters and upload records.
It was preserved, not zeroed to make fixture cleanup pass. Stage pin count/bytes
are both **0**. Six preexisting projects remain; workflow records, unsettled
jobs/uploads and disposable identity mappings are **0**. The marker remains
active/v9 under the same generation and capabilities.
Worker queue visible/inflight/delayed and DLQ visible/inflight approximate
counts were all zero; these are health observations, while the strong partition
and complete prefix checks above provide the fixture cleanup proof. Final
Free-plan/account-concurrency/SQS settings were read back unchanged.

Local verification passed: format/gofmt/lint, 52 frontend tests, 10 infrastructure
tests, frontend production build, six Go modules' tests/vet; full API/DynamoDB
Local race regressions and five Chromium fixture scenarios. AWS-tagged helper
compilation/vet passed. These results remain separate from genuine Cognito.

## Operator and operational boundaries

Follow the [numbered Cognito/browser checks](sequential-workflows-browser.md).
They provide exact controlled fixture URLs and expected states/redaction/
pin/deleted-definition/unknown behavior, cross-owner checks and deletion steps.
No browser fixture is precreated under a fake identity for Davian to discover:
the operator creates a project owned by the genuine signed-in Cognito account.
Operator results, resulting fixture IDs and independent cleanup audit are pending.

Synthetic expiry timestamps and SDK-side notification suppression are deliberate
test interventions; real elapsed thirty-day retention and an actual AWS queue/
Lambda outage are not claimed. Actual process-crash/ACK-loss injection,
comprehensive negative IAM, PITR/restore, clock skew, maximum-size Lambda
memory/latency, sustained quota saturation and billed cost remain unperformed.
No automatic HTTP replay or unreviewed uncertain-PUT absence proof was used.
