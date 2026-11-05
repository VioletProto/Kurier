# Authorized review-6 private-capture preview — stopped

**The single authorized campaign stopped on endpoint-held.** It identified the
reviewed destination **747336059622.s3-control.us-east-2.amazonaws.com** without
forwarding the denied connection. Actual usage is **231/500**: 200 prior,
one required STS verification and 30 provider attempts. Existing credentials
were valid; **no refresh occurred**. No restart, repair or policy widening ran.

Clean tooling was `31091700ea7b2348a2eb8dfbf1f74ebd859b5421`, and release source
remained `44f1fb79d943501c95e06fcc1a6114111a7c1ea4`. The launcher exited 1 and
reported sandbox exit -15 after supervised shutdown. No complete preview or
deployment approval resulted.

## Command and bounds

Preflight verified the clean reviewed tooling/source, SDK/provider/platform pins,
38 preserved proof files and prepared/rollback bytes, with the fresh output and
window-from-200 claim absent. The following command ran once from the detached
clean tooling commit in `/home/king/Neumont/Kurier`:

```sh
python3 -B scripts/preview-meter/run.py refresh-preview \
  --review-commit 31091700ea7b2348a2eb8dfbf1f74ebd859b5421 \
  --source /tmp/kurier-environments-release-source \
  --output /home/king/Neumont/Kurier/.build/project-environments-preview/review-6 \
  --private-denied-connect
```

**Historical evidence only; do not rerun.** The branch was restored after the
stop solely to publish results. No execution code changed.

| Authorized bound or actual result | Observed                                                                        |
| --------------------------------- | ------------------------------------------------------------------------------- |
| Shared window began               | 2026-10-29 03:19:45.592739 UTC                                                  |
| Stop recorded                     | 2026-10-29 03:19:58.890189 UTC                                                  |
| Elapsed                           | 13.297 seconds of the authorized 3,600 seconds                                  |
| Additional attempt limit          | 300, including authentication                                                   |
| Existing-session refresh          | 0; cached credentials met the SDK validity threshold                            |
| STS identity verification         | Ordinal 201, HTTP 200; same existing identity/account verified before providers |
| Provider                          | 30 attempts, ordinals 202–231; 23 HTTP 200, 7 HTTP 404                          |
| Campaign / cumulative             | **31/300 new; 231/500 cumulative**                                              |
| Held CONNECT                      | Not forwarded or charged as an AWS attempt                                      |

Every unique admitted ordinal has exactly one retained outcome; there are no
unmatched attempts or transport failures. Concurrent outcomes can finish out of
order. HTTP 404 observations remain failures, not repair authority. Provider reads
were SSM GetParameter 2; S3 GET 15/HEAD 2; STS GetCallerIdentity 7; Cognito
DescribeUserPool 1/DescribeUserPoolClient 1; DynamoDB DescribeTable 2. The
consumed handoff carried the authentication count and original deadline into the
provider meter. No post-stop AWS calls or target requests ran.

The remaining **269 calls are arithmetic headroom only**. The frozen tooling
checker still describes its 200-based parent; the new terminal receipts establish
actual 231. Future live work requires reviewed 231-based proof/accounting and
explicit authorization. All consumed/stopped claims remain intact. The same
cumulative **$1 procedural allowance** remains; billing, historical shared spend
and unused dollar balance are unmeasured.

## Private review and destination finding

The opt-in private receipt was inspected locally. It met the reviewed schema,
253-character hostname/512-byte record caps and private file permissions. The
hostname matched the pinned, previously documented account ID followed only by
fixed AWS S3 Control service, region and commercial DNS labels. It contains no
credential, arbitrary identifier, user, project, resource or session label.
**That hostname alone passed disclosure review. The raw receipt and its hash
are not published; the receipt was not hashed for this packet.** Its review
sidecar is also excluded from public evidence hashes.

The original sanitized provider ledger remains unchanged and still reports
`<redacted-host>`, endpoint-held, count 231, CONNECT, HTTP/1.1, port 443, connect
phase, unspecified scheme and no trailing dot. The approved hostname is a
separate finding; it was not inserted into or substituted for the original
public/private denial ledger. No paths, queries, headers, bodies, credentials or
raw provider diagnostics are published.

**Observed fact:** this preview attempted that exact same-account regional
S3 Control destination. It must pass the endpoint gate for that connection to
advance. No AWS request to it was forwarded.

