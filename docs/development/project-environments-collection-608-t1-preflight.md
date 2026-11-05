# release-608-t1: local preflight held before start

The requested initial collection **did not start**. Clean approved tooling
**4de31ab9a9c791a1fc224bbce8a7fe79220519ba** was selected, but its documented
`driver.py check` failed before authorization preparation. The pinned release-source
working directory **`/tmp/kurier-environments-release-source` is missing**.
Git's working-directory open raised `FileNotFoundError`, errno 2, at the source
revision check. Its retained worktree registration still names the unchanged
release source **44f1fb79d943501c95e06fcc1a6114111a7c1ea4**; a registration alone
does not provide the source bytes required for an isolated child.
[Machine outcome](project-environments-collection-608-t1-preflight.json).

## What ran and what remains available

Local inspection covered the clean branch/working tree, worktree registrations,
repository instructions, published timing workflow and unused campaign/claim/
authorization names. SDK, packager, model and tool pins passed before the source
check. Independent local inspection verified all seven retained campaign bindings
and cumulative accounting, plus adopted/rollback artifacts. Complete preflight
**did not pass**, and source working-tree cleanliness cannot be established while
that directory is absent.

The PR branch was restored for publication. No worktree was recreated or repaired,
registration pruned, artifact rebuilt, expected hash substituted, policy widened
or `.build` deleted. All original campaigns, consumed claims, checkpoints, private
evidence, ledgers and adopted/rollback bytes remain preserved. The missing path
was observed at the first inspection; this task did not remove it or establish
when/how it disappeared.

**Zero start-command invocations, AWS calls, renewals, STS calls or runtime
requests.** No authorization file, new claim, campaign, checkpoint or timer was
created. The new `release-608-t1`, `window-from-608-timing-1.json` and
`authorization-from-608-timing-1.json` names remain unused. No later phase ran.

Accounting is **608 + 0 = 608/1,799**, at most **1,191** further attempts under the
ceiling. The cumulative **$1 procedural allowance** is unchanged; billing is
unmeasured. There is **no campaign elapsed time or remaining campaign time** for
this request: the new 55-minute window and 240-second collection ceiling never
started. The expired historical f1 deadline was not reset or resumed.

## Requested fresh findings

| Finding                               | This request                          |
| ------------------------------------- | ------------------------------------- |
| Baseline, marker and quiescence       | Unavailable; collector never launched |
| Old-frontend readiness                | Not checked live                      |
| Configuration/IAM comparisons         | Unavailable                           |
| Complete resource diff                | Unavailable; preview never launched   |
| Generated four-ZIP/frontend agreement | Unestablished; no new packaging       |
| Collection proof                      | Absent                                |

The [successful f1 collection](project-environments-collection-499-f1.md) remains
historical evidence. Its baseline, complete diff and exact ZIP/frontend agreement
do not replace the requested fresh observations. Local adopted/rollback hash
checks establish preservation, not fresh generation or current AWS readiness.

## Next decision and verification

Review and separately authorize **local restoration of the exact pinned release-source
worktree**, preserving its registration and all unrelated/retained bytes. The
current request explicitly excludes repairs; no repair was attempted. Complete
preflight and any subsequent once-only collection authorization must be reviewed
before live work. This hold grants no automatic repair, continuation or restart.
The [published command](project-environments-timing-amendment.md) was not executed;
no replacement command is enabled by this record.

Publication checks reconcile the zero-attempt state, absent new files, missing
source path/retained registration and retained artifact/accounting proofs. JSON,
formatting and disclosure checks pass. Execution/application code is unchanged;
prior format/lint/test/build verification remains the code evidence, and no
completed live investigation or fixture suite was rerun to work around this hold.
