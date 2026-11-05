# Authorized review-7 operation diagnostic — stopped

**The single authorized campaign stopped before forwarding S3 Control.** Private
review identified **GET ListTagsForResource**, a unique match against the pinned
method/URI-shape catalogue. The receipt and its hash remain private. Usage is
**263/500**: 231 prior + one existing-session refresh + one STS verification +
30 provider attempts. No restart, repair, new permission or automatic
continuation occurred.

Clean tooling was `987b89422c4da2d06a75ce9bd49988de0790cd86`, authorized from
packet `ea19eb8f8056b67c17587908c11155ffa03b00be`. Release source remained
`44f1fb79d943501c95e06fcc1a6114111a7c1ea4`.

## Exact once-only execution and accounting

Local preflight verified the reviewed manifest, clean source/tooling, SDK and
provider pins, 56 original proof files and retained release/rollback bytes. The
fresh output and window-from-231 claim were absent. After the exact detached
checkout, this published command ran once in the original workspace:

```sh
python3 -B scripts/preview-meter/run.py refresh-preview \
  --review-commit 987b89422c4da2d06a75ce9bd49988de0790cd86 \
  --source /tmp/kurier-environments-release-source \
  --output /home/king/Neumont/Kurier/.build/project-environments-preview/review-7 \
  --private-s3control-operation
```

**Historical evidence only; do not rerun.** The feature branch was restored after
the stop solely for local results publication. Runtime code was not modified.

| Bound or actual result         | Observed                                                         |
| ------------------------------ | ---------------------------------------------------------------- |
| Window began                   | 2026-10-29 04:09:46.123909 UTC                                   |
| Stop recorded                  | 2026-10-29 04:10:02.331645 UTC                                   |
| Elapsed                        | 16.208 seconds of the authorized 3,600 seconds                   |
| Authorized additional attempts | At most 269, including authentication                            |
| Existing-session refresh       | Ordinal 232, HTTP 200, one attempt                               |
| Required STS verification      | Ordinal 233, HTTP 200, same existing identity/account verified   |
| Provider reads                 | 30 attempts, ordinals 234–263; 23 HTTP 200 and 7 HTTP 404        |
| Campaign / cumulative          | **32/269 new; 263/500 cumulative**                               |
| S3 Control AWS requests        | **0**; the HTTP request was rejected before admission/forwarding |
| Exit                           | Launcher 1; sandbox -15 after supervised shutdown                |

Authentication/provider phases shared the original deadline and durable count.
Every unique admitted ordinal has exactly one retained outcome; no unmatched
attempts or transport failures exist. Concurrent outcomes can finish out of
order. HTTP 404 remains a failed read, not repair authority. Provider operations
were SSM GetParameter 2; S3 GET 15/HEAD 2; STS GetCallerIdentity 7; Cognito
DescribeUserPool 1/DescribeUserPoolClient 1; DynamoDB DescribeTable 2.

The remaining **237 calls are arithmetic headroom, not authority to continue**.
The frozen tooling checker describes its 231 parent; terminal receipts establish
actual 263. A future campaign needs reviewed 263-based proof/accounting, a fresh
bounded window and explicit authorization. Consumed claims/stopped windows must
not be reset, deleted or reused. The same cumulative **$1 procedural allowance**
remains; billing, historical shared spend and unused dollar balance are unmeasured.

## Reviewed finding, not raw diagnostic publication

The opt-in private receipt was inspected locally and validated against its exact
schema, size/permissions and pinned model identity. Its candidate is a literal
operation name from the immutable route catalogue, with a fixed method/status.
It contains no path, query key/value, header, credential, body or resource value.
Only the following reviewed finding is disclosed:

**The blocked client request was GET and uniquely matched ListTagsForResource.**
This identifies the client API intent by method/URI shape. S3 Control did not
receive or execute it, and no AWS response to that API was obtained. The raw
receipt is not copied, projected, published or hashed for this packet. No hash
of that receipt or other private capture/review sidecar is in the public evidence.

