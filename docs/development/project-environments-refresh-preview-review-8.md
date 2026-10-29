# Authorized review-8 — completed read-only preview

**The one authorized preview completed successfully.** The exact frontend-bucket
ListTagsForResource read returned HTTP 200; no endpoint/operation denial,
unmatched attempt or transport failure was recorded. Usage is **296/500**:
263 prior + one refresh + mandatory STS verification + 31 provider attempts.
Live work has ended. No restart, policy change, repair or automatic continuation
occurred. The remaining 204 calls are arithmetic headroom, not authorization.

Tooling was the clean exact commit `1fd9812f8b974d527ba77e71ac78f7db63db276e`,
authorized from packet `67201d7e3db780d699049a44966d8be15f349180`.
Release source remained `44f1fb79d943501c95e06fcc1a6114111a7c1ea4`.

## Once-only execution and accounting

Local preflight verified the accepted packet manifest, 74 frozen original proof
files, clean source/tooling, SDK/provider/SST pins and retained release/rollback
bytes. The review-8 output, window-from-263 claim and private launch log were
absent. From the detached exact tooling checkout, the published command ran once:

```sh
python3 -B scripts/preview-meter/run.py refresh-preview \
  --review-commit 1fd9812f8b974d527ba77e71ac78f7db63db276e \
  --source /tmp/kurier-environments-release-source \
  --output /home/king/Neumont/Kurier/.build/project-environments-preview/review-8
```

**Historical evidence; do not rerun.** The feature branch was restored only for
results publication. No runtime/tooling code was changed. Output stayed private
until the following allowlisted summaries were reviewed; no raw resource state,
credentials, headers, bodies or URLs from diagnostics are published.

| Bound / actual result                            | Observation                                           |
| ------------------------------------------------ | ----------------------------------------------------- |
| Window began                                     | 2026-10-29 18:04:57.748049 UTC                        |
| Last durable provider receipt                    | 2026-10-29 18:05:10.916498 UTC                        |
| Elapsed through last receipt                     | 13.169 seconds of 3,600 allowed                       |
| New attempt allowance                            | At most 237, including authentication                 |
| Existing-session refresh                         | Ordinal 264, HTTP 200; one attempt                    |
| Mandatory STS verification                       | Ordinal 265, HTTP 200; same account/identity verified |
| Provider attempts                                | 31, ordinals 266–296; 24 HTTP 200 / 7 HTTP 404        |
| Exact scoped S3 Control read                     | One ListTagsForResource, HTTP 200                     |
| Denials / failed transports / unmatched attempts | 0 / 0 / 0                                             |
| Launcher / SST sandbox exit                      | 0 / 0                                                 |
| New / cumulative usage                           | **33/237 new; 296/500 cumulative**                    |

Every admitted ordinal has one outcome; all receipts fall inside the original
shared deadline. Authentication consumed its budget before provider work.
Provider counts: SSM GetParameter 2; S3 GET 15 / HEAD 2; STS GetCallerIdentity 7;
Cognito DescribeUserPool 1 / DescribeUserPoolClient 1; DynamoDB DescribeTable 2;
S3 Control ListTagsForResource 1. HTTP 404 reads still consumed attempts and did
not authorize repair. The session and non-root identity gate succeeded; earlier
uncertain remote refresh outcomes remain uncertain.

The same cumulative **$1 procedural allowance** remains. Billing and unused dollar
balance are unmeasured. The original ledgers, consumed campaign/provider claims
and all historical reports are retained. Frozen execution tooling still has its
263 baseline; these new receipts establish actual 296. Reusing its claim or
resetting its count is prohibited. Any later live campaign would require reviewed
296-based accounting and separate authorization. No further AWS calls ran during
local analysis/publication.

## Completed resource diff and its limits

SST exited zero and emitted a complete JSON **change list**, retained privately.
It has 60 entries concerning 44 unique resource URNs. Eight SST component creates
appear twice, and each of four code-object replacements has three phase entries;
these are not additional physical resources. Unchanged resources are omitted.

| Classification               | Unique proposed changes                                    |
| ---------------------------- | ---------------------------------------------------------- |
| SST route components         | 8 creates                                                  |
| API Gateway resources        | 8 routes + 8 Lambda proxy integrations created             |
| Lambda resource permissions  | 8 invoke permissions created                               |
| Application Lambda functions | 4 updates: API, cleanup, worker, controlled endpoint       |
| S3 Lambda code objects       | 4 replacements, including create/delete replacement phases |
| Frontend dynamic resources   | Asset files, KV keys and invalidation updated              |
| Existing frontend bucket     | 1 read, no bucket mutation                                 |

That is **32 unique creates, 7 updates, 4 replacements and 1 read**. Physical AWS
creates are the 24 route/integration/permission resources; the other eight create
resources are SST components. Lambda changed fields are `s3Key` / `lastModified`;
code-object fields are `key`, `source`, `versionId`. Frontend changed top-level
fields are `files`, `entries`, `version`. The machine result lists every logical
resource with its type, operations, category and changed field names; sensitive
old/new state is excluded.

