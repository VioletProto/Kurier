# ZIP external attributes: reproducible restrictive packaging

Tooling: **83863b02132cbd3c70ace136261bde4ee30dd192**. Release source remains
**44f1fb79d943501c95e06fcc1a6114111a7c1ea4**.
[Machine findings](project-environments-packaging-reproducibility.json).
No AWS calls, renewal, STS, collection or later phase ran. Accounting remains
**499/1,799**, at most **1,300** additional attempts, unchanged cumulative **$1**
procedural allowance; billing is unmeasured.

## Cause and offline reproduction

Adopted review-8 and stopped cp1 used identical cached SST `function.ts` packager
source. Its actual `createCode` function sorts entries, fixes their timestamps and
passes real filesystem files to the installed archiver without overriding their
modes. Archiver therefore carries staging permissions into ZIP external attributes.
The prior environment launcher did not set subprocess umask: SST, its Go builder
and the packager inherited the caller's mask.

Retained adopted staging directories/bootstrap files are 0700 and `resource.enc`
is 0600. Retained cp1 staging is 0755/0755/0644. The archives have identical member
contents; the only raw byte differences are central-directory external attributes,
as established by the [stopped campaign record](project-environments-collection-390-cp1.md).
Historical parent umasks were not directly recorded, so no historical process
snapshot is claimed.

Offline reproduction executes the **unmodified pinned `createCode` function**
verbatim from the cached TypeScript AST, using actual installed archiver/glob and
fresh staging files with retained member contents. Only post-packaging AWS resource
registration is stubbed. This is the real ZIP creation code, not a rewritten ZIP
algorithm. Existing Go bootstrap and encrypted binding bytes are supplied as
inputs; tests do not rebuild Go binaries, encrypt resources or invoke AWS.

| Inherited mask, without correction | Exact output                           |
| ---------------------------------- | -------------------------------------- |
| 0022                               | All four cp1 ZIP hashes reproduced     |
| 0077                               | All four adopted ZIP hashes reproduced |

This establishes the inherited-mask dependency and reproduces the entire observed
byte discrepancy, including executable bootstrap modes. It does not infer token
or binding changes from ZIP metadata. New offline outputs are separate evidence;
no stopped-campaign or adopted archive is edited.

## Narrow correction and regression coverage

The production namespace launch now sets **`umask=0o077` on the child process**,
before SST/build/staging creation. Descendants inherit it. The parent mask is not
changed; archiver code, fixed timestamps, release source and complete ZIP byte
checks remain unchanged. Files are created restrictively, without postprocessing
completed ZIPs or changing expected hashes.

The production launcher and sandbox run the actual pinned packager under parent
masks **0000, 0022, 0027 and 0077**. Every case reproduces all four exact adopted
ZIP hashes, identical member bytes and an executable bootstrap (0700). Checkpoints,
claims and private metadata remain outside the sandbox; no networking is used.
Packager source and relevant installed archiver/ZIP dependency files are now
hash-pinned in preflight. Drift stops rather than selecting another implementation.

**120 execution/frontend tests pass**, including 21 checkpoint/claim tests.
**16 isolated scenarios pass**: eight transport cases, three preview/read cases,
one documented-relative-campaign context case and four actual-launcher packaging
cases. Repository `npm run verify` (format/lint/tests/build/Go tests/vet), Ruff
lint/format and retained artifact/source/evidence checks pass. Downloads and external
networking were disabled. Exact-match results cover retained member inputs; a full
fresh cloud-backed preview with this correction has not run and remains required.

## Accounting and checkpoint/claim continuity

The tooling now verifies and carries **499**, including the cp1 attempts 391–499,
its bound authorization, consumed claim and original pre-campaign checkpoint.
All five stopped campaigns and claims remain unchanged. The next campaign is the
single fixed **`release-499-p1`**, with exclusive
**`window-from-499-packaging-1.json`**; it cannot reuse any consumed identity or
reset usage to 390. The next physical attempt would be **500**; ceiling remains
**1,799**.

`prepare-checkpoint` now **requires exact equality with the stopped parent's last
successful metered authentication checkpoint**, including full file metadata.
An unmatched fresh cache capture is not supported by this path. It checks identity,
account, structure, ownership and permissions under an exclusive lock and rejects
changes/replacement throughout preparation. Missing parent metadata or any drift
stops without silently adopting a new session/checkpoint. Existing records remain
immutable; no background credential provider is invoked.

The current cache matched that parent checkpoint. Local preparation completed once,
creating separate mode-0600 private approval/checkpoint/acceptance records under
the existing mode-0700 checkpoint directory:

- `.build/project-environments-execution/checkpoints/carry-approval-499-p1.json`
- `.build/project-environments-execution/checkpoints/carried-current-499-p1.json`

No original checkpoint or cache bytes were changed. The private approval explicitly
forbids drift adoption. Historical grant continuity remains unestablished. Public
records omit credential values, token hashes, private metadata tuples and record
digests. Future startup must match the carried record before any request; permitted
metered renewal still requires mandatory fresh same-identity/account STS, validity
through the phase and continuity at later separately approved boundaries.

## Proposed next command: authorization required

Use the exact clean tooling pin in the original workspace and unchanged release
source. Local checks/template review:

```sh
python3 -B scripts/environment-deploy/driver.py check
python3 -B scripts/environment-deploy/driver.py template --checkpoint .build/project-environments-execution/checkpoints/carried-current-499-p1.json
```

The template remains `authorized: false`; no new live campaign, claim or execution
authorization file was created. After **separate initial-collection authorization**,
prepare `authorization-from-499-packaging-1.json` exclusively at mode 0600, bind it
to this exact template and change only `authorized` to true. Exact proposed command:

```sh
python3 -B scripts/environment-deploy/driver.py start --campaign .build/project-environments-execution/release-499-p1 --authorization .build/project-environments-execution/authorization-from-499-packaging-1.json --checkpoint .build/project-environments-execution/checkpoints/carried-current-499-p1.json
```

Proposed bounds remain one original 55-minute shared campaign window, 240-second
collection phase, at most one reviewed existing-session renewal if needed,
mandatory fresh STS, read-only collection/guarded preview and stop after collection.
Any later closed-stage deadline remains bounded by the campaign deadline and
separate phase approval. No automatic restart, recapture, hash substitution,
normalization, repair, infrastructure/state writes, later phases, runtime requests
or rollback is authorized. All adopted, rollback and stopped-campaign artifacts
and private evidence remain preserved.
