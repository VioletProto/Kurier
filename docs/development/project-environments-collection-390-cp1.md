# Initial collection release-390-cp1: archive byte agreement held

The published start command ran **once**, from clean tooling
**42915e66301e272d28f5d9a3aac253239ca51e9e**, unchanged clean release source
**44f1fb79d943501c95e06fcc1a6114111a7c1ea4** and the original adopted
`adopted-current-1.json` checkpoint. It exited **1** after a successful reader
snapshot and complete guarded preview because fresh ZIP bytes did not match the
adopted archive hashes. The campaign is latched stopped; no later phase ran.
[Machine outcome](project-environments-collection-390-cp1.json),
[reader findings](project-environments-collection-390-cp1-readers.json),
[complete sanitized resource diff](project-environments-collection-390-cp1-diff.json),
[projected attempts](project-environments-collection-390-cp1-attempts.jsonl).

## Authorization, authentication and accounting

Local checks verified pinned tooling/models, clean source, adopted/rollback bytes,
all four historical campaigns/claims and the private checkpoint's approval and
acceptance bindings. The mode-0600 execution authorization was created exclusively
from the exact bound template with only `authorized` changed to true. The fresh
claim binds authorization, checkpoint and campaign directory. Nothing was recaptured
or substituted. The first boundary matched the adopted metadata before requests.

One reviewed existing-session renewal was sent at **391** (HTTP 200), followed by
mandatory same-identity/account STS at **392** (HTTP 200). Credential validity and
both child identity proofs passed. Nine STS calls in total include that host check;
all are metered. No background or mid-phase renewal occurred. Historical grant
continuity remains unestablished; this run does not rewrite that limitation.

**390 + 109 = 499/1,799**, leaving **1,300 arithmetic attempt headroom**. All 109
physical attempts have outcomes: **101 HTTP 200, eight HTTP 404**, no failed
transport or unmatched attempt. The only write-labelled operation was the approved
credential renewal; **zero infrastructure/state writes, uploads or runtime
requests** occurred. The cumulative **$1 procedural allowance** is unchanged;
billing is unmeasured.

The terminal stop was **61.891351 seconds** after the original monotonic campaign
start. It left **3238.108649 seconds** of the original 55-minute window and
**178.122144 seconds** of the shared 240-second collection phase at that stop.
The closed-stage window never began. No deadline was reset; remaining time does
not authorize continuation.

## Fresh baseline, quiescence and old frontend

The stage snapshot is **active**, version **15**, with the historical recovery
generation and schema-8 saved requests; the environment capability remains absent.
The stage marker was unchanged across collection. All existing capability and
inventory guards passed.

The projected inventory has **107 rows**, **zero active projects**, seven accounting
rows and eleven residual rows whose evidence prefixes are all empty. Queue visible,
in-flight and delayed counts are all **zero**. Stage quota remains version 75,
month 2026-10, count 32. Full kind counts appear in the reader record; no row keys,
identities, payloads or credential values are published.

All four functions match the accepted old baseline for deployed code, configuration,
IAM role policies/trust, runtime, architecture, memory and timeout. They are Active
with Successful updates and no reserved concurrency. Existing 45 routes, 45
integrations and invocation policies match inventory. The active free plan, account
and unreserved concurrency 10, batch 1 / max 2 queue mapping, enabled minute cleanup
schedule and API throttle guards passed.

**Accepted old-frontend readiness passed**, including marker/file bytes, old asset
KVS entries and distribution status. It was checked against the accepted baseline,
not the undeployed adopted release. These are bounded snapshot observations;
current cloud state after the snapshot is not established.

## Complete fresh resource diff

The successful SST change list has **60 entries / 44 unique resources**, and the
reviewed projection exactly matches review 8. It classifies **32 creates, seven
updates, four replacements and one existing-bucket read**. Eight route-component
create entries are duplicated; replacement phases appear separately. Unchanged
resources are omitted, so this is a complete change list, not a new full AWS inventory.

Physical creates remain eight API routes, eight proxy integrations and eight Lambda
invoke permissions. Gateway authorization is NONE for the eight route keys in the
machine diff; application authentication was not exercised. Permissions use
API Gateway principal, lambda:InvokeFunction and the accepted existing API-wide
source wildcard. Permission target inputs are encrypted in retained preview JSON;
this publication does not decrypt them or claim direct target-value validation.
No IAM role/managed-policy,
identity, table, queue, KMS, Cognito, distribution or stage metadata change appears.
No proposed resource change was applied.

