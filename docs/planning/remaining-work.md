# Remaining-work plan

Planning snapshot updated: **2026-10-27**. Davian owns all work. This is a delivery
forecast, separate from the proposal's preserved ten-week, 200-hour planned
timeline. Estimates include implementation, documentation, review and relevant
local verification; they are remaining effort, not recorded hours worked.

## Calendar and capacity

Davian confirmed **Week 4** on 2026-10-26 and requested completion **five more
weeks from the current week, on that week's Friday**. The date **Friday, December 4, 2026 (W9)** was inferred from that request;
it was not explicitly confirmed by Davian. There is no defined institutional
end date or submission deadline.

- Current week: **W4, October 26–November 1**.
- Remaining: **5 weeks after this week; 6 weekly slots including W4**, finishing
  during W9. W10 remains in the proposal's original plan but is not used here.
- Remaining capacity: **116 hours** as of Tuesday, October 27, before today's
  work: **16 hours this week** (4 hours today plus 3 other days at 4 hours each)
  plus **100 hours over the following 5 weeks** (20 hours/week). Monday is not
  counted as available working time.
- Capacity allocation below: **112–116 hours**, including **8 hours contingency**;
  **104–108 hours** for planned work. No actual time ledger or unused past-week
  capacity is inferred.
- The initial task estimate was **112–140 hours**. The accelerated target budgets
  the lower part of that range; the upper case exceeds capacity by **24 hours**.
  This is a schedule risk, not an estimate eliminated by changing the date.
- Review remaining estimates and actual available hours each Friday. If the
  upper case becomes likely, surface the effort/target tradeoff immediately;
  no scope reduction or deadline extension is assumed or authorized.

## Current state and dependencies

