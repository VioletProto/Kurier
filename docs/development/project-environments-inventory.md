# Project environments inventory and artifact preparation

Continuation of [PR #18](https://github.com/VioletProto/Kurier/pull/18) from
`68c022dd2734428cdc0eacb434de8af454411524`, 2026-10-27. Release source remains
**`44f1fb79d943501c95e06fcc1a6114111a7c1ea4`**. The original
[deployment packet](project-environments-deployment-review.md) still defines
compatibility, deployment order and rollback limits.

Read-only inventory and rollback retrieval completed. **Stopped before SST
preview: complete metering of its provider processes is not established.**
No closure, deployment, migration, activation, identity/project creation or live
target request occurred. OpenAPI production scope remains separate.

## Authority, window and metering

Davian authorized one 60-minute window and at most 500 physical AWS attempts,
including retries/pages, retaining the cumulative $1 procedural allowance.
Both configured credential caches were expired. Davian separately authorized
only the metered `signin:CreateOAuth2Token` refresh of the existing CLI session;
it consumed one call and updated its existing local credential cache. No IAM or
application identity was created. All other AWS operations were reads.

The window began **19:20:24.110209 UTC**; the last AWS outcome was
**19:24:31.970932 UTC**, 247.861 seconds later. Total **135/500** physical
attempts, including the refresh, four presigned Lambda-code S3 GETs, SDK retries
if any and every pagination request. The window is now stopped, with no automatic
resume or preview authority inferred from the unused 365 calls.

[Frozen attempt ledger](project-environments-inventory-attempts.jsonl) SHA-256:
`1e4e9f987fdf378ac2b6b968d65175a1a1a5d3d5b8085c8145d86c434b937171`.
It contains operation/service/count/timestamps and outcomes, never URLs, request
arguments, signed query strings, headers, credentials or response bodies.
134 read attempts retain HTTP 200 outcomes. The initial refresh retained its
attempt and successful credential-read completion, but not its HTTP status;
this is an explicit audit limitation, not a fabricated status.

The [inventory meter](../../scripts/environment-inventory-meter.py) charges and
fsyncs before each botocore send, including retries, denies operations outside
its read allowlist, and enforces a cumulative clock/count on resume. Presigned
downloads are individually charged before GET, forbid redirects and verify size/
digest. The retained [collector](../../scripts/inventory-project-environments.py)
and [supplement](../../scripts/inventory-project-environments-supplement.py) show
the exact calls. They cannot resume the stopped campaign. They are **not** a
meter for SST/Pulumi/Go/Node provider subprocesses.

## Genuine current AWS findings

[Sanitized machine inventory](project-environments-inventory.json) retains full
capability/accounting projections, resource configuration and IAM documents.
Application row payloads, encrypted envelopes, secret values and complete
Lambda environments were never exported. Control/Protected scans projected
coordination metadata only; no evidence or request bodies were downloaded.

| Surface          | Observed result                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               |
| ---------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Identity/stage   | Account `747336059622`, IAM user `davian-admin`, us-east-2/dev-api. Free plan ACTIVE; Lambda account/unreserved concurrency10/10.                                                                                                                                                                                                                                                                                                                                                                                                             |
| Durable marker   | Active **version15**, generation `dev-api-initial-v1`; savedRequests8, protectedSecrets1, localAgentExecutions1, cloudExecutions1, executionInputs1, evidencePinning1, sequentialWorkflows1, mcpReadOnly1, workflowAssertions1. projectEnvironments marker absent. Initial/final strong reads match.                                                                                                                                                                                                                                          |
| Stage accounting | October2026 count32, quota version75, pinnedCount0/pinnedBytes0. storageReservedBytes/inflightCount are absent on this row, not invented new zero-valued writes. Seven retained quota rows, including historical per-user/day charges, preserved. Initial scan and final strong stage quota read agree.                                                                                                                                                                                                                                       |
| Active data      | Control115 metadata rows, zero active project rows, 11 project tombstones and 11 execution residuals; Protected empty. No job/run/step/OUT/IN/upload/deletion-work kind observed. Nine retained application users/identity rows and retained credentials/audit/cursors are not disposable campaign resources. This is a sequential scan, not a closed-stage atomic quiescence proof.                                                                                                                                                          |
| Evidence/queues  | Zero current objects under evidence `dev-api/projects/`; queue and DLQ approximate visible/inflight/delayed counts all0. Approximate queue counts cannot establish absence of a concurrent send.                                                                                                                                                                                                                                                                                                                                              |
| Lambda           | All four functions Active/Successful, provided.al2023/ARM64. API256MiB/15s, cleanup256MiB/60s, worker256MiB/90s, fixture128MiB/65s; no reserved/provisioned concurrency. Code ZIPs exactly match the retained assertions release. No customer VPC attachment observed.                                                                                                                                                                                                                                                                        |
| IAM              | Full inline, trust, attached default managed-policy documents/versions retained. All four canonical IAM hashes match the historical reader baseline; repeat trust/inline/attachment/default-version reads agree. Each role attaches AWSLambdaBasicExecutionRole v1; API/cleanup/worker have their existing scoped inline policies; fixture has none. KMS purpose conditions and resource scopes remain. Code/environment hashes also agree on repeat reads. This proves stored configuration, not an effective-permission runtime simulation. |
| Worker/schedule  | One Enabled standard queue mapping, batch1, max concurrency2, ReportBatchItemFailures, no provisioned pollers. Cleanup enabled every minute with the expected Lambda target.                                                                                                                                                                                                                                                                                                                                                                  |
| Gateway          | 45 routes and45 integrations; none of the six environment/capability operations or two explicit preflights is deployed. Default rate10/burst20; no gateway CORS configuration. Existing function invocation policies retained.                                                                                                                                                                                                                                                                                                                |
| Persistence      | Both tables PAY_PER_REQUEST, PITR enabled/35days and TTL disabled; indexes/settings retained in JSON. Key enabled with annual rotation. Four inspected buckets retain full public-access blocks; encryption/versioning and web/evidence policies captured. No configuration was changed.                                                                                                                                                                                                                                                      |
| Cognito          | Existing pool/client, Essentials, default Cognito email, deletion protection; public client without a secret. Rotation/auth/lifetime configuration captured. No sign-in, provider-user listing, email or target execution was performed.                                                                                                                                                                                                                                                                                                      |
| Frontend         | CloudFront distribution `EMNTWD8Q14ZLM`, no aliases; current manifest release `bd511e3f9979d2ef7dd6b73d183dcf7620a423b62659ac89c64e4a1a08981cde`. Five current files, three private root-file archive copies and archive manifest all verify. LIVE viewer-request function and9 KV entries retained, including namespace76ef metadata and previous hashed assets. The placeholder origin is overridden by the retained function/KV configuration; its presence alone is not an origin-policy failure.                                         |
| Logs/bootstrap   | Four configured log groups each retain7days. Names differ from current Lambda names but match LoggingConfig, so no replacement is inferred. Bootstrap v5, parameter version1, existing Ohio state/asset buckets and ECR repository. Existing passphrase metadata and exact dev-api state object present; no passphrase value was read. State bucket has91 current objects, versus88 in the historical packet: no claim that historical tooling state was unchanged. No state object content was downloaded or repaired.                       |

No active project does **not** imply zero monthly usage or permission to erase
historical tombstones, credentials, audit records or quota. Stored schema shapes
were not fully decoded from payloads by this metadata-only inventory; unknown
shape validation and a fresh compatible-reader/quiescence gate remain necessary
before any closed-stage transition.

## Prepared release bytes and rollback proof

[Release manifest](project-environments-release.json) pins four ZIPs, bootstrap
hashes/sizes, toolchain/lockfile and the hosted frontend manifest. Retained locally:
`.build/project-environments-release/44f1fb79d943501c95e06fcc1a6114111a7c1ea4/`.
Artifacts and rollback bytes are ignored, not committed or uploaded to AWS.
Keep these bytes available; manifest hashes alone cannot restore lost archives.

Built from the exact clean detached source, using the existing ignored dependency
link, Go1.27.3-X:nodwarf5, Linux/ARM64 v8.0, CGO0 and `-ldflags='-s -w'`.
Every bootstrap embeds the release revision and `vcs.modified=false`.
ZIPs have a single executable `bootstrap`, DEFLATE9 and fixed2020-01-01 timestamp;
central-directory/CRC/extracted bytes were verified. These replace the earlier
trimpath build-check bytes. A future SST build that produces different bytes must
be re-reviewed or explicitly use these retained ZIPs; no hash substitution is
authorized by matching source alone.

| Prepared package (each)                            | ZIP bytes | ZIP SHA-256                                                        |
| -------------------------------------------------- | --------: | ------------------------------------------------------------------ |
| UsersProjectsApi / EmptyProjectCleanup             | 7,261,815 | `118621a9b8d84c7941b2ea795783da1183a018ae58295d2d761954ea176078c6` |
| CloudExecutionWorker / ExecutionControlledEndpoint | 6,770,846 | `d1bb890914413b1bad1e2108080296a5c05f63e3f023b851836860404b3a29fb` |

New hosted frontend release:
`38325083b003b3f320752dbfa0357c377d57d2b66c1175e06c88ba3638210236`.
Its five files total427,225 bytes; JS `assets/index-DXEG7-ST.js` is422,230 bytes,
SHA-256 `8b55e24fce2358c86d2a2feb385485888d2dc3b221c0a6ac4a25035cac74db3e`.
CSS is unchanged. Public configuration matches the inventoried pool/client/API
and retained current frontend configuration. Hosted-mode build, archive copying
and per-file manifest verification are local; no hosted requests/invalidation ran.

Linux/amd64 foreground local-agent and MCP binaries were also built from the
same clean source; their sizes/SHA-256 and VCS identity are in the release
manifest. They were not installed, paired or connected to live services. Other
client operating systems/architectures are not supplied by this preparation.

Read-only Lambda GetFunction and presigned S3 retrieval retained each original
ZIP and extracted bootstrap under `rollback/<logical-name>/`. Each downloaded
ZIP hash matches **both** current Lambda CodeSha256 and the historical accepted
manifest, without rebuilding:

| Rollback package (each) | ZIP bytes | ZIP SHA-256                                                        |
| ----------------------- | --------: | ------------------------------------------------------------------ |
| API / cleanup           | 9,251,049 | `f4be7af179049c70f479a45dd26e4ae24a19c3a9e8016a865318fa7a96965c32` |
| Worker / fixture        | 8,687,530 | `15e7e6db438de3c19fe962c95be0d3c9d52d17f81b21a0b58cd4603974021379` |

Current frontend root files and matching private archives were downloaded from
the private web bucket, not through the live site. All five size/SHA values match
the retained current manifest; archive manifest bytes equal root manifest bytes.
The LIVE CloudFront function hash is
`b61f6392a0b2e8d95e23f7c177949986c8a8c518b6290f297aebb93a23049684`;
its code, complete distribution settings, policies and exact KV entries/ETag are
retained for rollback review. Historical asset KV entries were observed; only
current release assets were downloaded/verified, so no claim covers every older
release. A rollback apply, propagation or browser A→B→A exercise was not performed.
After new-version data exists, the original packet's fix-forward/compatible-reader
limits still apply: rollback bytes do not make marker downgrade safe.

## Expected changes, not an executed preview

The current AWS baseline and pinned source support this expectation:

- Four code-object replacements and four Lambda code updates, without intended
  environment/memory/timeout/concurrency/data-access-IAM changes.
- Three existing frontend upload/KV/invalidation resources: publish the new
  release/archive and JS entry, retain previous objects/keys with purge=false,
  and a waited `/*` invalidation after upload/KV readiness.
- Eight new route declarations. The installed platform's
  `apigatewayv2-lambda-route.ts` creates one Route, Integration and Lambda
  Permission per declaration: **24 expected AWS resource additions**, plus
  eight SST route component records. Permissions authorize the existing API
  Gateway service principal on the existing function/API execution ARN; they
  change Lambda resource policy, not execution-role data access.

That suggests35 concrete resource changes plus component bookkeeping, not the
assertions eleven-change allowlist. Actual names, provider normalization and
complete operations remain unverified until an authorized, fully metered preview.
No new table/index/key/queue/pool/bucket, bootstrap repair, standing compute or
broader IAM change is proposed. Reject any such diff rather than treating this
expectation as permission to deploy.

## SST preview preflight and exact metadata-write boundary

Installed npm/platform version4.17.1 and the binary identify commit
`a0bd20f762883e72a35caccb4896c42ce5b3f707`; binary SHA-256
`539659fa60f00e8b9610b7dfd967928d382b4800ca25452d9697e31a4eb521fd`.
Reviewed the [exact run source](https://github.com/anomalyco/sst/blob/a0bd20f762883e72a35caccb4896c42ce5b3f707/pkg/project/run.go)
and tagged [AWS backend](https://github.com/anomalyco/sst/blob/v4.17.1/pkg/project/provider/aws.go)
and [passphrase provider](https://github.com/anomalyco/sst/blob/v4.17.1/pkg/project/provider/provider.go).

For `diff`, the run path uses a local Pulumi backend and skips remote lock,
partial/current-state pushes, update history and pruning. It still performs
bootstrap initialization and GetOrCreatePassphrase. Current metadata matches
the existing prerequisites; no normal-path remote metadata write is expected.
The **conditional, unapproved** write scopes are:

| Trigger                    | Potential write scope; currently held                                                                                                                                                                                                                                                                                                                               |
| -------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Missing stage passphrase   | SSM PutParameter creating SecureString `/sst/passphrase/kurier/dev-api`, overwrite=false, and associated service-side encryption. Never create it implicitly.                                                                                                                                                                                                       |
| Missing/outdated bootstrap | SSM PutParameter overwrite of `/sst/bootstrap` and version-dependent bucket/ECR/bootstrap policy/lifecycle mutations. Existing expected buckets are `sst-state-moshrdrrbeba` and `sst-asset-moshrdrrbeba`, repository `sst-asset`. Newly generated replacement names are not bounded by this packet: any mismatch must stop rather than authorize migration/repair. |
| Wrong command/path         | Lock/update/state/history writes under `lock/kurier/dev-api.json`, `app/kurier/dev-api.json`, `update/kurier/dev-api/`, `snapshot/kurier/dev-api/`, `eventlog/kurier/dev-api/` and related retention/pruning. These are excluded from the reviewed diff path, not authorized fallback operations.                                                                   |
| Dynamic provider mutation  | Package/static S3 PUT/DELETE, CloudFront KV update/invalidation and resource creation belong to apply methods, not authorized preview effects. Any attempted write must be denied and stop the campaign.                                                                                                                                                            |

Local work directories, bundled code, generated types, frontend archives, logs
and `.sst/outputs.json` may change during preview. Telemetry/update checks must
be disabled; missing dependencies/providers must not trigger unreviewed installs.
No passphrase value, raw state or generated preview should be published.

**Preview was not launched.** The Python-only inventory hooks cannot intercept
SST's Go SDK, Pulumi providers, Node SDKs and RPC children. Existing historical
proxy tests are not proof that this invocation is fully isolated and metered
within the same window/count. A network-isolated, fail-closed shared meter must
prove retry/page/redirect coverage and no bypass for all child processes, bind
to the current ledger/clock, permit only classified reads, and deny every write
above before a future preview. No attempt was made to bypass this stop through
an unmetered CLI, provider flag, alternate endpoint or local-only apparent diff.

## Verification, cost uncertainty and remaining decisions

Fresh local meter tests cover actual SDK retries, call/deadline latching,
read/write enforcement and cumulative resume. Local source builds and ZIP/hosted
manifest/rollback hash checks passed. Repository formatting/lint/tests/build/Go
verification and final documentation checks are recorded with this PR update.
These do not establish environment runtime behavior or deployed IAM enforcement.

The cumulative **$1 procedural allowance is preserved**, not renewed and not
reported as measured spend. No Cost Explorer or current price request was made.
S3 code/static reads and DDB scans may incur ordinary fees; SDK background service
operations and delayed/account-wide billing are not the135 operator attempts.
Free-plan credit metadata was observed but is not a feature-cost measurement;
the credit amount is omitted from the published packet. No unused dollar balance
or billing-enforced cap is claimed.

Next decisions:

1. Review/accept the current baseline, prepared release hashes, local retention
   and rollback proof. Fresh deployment gates must re-read live state; the
   sequential inventory is not closure authorization or a lease on AWS state.
2. Prepare and locally prove complete provider-process metering before separately
   authorizing preview. This campaign has stopped; any new window/remaining-call
   treatment must be explicit, not reset silently. Conditional metadata writes
   stay held; any prerequisite mismatch requires exact repair scope review.
3. Review a successful preview separately before deployment, stage closure,
   migration or activation. There is no automatic continuation. The guarded
   environment activator and genuine bounded runtime/cleanup campaign remain
   separate decisions under the original packet.
