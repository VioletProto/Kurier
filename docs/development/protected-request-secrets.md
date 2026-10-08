# Protected saved-request development slice

Branch: feat/protected-request-secrets, from merged main d8a534e.
[Accepted contract](../architecture/protected-request-secrets-contract.md).
[Progress card](https://trello.com/c/t9HIMQpm).

## Development resources and cost

Account 747336059622, non-root davian-admin, region us-east-2, stage dev-api.
Existing Control, Cognito, cursor key and schedule are reused.

- Protected: kurier-dev-api-ProtectedTable-szbvcchc, Standard on-demand,
  no indexes/TTL/stream, PITR enabled and deletion protection.
- KMS: arn:aws:kms:us-east-2:747336059622:key/cc369e79-af59-4b2a-921b-ca9e476997dd,
  alias alias/kurier/dev-api/protected, annual 365-day rotation and retained key.
  Inventory found no existing customer stage key before provisioning.
- Existing API Lambda: Protected GetItem/Query/PutItem/ConditionCheckItem and
  GenerateDataKey only on the exact key with app/stage/purpose context.
  No KMS Decrypt permission or plaintext read route.
- Existing scheduled cleanup Lambda: Protected Query/DeleteItem only; no KMS.
- GET project secrets, PATCH shared secret and DELETE shared secret routes.
- Conditional stage activation: savedRequestsSchemaVersion=2 and
  protectedSecretsSchemaVersion=1; recovery generation was preserved.

The preview and deployment introduced no Control replacement, new Cognito pool,
worker, queue, S3 evidence bucket, bootstrap change or additional schedule.
Readback confirmed ACTIVE/PITR/protection, annual rotation and scoped IAM.
The authorized incremental forecast is **$1.10–$2/month**, before free tiers,
for 10,000 extra API calls, 1,000 secret writes and 1,000 retained secrets averaging
4 KiB. KMS key is $1/month; 1,000 symmetric calls are about $0.003. The remainder
is a modest DynamoDB/PITR/Lambda/API/logs allowance, not observed spend or a cap.
The first and second automatic rotations each add $1/month for key versions.
See [AWS KMS pricing](https://aws.amazon.com/kms/pricing/) and
[DynamoDB pricing](https://aws.amazon.com/dynamodb/pricing/on-demand/).

## Authoring and lifecycle

Run npm run dev:cloud after deploying dev-api and use http://localhost:5173/.
Sign in with a genuine Cognito user. Mark a header/query Protected before
entering a disposable value. Authorization takes the complete Bearer value.
Write-only inputs clear after submission, including errors; refresh never
returns a stored value. Unrelated request edits preserve stable bindings.

Row preserve identity is bindingId + collection + exact name, independent of
order/enabled state. Duplicate names have different IDs. Body identity includes
type and whole-body/canonical RFC 6901 pointer. Rename, collection move, pointer
change and body-type change require explicit set/reference. Preserve cannot
introduce a binding, resurrect a removed binding or create on POST.

Whole text/JSON bodies can be protected. Partial JSON bodies use literal null
placeholders and separate canonical pointer inputs. A protected scalar string
is entered as JSON, including quotes; objects/arrays require whole-body protection.
Sensitive paths and inline credential query authoring are unsupported. Named
URL credential patterns/userinfo/templates are rejected; arbitrary embedded
secrets cannot be reliably detected.

Request-local set creates a new project-owned secret; detach/request deletion
does not revoke it. Existing project secrets can be explicitly reused. Shared
replacement changes the same ID for all consumers; shared revocation clears its
active envelope and leaves a safe tombstone. Existing unavailable bindings can
be preserved, but cannot be reused in new bindings.

Failed/uncertain writes are not automatically retried. Request refresh/review
resolves request uncertainty; refreshing secret metadata cannot unblock an
uncertain request. Shared writes require separate metadata review. All writes
use request/secret revision CAS and project gates. Project cleanup proves both
partitions empty; unknown records remain and incomplete work stays pending.

## Verified results

- Final formatting/lint, 35 frontend tests, 8 infrastructure tests, frontend build,
  all service Go tests/vet and API/worker/local-agent binary builds passed.
- Actual DynamoDB Local race tests: ownership/reference misuse, stable identity,
  duplicate/reordered bindings, JSON pointers, shared/local replacement and
  revocation, one-winner edits, interrupted cleanup and unknown-record retention.
- Failure injection: KMS errors, lost acknowledgements without replay, rollback
  with no orphan and resumed cleanup. These injections are local fixtures.
- AES-GCM/context/tamper tests; disposable values absent from Control records
  and API responses. Browser storage checks cover local/session storage,
  IndexedDB and Cache Storage without recording traces/screenshots.
- Chromium-to-Go/Local authoring passed, including **direct creation** of a
  protected Authorization request, cleared input and unrelated preserve.
  Cognito is simulated in this test.
- Actual AWS KMS wrapped-key decrypt matched the memory-only disposable value;
  incorrect encryption context was rejected. API IAM cannot perform this decrypt.
- Actual AWS DynamoDB ownership/reference misuse, replacement/revocation,
  concurrency, transaction rollback, index propagation and resumed cleanup passed.
- Scheduled Lambda drained 25 requests and 25 Protected records in 145.6 seconds,
  across multiple bounded invocations, without manual cleanup invocation.
- Resource/IAM readback passed; final SST diff reports no changes. A memory-only
  operator audit of the genuine disposable browser probe decrypted active envelopes
  and checked three value representations against 138 Control records, Protected
  items and 1,741 API/cleanup log events: no protected values found. This covers
  active disposable values; it does not reconstruct already-revoked plaintext.

## Genuine browser validation: in progress

Davian's first genuine Cognito/browser test returned HTTP 400 when creating a
protected request. The generic validation message was reported without a
secret value. Davian identified a header name containing spaces; changing it to a valid name saved successfully. The browser now rejects invalid HTTP header names before sending and explains that bearer-token APIs normally require Authorization. The wider browser acceptance remains in progress; the slice is not declared complete.

After resolution, validate protected Authorization, protected API-key query,
whole body and JSON pointer creation; refresh and unrelated rename; local/shared
replacement, reuse and revocation. Confirm every submitted secret input clears.
Report only pass/fail and safe identifiers, never values or credential payloads.

The signed-in console helper validateCloudProtectedSecrets from
/src/cloud-validation.ts creates disposable data with memory-only random values,
exercises reuse, local/shared replacement, revocation, stale revisions,
cross-project rejection, JSON pointers and browser storage. Output contains
safe IDs/booleans only. Retain its returned probe in the same tab.
Sign out and sign in as a second Cognito user, then call
validateProtectedIsolation(probe). Sign in as the original owner and call
finishProtectedBrowserProbe(probe) to wait for actual scheduled deletion.
No outbound HTTP is executed. The probe exists in AWS and its active-value
Control/encrypted-persistence/cloud-log audit passed; returned browser flags,
second-user denial and owner cleanup still await Davian's report.

## Checks

Use npm run verify, npm run test:ownership:integration and
npm run test:browser:integration. Local integration requires
KURIER_DYNAMODB_TEST_ENDPOINT=http://127.0.0.1:8000. In this sandbox use
GOCACHE=/tmp/kurier-protected-go-build.
Actual AWS tests run through scripts/aws-integration.mjs with
TestAWSProtected or TestAWSControl filters.
enable-protected-secrets.py previews by default; --apply checks identity,
resources, Lambda markers and conditionally changes capability. Do not reset
recovery generation or replay uncertain writes.

## Remaining limitations

No worker/decryption delivery, outbound HTTP, response evidence, workflows,
MCP secret access or local-agent credential delivery. No restore/PITR recovery
was performed. Backups may restore deleted ciphertext under accepted recovery
rules. Plaintext exists transiently in browser/server memory; garbage-collected
strings cannot guarantee physical erasure. Annual key rotation is configured,
not elapsed/runtime-tested. Unknown records intentionally keep deletion pending.
Genuine browser acceptance remains open; submission status is recorded in Trello. The active-value
cloud audit passed; browser flags/second-user denial/owner deletion remain to be
reported.
