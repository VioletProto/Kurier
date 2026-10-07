# Saved requests within owned projects

Davian accepted the [slice contract](../architecture/saved-requests-contract.md)
on 2026-10-07, including explicit partial PATCH clearing/replacement rules.
The Go API and React editor create, list, view, edit and delete public saved
requests in owned projects. This slice adds no execution/outbound HTTP, response
evidence, workflows, environments/imports or encrypted credential storage.

## Authoring and safety

Select a project, choose **New request**, and enter public method/URL, ordered
headers/query parameters and optional text/JSON body. Disabled rows and duplicate
names retain their order. JSON body text/formatting is preserved. Select **No body**
to clear a body, and remove all rows to clear a collection. The API PATCH omits
unchanged fields, uses body:null to clear and replaces any supplied header/query
array completely. The UI submits the complete edited configuration with the
request revision ETag; successful writes advance both request revision and the
project gate. Refresh project details before rename/project deletion.

**64 KiB is the complete saved configuration**, including name, method, URL,
headers, query, body, descriptors/defaults and serialized JSON overhead. The
editor shows its normalized UTF-8 byte count; backend enforcement after PATCH
merge is authoritative. Raw body size alone is insufficient. Transport JSON is
separately bounded to 128 KiB, including decoded Gateway base64 inputs. Large
JSON responses use Gateway-transparent base64 to bound Lambda envelope overhead.
Lists stop at a conservative 2 MiB serialized item budget and return a cursor;
requested limit is an upper bound, never a guarantee of a full page.

Credentials/cookies/API keys and user-designated sensitive data cannot be saved.
The UI explains the limitation and requires public-data confirmation. API inputs
reject sensitive=true, secretWrite/secretRef, URL userinfo, recognized credential
names (even disabled), signed-query credential names, credential JSON keys and
obvious Bearer/Basic/private-key text. Unknown DTO fields and duplicate keys fail
closed. These named checks cannot identify arbitrary undisclosed secrets: use
only public data. No clipboard, browser storage or request logging is introduced.
Existing accepted encrypted set/preserve/remove/reference semantics remain future
work; there are no eligible saved secrets to bind in this stage. No fake mask or
plaintext credential fallback exists.

Loading disables writes. Validation preserves the draft and reports safe errors.
412/409 or uncertain responses block resubmission until explicit detail refresh.
Drafts remain until refresh. Uncertain creation requires deliberate list review
and starting over; an eventual list refresh cannot prove absence. Writes are
never automatically replayed. Individual deletion returns 204 and removes all
configuration, retaining a minimal revision tombstone until project cleanup.

## Scoped capability migration and deployment

Keep existing stage/generation/recovery gates. New local tables initialize
`savedRequestsSchemaVersion=1`. Existing legacy stages retain users/projects
support but saved-request writes return 503 until explicitly activated. Do not
reinterpret `localEmptyProjectsOnly=true` as permission for children.

The helper defaults to read-only preview. For existing Local data, after starting
the updated code use `python3 scripts/enable-saved-requests.py --local`, then add
`--apply`. It changes only the active known stage marker using version/generation
conditions; no data clearing, stage reactivation or recovery-generation reset.
Local operation is hard-coded to loopback and fake Local credentials.

For development AWS, use the existing account 747336059622, non-root identity,
`us-east-2` and dev-api resources listed in
[the AWS guide](aws-users-projects.md). Preview/deploy with:

```sh
export KURIER_AWS_ACCOUNT_ID=747336059622
export KURIER_COGNITO_POOL_ID=us-east-2_qTDZQT1FE
export KURIER_COGNITO_CLIENT_ID=5jg9sb38i6ae9c0adm5rtdfkno
export KURIER_FRONTEND_ORIGIN=http://localhost:5173
npm run sst:diff:api
python3 scripts/enable-saved-requests.py
npm run sst:deploy:api
python3 scripts/enable-saved-requests.py --apply
npm run dev:cloud
```

The helper checks both deployed handlers have completed updates and the schema-1
capability environment marker before activation. The one-item conditional update
sets savedRequestsSchemaVersion=1, increments stage version and removes the legacy
empty-project-only flag. Strong readback confirms generation unchanged. Rollback
to older binaries fails closed at the old stage gate; do not restore the legacy
flag while request records exist. SST ignores subsequent initial marker item
changes; this explicit migration is not a table replacement.

Preview scope: five routes/integrations/invocation permissions, two existing ARM64
Lambda packages/environment markers and existing SST asset archive versions.
No new table/index, Cognito, KMS key, schedule/cadence or runtime IAM expansion.
Existing table actions already support scoped transactional request CRUD/draining.
No bootstrap bucket/ECR repository is added. No hosted frontend.

