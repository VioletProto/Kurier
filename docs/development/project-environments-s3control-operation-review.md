# Exact-host S3 Control hold and private operation diagnostics

**Implemented the accepted endpoint-only proposal; every S3 Control HTTP
operation remains blocked.** Tooling commit
`987b89422c4da2d06a75ce9bd49988de0790cd86` implements the
[proposal accepted from 3579f56](https://github.com/VioletProto/Kurier/blob/3579f56c9856435dde0733b65b4a74d2fbbe733f/docs/development/project-environments-s3control-connect-proposal.md).
This task used local source inspection and offline fixtures only. No AWS call,
refresh, live preview or S3 Control AWS operation ran. Usage remains **231/500**;
release source remains `44f1fb79d943501c95e06fcc1a6114111a7c1ea4`.

## Operation attribution was attempted first

The retained trace, generated bindings, earlier exact-binary call-site evidence
and installed S3 Control API model were inspected. The pinned provider's S3 tag
helper constructs S3ControlClient, but the retained CONNECT contains no HTTP
request and no call-stack linkage. Supported client methods and nearby resource
reads do not establish which API was requested. No source download, external
probe or live reproduction was attempted.

**The historical requested operation remains unproven.** The offline diagnostic
fixtures described below are synthetic protocol examples, not a reconstruction
of that request. No specific S3 Control read permission is proposed or granted.

## Exact endpoint change, unconditional HTTP hold

[policy.py](../../scripts/preview-meter/policy.py) differs from the accepted
parent by exactly four inserted lines:

```python
# service(), after the existing lowercase conversion:
if host == "747336059622.s3-control.us-east-2.amazonaws.com":
    return "s3control"

# classify(), immediately after svc = service(host):
if svc == "s3control":
    raise Stop("s3control-operation-held")
```

CONNECT still requires port 443, and request processing still requires HTTPS/443.
Other accounts, regions, base hosts, aliases, suffixes, dual-stack/FIPS spellings,
prefixes, explicit authority ports and trailing dots are rejected. Case handling
retains the existing hostname lowercase conversion. S3 Control is **not** added
to REST, JSON_READS or QUERY_READS. GET/HEAD, mutation methods and unknown methods
are all held before admission or upstream send. The proxy also checks this exact
service before accessing authorization headers or body data.

The proxy remains lazy-upstream with upstream_cert=false. Local CONNECT/TLS
processing can therefore reach HTTP classification without an upstream AWS
request. Synthetic HTTPS fixtures reached the request-phase hold with **zero
upstream requests/attempts charged**. A held request sets the terminal response,
latches the budget stop and terminates descendants. An IO/unexpected diagnostic
failure kills the flow/meter and the supervisor stops descendants; no fallback
forwarding or retry is possible.

The new policy SHA-256 is
`cf2f92ae8f9b720505141d9b8be8310454d4bbec0d1aae59f9879888d286ac6e`.
A test compares the complete policy text against the reviewed parent plus only
these four lines. All other authorization rules remain byte-for-byte identical.
The authentication endpoint, TLS/region/no-proxy/one-attempt restrictions, STS
identity gate, SDK pins, public sanitizer and original-deadline code are unchanged.

## Minimal private diagnostics, default off

The new opt-in launcher flag is `--private-s3control-operation`. Its host config
key is privateS3ControlOperationCapture. The
[diagnostic module](../../scripts/preview-meter/operation_diagnostics.py) runs only
on the first denied exact-host S3 Control HTTP request, after rejection and before
the terminal response. Diagnostic output never changes the authorization result.

**No raw path or query data is retained at all.** The interface receives only
method, request target and cumulative count, with no header or body argument.
A target can contain sensitive URL data; it is used transiently to match method
and URI shape against a hash-pinned static catalogue and is never retained.
Only model-derived operation names are emitted; unknown/invalid/overlong or
ambiguous inputs remain marked as such, never guessed or expanded into raw data.

The fixed `s3control-operation.private.json` contains exactly:

- version, phase, cumulative count and public source-model SHA-256;
- HTTP method from a fixed enum, or OTHER;
- matchStatus and at most four model-derived candidateOperations.

It is capped at **1,024 bytes**, exclusive/no-overwrite, mode 0600 inside an
owner-matched mode-0700 directory beside the private provider ledger, outside
Git and all child mounts. Existing or partial receipts cannot be retried or
replaced. The existing bounded writer supplies symlink refusal, short-write
handling, file/directory fsync and failure shutdown. Input processing is capped
at 8,192 ASCII target characters and 64 query fields. Query keys/values and path
segments are never persisted or hashed. No raw-data fallback exists.

**Every private receipt requires local disclosure review before publication.**
Do not publish its raw bytes/hash, or auto-export it. This includes the original
review-6 hostname receipt, which remains private and excluded from accounting/
publication hashes. No resource-bearing path or query value is proposed for
publication. Even a unique model match is diagnostic evidence, not an API
allowance or proof that a service accepted/executed a request.

The proxy's isolated Python environment does not contain the host SDK. The first
offline integration run detected that dependency gap and failed closed. The final
implementation bundles a **13,760-byte route-only catalogue** derived from the
already installed/pinned botocore 1.43.108 S3 Control model. The proxy uses only
the standard library; no dependencies were installed or caches repaired.
Both catalogue bytes and source-model identity are pinned and checked. The
catalogue has 97 operation names/methods/templates and no live resource data.
Regeneration tests match it against the pinned local model. All 97 synthetic
routes match their source operation: 94 uniquely, three with explicit ambiguity.
Ambiguous results retain candidates and require review rather than choosing one.

## Accounting, retention and verification

[Accounting](../../scripts/preview-meter/accounting.py) verifies the original
review-6 STS verification plus 30 provider attempts, their outcomes/stop, usage,
consumed claims and preceding proof chains before reporting **231 prior,
500 ceiling, 269 arithmetic headroom**. It checks **56 frozen files**. Original
167/168/200 chains remain available; no old ledger, projection, claim, deadline or
report was rewritten. The private hostname receipt/review sidecar are explicitly
excluded from hashes. No window-from-231 or new live output directory exists.
The same cumulative **$1 procedural allowance** remains; billing/unused dollar
balance are unmeasured. Both earlier uncertain refresh outcomes remain uncertain.

**80 unit tests and 22 offline process/transport scenarios passed.** Coverage
includes exact account/region matching, all-method holds before headers/body or
budget admission, all source-model route matches, model-pin failure, bounds,
canary exclusion, default-off behavior, exclusive-file collisions, zero upstream
requests, and parent/child/grandchild shutdown. Synthetic read and mutation
routes both produce useful private names while remaining blocked. Wrong account
and region fixtures stop at CONNECT, not at HTTP. Existing shared SST/Pulumi/Go/
Node retry, pagination, exhaustion, deadline and no-reset regressions remain
passing. The synthetic cumulative baseline is now 231.

The complete suite and repository format/lint/test/build/Go checks ran with
external networking and dependency downloads disabled. Python lint/format and
local preflight passed. Local SDK comparison tests read existing cache/config
metadata without writing or refreshing it; authentication fixtures use generated
keys and synthetic caches/sends. No live operation was executed by verification.

Original prepared packages/bootstraps, frontend/client files and rollback bytes
passed their manifests before and after checks. **No release packaging or release
artifact-hash differences.** Release-manifest SHA-256 remains
`94d658f4cdae67103c755e3cf5578a50252f8f780c2df82c46c2c2cf18f8c6c5`.
New tooling/catalogue hashes are in the
[machine packet](project-environments-s3control-operation-review.json) and
[manifest](project-environments-s3control-operation-review.sha256).

## Proposed execution command — not authorized or run

Review the exact implementation commit above first. The proposal is one new
60-minute window with at most **269 additional physical AWS attempts**, prior
231, cumulative ceiling 500, and the same cumulative $1 procedural allowance.
It would permit only the reviewed existing-session refresh if needed, required
same-identity STS verification and existing read-only preview operations.
**Every S3 Control request still stops before forwarding**, regardless of the
captured API name. No actual S3 Control AWS operation would be authorized.

The packet is a later documentation commit. After separate execution approval,
select the exact clean implementation commit in the original workspace containing
the private proof chain and retained artifacts. The following checkout and command
are proposed only; neither has been executed by this preparation task:

```sh
cd /home/king/Neumont/Kurier
git switch --detach 987b89422c4da2d06a75ce9bd49988de0790cd86
python3 -B scripts/preview-meter/run.py refresh-preview \
  --review-commit 987b89422c4da2d06a75ce9bd49988de0790cd86 \
  --source /tmp/kurier-environments-release-source \
  --output /home/king/Neumont/Kurier/.build/project-environments-preview/review-7 \
  --private-s3control-operation
```

The proposed command enables only operation-name diagnostics. The launcher
requires a fresh output and exclusively claims window-from-231;
all authentication/provider calls share the original deadline and physical count.
Stop after success/failure/hold. Inspect the private name-only diagnostic locally
and publish only reviewed findings; do not publish raw receipts/hashes. No restart,
repair, broader capture, operation allowance or automatic continuation follows
from a unique diagnostic match. Further read-operation permission would need its
own exact proposal, tests, review and explicit authorization.

**Stopped for review.** Deployment, closure, migration, activation, identity/
project creation, live targets and OpenAPI production work remain held/separate.
