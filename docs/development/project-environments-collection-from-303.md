# Authorized collection from 303 — reader snapshot complete, preview held

The exact authorized start command ran once. Reader collection completed and
passed its historical configuration/IAM/routing and metadata guards. Guarded
preview then stopped at the request policy gate before forwarding the rejected
request. **The collection phase did not complete:** no complete resource diff,
four-archive comparison or collection proof was produced. No retry, policy
widening, repair, deadline reset or later phase occurred.

Actual usage is **390/1,799**: 303 carried forward plus **87 physical attempts**,
with zero unmatched outcomes. The remaining **1,409** attempts and recorded time
do not authorize continuation. The cumulative **$1 procedural allowance** is
unchanged; billing remains unmeasured. All forwarded operations were reads except
the one explicitly authorized existing-session renewal. No infrastructure/state
write, runtime request, stage transition, identity/project creation or rollback ran.

## Exact execution, limits and accounting

The clean original workspace selected tooling commit
`fb0cd0e7f92751a6991a12a2af6e38827e7645ed`. Release source stayed the clean detached
worktree at `44f1fb79d943501c95e06fcc1a6114111a7c1ea4`. The documented local
check passed with zero AWS calls, verifying retained source/artifact/SDK pins and
both earlier stopped campaign proofs. No adopted archive or dependency was rebuilt.

The private template was checked against the exact tooling/manifest/scope,
303 baseline, 1,799 ceiling, 3,300-second original window and bounded credential
policy. Only `authorized` was changed to true. The authorization was created
exclusively with mode 0600; the new campaign and claim were absent beforehand.
The accepted scope was initial collection only, with a 240-second phase ceiling,
at most one existing-session renewal if needed, mandatory STS, no background or
mid-phase renewal and no later phase approval.

Historical once-only command — **do not rerun**:

```sh
python3 -B scripts/environment-deploy/driver.py start --campaign .build/project-environments-execution/release-from-303 --authorization .build/project-environments-execution/authorization-from-303.json
```

| Observation                     | Recorded result                                       |
| ------------------------------- | ----------------------------------------------------- |
| Renewal                         | Ordinal 304, HTTP 200; one attempt                    |
| Mandatory host STS              | Ordinal 305, HTTP 200; same account/non-root identity |
| All STS, including descendants  | Nine attempts total, all HTTP 200                     |
| New attempts                    | 87 contiguous ordinals, 304–390                       |
| HTTP outcomes                   | 84 HTTP 200; three HTTP 404                           |
| Failed transport / unmatched    | 0 / 0                                                 |
| Policy-gate stop                | 51.410416 seconds from original campaign start        |
| Driver stopped                  | 51.638459 seconds from original campaign start        |
| Campaign time remaining at stop | 3,248.361541 seconds                                  |
| Phase time remaining at stop    | 188.395764 seconds                                    |
| Closed-stage window             | Never started                                         |

Every physical attempt was durably admitted before transport and has one
recorded outcome. The three 404 reads consumed attempts and authorized no repair.
The host credential-validity guard passed. Both child proxies verified identity.
Only one renewal occurred, before reader work; no background/mid-phase renewal
ran. Neither deadline expired or was reset. Earlier historical uncertain refresh
outcomes remain unchanged.

## Fresh reader snapshot and comparisons

The stage was **active, version 15**, with savedRequests8; protectedSecrets,
localAgentExecutions, workflowAssertions, cloudExecutions, executionInputs,
evidencePinning, sequentialWorkflows and mcpReadOnly all at 1. The environment
capability was absent. Recovery generation matched the historical baseline and
the before/after collection marker matched. These are snapshot observations,
not a guarantee of current state after collection.

The metadata collector inspected **107 projected rows**, with **zero active
projects**, seven accounting rows and **11 residual rows**. All inventory,
recognized-schema, settled-work and accounting guards passed; all 11 checked
residual evidence prefixes were empty. The observed main queue's visible,
not-visible and delayed counters were all zero. Stage quota was version **75**,
month **2026-10**, count **32**. No counters or generations were changed. No
separate DLQ observation or ongoing traffic-fence proof is claimed.

| Reader                      | Code/configuration/IAM vs historical baseline | State/update      |
| --------------------------- | --------------------------------------------- | ----------------- |
| UsersProjectsApi            | All matched                                   | Active/Successful |
| EmptyProjectCleanup         | All matched                                   | Active/Successful |
| CloudExecutionWorker        | All matched                                   | Active/Successful |
| ExecutionControlledEndpoint | All matched                                   | Active/Successful |

Runtime, architecture, memory and timeout also matched. No function had reserved
or provisioned concurrency. Free plan status was ACTIVE; account and unreserved
concurrency were both 10. The worker queue mapping was Enabled, batch 1, maximum
concurrency 2, with no provisioned pollers. Cleanup remained ENABLED at
`rate(1 minute)` with the expected target, and API throttle controls passed.
All **45 routes and 45 integrations**, together with the existing invoke
policies, matched the historical routing baseline.

**Deployed frontend readiness against the adopted release was false.** This
comparison targets the adopted release and does not establish drift from a
separately verified historical frontend baseline. It is not a successful new
frontend deployment/readiness observation.

## Incomplete preview and artifact agreement

The preview proxy latched `operation-resource-byte-or-budget-held` before
forwarding a request. The retained record does not identify that request's exact
operation or rejected field; request/response flow logging was disabled, the
meter error log is empty, and diff stdout is empty. No specific endpoint,
resource, byte mismatch or policy defect is inferred from the broad reason.
Call/time limits were still available. No diagnostic rerun or repair occurred.

- Complete resource diff: **unavailable**; no agreement with review 8 established.
- Fresh generated ZIPs: **0 of 4**; archive agreement is unestablished.
- Generated frontend metadata marker: **matches the adopted manifest**.
- Confirmed uploads: **0**; no infrastructure/state writes forwarded.
- Collection proof: **absent**; campaign is stopped with no later phase begun.

Historical review-8 diff and artifact checks remain historical evidence. They
are not substituted for the missing fresh diff/ZIP proof. Reader snapshot success
does not authorize closure or establish deployment readiness.

## Publication, preservation and next decision

[Machine accounting/stop record](project-environments-collection-from-303.json),
[sanitized reader findings](project-environments-collection-from-303-readers.json)
and [projected attempts](project-environments-collection-from-303-attempts.jsonl)
publish only reviewed counters, booleans, logical resource labels and scoped
metadata. No raw environments, credentials, keys, private captures or their hashes,
identities, row keys, payloads, headers or raw diagnostics are published.

The private authorization, ledger, consumed `window-from-303.json`, reader output,
preview work and proxy records remain retained under ignored `.build`. Both
earlier stopped campaigns/claims, adopted ZIPs, rollback bytes, private evidence
and worktree registrations remain intact. No cleanup, archive replacement or
worktree pruning occurred. The review branch was restored for publication.
Publication validation reconciles the attempt projection, outcomes, authorization,
claim, timing, reader guards and artifact conclusions and checks formatting.
Unchanged application/tooling builds and historical investigations were not repeated.

The next decision is a targeted **local review of the preview gate denial**
without widening policy or replaying this stopped campaign. Any future live
campaign needs separately reviewed tooling/accounting from **390** and explicit
authorization. Preserve all three consumed claims. No restart, renewal, repair,
closure, deployment, verification, activation or final phase is authorized here.
