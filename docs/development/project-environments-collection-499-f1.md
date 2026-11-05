# Initial collection release-499-f1: completed

The exact approved start command ran **once**, from clean tooling
**bf1effb2835ae6be00bdbd753a802dc0b9bf6741** and unchanged clean release source
**44f1fb79d943501c95e06fcc1a6114111a7c1ea4**. It exited **0** after collection.
No later phase ran. [Machine outcome](project-environments-collection-499-f1.json),
[reader findings](project-environments-collection-499-f1-readers.json),
[complete sanitized change list](project-environments-collection-499-f1-diff.json),
[all projected attempts](project-environments-collection-499-f1-attempts.jsonl).

## Authorization and checkpoint

Local checks passed for clean tooling/source, pinned SDK/provider models, adopted
and rollback artifacts, historical ledgers/claims and read scope. The private
mode-0600 authorization was created exclusively from `template --fresh-checkpoint`,
reviewed for the exact inventoried identity/account and limits, with only
`authorized` changed to true. The consumed fresh claim, authorization, campaign,
initial checkpoint and acceptance record bindings were independently reconciled.

The explicitly approved initial capture validated the current existing cache under
an exclusive lock immediately before authentication. Historical grant continuity
remains **unproved**. No previous checkpoint was replaced. Exactly one metered
existing-session renewal at **500** and mandatory fresh host STS at **501** returned
HTTP 200. Identity/account, credential validity and both child identity proofs
passed. Nine STS attempts total include host, collector and provider checks. No
background or mid-phase renewal, recapture or alternate session occurred.

## Fresh baseline and configuration

The bounded snapshot is **active**, stage version **15**, with historical recovery
generation and schema-8 saved requests; the environment capability is absent.
The stage marker stayed unchanged across collection. The inventory contains
**107 projected rows**, **zero active projects**, seven accounting rows and eleven
residual rows with empty evidence prefixes. Visible, in-flight and delayed queue
counts are all **zero**. Quota remains version **75**, month **2026-10**, count **32**.
Kind counts are published without row keys, identities, payloads or secret values.

All four functions match the accepted deployed code/configuration/IAM baseline,
including role policies/trust, runtime, architecture, memory and timeout. Their
state/update status is Active/Successful with no reserved concurrency. The
**45 routes / 45 integrations**, invocation policies, active free plan, account and
unreserved concurrency **10**, queue batch **1** / max concurrency **2**, enabled
minute cleanup schedule and API throttle guards passed.

**Accepted old-frontend readiness passed**, including old file bytes, asset KVS
entries and deployed distribution. Pre-deployment checks used the accepted old
baseline; generated release frontend metadata independently matched adoption.
These observations do not establish cloud state after the bounded snapshot.

## Complete diff and artifact agreement

The complete successful change list has **60 entries / 44 unique resources**,
matching review 8's projection: **32 creates, seven updates, four replacements,
one existing-bucket read**. Duplicate route-component creates and separate
replacement steps account for repeated entries. Unchanged resources are omitted;
this is the complete change list, not a new full AWS inventory.

Physical creates are eight routes, eight proxy integrations and eight Lambda
invoke permissions. Gateway authorization is NONE on the eight route keys;
application authentication/runtime behavior was not exercised. Invocation
permissions use the accepted API Gateway principal/action and existing API-wide
source scope. Target inputs remain encrypted in retained preview data; they were
not decrypted or claimed directly validated. No IAM role/managed-policy, identity,
table, queue, KMS, Cognito, distribution or stage metadata change is proposed.
No proposed resource change was applied.