**Local source evidence:** the exact pinned Pulumi AWS provider binary includes
S3 Control SDK v1.73.0. ELF Go function-table resolution and direct-call
inspection of its S3 `(*servicePackage).ListTags` helper show a call to
`(*AWSClient).S3ControlClient`. This links S3 Control support to provider tag-read
functionality. Its binary SHA-256 remains
`741bfe58a4f6100dfa839beb98d53e0f27c9b63998d53a293cbbfb7a6b23a797`.
The generated provider still targets us-east-2, without an endpoint override.

**Limit:** CONNECT precedes HTTP classification. Neither the HTTP method/API,
resource parameters, dynamic caller of this particular connection, nor the
read-only safety/necessity of a specific API is established by the private host
receipt or static call sites. Installed SDK operation models list supported
APIs; they cannot replace an observed request. No read operation is guessed
from the hostname, timing or nearby resource entry. The endpoint is a valid,
supported provider endpoint, but this is not approval to pass arbitrary S3
Control operations. Previous failed refresh outcomes and historical timeout
causation remain uncertain.

## Separate proposal — no policy applied

The [exact endpoint-only proposal](project-environments-s3control-connect-proposal.md)
would recognize only this account/region/host for CONNECT and explicitly deny
**all** S3 Control HTTP operations before admission/forwarding. It is a proposed
local-proxy diagnostic step, not a completed read-operation allowance or a promise
to finish preview. It adds no service to the generic REST read list. A separate
source-linked or reviewed operation-level observation is needed before proposing
an actual AWS read permission. No proposal was applied or tested live here.

The preview policy, public sanitizer, authentication endpoint, TLS/region,
existing-session/STS restrictions, SDK pins and execution tooling remain unchanged.
Policy SHA-256 remains
`ed237fbf3fe24ccd06e5ed08a73df641499e5c19c9342c56f14e93198764a560`.

## Resource diff and artifacts

There is **no final summary or complete AWS/IAM diff**. The current event stream
contains 109 resource pre-events: 8 new SST route-component entries, 100 same
entries and one bucket read. These are partial preview registrations, not
performed AWS changes. Component creates cover environment CRUD, OPTIONS on
both environment paths and local-agent capabilities. Existing IAM role entries
are provisional; complete role-policy/permission changes are unverified.
Output events include duplicates and must not be counted as additional proposed
resources. The machine result retains selected names/types/operations only.

The locally rebuilt frontend matches prepared release
`38325083b003b3f320752dbfa0357c377d57d2b66c1175e06c88ba3638210236`;
all five file sizes/hashes were verified individually. No Lambda ZIP completed.
An empty generated ZIP list is not proof of ZIP equality. Prepared packages,
bootstraps, clients, frontend and rollback bytes passed their original manifests
before and after the campaign. **No release artifact-hash differences or
production artifact uploads.** Release manifest remains
`94d658f4cdae67103c755e3cf5578a50252f8f780c2df82c46c2c2cf18f8c6c5`.

## Evidence and stop

[Machine result](project-environments-refresh-preview-review-6.json),
[sanitized authentication projection](project-environments-refresh-preview-review-6-auth-projection.jsonl),
[provider projection](project-environments-refresh-preview-review-6-provider-projection.jsonl)
and [checksum manifest](project-environments-refresh-preview-review-6.sha256)
provide reviewed facts. All 38 prior proof files, original ledgers and failed/
consumed claims are preserved. New original window, ledgers, handoff, consumed
campaign/provider claims, usage, packaging and logs remain retained. The private
hostname receipt remains private and is **explicitly excluded from every public
file/hash list**, as is its review sidecar. It is never copied into Git.

Publication verification reconciles unique outcomes, bounds/deadline, safe
projections, source/tooling integrity, event timestamps and artifact equality
using local reads only. Full repository format/lint/test/build/Go verification
passed with external networking and dependency downloads disabled. No live-work
continuation followed the hold.

**Stopped for review.** Next decisions are whether to accept the separate
endpoint-only proposal, how to establish the necessary HTTP read operation
without sensitive disclosure, and whether to authorize future reviewed tooling/
231-based accounting and a fresh bounded campaign. No deployment, closure,
migration, activation, identity/project creation, live target requests, repairs,
policy widening or automatic restart occurred. OpenAPI production scope remains
separate.