| Logical resource                                         | Type                                       | Operations                                   | Changed top-level fields |
| -------------------------------------------------------- | ------------------------------------------ | -------------------------------------------- | ------------------------ |
| CloudExecutionWorkerCode                                 | `aws:s3/bucketObjectv2:BucketObjectv2`     | create-replacement, delete-replaced, replace | key, source, versionId   |
| CloudExecutionWorkerFunction                             | `aws:lambda/function:Function`             | update                                       | lastModified, s3Key      |
| DevelopmentHttpApiRouteBateco                            | `sst:aws:ApiGatewayV2LambdaRoute`          | create                                       | —                        |
| DevelopmentHttpApiRouteBatecoIntegration                 | `aws:apigatewayv2/integration:Integration` | create                                       | —                        |
| DevelopmentHttpApiRouteBatecoPermissions                 | `aws:lambda/permission:Permission`         | create                                       | —                        |
| DevelopmentHttpApiRouteBatecoRoute                       | `aws:apigatewayv2/route:Route`             | create                                       | —                        |
| DevelopmentHttpApiRouteBbrzwo                            | `sst:aws:ApiGatewayV2LambdaRoute`          | create                                       | —                        |
| DevelopmentHttpApiRouteBbrzwoIntegration                 | `aws:apigatewayv2/integration:Integration` | create                                       | —                        |
| DevelopmentHttpApiRouteBbrzwoPermissions                 | `aws:lambda/permission:Permission`         | create                                       | —                        |
| DevelopmentHttpApiRouteBbrzwoRoute                       | `aws:apigatewayv2/route:Route`             | create                                       | —                        |
| DevelopmentHttpApiRouteBcuohn                            | `sst:aws:ApiGatewayV2LambdaRoute`          | create                                       | —                        |
| DevelopmentHttpApiRouteBcuohnIntegration                 | `aws:apigatewayv2/integration:Integration` | create                                       | —                        |
| DevelopmentHttpApiRouteBcuohnPermissions                 | `aws:lambda/permission:Permission`         | create                                       | —                        |
| DevelopmentHttpApiRouteBcuohnRoute                       | `aws:apigatewayv2/route:Route`             | create                                       | —                        |
| DevelopmentHttpApiRouteKbebwh                            | `sst:aws:ApiGatewayV2LambdaRoute`          | create                                       | —                        |
| DevelopmentHttpApiRouteKbebwhIntegration                 | `aws:apigatewayv2/integration:Integration` | create                                       | —                        |
| DevelopmentHttpApiRouteKbebwhPermissions                 | `aws:lambda/permission:Permission`         | create                                       | —                        |
| DevelopmentHttpApiRouteKbebwhRoute                       | `aws:apigatewayv2/route:Route`             | create                                       | —                        |
| DevelopmentHttpApiRouteKotabw                            | `sst:aws:ApiGatewayV2LambdaRoute`          | create                                       | —                        |
| DevelopmentHttpApiRouteKotabwIntegration                 | `aws:apigatewayv2/integration:Integration` | create                                       | —                        |
| DevelopmentHttpApiRouteKotabwPermissions                 | `aws:lambda/permission:Permission`         | create                                       | —                        |
| DevelopmentHttpApiRouteKotabwRoute                       | `aws:apigatewayv2/route:Route`             | create                                       | —                        |
| DevelopmentHttpApiRouteRdfcms                            | `sst:aws:ApiGatewayV2LambdaRoute`          | create                                       | —                        |
| DevelopmentHttpApiRouteRdfcmsIntegration                 | `aws:apigatewayv2/integration:Integration` | create                                       | —                        |
| DevelopmentHttpApiRouteRdfcmsPermissions                 | `aws:lambda/permission:Permission`         | create                                       | —                        |
| DevelopmentHttpApiRouteRdfcmsRoute                       | `aws:apigatewayv2/route:Route`             | create                                       | —                        |
| DevelopmentHttpApiRouteSndcdb                            | `sst:aws:ApiGatewayV2LambdaRoute`          | create                                       | —                        |
| DevelopmentHttpApiRouteSndcdbIntegration                 | `aws:apigatewayv2/integration:Integration` | create                                       | —                        |
| DevelopmentHttpApiRouteSndcdbPermissions                 | `aws:lambda/permission:Permission`         | create                                       | —                        |
| DevelopmentHttpApiRouteSndcdbRoute                       | `aws:apigatewayv2/route:Route`             | create                                       | —                        |
| DevelopmentHttpApiRouteVxffaw                            | `sst:aws:ApiGatewayV2LambdaRoute`          | create                                       | —                        |
| DevelopmentHttpApiRouteVxffawIntegration                 | `aws:apigatewayv2/integration:Integration` | create                                       | —                        |
| DevelopmentHttpApiRouteVxffawPermissions                 | `aws:lambda/permission:Permission`         | create                                       | —                        |
| DevelopmentHttpApiRouteVxffawRoute                       | `aws:apigatewayv2/route:Route`             | create                                       | —                        |
| EmptyProjectCleanupCode                                  | `aws:s3/bucketObjectv2:BucketObjectv2`     | create-replacement, delete-replaced, replace | key, source, versionId   |
| EmptyProjectCleanupFunction                              | `aws:lambda/function:Function`             | update                                       | lastModified, s3Key      |
| ExecutionControlledEndpointCode                          | `aws:s3/bucketObjectv2:BucketObjectv2`     | create-replacement, delete-replaced, replace | key, source, versionId   |
| ExecutionControlledEndpointFunction                      | `aws:lambda/function:Function`             | update                                       | lastModified, s3Key      |
| FrontendWebAssetFiles.sst.aws.BucketFiles                | `pulumi-nodejs:dynamic:Resource`           | update                                       | files                    |
| FrontendWebAssets                                        | `aws:s3/bucket:Bucket`                     | read                                         | —                        |
| FrontendWebInvalidation.sst.aws.DistributionInvalidation | `pulumi-nodejs:dynamic:Resource`           | update                                       | version                  |
| FrontendWebKvKeys.sst.aws.KvKeys                         | `pulumi-nodejs:dynamic:Resource`           | update                                       | entries                  |
| UsersProjectsApiCode                                     | `aws:s3/bucketObjectv2:BucketObjectv2`     | create-replacement, delete-replaced, replace | key, source, versionId   |
| UsersProjectsApiFunction                                 | `aws:lambda/function:Function`             | update                                       | lastModified, s3Key      |

