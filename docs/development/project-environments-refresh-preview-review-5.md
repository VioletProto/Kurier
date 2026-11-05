# Authorized review-5 refresh/preview — stopped at endpoint hold

**Authentication succeeded; the read-only preview stopped on endpoint-held.**
The authorized command ran once using clean tooling
`4a0faafc673e97d9faae3f26906a327a8bd06e71`, as published in packet
`a62bea8468145f1db9fb406d597b46fd0bee4ad2`. Release source remained
`44f1fb79d943501c95e06fcc1a6114111a7c1ea4`.

The launcher exited 1, reporting sandbox exit -15 after the supervisor stopped
it. No restart, repeated refresh, repair or policy widening followed. This is
**not a completed preview or release approval**.

## Execution and shared accounting

The published detached checkout and this exact launcher command ran once from
`/home/king/Neumont/Kurier`:

```sh
python3 -B scripts/preview-meter/run.py refresh-preview \
  --review-commit 4a0faafc673e97d9faae3f26906a327a8bd06e71 \
  --source /tmp/kurier-environments-release-source \
  --output /home/king/Neumont/Kurier/.build/project-environments-preview/review-5
```

**Historical evidence only: do not rerun.** The feature branch was restored only
for local results publication after the campaign had stopped. No execution code
was edited. Before execution, local checks verified the reviewed tooling
manifest, SDK pins, clean source, prior proofs, prepared/rollback bytes, and
absence of the fresh output and window-from-168 claim. The launcher repeated its
own preflight and required the exact clean implementation commit.

| Bound or result                     | Actual                                                                     |
| ----------------------------------- | -------------------------------------------------------------------------- |
| Shared window began                 | 2026-10-28 22:36:54.833473 UTC                                             |
| Stop recorded                       | 2026-10-28 22:37:23.203108 UTC                                             |
| Elapsed                             | 28.370 seconds of the authorized 3,600 seconds                             |
| Prior usage                         | 168 physical AWS attempts                                                  |
| Existing-session token refresh      | Ordinal 169, HTTP 200, corrected exact endpoint enforced                   |
| Required STS identity verification  | Ordinal 170, HTTP 200, same account and existing identity verified         |
| Provider attempts                   | 30, ordinals 171–200                                                       |
| This campaign                       | **32 of at most 332 additional attempts**                                  |
| Cumulative                          | **200/500**                                                                |
| Outcomes                            | Authentication/verification: 2 HTTP 200; provider: 23 HTTP 200, 7 HTTP 404 |
| Unmatched/transport-failed attempts | None in either ledger                                                      |

Authentication completed in 0.783 seconds. Its durable ledger and consumed
handoff show successful identity verification before provider work; refresh/STS
usage and the original deadline were carried into the provider meter. Concurrent
provider outcomes can arrive out of order; every unique admitted ordinal has
exactly one retained outcome. HTTP 404 responses remain observed failures, not
successful reads or authority to repair resources.

Provider operations were SSM GetParameter 2; S3 GET 15/HEAD 2; STS
GetCallerIdentity 7; Cognito DescribeUserPool 1/DescribeUserPoolClient 1; DynamoDB
DescribeTable 2. The endpoint hold was not forwarded and added no AWS attempt.
No post-stop AWS inspection or target request ran.

The remaining 300 calls are **arithmetic headroom, not continuation authority**.
The frozen tooling checker still describes its 168-based parent; the new terminal
ledgers establish actual 200. Future live work requires separately reviewed
200-based proof/accounting and explicit authorization. The consumed
window-from-168 claim and provider claim must remain intact; neither stopped
window nor handoff may be resumed or deleted to obtain another attempt.

The same cumulative **$1 procedural allowance** remains. Billing, shared
historical spend and unused dollar balance were not measured.

## Sanitized denial and attribution limit

The retained denial reports:

| Field                     | Value                        |
| ------------------------- | ---------------------------- |
| Reason / cumulative count | endpoint-held / 200          |
| Phase / method            | connect / CONNECT            |
| HTTP version / port       | HTTP/1.1 / 443               |
| Hostname / status         | `<redacted-host>` / redacted |
| Scheme / trailing dot     | unspecified / false          |

The reviewed sanitizer emits only its static hostname catalogue and replaces
other names entirely. Its denial record therefore provides no attributable
hostname or AWS operation. **No endpoint is inferred to be harmless, metadata,
telemetry or a particular service.** A CONNECT hold establishes neither the
remote protocol beyond the requested port nor a TLS handshake to that target.
The supervisor stopped the sandbox; no policy amendment or diagnostic expansion
was attempted. Raw Pulumi/state material remains private, outside the public
projection. Raw stdout is empty. No request URL paths/query strings, headers,
bodies, credentials or raw provider diagnostics are published.