The [board](https://trello.com/b/dRYhnjS6/kurier-capstone), accepted
[architecture index](../architecture/README.md), native
[API contracts](https://docs.google.com/document/d/1KSfRYOb2UxmrIl8VoFjc3YP38-PMu7k_fM9wrVkEenM/edit)
and slice records were read for the original October 26 snapshot. PR merge
status was refreshed from GitHub on October 27. Trello's configured Doing WIP
limit is **3**; environments was the only Doing card in that board snapshot. This planning task does
not move cards, approve contracts, merge PRs or start product work.

| Area                                                           | State at snapshot                                                                                                                                                                               | Remaining outcome / dependency                                                                                                                                                    |
| -------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Ownership, requests, protected inputs, cloud evidence, pinning | Accepted development slices                                                                                                                                                                     | Preserve their tested behavior in final regression; no reimplementation planned.                                                                                                  |
| Sequential workflows and assertions                            | Accepted and merged; bounded genuine evidence retained                                                                                                                                          | Keep operational gaps distinct; assertion teardown/meter correction remains open.                                                                                                 |
| Local agent, read-only MCP and frontend hosting                | Accepted and merged development/demo slices                                                                                                                                                     | Final demonstration and selected coverage; no claim of full operational certification.                                                                                            |
| [Environments](https://trello.com/c/K2e3Nv2f)                  | Accepted contract; [PR #15](https://github.com/VioletProto/Kurier/pull/15) and [PR #16](https://github.com/VioletProto/Kurier/pull/16) merged on October 26; implementation locally/CI verified | Deployment/activation and runtime validation remain held for separate review. Merged source is not deployed runtime evidence.                                                     |
| [OpenAPI import and validation](https://trello.com/c/zUsI4e6l) | Unimplemented; [libraries selected](../spikes/openapi-library-selection.md), production design unaccepted                                                                                       | Accept dialect/features, generation, safe examples/sources, DTOs, evidence/result version, targets and sanitization; implement import, generation and all four violation classes. |
| [MCP contract results](https://trello.com/c/jQR3w1Zt)          | Unimplemented                                                                                                                                                                                   | Depends on stored OpenAPI results and accepted tool/DTO/compatibility extension; preserve audit, scope, omissions and secret containment.                                         |
| [Final contract reconciliation](https://trello.com/c/b5LdvuTo) | Deferred; full v0.1 not accepted                                                                                                                                                                | Reconcile superseded generic passages with accepted slices and new scope; settle remaining interface choices and executable HTTP specification.                                   |

## Weekly outcomes and effort

The allocation changes implementation order from the proposal: workflows,
local-agent, MCP foundations and hosting were delivered before OpenAPI. The
original W5 OpenAPI milestone remains planned history; this forecast schedules
OpenAPI import/validation in W5–W6, the dependent MCP extension in W7, feature
freeze/regression in W8 and presentation-ready completion by **W9 Friday**.
Moving the proposal's W10 finish activities into W9 is a substantive accelerated
forecast revision inferred from the request on 2026-10-26; the exact finish date
was not confirmed. MVP scope is unchanged. All rows remain
**planned**, not completed; this is not faculty approval.

| Week / dates                                        | Allocated effort                                                                        | Weekly outcome and completion evidence                                                                                                                                                                                                                                                                                                                                                                 |
| --------------------------------------------------- | --------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **W4 — current**, Oct 26–Nov 1                      | **16 h**: environments rollout preparation 6; OpenAPI contract review 8; coordination 2 | Environment PRs #15/#16 are merged; remaining work prepares a reviewable rollout packet with deployment and runtime-validation holds explicit. OpenAPI decisions form a reviewable contract covering proposal requirements and compatibility. Unaccepted choices stay open; no implementation inferred from library selection.                                                                         |
| **W5**, Nov 2–8                                     | **20 h**: import/generation 16; tests/docs/review 4                                     | After contract acceptance, bounded JSON/YAML import/status and supported-operation request generation work locally. Demonstrate path/query/header parameters, optional JSON bodies and safe examples; invalid/unsupported/ref/oversize/partial-publication cases fail explicitly. Preserve ownership, source sanitation and deletion.                                                                  |
| **W6**, Nov 9–15                                    | **20 h**: validation/evidence/UI 16; tests/docs/review 4                                | Response validation persists immutable sanitized results for undocumented status, missing required properties, primitive type mismatch and required headers. Browser shows results; frozen operation/schema identity, failure semantics, retention/pins and deletion pass local tests for accepted targets. Start the final schema/contract reconciliation alongside this work.                        |
| **W7**, Nov 16–22                                   | **20 h**: MCP results 12; integration/rollout preparation and tooling 8                 | Accepted MCP extension reads stored pass/fail/not-run/unsupported/unavailable states without fetching schemas or executing validation. Scope/revocation/audit/size/redaction tests pass. Environment/OpenAPI release and compatibility packets identify live gates. Guard tooling has deterministic stop/cancellation/in-flight/cleanup evidence before reuse.                                         |
| **W8**, Nov 23–29                                   | **20 h**: reviewed rollout/genuine validation 12; regression/usability 8                | If separately authorized, environment/OpenAPI/MCP development rollout and bounded genuine browser/agent checks produce evidence and cleanup records. Otherwise record held gates and the MVP gap. Freeze after feature-completion evidence; browser-to-backend and ownership/redaction/history/workflow/agent/MCP regressions pass. Exercise critical loading/error/conflict and keyboard/focus paths. |
| **W9 — inferred finish Friday Dec 4**, Nov 30–Dec 4 | **16–20 h**: final contracts/docs 4–6; demonstration/materials 4–6; contingency 8       | Resolve blocking defects within reserve, finalize contracts/executable schemas and setup/security/recovery docs, prepare sanitized demo data, rehearse, record a backup demonstration and finish presentation/submission materials by Friday. Record actual deployment checks and remaining limitations. No institutional submission deadline is claimed.                                              |
| **Total**                                           | **112–116 h**                                                                           | **104–108 h planned work plus 8 h contingency**; W4 is limited to 16 h, later weeks to 20 h. Upper-case effort risk remains 140 h.                                                                                                                                                                                                                                                                     |

## Completion checks and risks

- [x] Confirm current W4 and remaining capacity: 16 hours this week plus
      5 weeks at 20 hours/week, totaling 116 hours.
- [ ] Confirm exact finish date; December 4 is inferred, and no institutional
      submission date is defined.
- [x] Environment contract PR #15 and implementation PR #16 merged.
- [ ] Environment deployment/activation and genuine runtime validation; held
      for separate review.
- [ ] Track effort against the 116-hour capacity and revisit the 140-hour
      upper-case risk each Friday.
- [ ] OpenAPI contract accepted; import, generated requests, four validation
      classes, immutable safe results and accepted target coverage delivered.
- [ ] MCP retrieves stored contract results with explicit unavailable/not-run
      behavior and preserved ownership, audit and secret filtering.
- [ ] Guard tooling corrected and checked before reuse in a live campaign.
- [ ] MVP regression, contract/schema reconciliation, demonstration, backup
      recording and final submission completed with evidence.

The principal schedule risks are unaccepted OpenAPI semantics, review turnaround,
compatibility of immutable evidence/readers and genuine runtime authorization.
Reassess weekly using accepted scope, open findings, actual effort and remaining
capacity. If a dependency slips, revise this forecast explicitly and raise a
scope/deadline decision; keep affected checklist items open. The proposal's MVP
does not become smaller because a restricted prototype works.

General-public signup/SES, custom-domain readiness, comprehensive cross-engine
accessibility, real elapsed expiry/retention, PITR/fault/IAM/performance exercises
and measured billing remain tracked operational follow-ups, with status and
authorization separate from accepted development/demo functionality. Select
presentation-critical cases during W8; do not claim unperformed cases pass.
Historical publication attribution cannot be reconstructed by replaying HTTP.
Stretch features remain outside this remaining-MVP estimate.

Detailed historical budgets, observed counters, deployment and verification
records belong in [internal operational records](../development/proposal-operational-records.md)
and the linked slice guides. This plan and its future work estimates authorize
no application changes, AWS reads/mutations, deployment or activation.
