# Old-release recovery: read-only inspection supports a guarded reopen proposal

**The stage remains closed: `workflow-migration`, version 16. No stage write ran.**
The single authorized inspection completed and supports proposing a return to the
unchanged old release at **`active`, version17**, preserving capabilities,
generation and accounting. [Machine findings](project-environments-recovery-inspect-868-r1.json)
and [all 87 projected attempts](project-environments-recovery-inspect-868-r1-attempts.jsonl).

Exact tooling pin: **e0aa49c9f0e5201f2c85bdad5ae00a40db599d9a**, including the
inspection implementation at **e208e24b65b326be9f1fb5fb506949db157c829c**.
Release source remains **44f1fb79d943501c95e06fcc1a6114111a7c1ea4**.

## Separate inspection authority and unchanged original campaign

The new single-use identity **`recovery-inspect-868-r1`** used an independent
**600-second, 200-attempt read-only inspection window**, starting at **868**.
Its hard ledger maximum is **1068**, within the unchanged global **1799** ceiling.
The private bound authorization records the exact source/tooling/manifest/scope,
original ledger and closure proof, same inventoried identity/account, explicit
fresh capture, at most one renewal, mandatory STS and **no stage writes**.
The claim is `recovery-window-from-868-r1.json` and cannot be reused.

The original `release-608-t1` ledger was read-only/immutable. All attempts609–868,
collection/closure proofs, original claim/authorization/checkpoint acceptance and
original deadlines reconcile exactly. Its count remains **868**, with **zero
pending outcomes and zero deploy attempts**. The original campaign and closed
window are now **expired on the same boot**. Their byte hashes and deadlines are
unchanged; this inspection did not resume, reset, replace or extend either.
Historical uncertainty in older unrelated authentication records is not rewritten.

The recovery ledger blocks every infrastructure/state write before admission;
only the one reviewed Sign-In renewal can be write-labelled. The proxy runs the
existing collect read policy, which independently rejects stage transactions.
The recovery CLI has only `check`, `template` and `start`; its Node entrypoint has
`inspect` and offline `plan`. **There is no reopen/write executor.** No new API,
resource, IAM, signing/framing or generic read grant was introduced.

Fresh capture occurred once under the existing exclusive-lock, ownership,
permissions, structure, exact identity/account and race checks. Historical grant
continuity remains **unproved**. Renewal at **869** and mandatory fresh host STS
at **870** returned HTTP200; validity/identity checks passed. Strict continuity
remained in force afterward, with no recapture or background/mid-phase renewal.

## Fresh inspection findings

| Check                       | Observed result                                                                                                             |
| --------------------------- | --------------------------------------------------------------------------------------------------------------------------- |
| Exact closed marker         | `workflow-migration`/16, same hash as verified closure                                                                      |
| Old capabilities/generation | Exactly preserved; saved requests schema 8, other existing capability versions 1, environment capability absent             |
| Accounting                  | Seven rows match the closed receipt; no changes                                                                             |
| Deployed packages           | All four AWS CodeSha256 values equal the exact retained old rollback ZIP hashes                                             |
| Configuration/IAM           | All four old configurations, role policies/trust, runtime, architecture, memory, timeout and Active/Successful status match |
| Routing/invoke policies     | Original 45 routes/integrations and old invoke policies match; no eight-route deployment                                    |
| Old frontend                | Accepted old manifest, object bytes, KVS entries and deployed distribution pass                                             |
| Inventory/quiescence        | 107 projected rows; zero active projects; eleven residual prefixes empty; queue depth zero                                  |
| New-format data             | None detected by the accepted projected-metadata kind/schema/environment/scalar guards                                      |
| SST state bookkeeping       | No current deploy lock; app object last modified **2026-10-21 15:28:52 UTC**                                                |
| Scoped SST history metadata | App 1, snapshot 21, eventlog 21, update 21, summary 0, lock 0; no observed post-collection mutation                         |

Code/package comparisons use AWS-reported package digests, independently decoded
and compared to the retained rollback ZIP SHA256 values. No code download, package
rebuild, new adoption or upload occurred during this inspection.

SST checks used HEAD for the exact app state object, GET for the exact lock, and
bounded ListObjectsV2 pages for six existing stage-scoped prefixes. Object/body
payloads were not collected except the bounded lock descriptor when present.
The cutoff was derived from the original campaign start and its retained, bound
UTC entry proof. State metadata newer than that cutoff, a live lock, pagination
exhaustion, marker/accounting/configuration drift or any new-format data stops the
inspection. Marker was read again after state checks.

These are scoped snapshot observations plus reconciliation of the controlled
ledger. They establish **no observed deployment/state changes**, not a complete
CloudTrail/history/deletion audit. They do not guarantee that state stays unchanged
after inspection. Fresh before-write checks are required for any later reopen.

## Exact proposed guarded transaction — no execution approval

The **complete exact request**, including all seven private accounting keys and
every expected value, is retained mode0600 at:

`.build/project-environments-execution/recovery-inspect-868-r1/proposed-reopen-transaction.json`

Canonical transaction SHA256:
`4bb06443f6c2d3eb578a2c15f7ced1ea1b2b7bd8ea003864d47e97610dec3f1f`.

The full proposal containing both exact before/after markers is retained privately:

`.build/project-environments-execution/recovery-inspect-868-r1/proposed-reopen.json`

Proposal SHA256:
`307a987a15b4e372b61bda5df4fbe12bba6b270f4a4c207470461d4286e01251`.

Raw generation and private accounting keys are intentionally not committed or
exposed in the public projection. The private files provide the concrete exact
payload for deliberate review; the public machine record binds their hashes.

The request contains **eight actions**, in one TransactWriteItems call:

1. One update of the exact owned stage marker, guarded by equality of **every
   before-marker field**, plus absence of `projectEnvironmentsSchemaVersion`.
   Its update expression is exactly **`SET #s = :active, #v = :next`**.
2. Seven existing accounting **ConditionChecks**, using the same reviewed full
   kind/schema/counter-version/count/month and absent-field guards. No quota or
   accounting row is initialized, decremented or rewritten.

| Marker                 | Expected before                  | Proposed after |
| ---------------------- | -------------------------------- | -------------- |
| State                  | `workflow-migration`             | `active`       |
| Version                | **16**                           | **17**         |
| Saved requests schema  | 8                                | 8              |
| Existing capabilities  | 1                                | 1              |
| Environment capability | absent                           | absent         |
| Recovery generation    | exact current private value      | identical      |
| Accounting             | exact seven-row private preimage | unchanged      |

Before-marker SHA256:
`353cd6632e9351399bf4e62d853f500ec02e4fa2b5ba1fc7148b37965818a708`.
After-marker SHA256:
`c89cdb1316d80dc2d497ff13a13080e3a55ad0fa85cc4f6b861781aad5489e95`.

Only state/version would change. Version **increments** rather than returning to 15. No environment activation, new-generation creation, capability upgrade,
deployment, code/frontend rollback or data migration is proposed. A later writer
must bind the exact request and before marker, recheck fresh evidence and identity,
account for its physical attempt, permit one guarded transaction and verify marker
and accounting afterward. Unknown/uncertain results stop with no retry, automatic
reopen, rollback, repair or clock reset. Such a writer and its live permission
were **not** added or authorized by this task.

## Offline verification, inspection accounting and preservation

**178 execution/frontend/recovery tests pass**, including 14 Python recovery tests
and 13 Node planner tests. Coverage includes actual driver→locked capture→pinned SDK
renewal/STS→synthetic collector→offline proposal; same account/identity and strict
continuity; short/failed credentials; replay; 600-second expiry; 200-attempt cap;
read-only meter; real SDK serialization; mutation denial; new-format/marker/
accounting/code/IAM/frontend/routing/state drift; and exact before/after proposal.
The original 17 shared isolated transport/context/preview/artifact scenarios pass.
Repository format/lint/tests/build/Go verification and Ruff 33 files pass with
networking/downloads disabled for offline checks. No fixture proves live service
completion time or comprehensive cloud audit history.

Before the one live inspection, a regression caught that successful HEAD on an
existing lock has no JSON body, while the existing response gate validates lock
JSON. The helper now uses the **already-permitted exact-key GET**, with absent,
empty and live-lock regressions. No policy widening was needed. An older test's
roundoff-prone deadline subtraction assertion was changed to compare the exact
`deadline == started + duration` construction; production clock bounds were not
relaxed.

The inspection completed in **71.167982 seconds**, leaving **528.832018 seconds at
completion** of its 600-second inspection-only window. **868 + 87 = 955/1,799**;
**844** cumulative attempts remain. Inspection attempts were **85 HTTP200 / two
HTTP404**, all with durable outcomes; no failed transport or unmatched attempt.
Only the approved renewal is write-labelled. **Zero stage/infrastructure writes,
uploads, deployments, activations, runtime requests, reopen or rollback.**
The inspection's unused 113-attempt capacity is not permission to run it again or
perform a write. The cumulative **$1 procedural allowance** is unchanged; billing
is unmeasured.

All original ledger/claim/checkpoint/proof/registration hashes and adopted/rollback
bytes remain unchanged. New inspection authorization, consumed claim, captured
checkpoint, ledger, private outputs/proposal and proof are preserved. No original
evidence was deleted, rewritten or silently reclassified.

Historical once-only inspection command — **do not replay**:

```sh
python3 -B scripts/environment-deploy/recovery.py start --authorization /home/king/Neumont/Kurier/.build/project-environments-execution/recovery-authorization-from-868-r1.json
```

**Stop before any stage write.** The next decision is deliberate review of the
exact private proposed transaction and this evidence, then separate explicit
approval of a bounded, reviewed one-shot recovery write with fresh guards. The
original expired campaign and deadlines remain preserved; neither they nor the
consumed read-only inspection become a reopen execution window.