Current successful refresh is evidence for this campaign's corrected path;
**it does not prove the historical timeout cause**. Both earlier failed remote
refresh outcomes remain uncertain.

## Classified partial resource evidence

The fresh private Pulumi event log contains 1,101 events: 109 resource pre-events
and 175 resource-output events, **zero summary events**. Event timestamps fall
inside this campaign's window. SST's stdout is empty. Resource-output events
include duplicates and are not additional proposed resources. The sanitized
pre-event set matches the first interrupted preview; that agreement does not
make either plan complete.

| Classification                  | Observed current pre-event evidence                     | Limit                                                                                        |
| ------------------------------- | ------------------------------------------------------- | -------------------------------------------------------------------------------------------- |
| New route components            | 8 create entries, all sst:aws:ApiGatewayV2LambdaRoute   | Local component registration, not deployed AWS route creation                                |
| Existing entries                | 100 same entries                                        | Provisional partial plan; not a complete no-change finding                                   |
| Read                            | 1 FrontendWebAssets aws:s3/bucket:Bucket entry          | Preview resource read, not a mutation                                                        |
| IAM                             | Existing role entries are provisionally same            | No complete inline-policy, permission or final IAM diff                                      |
| Application/frontend/infra plan | No final summary or complete AWS add/update/delete plan | Source-derived expectations in the deployment packet remain unverified by a complete preview |

The eight component creates correspond to:

- POST/GET `/api/v1/projects/{projectId}/environments`
- GET/PATCH/DELETE `/api/v1/projects/{projectId}/environments/{environmentId}`
- OPTIONS on both environment paths
- POST `/api/v1/local-agent/capabilities`

The [machine result](project-environments-refresh-preview-review-5.json) contains
only selected resource names/types/operations and source-derived route mappings.
It excludes Pulumi resource inputs/outputs, secrets and parameter values. No
claim is made that another actor changed nothing in AWS during this interval.

## Artifacts and retained proof

The locally rebuilt frontend's five files and release metadata match the
prepared release, ID
`38325083b003b3f320752dbfa0357c377d57d2b66c1175e06c88ba3638210236`.
No Lambda ZIP completed before the hold. The empty generated-package list does
not establish ZIP equality; there are no new ZIP hashes to review.

All prepared packages/bootstraps, clients, frontend files and rollback bytes
passed their original-manifest checks before and after execution. No production
artifacts were uploaded/replaced. **No release artifact-hash differences.**
Release-manifest SHA-256 remains
`94d658f4cdae67103c755e3cf5578a50252f8f780c2df82c46c2c2cf18f8c6c5`;
byte-identical preview policy SHA-256 remains
`ed237fbf3fe24ccd06e5ed08a73df641499e5c19c9342c56f14e93198764a560`.
The corrected exact authentication gate was unchanged during execution.

[Sanitized authentication projection](project-environments-refresh-preview-review-5-auth-projection.jsonl),
[provider projection](project-environments-refresh-preview-review-5-provider-projection.jsonl)
and [checksum manifest](project-environments-refresh-preview-review-5.sha256)
accompany the machine result. All 21 prior protected files remain unchanged,
including old failed claims and diagnostic receipts. New original window,
authentication/provider ledgers, handoff, consumed campaign/provider claims,
usage, packaging and private event logs remain retained with hashes. Original
ledgers were not replaced by projections. Publication checks reconcile ordinals,
unique outcomes, counts/deadline, safe field projection, current event times,
artifact equality and private/original hashes using local reads only. Full
repository format/lint/test/build/Go verification passed with external networking
and dependency downloads disabled. Five rebuilt frontend files were also
individually rehashed against the matching prepared metadata.

## Stop and remaining decisions

**Stopped after the single authorized attempt.** No deployment, stage closure,
migration, activation, identity/project creation, live target requests, repairs,
repeated refresh or automatic continuation occurred. OpenAPI production scope
remains separate.

The unresolved endpoint hold needs review before any further preview. Any local
attribution or narrower diagnostic proposal, tooling amendment, 200-based
accounting update, new window or live attempt requires its own scope. This result
authorizes none of them. The resource/IAM plan remains incomplete; no deployment
or capability activation decision can be based on a completed preview here.