| Resource                                                 | Type                                       | Operations                                   | Changed fields         |
| -------------------------------------------------------- | ------------------------------------------ | -------------------------------------------- | ---------------------- |
| CloudExecutionWorkerCode                                 | `aws:s3/bucketObjectv2:BucketObjectv2`     | create-replacement, delete-replaced, replace | key, source, versionId |
| CloudExecutionWorkerFunction                             | `aws:lambda/function:Function`             | update                                       | lastModified, s3Key    |
| DevelopmentHttpApiRouteBateco                            | `sst:aws:ApiGatewayV2LambdaRoute`          | create                                       | —                      |
| DevelopmentHttpApiRouteBatecoIntegration                 | `aws:apigatewayv2/integration:Integration` | create                                       | —                      |
| DevelopmentHttpApiRouteBatecoPermissions                 | `aws:lambda/permission:Permission`         | create                                       | —                      |
| DevelopmentHttpApiRouteBatecoRoute                       | `aws:apigatewayv2/route:Route`             | create                                       | —                      |
| DevelopmentHttpApiRouteBbrzwo                            | `sst:aws:ApiGatewayV2LambdaRoute`          | create                                       | —                      |
| DevelopmentHttpApiRouteBbrzwoIntegration                 | `aws:apigatewayv2/integration:Integration` | create                                       | —                      |
| DevelopmentHttpApiRouteBbrzwoPermissions                 | `aws:lambda/permission:Permission`         | create                                       | —                      |
| DevelopmentHttpApiRouteBbrzwoRoute                       | `aws:apigatewayv2/route:Route`             | create                                       | —                      |
| DevelopmentHttpApiRouteBcuohn                            | `sst:aws:ApiGatewayV2LambdaRoute`          | create                                       | —                      |
| DevelopmentHttpApiRouteBcuohnIntegration                 | `aws:apigatewayv2/integration:Integration` | create                                       | —                      |
| DevelopmentHttpApiRouteBcuohnPermissions                 | `aws:lambda/permission:Permission`         | create                                       | —                      |
| DevelopmentHttpApiRouteBcuohnRoute                       | `aws:apigatewayv2/route:Route`             | create                                       | —                      |
| DevelopmentHttpApiRouteKbebwh                            | `sst:aws:ApiGatewayV2LambdaRoute`          | create                                       | —                      |
| DevelopmentHttpApiRouteKbebwhIntegration                 | `aws:apigatewayv2/integration:Integration` | create                                       | —                      |
| DevelopmentHttpApiRouteKbebwhPermissions                 | `aws:lambda/permission:Permission`         | create                                       | —                      |
| DevelopmentHttpApiRouteKbebwhRoute                       | `aws:apigatewayv2/route:Route`             | create                                       | —                      |
| DevelopmentHttpApiRouteKotabw                            | `sst:aws:ApiGatewayV2LambdaRoute`          | create                                       | —                      |
| DevelopmentHttpApiRouteKotabwIntegration                 | `aws:apigatewayv2/integration:Integration` | create                                       | —                      |
| DevelopmentHttpApiRouteKotabwPermissions                 | `aws:lambda/permission:Permission`         | create                                       | —                      |
| DevelopmentHttpApiRouteKotabwRoute                       | `aws:apigatewayv2/route:Route`             | create                                       | —                      |
| DevelopmentHttpApiRouteRdfcms                            | `sst:aws:ApiGatewayV2LambdaRoute`          | create                                       | —                      |
| DevelopmentHttpApiRouteRdfcmsIntegration                 | `aws:apigatewayv2/integration:Integration` | create                                       | —                      |
| DevelopmentHttpApiRouteRdfcmsPermissions                 | `aws:lambda/permission:Permission`         | create                                       | —                      |
| DevelopmentHttpApiRouteRdfcmsRoute                       | `aws:apigatewayv2/route:Route`             | create                                       | —                      |
| DevelopmentHttpApiRouteSndcdb                            | `sst:aws:ApiGatewayV2LambdaRoute`          | create                                       | —                      |
| DevelopmentHttpApiRouteSndcdbIntegration                 | `aws:apigatewayv2/integration:Integration` | create                                       | —                      |
| DevelopmentHttpApiRouteSndcdbPermissions                 | `aws:lambda/permission:Permission`         | create                                       | —                      |
| DevelopmentHttpApiRouteSndcdbRoute                       | `aws:apigatewayv2/route:Route`             | create                                       | —                      |
| DevelopmentHttpApiRouteVxffaw                            | `sst:aws:ApiGatewayV2LambdaRoute`          | create                                       | —                      |
| DevelopmentHttpApiRouteVxffawIntegration                 | `aws:apigatewayv2/integration:Integration` | create                                       | —                      |
| DevelopmentHttpApiRouteVxffawPermissions                 | `aws:lambda/permission:Permission`         | create                                       | —                      |
| DevelopmentHttpApiRouteVxffawRoute                       | `aws:apigatewayv2/route:Route`             | create                                       | —                      |
| EmptyProjectCleanupCode                                  | `aws:s3/bucketObjectv2:BucketObjectv2`     | create-replacement, delete-replaced, replace | key, source, versionId |
| EmptyProjectCleanupFunction                              | `aws:lambda/function:Function`             | update                                       | lastModified, s3Key    |
| ExecutionControlledEndpointCode                          | `aws:s3/bucketObjectv2:BucketObjectv2`     | create-replacement, delete-replaced, replace | key, source, versionId |
| ExecutionControlledEndpointFunction                      | `aws:lambda/function:Function`             | update                                       | lastModified, s3Key    |
| FrontendWebAssetFiles.sst.aws.BucketFiles                | `pulumi-nodejs:dynamic:Resource`           | update                                       | files                  |
| FrontendWebAssets                                        | `aws:s3/bucket:Bucket`                     | read                                         | —                      |
| FrontendWebInvalidation.sst.aws.DistributionInvalidation | `pulumi-nodejs:dynamic:Resource`           | update                                       | version                |
| FrontendWebKvKeys.sst.aws.KvKeys                         | `pulumi-nodejs:dynamic:Resource`           | update                                       | entries                |
| UsersProjectsApiCode                                     | `aws:s3/bucketObjectv2:BucketObjectv2`     | create-replacement, delete-replaced, replace | key, source, versionId |
| UsersProjectsApiFunction                                 | `aws:lambda/function:Function`             | update                                       | lastModified, s3Key    |

