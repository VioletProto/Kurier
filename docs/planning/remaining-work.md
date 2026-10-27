# Remaining-work plan

Planning snapshot updated: **2026-10-27**. Davian owns all work. This is a delivery
workload plan, separate from the proposal's preserved ten-week, 200-hour planned
timeline. Estimates below are provisional active effort projected by Davian from his
past implementations and observations of how long that work took. They include
implementation, documentation, review and relevant verification, and remain
subject to revision. They are estimates, not promises of task duration.

## Calendar and required effort

Davian confirmed **Week 4** on 2026-10-26 and requested completion **five more
weeks from the current week, on that week's Friday**. The date **Friday, December 4, 2026 (W9)** was inferred from that request;
it was not explicitly confirmed by Davian. There is no defined institutional
end date or submission deadline.

- Current week: **W4, October 26–November 1**.
- Remaining: **5 weeks after this week; 6 weekly slots including W4**, finishing
  during W9. W10 remains in the proposal's original plan but is not used here.
- **Remaining required effort under the current assumptions: 116 hours.** This
  is the working-time commitment for this planning period: **16 hours this
  week** (4 hours Tuesday, October 27 plus 3 other days at 4 hours each), then
  **100 hours over the following 5 weeks** at **20 hours/week**. It is not merely
  available capacity, and it is not a prediction that unfinished MVP features
  alone need 116 hours. No time already worked is inferred from this baseline.
- Fill that workload in order: **required MVP work first, useful refinement
  second, optional scope if the earlier priorities finish early**. Retain the
  historical task estimates rather than inflate feature durations to fill time.
- The four forecast work areas total **26–50 provisional active hours**. The
  other **66–90 hours of the 116-hour workload** are for further useful
  refinement and, if appropriate, selected optional scope. That is a flexible
  workload allocation, not an idle-time recommendation or extra MVP requirement.
- Keep **5–10 hours of troubleshooting reserve separate** as a contingency
  allowance. Reserve is not worked time, not a prefilled weekly task, and not an
  extra required 5–10 hours on top of 116. Only actual troubleshooting consumes
  required working hours; it displaces lower-priority refinement/optional work
  and is logged once against the affected task. Unused reserve contributes zero
  worked hours and is never logged as spent.
- The earlier 112–140-hour feature estimate and acceleration implication remain
  superseded. Review actual work, unfinished outcomes and required hours still
  to work each Friday. Approval waits and elapsed-time tests can affect calendar
  duration; passive waiting is not counted as active work.

## Provisional effort forecast and basis

Davian supplied this forecast on 2026-10-27 after reviewing historical
implementations/work and the time those tasks took, then projecting that pace
onto the remaining scope. This plan adopts those observations as its basis; it
does not invent individual task timings or claim an independently audited time
ledger. Completed work remains evidence of pace, not a guarantee that a new
feature will take the same time. Update these ranges as work supplies new data.

| Remaining work                                            | Provisional active effort |
| --------------------------------------------------------- | ------------------------- |
| Environment rollout preparation and validation            | **4–8 hours**             |
| OpenAPI contract, implementation and validation           | **10–20 hours**           |
| MCP contract-result extension                             | **2–5 hours**             |
| Product refinement, documentation and final demonstration | **10–17 hours**           |
| **Forecast work subtotal**                                | **26–50 hours**           |

**Separate troubleshooting reserve: 5–10 hours**, used only if needed. The
previous **31–60-hour** combined forecast described work plus potential reserve;
it is not the weekly workload or hours already worked. Keep reserve outside the
work subtotal and time ledger until an actual troubleshooting task occurs.

The **116-hour remaining required workload** is a planning commitment across MVP,
refinement and optional scope. The provisional **26–50-hour task forecast** is
a duration estimate for the four named areas. These measure different things.

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

The proposal's original ten-week/200-hour planned timeline remains intact.
Actual delivery order differs: workflows, local-agent, MCP foundations and
hosting preceded OpenAPI. The current workload is **16 hours remaining in W4,
then 20 hours each week in W5–W9**, totaling **116 required hours** under current
assumptions. The W9 Friday date remains inferred and unconfirmed. This workload
commitment does not imply that MVP implementation must accelerate.

The primary outcome in each row has first claim on that week's hours. When it
finishes, use the rest for refinement; only move to optional scope when required
MVP and useful refinement priorities are satisfied. Carry unfinished MVP work
forward ahead of that later week's refinement/optional work. All future outcomes
remain **planned**, not completed.

