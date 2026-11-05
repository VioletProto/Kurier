# Initial collection release-499-p1: checkpoint drift held

The authorized start command ran **once**, using clean tooling
**83863b02132cbd3c70ace136261bde4ee30dd192**, unchanged clean release source
**44f1fb79d943501c95e06fcc1a6114111a7c1ea4** and the original
`carried-current-499-p1.json`. It exited **1** at the first credential boundary
before any physical attempt was admitted. The campaign is latched stopped.
[Sanitized accounting and guard record](project-environments-collection-499-p1.json).

## Authorization and exact outcome

The documented local checks passed: retained campaign/claim/accounting proofs,
source/tool/model/packager pins, private checkpoint bindings and adopted/rollback
bytes verified. No pending read-permission gap was reported. A mode-0600 execution
authorization was generated exclusively from the exact tooling's bound template;
only `authorized` changed to true. Authorization, consumed claim and checkpoint
bindings agree. The carried checkpoint was not regenerated or substituted.

The recorded diagnostic is **`session-changed-no-unmetered-refresh`** at
**`checkpoint-check`**. In the pinned boundary, this guard compares the opened
cache's inode/size/mtime against the carried checkpoint, before SDK credential
fetching, renewal or STS. This establishes checkpoint drift at startup. It does
not establish when, why or by whom the cache changed. No raw metadata tuple,
credential value, token hash or private-checkpoint digest is published.

No renewal or STS request was sent; fresh identity/credential verification did
not complete. No reader or preview child was created. Retained stop reasons are
`phase-authentication-held-no-retry`, followed by
`phase-failed-or-uncertain-no-restart`. The boundary was not replayed for diagnosis.

**499 + 0 = 499/1,799**, with **1,300 arithmetic attempt headroom**, zero unmatched
attempts and zero failed transports. The cumulative **$1 procedural allowance**
is unchanged; billing is unmeasured. No infrastructure/state writes, uploads,
runtime requests, later phases, repairs or automatic restart occurred.

The terminal stop was **0.035185 seconds** after the original monotonic campaign
start. Original campaign/phase time remaining at that stop was
**3299.964815 / 239.993013 seconds**. The shared bounds remained
55 minutes / 240 seconds; no closed-stage window began and no deadline was reset.
Remaining time and call headroom do not authorize continuation.

## Requested fresh findings

| Finding                                       | This campaign's evidence                     |
| --------------------------------------------- | -------------------------------------------- |
| Baseline/capabilities                         | Unavailable; reader never started            |
| Quiescence, residual prefixes and queue depth | Unavailable                                  |
| Accepted old-frontend readiness               | Not checked                                  |
| Deployed configuration/code/IAM comparisons   | Unavailable                                  |
| Complete fresh resource diff                  | Unavailable; preview never started           |
| Four adopted whole-ZIP hash matches           | Unestablished; zero fresh archives generated |
| Generated frontend agreement                  | Unestablished                                |
| Collection proof                              | Absent                                       |

The [preceding cp1 snapshot/diff](project-environments-collection-390-cp1.md) and
[offline packaging proof](project-environments-packaging-reproducibility.md) remain
historical evidence. They do not substitute for this attempt's missing fresh
observations or establish current cloud readiness. No generated ZIP hashes were
substituted, and the exact whole-archive checks remain unchanged.

## Preservation, publication checks and next decision

The original carried checkpoint, its approval/acceptance records, all earlier
checkpoints and all **six** campaigns/consumed claims remain preserved. The new
private authorization, claim and stopped ledger are retained under ignored
`.build`. Adopted archives, stopped preview archives, rollback bytes and all private
evidence are unchanged. No cache/session setting change, checkpoint recapture,
artifact rebuild/patching, worktree pruning or cleanup followed the stop. The PR
branch was restored for publication.

Publication validation independently reconciles zero-attempt accounting,
authorization/claim/checkpoint bindings, the diagnostic and original deadlines;
checks absence of reader/preview children and collection proof; and rechecks
artifact/evidence/source integrity. Formatting and disclosure checks pass. No
implementation changed, so prior format/build/lint/test proofs were not rerun
for this evidence-only publication. No AWS request followed the stop.

Executed once, **historical command only — do not rerun**:

```sh
python3 -B scripts/environment-deploy/driver.py start --campaign .build/project-environments-execution/release-499-p1 --authorization .build/project-environments-execution/authorization-from-499-packaging-1.json --checkpoint .build/project-environments-execution/checkpoints/carried-current-499-p1.json
```

The next decision is a local review of checkpoint drift, if desired. **No adoption
of the current cache, recapture, session replacement or new campaign is authorized.**
Do not overwrite/delete the carried record or consumed claim, restart the command,
reset clocks or infer permission from headroom. Any future live attempt needs
explicit checkpoint/claim review and separate authorization. Closure, deployment,
verification, activation, final, runtime and rollback work remain held.