All **four whole-archive SHA256 hashes match the adopted ZIPs exactly**. Independent
comparisons confirm every byte, member list, uncompressed member, external
attribute and executable bootstrap. API/cleanup ZIP SHA256 is
`58f2bbf364042ac209d4f9b3a4d6ca583da18914fb5851f0c166d34b9c757373`;
worker/controlled endpoint is
`82af7beeda4b619afa6831b342c693bb1fb3e8d08e5e6c0416ce253526f59098`.
The generated frontend artifact equals the adopted manifest. No ZIP was patched,
expected hash substituted or artifact adopted anew. The private collection proof
was produced and its ledger hash verified. No uploads occurred.

## Accounting, original clocks and preservation

**499 + 109 = 608/1,799**, leaving **1,191 arithmetic attempt headroom**.
All 109 attempts have durable outcomes: **101 HTTP 200 / eight HTTP 404**, zero
failed transports or unmatched attempts. The only write-labelled operation was
the approved credential renewal. There were **zero infrastructure/state writes,
runtime requests or later phases**. The cumulative **$1 procedural allowance**
is unchanged; billing remains unmeasured.

Collection completed **90.598714 seconds** after campaign start, with
**3209.401286 seconds** left at completion in the original 55-minute campaign.
Measured from the phase-start event, collection consumed **90.566592906 seconds** of its 240-second ceiling, leaving
**149.433407094 seconds** of approximate ceiling headroom at completion. Capture and
authentication counted within those clocks. The collection timer was cleared on
successful phase completion; the campaign deadline was not reset. No closed-stage
window began. These are completion-time observations, not current remaining time
or authority to continue.

All **seven campaigns and consumed claims**, every old checkpoint, new private
initial/renewed continuity evidence, ledgers, authorization, adopted/rollback bytes
and worktree registrations remain preserved. The collection is complete and
awaiting separate approval; its failure latch is not set. No subsequent action
was run to close the stage, deploy, verify, activate or finalize it.

Publication verification reconciled every ordinal 500–608 and outcome, private
bindings, original deadline, proof, reader comparisons, complete diff and exact
ZIP/frontend agreement. Local source/adoption verification and disclosure/format
checks passed. Tooling/application code is unchanged; the already-passed
format/lint/build/test and offline regression records remain the relevant code
verification and were not repeated for this evidence-only publication.

Executed historical command — **do not replay**:

```sh
python3 -B scripts/environment-deploy/driver.py start --campaign .build/project-environments-execution/release-499-f1 --authorization .build/project-environments-execution/authorization-from-499-fresh-1.json --fresh-checkpoint
```

The next decision is review of this completed collection. Close and every later
phase require separate explicit approval, current strict cache continuity,
cumulative accounting and the **same original campaign deadline**. An expired
window cannot be reset or reused. No recapture, restart, repairs, widening or
later-phase permission is implied by this report.
