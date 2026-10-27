# Remaining-work plan

Planning snapshot updated: **2026-10-27**. Davian owns all work. This is a delivery
forecast, separate from the proposal's preserved ten-week, 200-hour planned
timeline. Estimates below are provisional active effort projected by Davian from his
past implementations and observations of how long that work took. They include
implementation, documentation, review and relevant verification, and remain
subject to revision. They are estimates, not promises of task duration.

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
- Provisional remaining active effort: **31–60 hours**, including a **5–10-hour
  troubleshooting reserve**. Capacity is available time, not a work estimate or
  a requirement to use every available hour.
- Against 116 available hours, that forecast leaves **56–85 hours unallocated**.
  There appears to be substantial room to slow the pace, allow review and
  integration time, and revise the forecast without assuming acceleration.
- This replaces the earlier 112–140-hour estimate and 112–116-hour allocation.
  Those figures lacked support from Davian's observed implementation pace;
  their implied capacity shortfall and need to accelerate are withdrawn.
- Revisit actual active effort, unfinished outcomes and available time each
  Friday. Approval waits, elapsed-time tests and other external dependencies
  can affect calendar duration even when active effort remains modest.

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
| Troubleshooting reserve                                   | **5–10 hours**            |
| **Total**                                                 | **31–60 hours**           |

The four work areas total **26–50 hours**, plus the reserve once. The reserve is
for troubleshooting as needed, not a scheduled requirement to spend time.

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

The forecast keeps the proposal's original ten-week/200-hour planned timeline
intact. Actual delivery order differs: workflows, local-agent, MCP foundations
and hosting preceded OpenAPI. The weekly sequence below spreads the remaining
work over W4–W9 with room for a slower pace; it does not require filling each
week's capacity or accelerating to finish. The W9 Friday date remains inferred
and unconfirmed, not a deadline or evidence of schedule pressure. MVP scope is
unchanged. Every future outcome remains **planned**, not completed.

| Week / dates                                        | Provisional active effort           | Weekly outcome and completion evidence                                                                                                                                                                                                                                                                                                                                                             |
| --------------------------------------------------- | ----------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **W4 — current**, Oct 26–Nov 1                      | **4–8 h**: environments             | PRs #15/#16 are merged. Prepare the environment rollout/compatibility packet; deployment, activation and runtime validation remain held for separate review. Complete and record validation only if separately authorized; otherwise carry that outcome forward explicitly.                                                                                                                        |
| **W5**, Nov 2–8                                     | **10–20 h**: OpenAPI                | Review and accept the production contract before implementation. Deliver bounded JSON/YAML import, supported-operation request generation, four response violation classes and immutable sanitized results/UI for accepted targets. Verify invalid/unsupported/ref/oversize/partial-publication cases, ownership, redaction, retention/pins and cleanup. Record any separately held runtime gates. |
| **W6**, Nov 9–15                                    | **2–5 h**: MCP extension            | After stored OpenAPI results and tool/DTO compatibility acceptance, expose stored pass/fail/not-run/unsupported/unavailable states. Verify scope, revocation, fail-closed audit, size limits and secret containment without schema fetching or validation on reads. Carry incomplete dependencies forward rather than claim completion.                                                            |
| **W7**, Nov 16–22                                   | **4–7 h**: refinement/docs          | Run the primary browser-to-backend and ownership/redaction/history/workflow/agent/MCP regressions. Refine loading/error/conflict and keyboard/focus paths; reconcile accepted contracts and executable schemas. Record genuine-runtime gaps separately.                                                                                                                                            |
| **W8**, Nov 23–29                                   | **4–6 h**: refinement/docs          | Resolve presentation-critical findings, review completion against the MVP, finish setup/security/recovery guidance and prepare sanitized demonstration data. Any authorized rollout/validation carried forward consumes its original work-area estimate, not a duplicate allocation.                                                                                                               |
| **W9 — inferred finish Friday Dec 4**, Nov 30–Dec 4 | **2–4 h**: final demonstration      | Rehearse the demonstration, record a backup and finish presentation/submission materials. Document actual checks and remaining limitations. The inferred date is not an institutional submission deadline.                                                                                                                                                                                         |
| **Across W4–W9, as needed**                         | **5–10 h**: troubleshooting reserve | Apply only when problems arise; record actual use once and revise affected work estimates when evidence warrants it.                                                                                                                                                                                                                                                                               |
| **Total**                                           | **31–60 h**                         | **26–50 h work plus 5–10 h reserve**, within 116 h available capacity; **56–85 h unallocated**. Refinement/docs/demonstration rows together total 10–17 h.                                                                                                                                                                                                                                         |

The weekly effort ranges distribute the work-area forecast; they are not extra
hours on top of it. Dates and sequencing can move as dependencies resolve.
Unused capacity can remain unused; it does not authorize stretch features or
additional operational campaigns.

## Completion checks and risks

- [x] Confirm current W4 and remaining capacity: 16 hours this week plus
      5 weeks at 20 hours/week, totaling 116 hours.
- [ ] Confirm exact finish date; December 4 is inferred, and no institutional
      submission date is defined.
- [x] Environment contract PR #15 and implementation PR #16 merged.
- [ ] Environment deployment/activation and genuine runtime validation; held
      for separate review.
- [ ] Track actual active effort against the provisional 31–60-hour forecast
      and 116-hour capacity; revise ranges and weekly outcomes each Friday.
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
capacity. If a dependency slips, update its weekly outcome and revise the forecast
using the remaining headroom; keep affected checklist items open. Raise a
scope/date tradeoff only if new evidence actually requires one. The proposal's MVP
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
