# Metered replacement rollout stopped before closure

Davian accepted `e702f434eab1837cecc5ccfab100b861067b39e5` and authorized
metering implementation/local verification, followed conditionally by the pinned
replacement rollout and normal cleanup. Metering passed. Execution then stopped
on **HTTP 503 `service_unavailable` from the first legacy-A status GET**.
The workflow was admitted once; it was never replayed. No maintenance closure,
deployment, activation, automatic reopening or stage-state write occurred.
No close or activation approval record was issued.

The [sanitized receipt](workflow-assertions-replacement-rollout.json),
[complete attempt ledger](workflow-assertions-replacement-attempts.jsonl) and
[fresh equivalent preview](workflow-assertions-replacement-preview.json) retain
the actual outcome. Earlier packets remain historical review records.

## Metering prerequisite and actual usage

Tooling commits `6066bc8` and `56e4cb2` implement the
[isolated operator meter](workflow-assertions-attempt-metering.md).
Ten budget tests pass, including exact 500 API / 240 poll / 1,500 AWS ceilings.
Controlled-origin tests verify nine actual CLI/Go SDK/Node SDK retry/page
attempts, three native-fetch/Chromium read attempts including HTTP/2, isolated
localhost RPC, external `ENETUNREACH`, and one lost-acknowledgment write with its
second attempt blocked. These tests make zero AWS/live-target calls.
The actual local cleanup regression passes at the maximum replacement shape,
including reserve: 26 admitted steps / 22 fixture sends, with the two project
drains finishing in 19 and 15 scheduler passes. Full repository verification,
Python format/lint and all six release-pin checks pass.

Every live AWS/application operator process ran through the same meter. Its
ledger contains **597 attempts and 597 outcomes**; no counter was reset between
failed preflights, seed, cleanup authentication or final verification. Every
write received a success response. One AWS read timed out and remains counted.
The status GET's 503 also remains counted. There was no uncertain write.

| Quantity                                  | Replacement observed |    Reviewed replacement ceiling |
| ----------------------------------------- | -------------------: | ------------------------------: |
| Application API attempts                  |                   23 |                             500 |
| Status polls, included above              |                    2 | 240, at least two seconds apart |
| AWS attempts charged to read allowance    |                  574 |                           1,500 |
| Cloud execution/verification window       |    1,304.656 seconds |                   7,200 seconds |
| Normal cleanup                            |      244.987 seconds |          1,200 seconds reserved |
| Additional disposable identities/projects |      2 each, cleaned |                          2 each |
| New admitted steps / audited target sends |                2 / 2 |   A–L 24 steps plus two reserve |

The 574 AWS charges include 551 read-classified and 23 write/authentication
attempts; charging **all** AWS attempts conservatively consumes the read ceiling.
Live AWS traffic includes 67 HTTP/2 and 507 HTTP/1.1 attempts. Internal application
AWS calls are not measured operator reads. There were zero hosted-browser/static
requests in this stopped attempt. The local browser test is not hosted runtime
validation.

Known cumulative consumption is **4 admitted steps / 4 audited sends**, and
**four identities / four projects created and cleaned** across both attempts.
Known cumulative application API calls/polls are 48/4. Historical total AWS
reads and billing remain **unknown**; no coordinator operation count is used as
an AWS-read total. Replacement actual billing is also unverified. The additional
$1 is a procedural allowance, not an enforced spending cap or a verified bill.
Unused allowance does not authorize another attempt or additional identities.

## Independently verified runtime and cleanup

After the coordinator stopped, read-only DynamoDB and UUID log auditing proved
that A completed both steps, with exactly two distinct gateway request IDs and
correct controlled-consumer authorization. Its run is
`37d1a805-d543-4f2e-a2cd-3032320cf2ba` in project
`89e23639-6d02-4794-99ac-2760374ecc48`.

Both outboxes were `published`, their matching terminal jobs/anchors and immutable
manifests agreed, and both uploads were `published` with `writeSettled:true`.
The accepted corrected pure inspector passed with zero writes while evidence
was retained. Protected rows were absent. Before normal deletion, the two S3
objects matched their manifest lengths and SHA-256 values, remained schema 1
without assertions, and contained no synthetic binding value:

| Execution                              | Stored bytes | SHA-256                                                            |
| -------------------------------------- | -----------: | ------------------------------------------------------------------ |
| `50c2f522-857c-4655-9abc-744020bb91a8` |        1,322 | `7993f297221b227dec107c109106988bef35da9a5fbb76c286cc29d5d1b4d6c2` |
| `aacd438f-3e88-4d47-9ca8-b1668eca076f` |        1,205 | `67bd7a03d621ed97d5712f6be8a6308f62b7eb6aa35e592dcdba45016be13a54` |

Normal deletion covered that project and
`454bed8d-6b9c-4d03-808d-7504593fe453`. Owner reads immediately returned 404.
The existing minute scheduler completed both deletions: each project partition
contains only its deleted tombstone, both Protected/S3 prefixes are empty, and
both permanent residual registrations remain. No manual maintenance invocation,
row repair or object drain was used. Foreign-owner and anonymous reads returned
404/401 before deletion. No MCP credential was created.

