# Context-path correction — offline review

Tooling commit **fb0cd0e7f92751a6991a12a2af6e38827e7645ed** fixes the
[reader-child startup failure](project-environments-collection-from-301.md).
Actual usage remains **303/1,799**, with **1,496** additional attempts available
only under future authorization and the unchanged cumulative **$1 procedural
allowance**. Billing remains unmeasured. No AWS call, renewal, restart or live
phase occurred during this correction. Release source stays
**44f1fb79d943501c95e06fcc1a6114111a7c1ea4**.

## Path handling and isolation

The documented command supplied a relative campaign argument. The old driver
passed a relative context filename to the child, whose working directory was
the detached release worktree. That worktree's `.build` was separately mounted
from the child's private build directory, so the context was not available at
the path the entrypoint opened.

The correction resolves campaign arguments against the original repository,
validates their owned location and rejects path aliases. `Campaign` retains an
absolute directory. Approval/authorization files, host plan inputs, child source
and output directories, and required read-only mount sources are validated before
launch. Contexts must be existing regular files without symlink aliases. Missing
required inputs stop before proxy/child startup; no directory repair is attempted.

Each child receives one explicitly mapped, read-only context file at
**`/meter/phase-context.json`**. Host campaign directories, SQLite ledgers, claims,
authorization files, sibling contexts, proxy state and private logs remain outside
the child mounts. The inherited source/dependency mounts, network namespace,
relay, wire policy, account/region/resource scopes and artifact gates remain in
place. No broad campaign mount or live fixture-response hook was added.

Both collection children use the same fixed entrypoint/context mapping; the diff
action no longer constructs a separate command with another relative filename.
Close, deploy, verify, activate and final use that same validated helper. The
offline close/activate planner receives an absolute, validated host context path.
The generic internal command override was removed. No live phase was exercised
while reviewing these paths.

Accounting now starts at **303**. Local checks bind both public stop records,
original authorizations and consumed claims, verify the 300–303 attempt outcomes
through immutable read-only ledger inspection, and seed the existing-session
checkpoint from the latest stopped campaign. Neither old ledger is reopened for
writes or changed. A future campaign would use a new `window-from-303.json`
claim; both earlier claims remain consumed.

## Actual-entrypoint offline regression

[The isolated regression](../../scripts/environment-deploy/test_context_isolated.py)
uses the exact proposed relative argument
`.build/project-environments-execution/release-from-303`, the actual driver child
helper and `phase.mjs` entrypoint, the pinned release-worktree working directory,
and the production sandbox/relay mount builder.

The outer offline namespace overlays the campaign base with a fresh private
fixture directory. The real stopped campaigns are never writable there. The
inner namespace adds only a test-owned synthetic executable at `/usr/bin/aws`;
production code has no response-injection path. No AWS credentials are supplied.
The executable forbids socket/DNS use and returns fixed STS and marker responses
without HTTP requests. The real collector consumes both responses. Its deliberately
invalid synthetic marker then triggers the expected guarded stop before platform
collection. This proves context loading and collector entry, not successful
collection of a genuine cloud baseline.

Assertions verify the original source working directory, exact context contents,
read-only context access, absence of host context paths, ledgers, claims,
authorization files, sibling canaries and `.aws`, two synthetic responses, zero
network forwards/uploads, and unchanged synthetic accounting. The test creates
no real `release-from-303` campaign, claim or approved authorization.

The [nine path unit tests](../../scripts/environment-deploy/test_paths.py)
cover owned relative campaign resolution, parent traversal, missing/directory/
symlink contexts, mount validation before process startup, the shared command
and context mapping for both collection actions and every later phase, and
absolute offline planning inputs. Existing 17 policy/accounting tests and 22
credential tests also pass: **48 unit tests total**. All **eight isolated
transport cases** pass in addition to the new actual-entrypoint case. Fixture
logs and synthetic campaign files are retained in fresh private run directories.

`npm run verify` passed with external networking and dependency downloads
disabled: format, lint, application/infrastructure/tooling tests, frontend build
and Go tests/vet. Ruff lint and format checks passed. The exact clean code commit's
local template check passed, including retained adopted/rollback bytes, source,
SDK/model pins and both stopped campaign proofs. The printed template remained
`authorized: false`; no real new authorization or claim was created.

## Exact proposal for later review

The revised [execution packet](project-environments-deployment-execution.md)
contains clean-checkout and private template preparation instructions. After a
separate authorization, use its pinned tooling commit in the original workspace
and the unchanged detached release source. The proposed once-only command is:

```sh
python3 -B scripts/environment-deploy/driver.py start --campaign .build/project-environments-execution/release-from-303 --authorization .build/project-environments-execution/authorization-from-303.json
```

It remains unexecuted and unapproved. The accepted credential policy retains
one original **55-minute** campaign window, a **240-second** collection ceiling,
at most one initial existing-session renewal if needed, mandatory STS, sufficient
phase credential validity and no background/mid-phase renewal. Closed-stage
deadlines, later phase approvals and all failure/no-replay rules remain in place.
No fresh baseline/quiescence, configuration/IAM comparison, complete live diff,
generated artifact agreement, propagation or runtime behavior is claimed by
these offline results.

All stopped campaigns, consumed claims, original ledgers, historical records,
adopted ZIPs, rollback bytes and private evidence remain preserved. No `.build`
cleanup, worktree pruning, archive rebuild, cache replacement, installation,
authentication-setting change or cloud repair was performed.