Incremental forecast: **$0.10–$1.50/month before free tier**, assuming 10,000
operations and up to 1,000 stored public definitions, from roughly 4 KiB to near
64 KiB. At Ohio's existing screening rates, 10,000 transactional near-64-KiB
writes can consume about 1.3 million write units (~$0.84); typical 4-KiB writes
are much lower. Allow for gate/guard reads, GSI writes, API calls, Lambda, logs,
storage/PITR, egress and asset versions. The existing idle maintenance baseline
is already paid by the project slice. Budget $2 for validation; no hard cap or
actual-billing claim. Official references checked 2026-10-07:
[AWS DynamoDB](https://aws.amazon.com/dynamodb/pricing/),
[Lambda](https://aws.amazon.com/lambda/pricing/),
[API Gateway](https://aws.amazon.com/api-gateway/pricing/).

## Resumable project cleanup

The existing scheduler still discovers durable WORK through GSI2's eight shards.
Each invocation strongly queries at most 20 REQ candidates/project, transactionally
deletes only recognized schema-1 request/tombstone items and persists a WORK
continuation cursor with gate version/epoch CAS. At most 23 actions, key-only
cleanup writes, below the accepted transaction byte/action budgets. New processes
resume from that cursor; the cursor cycles to revisit unknown or uncertain pages.
Overlapping cleaners cannot skip pages. Unsupported kinds/schemas/attributes and
other children are preserved. They block completion without blocking known
requests on later pages. Only a fresh strong whole-partition emptiness proof and
conditional gate/work finalization may report completion. At most 20 candidates
per minute tick means large projects take multiple minutes. No cross-store
cleanup assumptions, unknown-child erasure or public maintenance route.

## Validation and genuine browser steps

```sh
GOCACHE=/tmp/kurier-go-build npm run verify
GOCACHE=/tmp/kurier-go-build KURIER_DYNAMODB_TEST_ENDPOINT=http://127.0.0.1:8000 npm run test:ownership:integration
GOCACHE=/tmp/kurier-go-build KURIER_DYNAMODB_TEST_ENDPOINT=http://127.0.0.1:8000 npm run test:browser:integration
# Actual AWS Control; requires intended non-root session and active capability:
KURIER_AWS_ACCOUNT_ID=747336059622 KURIER_AWS_TEST_TABLE=kurier-dev-api-ControlTable-bdxbaoxb go test -race -tags=integration,aws -run TestAWSControl -count=1 -v ./internal/ownership
```

Run the AWS command from services/api, or use `npm run test:aws:integration`
from the repository root to bridge an AWS CLI login session without printing credentials. Signed-fixture identities exercise real
AWS persistence and the local HTTP adapter; they never authenticate the public
endpoint. Scheduled runtime cleanup separately establishes Lambda IAM. Local
Chromium tests simulate Cognito while exercising real Go/DynamoDB Local.

For genuine browser validation:

1. Open http://localhost:5173 and sign in through Cognito. Use passwords/codes
   only in the browser. Select/create a project and author a public request
   (https://example.com/public, Accept: application/json, JSON public data).
   Verify count/limitation text, view, edit, clear body/headers and delete.
2. In browser DevTools Console, import the opt-in helper and run it with the
   memory-only session. It returns safe IDs/statuses only:

```js
const checks = await import("/src/cloud-validation.ts");
await checks.validateCloudSavedRequests();
```

3. Record the returned currentUserId/projectId/requestId. It checks 26 live
   requests/pagination, field order/body preservation, partial clearing, stale
   request/project writes, credential rejection, actual complete-64-KiB creation,
   413 after adding headers, and individual deletion. It leaves a public fixture
   project for owner review/second-user isolation.
4. Sign out and sign in as an independently created second user. Run
   `checks.validateForeignSavedRequest(projectId, requestId, currentUserId)` using
   the opaque IDs from step 3. All five routes must return 404. Then sign back
   in as the original owner and run
   `checks.cleanupSavedRequestValidation(projectId, requestId, currentUserId)`.
   It checks probe integrity, immediate denial and scheduled multichunk completion
   over up to three minutes. Reloading requires sign-in; re-import checks if needed.
5. Report only helper statuses/opaque IDs and observations. Never send tokens,
   passwords, codes, request payloads containing private data or auth screenshots.
   The fixture browser suite covers injected lost acknowledgements; manual
   offline tests alone cannot prove that a lost write committed.

## Recorded results

Repository verify passed (32 frontend tests, eight infrastructure tests, formatting,
lint, frontend build, all Go tests/vet). Local race integration passed, including
complete-limit/byte-bounded pagination, one-winner request CAS, request create/edit
versus deletion, unknown-schema preservation, cleanup completion fencing,
interrupted chunks and lost-ACK no-replay. Chromium fixture browser-to-Go/Local
flow passed create/edit/clear/delete, uncertain edit review and project cascade.

SST preview/deployment added five routes and updated two existing Lambda packages
plus capability markers. No IAM policy expansion, table/index, Cognito or schedule
change. The explicit stage activation incremented only stage version, retained
active state/generation, and removed the legacy empty-project-only flag.
AWS route/handler readbacks succeeded. HTTPS request preflight returned 204 and
unauthenticated request access returned 401.

Actual AWS integration passed again in 73.0 seconds: five request operations/validation,
ownership, pagination, stale request/project writes, lost ACKs, deterministic
request creation versus project deletion, unknown children and interrupted
cleanup; original project cancellation/rollback and one-winner CAS also passed.
The actual minute Lambda drained both a live request and request tombstone in
33.5 seconds without manual invocation, proving runtime transaction IAM.
Fixture user/identity rows were removed; only name-free project tombstones remain.
Signed fixtures never reached the public endpoint and do not prove Cognito sign-in.

Genuine Cognito/browser owner authoring passed on 2026-10-07: Davian confirmed
create/view/edit/clear headers and body/delete and the complete-64-KiB UI message.
The browser helper returned public create/detail/PATCH, clearing/replacement and
pagination passed; exact 65,536-byte creation passed; credential rejection 400,
merged oversize 413, stale request PATCH/DELETE and project DELETE 412,
individually deleted detail 404. An independently signed-in second Cognito user received 404 on all five routes.
The original owner probe remained unchanged, request detail returned 404
immediately after project deletion acceptance, and scheduled multichunk cleanup
reported completed. Independent strong DynamoDB readback found exactly one
projectTombstone in deleted state, with all request/tombstone/WORK children gone.
These genuine browser results are distinct from fixture identities.
No restore/PITR recovery, encrypted-secret, outbound HTTP or response evidence
behavior was tested or implemented. Price is a forecast, not measured spend.