## Artifact agreement and exact hold

All four fresh ZIPs exist, but **zero of four whole-archive hashes match adoption**.
Each archive has the same length, exact member list and **identical uncompressed
member bytes** as its adopted counterpart, including `bootstrap` and `resource.enc`.
The generated frontend metadata marker matches the adopted manifest.

A read-only raw-byte comparison establishes that **every archive byte difference
is confined to the central-directory external-attribute fields**. The file-mode
portions differ identically for all four packages:

| ZIP member     | Adopted mode | Fresh mode |
| -------------- | ------------ | ---------- |
| `./`           | `040700`     | `040755`   |
| `bootstrap`    | `0100700`    | `0100755`  |
| `resource.enc` | `0100600`    | `0100644`  |

No ciphertext/plaintext change is inferred: encrypted member bytes are already
identical. The complete ZIPs still differ, and the approved gate compares whole
archive hashes. Retained evidence plus pinned control-flow order identifies the
first failed condition as `fresh-package-drift-held`: fresh plan projection passes,
then the first whole-archive check fails. The ledger retains the generic terminal
`phase-failed-or-uncertain-no-restart` reason. No collection proof was emitted.

No file modes, ZIP bytes, expected hashes or adopted manifests were repaired,
normalized or replaced. Fresh archives remain private evidence beside the original
adopted archives and rollback bytes. This report does not approve their adoption
or change the byte-agreement requirement.

## Preservation, verification and next decision

Executed once, **historical command only — do not rerun**:

```sh
python3 -B scripts/environment-deploy/driver.py start --campaign .build/project-environments-execution/release-390-cp1 --authorization .build/project-environments-execution/authorization-from-390-checkpoint-1.json --checkpoint .build/project-environments-execution/checkpoints/adopted-current-1.json
```

All five campaigns, consumed claims, original/adopted checkpoints, private evidence,
ledgers, adopted ZIPs and rollback bytes are preserved. Metered renewal advanced only
the new campaign's continuity checkpoint, without editing the adopted pre-campaign
record or earlier checkpoints. No worktree registration was pruned or artifact rebuilt
after the stop. The PR branch was restored for publication.

Publication verification independently reconciles attempt ordinals 391–499,
outcomes, authorization/claim/checkpoint binding and original deadlines; validates
reader and full diff projections; compares all ZIP members and raw difference
locations; and rechecks preserved artifact/source pins. Formatting and disclosure
checks pass. Tooling/application code is unchanged, so prior build/lint/test proofs
were not repeated for this evidence-only publication.

The next decision is a **local review of ZIP external-attribute reproducibility**
against the adopted archives. No repair, repackaging, hash substitution or adoption
is authorized by this result. Frozen tooling still describes prior 390; the terminal
ledger establishes **499**, and its claim is consumed. Any future live work requires
reviewed accounting from 499, fresh unique claims/checkpoint handling and explicit
authorization. No further live command, renewal, closure, deployment, verification,
activation, final phase, target request or rollback is proposed or authorized.