| Week / dates                                        | Required workload | MVP outcome first                                                                                                                                                                                                                                                                                                                                                                                                                              | Useful refinement second; optional work if ready                                                                                                                                                                                                  |
| --------------------------------------------------- | ----------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **W4 — current**, Oct 26–Nov 1                      | **16 h**          | Environment PRs #15/#16 are merged. Prepare the rollout/compatibility packet. Deployment, activation and runtime validation remain held; complete validation only if separately authorized, otherwise carry it forward. Historical environment estimate: 4–8 h.                                                                                                                                                                                | Use the remaining 8–12 h first for OpenAPI contract preparation (required MVP work), then existing request/environment UX, setup guidance and regression review. Reallocate within 16 h if findings change the estimate.                          |
| **W5**, Nov 2–8                                     | **20 h**          | Accept the OpenAPI production contract before implementation. Deliver bounded JSON/YAML import, operation/request generation, four response violation classes and immutable sanitized results/UI for accepted targets. Verify unsupported/invalid/ref/oversize/partial-publication, ownership, redaction, retention/pins and cleanup. Historical estimate: 10–20 h across contract, implementation and validation; count any W4 progress once. | Refine import diagnostics and usability, example safety and documentation. Remaining hours depend on actual MVP effort; carry incomplete validation forward and preserve live gates.                                                              |
| **W6**, Nov 9–15                                    | **20 h**          | Complete OpenAPI carryover before the dependent MCP extension. Accept its tool/DTO compatibility and expose stored pass/fail/not-run/unsupported/unavailable states. Verify scope, revocation, fail-closed audit, size limits and secret containment. Historical MCP estimate: 2–5 h.                                                                                                                                                          | Refine MCP setup/error guidance, browser-to-backend regressions and executable schema/contract consistency. Apply the remainder of the week to prioritized refinement, then optional work if ready.                                               |
| **W7**, Nov 16–22                                   | **20 h**          | Close remaining required MVP gaps and record actual validation boundaries; source implementation is not deployed runtime proof.                                                                                                                                                                                                                                                                                                                | Improve request/history/workflow/agent/MCP loading, error and conflict states; keyboard/focus behavior, responsive layouts, regression coverage and setup/security/recovery docs. Select optional work only after these priorities are satisfied. |
| **W8**, Nov 23–29                                   | **20 h**          | Review MVP completion and resolve presentation-critical findings. Any held validation remains open unless separately authorized and performed.                                                                                                                                                                                                                                                                                                 | Refine the integrated product, documentation and sanitized demo data; conduct usability checks and address findings. Use remaining hours for a selected bounded optional enhancement with its own tests/docs.                                     |
| **W9 — inferred finish Friday Dec 4**, Nov 30–Dec 4 | **20 h**          | Final required regression, demonstration rehearsal, backup recording and presentation/submission materials; report actual checks and limitations.                                                                                                                                                                                                                                                                                              | Polish demonstration reliability and onboarding, recheck docs from a fresh-user perspective and close small usability defects. Only finish optional work that fits without jeopardizing required outcomes.                                        |
| **Total**                                           | **116 h**         | **16 + (5 × 20)**                                                                                                                                                                                                                                                                                                                                                                                                                              | Work across required MVP, useful refinement and selected optional scope; troubleshooting counts only when actually performed.                                                                                                                     |

The historical 10–17-hour refinement/documentation/demonstration estimate is a
starting forecast for those named completion tasks, not a cap on useful product
refinement during the required weekly workload. Additional refinement should
have a specific user benefit, observable outcome and appropriate checks. Avoid
busywork, speculative abstractions and repeating checks without a new reason.

## Priority queue and optional scope

1. **Required MVP:** environments rollout/validation gates, accepted OpenAPI
   import and validation, the MCP stored-result extension, required contract/
   schema reconciliation, regression, setup/security/recovery docs and final
   demonstration. Respect existing review/authorization boundaries.
2. **Useful refinement:** prioritize observed defects and friction in the
   existing vertical slice: clearer errors and unsupported-feature explanations,
   loading/conflict/revocation states, keyboard/focus/responsive behavior,
   onboarding examples, focused regression gaps and a reliable demonstration.
   The open browser teardown/stop-enforcement tooling correction takes priority
   before that tooling is reused in a live campaign.
3. **Optional candidates if earlier priorities finish early:** compare two
   sanitized JSON execution records (a proposal stretch feature); provide a
   bounded report/export of already sanitized execution summaries; improve
   nonessential history filtering or navigation. Choose one small candidate at
   a time based on remaining hours and user value. Record its scope, acceptance
   criteria and estimate before implementation. These are choices for the
   workload, not additional MVP commitments or authorization from this docs task
   to implement them. Contract changes and live/AWS work retain separate gates.

At each weekly review, record actual MVP/refinement/optional hours and actual
troubleshooting, total them once, and subtract actual worked hours from 116.
Do not subtract the unused reserve or count passive waits. Reprioritize the next
week to maintain 20 hours of actual work (16 for the current partial week) under
the stated assumptions. If useful in-scope work cannot fill the commitment,
raise that planning issue instead of logging unworked reserve or inventing work.

## Completion checks and risks

- [x] Establish current W4 and remaining required effort: 16 hours this week plus
      5 weeks at 20 hours/week, totaling 116 hours.
- [ ] Confirm exact finish date; December 4 is inferred, and no institutional
      submission date is defined.
- [x] Environment contract PR #15 and implementation PR #16 merged.
- [ ] Environment deployment/activation and genuine runtime validation; held
      for separate review.
- [ ] Track actual worked hours against the 116-hour requirement, keeping
      provisional task estimates and unused troubleshooting reserve separate.
- [ ] Review the MVP-first, refinement-second, optional-third queue each Friday
      and maintain the 20-hour weekly workload after the current 16-hour week.
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
required effort still to work. If a dependency slips, update its weekly outcome
and use independently actionable refinement to maintain the workload; keep affected checklist items open. Raise a
scope/date tradeoff only if new evidence actually requires one. The proposal's MVP
does not become smaller because a restricted prototype works.

General-public signup/SES, custom-domain readiness, comprehensive cross-engine
accessibility, real elapsed expiry/retention, PITR/fault/IAM/performance exercises
and measured billing remain tracked operational follow-ups, with status and
authorization separate from accepted development/demo functionality. Select
presentation-critical cases during W8; do not claim unperformed cases pass.
Historical publication attribution cannot be reconstructed by replaying HTTP.
Optional candidates remain outside the required MVP and the historical task
forecast; their selected effort belongs within the 116-hour workload.

Detailed historical budgets, observed counters, deployment and verification
records belong in [internal operational records](../development/proposal-operational-records.md)
and the linked slice guides. This plan and its future work estimates authorize
no application changes, AWS reads/mutations, deployment or activation.