The original provider ledger remains unchanged: s3control-operation-held at
count 263, `<redacted-host>`, HTTPS/443, HTTP/2.0, GET, request phase, no trailing
dot. The API name is a separate reviewed finding, not a new public ledger field.
The exact-host-only rule and unconditional all-operation hold remained active.

The diagnostic deliberately does not retain the resource ARN or query values.
**The requested resource scope is not established by this receipt.** No resource-
bearing path/value is disclosed. Any proposal to permit ListTagsForResource must
separately establish and review exact resource ARN scope, account/region and
request constraints, with suitable tests and explicit authorization. This result
adds no S3 Control read permission and supplies no executable allowance proposal.
Neither a name match nor the API's read semantics permits automatic forwarding.
Both earlier failed remote refresh outcomes and historical timeout cause remain
uncertain.

## Resource evidence and artifact comparisons

The current Pulumi event stream includes 109 pre-events: 8 SST route-component
creates, 100 same entries and one bucket read; 175 output events include
duplicates. The component creates correspond to environment CRUD, both
OPTIONS environment paths and local-agent capabilities. They are local preview
registrations, not performed AWS route creations.

Unlike earlier CONNECT-only stops, this run emitted **one preview summary event**
with 8 create / 97 same and duration 13 seconds. The log also contains **three
error diagnostics**. SST stdout is empty and the launcher terminated on the
explicit hold. This is an error-time engine summary, **not a successful or
complete AWS/application/IAM diff**. It does not establish that Lambda code,
all route children or final inline IAM policies were fully planned. Selected
names/types/operations and safe summary fields are in the machine result;
resource inputs/outputs and raw error messages remain private.

The rebuilt frontend matches prepared release
`38325083b003b3f320752dbfa0357c377d57d2b66c1175e06c88ba3638210236`;
all five file sizes and SHA-256 hashes were verified individually. No Lambda ZIP
completed. The empty generated-package list does not establish new ZIP equality.
Prepared packages/bootstraps, clients, frontend and retained rollback bytes passed
their original manifests before and after the run. **No release artifact-hash
differences or production artifact uploads.** Release-manifest SHA-256 remains
`94d658f4cdae67103c755e3cf5578a50252f8f780c2df82c46c2c2cf18f8c6c5`.

The accepted exact-host/all-operation-hold policy stayed unchanged at SHA-256
`cf2f92ae8f9b720505141d9b8be8310454d4bbec0d1aae59f9879888d286ac6e`.
Authentication endpoint/TLS/region/one-attempt controls, STS gate, SDK/catalogue
pins, public sanitizer and execution tooling were not changed.

## Preservation, verification and stop

[Machine result](project-environments-refresh-preview-review-7.json),
[sanitized authentication projection](project-environments-refresh-preview-review-7-auth-projection.jsonl),
[provider projection](project-environments-refresh-preview-review-7-provider-projection.jsonl)
and [manifest](project-environments-refresh-preview-review-7.sha256) contain
reviewed facts. All 56 prior protected files, original ledgers and failed/
consumed claims remain intact. New original window, auth/provider ledgers,
handoff, campaign/provider claims, usage, packaging and logs remain retained.
The private operation receipt and every capture/review sidecar are explicitly
excluded from public file/hash lists and Git; projections never replace originals.

Local publication checks reconcile unique outcomes, counts/deadline, public
projections, model-derived disclosure, summary timing and retained artifact bytes.
Full repository format/lint/test/build/Go checks passed with external networking
and dependency downloads disabled. No post-stop AWS inspection, request, refresh
or repair was performed.

**Stopped after the one authorized attempt.** A separate read-permission review
is required before changing the S3 Control hold. No restart, policy widening,
deployment, closure, migration, activation, identity/project creation, live target
request or automatic continuation occurred. OpenAPI production scope remains
separate.