Both replacement provider identities were globally signed out, deleted and
independently confirmed absent. Normal retained application user metadata and
usage rows remain. The stopped coordinator had no cleanup RPC; a distinct
cleanup-only process rotated passwords of **those same two disposable users**
in memory to authenticate the normal deletion API. It could not admit work and
did not resume the rollout or export the stopped process's credentials. Both
credential-bearing processes exited; temporary proxy CA material and the local
test container/network were removed. Offline JWT verification may still accept
an issued access token until its original expiry.

Stage monthly admission usage increased **4 → 6** and was not refunded. The new
owner's daily count remains 2, the earlier attempt's owner count remains 2, and
storage reservations/pin accounting returned to baseline. Generation, quotas,
concurrency, IAM, package/configuration snapshots and the normal maintenance
schedule match the accepted baseline. The queue is empty. All four Lambda log
groups were scanned with complete bounded pagination over the recorded runtime
window; 161 events contained no synthetic binding token. Raw messages were not
retained.

Final accepted CLI inspection is read-only, reports zero active projects and
zero writes, and retains the original marker:
`active`, version 13, savedRequests 7, assertions capability absent,
generation `dev-api-initial-v1`.
Marker SHA-256 is
`dca180063fb02b0c7bf7b1a5e582ae4b5b19109cadaae8eea61983412a09d10d`.
Its `readersReady:false` correctly records that the proposed release was never
deployed; this is not an activation success claim.

## Release and artifact identity

The final complete fresh preview matches the accepted **11** resource changes,
all exact package/frontend bytes, and zero IAM/infrastructure changes. Existing
bootstrap v5, passphrase metadata and all 88 current SST state-object metadata
entries remained unchanged. Preserved old rollback ZIPs also passed their hashes.

| Artifact                                   | SHA-256 / release ID                                               |
| ------------------------------------------ | ------------------------------------------------------------------ |
| Unchanged accepted manifest                | `920667777eb69cabfb7800910451062a18e6ff06ce22ad4fb543f96c78321dd6` |
| Pinned API / cleanup ZIPs, not deployed    | `f4be7af179049c70f479a45dd26e4ae24a19c3a9e8016a865318fa7a96965c32` |
| Pinned worker / fixture ZIPs, not deployed | `15e7e6db438de3c19fe962c95be0d3c9d52d17f81b21a0b58cd4603974021379` |
| Pinned frontend, not deployed              | `bd511e3f9979d2ef7dd6b73d183dcf7620a423b62659ac89c64e4a1a08981cde` |
| Still-live API / cleanup ZIPs              | `cf65f8badc4145962fee4cdc79ab4d4cb1f1829d302f41ce0f30833ae19a326e` |
| Still-live worker / fixture ZIPs           | `2fb39ef077362119c0d35206e935f291f4085c8017032f5cc24800a43a170900` |
| Still-live frontend                        | `acb951896da16807815751c0509b1f25647656d8a0963932c7fc1f3a8ccac67e` |
| Fresh equivalent preview                   | `101ef3c5dd7def146d0d644f949fbc05a4e1eff36bde2da26a638508f230f6aa` |
| Sanitized execution receipt                | `d35f18ff7a322f30b41249e3cb9368eb049d7c7565006c34a32454be3194bd5b` |
| Exact attempt ledger                       | `e3937c7f995b58e041bb42768bf2358ab89a436f82fdb56fec55b2eaf7ed8f9a` |

Application source remains `68d9f218225c7092a1e4fca0b66a21421d862a22`.
No application implementation or release manifest changed in this task.

## Observations, remaining gaps and hold

Operator/process observations are listed separately in the receipt: early
read-only preview failures exposed profile selection and local RPC issues;
local regressions then verified the corrections. A later IAM read timed out;
one diagnostic used a mistyped role name, followed by a successful read of the
actual inventoried role. All attempts remain charged. The subsequent complete
preview passed. These observations are not substituted for durable AWS proofs.
No external operator report is used as independent validation.

The unexpected status GET's **root cause is not independently established**.
Do not claim a worker, assertion, IAM or transaction-conflict defect from the
single response. The durable run's later success does not cancel the required
stop. A must remain historical after its cleanup; an empty-project inspection
cannot silently replace the retained legacy seed for a resumed rollout.

Closure, deployment, reader/frontend upgrade verification, activation and B–L
remain unperformed. Hosted numeric round trips, schema 2 assertions, frozen
reruns, MCP projection and pin/unpin checks therefore have no new runtime result.
Broad live fault injection and elapsed-retention/PITR remain excluded.
Any further rollout needs review of the 503 and a deliberate bounded recovery
allowance accounting for four consumed admissions/sends and all four cleaned
identities/projects. No further creation, stage write or target request follows
automatically from this stopped attempt.
