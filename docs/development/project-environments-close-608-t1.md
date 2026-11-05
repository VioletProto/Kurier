# release-608-t1: stage closure verified

The separately authorized **close phase ran once**, from clean tooling
**4de31ab9a9c791a1fc224bbce8a7fe79220519ba**, unchanged release source
**44f1fb79d943501c95e06fcc1a6114111a7c1ea4** and the successful
[608 collection](project-environments-collection-608-t1.md). It exited **0** and
stopped after verified closure. **No deployment or later phase ran.**
[Machine receipt and clocks](project-environments-close-608-t1.json) ·
[all 152 projected close attempts](project-environments-close-608-t1-attempts.jsonl).

## Reviewed entry and exact transaction

A read-only local ledger check first confirmed the original campaign was on the
same boot, unexpired, with **2697.254178 seconds remaining**, above the required
**1,800 seconds**. Local source, adopted/rollback, SDK/model/packager/tool and parent
checks passed. The exact clean tooling was selected; no source or policy changed.

The private mode-0600 close approval was created exclusively from `prepare --phase
close`. Review bound it to the collection proof, original execution authorization,
exact source/tooling and adopted manifest. The offline plan was exercised before
authorization: precisely **eight actions**, one full-marker guarded update and
**seven accounting ConditionChecks**, with no accounting writes. The complete
private planned transaction matches the context actually enforced by the proxy.
Only the approval's `authorized` flag changed after the user's explicit approval.

An immediate second local guard, recorded privately and projected in the machine
receipt, again required **at least 1,800 seconds remaining** before invoking the
phase. It also required usage716, nextPhase close, no active/closed phase, matching
boot and no failure latch. No clock was reset or campaign replaced.

One metered existing-session renewal at **717** and mandatory fresh host STS at
**718** returned **HTTP200**. Exact identity/account, actual required credential
validity and strict cache continuity passed. No recapture, second renewal,
background/mid-phase renewal or alternate identity occurred.

Live read-only checks covered the original marker, accepted platform/configuration/
IAM baseline, metadata inventory/accounting, empty queue and accepted old frontend.
The marker, inventory/accounting, platform, queue and frontend were rechecked before
the write. The sole exact guarded **TransactWriteItems at ordinal853** returned
**HTTP200**, with a recorded outcome. There was no write retry or compensation.
The marker and accounting were read back after the transaction; all proof and
receipt hashes agree with the independently reviewed expected next marker.

## Verified resulting marker and receipt

| Field                        | Before                 | Verified after       |
| ---------------------------- | ---------------------- | -------------------- |
| State                        | `active`               | `workflow-migration` |
| Version                      | 15                     | **16**               |
| Saved requests schema        | 8                      | 8                    |
| Existing capability versions | 1                      | 1, unchanged         |
| Environment capability       | absent                 | absent               |
| Recovery generation          | private existing value | exactly preserved    |
| Accounting                   | seven guarded rows     | exactly preserved    |

Only **state/version** changed. Full raw marker and durable closure proof remain
private. The machine record publishes the resulting marker projection, omitting
row keys and the generation value while proving preservation. Marker SHA256:
`353cd6632e9351399bf4e62d853f500ec02e4fa2b5ba1fc7148b37965818a708`.

The original engine closure receipt is:

```json
{
  "closedAt": 12366340,
  "deadline": 14166340,
  "markerSha256": "353cd6632e9351399bf4e62d853f500ec02e4fa2b5ba1fc7148b37965818a708"
}
```

These engine times are monotonic milliseconds, not UTC timestamps. The host's
**fixed closed-stage deadline started before authentication**, earlier than this
child engine receipt. Both records are preserved unchanged. The **earlier host
closed deadline remains the effective limit** for any later phase; the later
engine receipt never extends it. Activation additionally requires the receipt,
but was not authorized or run.

Accepted **old-frontend readiness passed**. `backendReadersReady` and `readersReady`
for the adopted deployment are **false**, as expected before deployment; closure
preserves the old baseline rather than claiming the new backend is deployed.
No capabilities were activated and no runtime behavior was tested.

## Accounting and original deadlines

**716 + 152 = 868/1,799**, leaving **931 arithmetic attempt headroom**.
All 152 attempts returned **HTTP200** with durable outcomes; no failed transport
or unmatched attempt. The two write-labelled attempts were the approved credential
renewal and the **single stage-close transaction**. There was one marker update
and no other infrastructure/state mutation. The cumulative **$1 procedural
allowance** is unchanged; billing remains unmeasured.

Close completed **89.123204 seconds** after its phase-start event, within the
**240-second** ceiling. At close completion the unchanged original campaign had
**2402.630301 seconds** remaining and the fixed host closed window had
**1710.876759 seconds** remaining. The campaign remains bounded to its original
55 minutes; the closed stage to its original once-started 30 minutes. Renewal,
read checks, publication and review do not reset or pause either clock.

At the later timestamped local publication check in the machine record, those
remainders were approximately **2250.087 / 1558.333 seconds**. These are historical
observations tied to that check, not current remaining time or continuation
permission. No subsequent AWS call was made to refresh them; local monotonic
inspection is sufficient for checking expiry.

## Stop and preservation

NextPhase is deploy, activePhase is null and the failure latch is unset. Execution
stopped after the close proof was recorded. No deploy, verify, activate, final,
runtime request, automatic reopen, repair, rollback or write retry occurred.
The bound approval, offline review plan, actual context, private logs, original
collection proof, checkpoint records, closure proof, all historical claims/ledgers
and adopted/rollback bytes are preserved. The PR branch was restored for publication.

Publication assertions reconcile every ordinal717–868, original entry guard,
authorization/collection/approval/transaction/proof/marker bindings, eight exact
actions, accounting preservation and both original clock bounds. Local source/
artifact, JSON, disclosure and formatting checks pass. Tooling/application code is
unchanged; existing build/lint/test proofs remain applicable.

**Next decision:** review the receipt and check current remaining original clocks
locally. Deployment requires separate explicit approval under the same campaign,
strict continuity, cumulative accounting and fixed host closed deadline. Remaining
time/calls do not authorize it. An expired limit or uncertain result must stop;
never reset the clocks or automatically reopen/replay a write.
