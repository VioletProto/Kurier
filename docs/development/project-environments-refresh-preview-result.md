# Authorized refresh-preview — authentication transport stop

The exact command reviewed in PR #18 ran once at clean tooling commit
`9cf8d741214a2eb41e398634b0194162b87668a7`, preserving clean release source
`44f1fb79d943501c95e06fcc1a6114111a7c1ea4`. It stopped during the existing-session
refresh, before identity verification or provider startup. Exit1; the launcher
reported: “Preview launcher failed closed. No automatic retry or continuation.”
No restart, repeated refresh, repairs or policy changes followed.

```sh
python3 -B scripts/preview-meter/run.py refresh-preview \
  --review-commit 9cf8d741214a2eb41e398634b0194162b87668a7 \
  --source /tmp/kurier-environments-release-source \
  --output /home/king/Neumont/Kurier/.build/project-environments-preview/review-3
```

This command is historical execution evidence, not permission or instructions
to repeat it. Its campaign claim and stopped authentication ledger remain held.

## Confirmed evidence and uncertainty

The durable window began **2026-10-28 19:53:34.056 UTC** (13:53:34 MDT), with
one60-minute deadline nominally at20:53:34.056 UTC. Authentication stopped at
**19:53:44.304 UTC**,10.248seconds after window creation. The refresh send/outcome
interval was10.032seconds. The duration alone does not identify a timeout type.

The private authentication ledger records exactly one physical attempt:
ordinal166, `signin.CreateOAuth2Token`, HTTP/1.1. Its outcome has
`transportError=true`, `status=null`, followed by a latched `transport-failure`
stop at166. No authenticated marker, handoff, provider claim, provider directory,
provider ledger, SST/Pulumi output or packaging receipt exists for this run.
The existing-session cache was the only credential source available to the
reviewed authentication path; no identity response was obtained.

The confirmed finding is a **transport failure with uncertain remote outcome**.
The reviewed tooling discarded the exception subtype and message. There is no
retained evidence establishing DNS, TCP, TLS, timeout, service-side behavior or
any specific root cause. This is not an endpoint-policy denial and does not
attribute the older endpoint-held preview stop. No diagnostic probe or further
AWS request was made to investigate. The classifier admits this operation only
for the reviewed static signin hostname/HTTPS port443; that is source-derived
routing evidence, not a retained transport-layer diagnosis.

Refresh success and credential issuance/rotation are **not confirmed or ruled
out remotely**. No fresh account/non-root identity verification completed and
no provider request was forwarded. Raw credentials, cache contents, token
hashes, headers, request/response bodies, URL paths/queries and SDK exception
material are not published.

[Sanitized machine result](project-environments-refresh-preview-result.json)
records the command, timing, accounting, private receipt hashes and artifact
checks. The [authentication projection](project-environments-refresh-preview-auth-projection.jsonl)
contains only reviewed event/accounting fields and a window-file hash. It is a
sanitized derivative; the original five-row private ledger remains byte-for-byte
retained, not replaced by this projection. The campaign claim, window receipt
and terminal usage receipt are also retained at their original paths.

## Accounting and artifacts

| Phase                      | Physical AWS attempts |
| -------------------------- | --------------------: |
| Prior inventory            |                   135 |
| Prior stopped preview      |                    30 |
| This refresh               |                 **1** |
| This identity verification |                 **0** |
| This provider preview      |                 **0** |
| Cumulative                 |           **166/500** |

This attempt consumed1 of335 authorized additional attempts. The334 remaining
calls are arithmetic headroom, not authorization to continue, refresh or reset
the window. The persistent cumulative165 campaign claim is preserved and blocks
replay. The reviewed `check` action still reports its historical parent baseline
of165; it verifies frozen inputs and is not the new live cumulative total. The
terminal usage receipt and this result establish the actual166 cumulative.
No accounting/tooling repair was made under this execution authorization.

The same cumulative **$1 procedural allowance** remains. Actual billing, shared
historical spend and unused dollar balance are unmeasured; no billing request
was made and no measured cost claim is inferred from a failed transport.

No complete or partial resource diff exists for this run because authentication
stopped before SST/Pulumi. No ZIP or frontend build ran and no new packaging
receipt was generated. There are **no artifact-hash differences**. Before and
after the attempt, local integrity checks passed for retained prepared packages,
bootstraps, native clients, frontend files, rollback ZIPs/bootstraps/frontend and
CloudFront function bytes. No rollback retrieval or extra inventory call ran.

Release-manifest SHA-256 remains
`94d658f4cdae67103c755e3cf5578a50252f8f780c2df82c46c2c2cf18f8c6c5`;
preview-policy SHA-256 remains
`ed237fbf3fe24ccd06e5ed08a73df641499e5c19c9342c56f14e93198764a560`.
Both original historical ledgers and all reviewed tooling are unchanged.
The new private authentication ledger SHA-256 is
`5316b2ec89fd7ff682f175a6cd14008a840fe2438814c368ab15a05023b997d7`.

## Publication checks and remaining decision

The sanitized projection was checked against the private ledger's event order,
ordinal, operation, outcome and stop. Original/final artifact, source, tooling,
policy and frozen checksum checks passed locally. Repository format/lint/tests/
build/Go verification passed via `npm run verify` (exit0); these local checks
performed no AWS work. The final projection/receipt validation and retained
private-receipt hash checks passed.
Current AWS stage/capabilities, generation, IAM/resources and runtime behavior
were not inspected by this failed authentication attempt.

Live work stopped. No deployment, stage closure, migration, activation,
identity/project creation or live target request occurred. No repair, endpoint
amendment, retry or automatic continuation is authorized or proposed here.
OpenAPI production scope remains separate.

The next decision is whether to authorize a separate **local-only investigation
or diagnostic improvement** for the unclassified transport failure. Any later
live attempt needs separately reviewed cumulative166 accounting and explicit
execution authorization. This result makes no claim about the needed network
repair, if any, and does not widen endpoint permissions.