The eight route keys are:

- GET / POST `/api/v1/projects/{projectId}/environments`
- GET / PATCH / DELETE `/api/v1/projects/{projectId}/environments/{environmentId}`
- OPTIONS for both environment paths
- POST `/api/v1/local-agent/capabilities`

All routes/integrations reference the inventoried existing API. Integrations use
AWS_PROXY, payload version 2.0. Gateway route authorization is **NONE**, as shown
by this diff; application authentication remains the handler's responsibility
and was not exercised by preview. Each proposed invoke permission has principal
`apigateway.amazonaws.com`, action `lambda:InvokeFunction`, and the existing API's
execution ARN with `/*`. This is an API-wide source scope, not a method/path-only
scope. It requires review as part of the deployment diff; this run changed no IAM.

The change list contains no IAM role/identity/managed-policy change, table/queue,
KMS, Cognito, CloudFront distribution or stage-metadata mutation. It does contain
the eight Lambda resource-policy additions described above. No absence claim is
made about independently changed live configuration outside this diff. Preview
used its permitted reads and retained state; it was not a fresh full AWS inventory.
No deployment, stage closure, migration, activation or runtime target validation
occurred. Previously inventoried stage/capability values were not re-read here.

## Artifact comparisons — ZIP differences need review

All four preview ZIP hashes differ from the earlier prepared bootstrap-only ZIPs.
**Every uncompressed `bootstrap` hash matches the prepared release exactly.**
Each new SST ZIP additionally contains `resource.enc` and a root directory entry.
This establishes an archive-content difference, not just ZIP metadata drift.
Encrypted binding contents were not decrypted/validated; the new packages have
not been adopted as final release artifacts. No packaging repair/rebuild or
replacement of prepared/rollback bytes was performed.

| Functions sharing bytes                            | Prepared ZIP SHA-256                                               | Preview ZIP SHA-256                                                |
| -------------------------------------------------- | ------------------------------------------------------------------ | ------------------------------------------------------------------ |
| UsersProjectsApi / EmptyProjectCleanup             | `118621a9b8d84c7941b2ea795783da1183a018ae58295d2d761954ea176078c6` | `58f2bbf364042ac209d4f9b3a4d6ca583da18914fb5851f0c166d34b9c757373` |
| CloudExecutionWorker / ExecutionControlledEndpoint | `d1bb890914413b1bad1e2108080296a5c05f63e3f023b851836860404b3a29fb` | `82af7beeda4b619afa6831b342c693bb1fb3e8d08e5e6c0416ce253526f59098` |

API/cleanup preview ZIP size is 9,363,094 bytes each; worker/controlled endpoint
is 8,738,299 bytes each. Matching bootstrap hashes are
`6c85373de3c464b12cd502e21496f799da6a229a72db4ca3d34f7fb1f124b9f4`
and `903309b98c8639062a24685b639e54f43968b59b0cb8331cf12865e387f7db4f`
respectively. The JSON result records every package/member comparison.

Frontend hosting metadata matches the prepared frontend exactly. The original
release manifest hash remains
`94d658f4cdae67103c755e3cf5578a50252f8f780c2df82c46c2c2cf18f8c6c5`.
`valid_parent()` reverified prepared/rollback ZIPs, bootstraps, clients, frontend
files and CloudFront function bytes. All 74 protected historical proof files
still match. The read gate policy remains
`7592b876c4a81315dc4cd1b51f9406d6e6689d42e74594f209c95096cf4eaa75`.
No endpoint policy, authentication logic or resource scope changed during the run.

## Evidence, local verification and remaining decisions

The [machine result](project-environments-refresh-preview-review-8.json),
[sanitized authentication projection](project-environments-refresh-preview-review-8-auth-projection.jsonl),
[provider projection](project-environments-refresh-preview-review-8-provider-projection.jsonl)
and [file manifest](project-environments-refresh-preview-review-8.sha256) support
these findings. Original raw logs/state remain private, with explicit non-capture
file hashes in the machine result. No raw private diagnostic receipt or its hash
was included. No diagnostic-capture flag was enabled in this campaign.

Local reconciliation checked contiguous unique attempts/outcomes, HTTP statuses,
identity success, shared deadline, consumed claims, the complete typed resource
list, new ZIP members/bootstrap hashes and all original proofs. `npm run verify`
passed with external networking and dependency downloads disabled (format, lint,
tests, builds and Go test/vet). These checks do not establish deployed behavior,
production billing, new runtime-data compatibility or successful rollback after
new-version data exists; the accepted contracts/release packet retain those limits.

The next decisions are review of the completed resource diff (including gateway
NONE/application authentication and API-wide Lambda invoke scope), review/selection
of the new SST ZIPs and encrypted bindings, and separate explicit approval for any
subsequent release stage. **Deployment, closure, migration, activation, IAM/identity
or project creation, and live target requests remain held.** Production OpenAPI
scope remains separate. No next live command is authorized or run automatically.
