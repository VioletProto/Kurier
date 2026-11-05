# Three approved read grants — local implementation review

[Machine implementation record](project-environments-approved-read-correction.json).

Tooling commit **515a4ae2100fad952c519b288f0369aeca5b3762** implements exactly
the three execution-gate reads approved from
[the review at 8a14fad](project-environments-preview-denial-review.md).
The release source remains **44f1fb79d943501c95e06fcc1a6114111a7c1ea4**.
No AWS call, credential renewal, live start or live phase occurred. Actual usage
remains **390/1,799**, with **1,409** additional attempts within the accepted
ceiling and the unchanged cumulative **$1 procedural allowance**. Billing is
unmeasured. All later phases remain held.

## Exact grants, scope and pins

The original scope data is unchanged except for `approvedAdditionalReads`, whose
three entries equal the proposals preserved at commit
`8a14fad485a8869cda8262b121e2842f979a66b6`. The new scope SHA-256 is
`9289a837814f1f27d21d01893a8074ba12844f12647923536134486e6466d5cb`.
`pins.json` binds that hash and the historical review JSON's hash/commit;
`driver.py check` verifies the grant list against that record. The historical
review is not rewritten to imply it granted permissions at the time.

| Approved operation | Exact resource                                                                                                                |
| ------------------ | ----------------------------------------------------------------------------------------------------------------------------- |
| S3 GetBucketPolicy | `kurier-dev-api-web-fc2927a49a0b246240e5a17774`, root empty `policy` query selector                                           |
| S3 GetObject       | Same bucket, `assets/index-CD_8s4hz.js`                                                                                       |
| KVS GetKey         | `arn:aws:cloudfront::747336059622:key-value-store/0e7ac6e9-b4b6-4a9e-b673-4d3838436ecd`, key `76ef:/assets/index-CD_8s4hz.js` |

These grants use dedicated read branches. The old JS object is not added to
upload payloads or deletion targets; its KVS entry is not added to update sets.
No wildcard, other resource/operation or mutation is granted.

S3 grants require GET with no body and canonical empty-body framing. Bucket
policy reads require only the empty `policy` selector at the bucket root; object
reads require the exact canonical path, no version/extra query, range or
conditional selector. The existing verified `x-id=GetObject` transport selector
retains its existing handling. Empty `policy`/`policy=` wire encodings represent
the same selector; no other query alias or selector is accepted. Expected-owner
metadata, if present, must match the scoped account. Requester-pays selectors,
encoded bodies and nonzero/chunked body framing remain held.

The added KVS GetKey branch requires exactly the two approved ARN/key inputs
and an empty body. Different stores, namespaces, keys, extra selectors or bodies
are rejected. The approved S3 Control helper, authentication/signing path,
identity checks, static child credentials, framing/byte gate, durable accounting,
original campaign/closed-stage deadlines and transition core are unchanged.
No IAM policy or infrastructure mutation is included.

## SDK and actual-entrypoint fixtures

Real pinned SDK serialization covers all three positive reads and negative
resource/account/store/key/namespace variants, duplicate/extra query selectors,
versions, bodies, range/conditional framing, HEAD and mutation attempts. Tests
also prove that replacing the old JS grant with a wildcard does not authorize
it, KVS GetKey does not become ListKeys or PutKey authority, and removing the
three grants restores the original preflight gaps.

The namespace fixture runs the actual `phase.mjs diff` entrypoint, SST 4.17.1
and Pulumi AWS provider 7.40.0 with synthetic credentials and loopback responses.
**GetBucketPolicy now reaches its exact synthetic origin once.** The provider
then requests **GetBucketAcl**, which remains unapproved and is stopped before
forwarding. All nine admitted fixture reads reconcile with receipts; no write
or unmatched attempt occurs. This is a passing boundary regression, not a
successful complete release preview.

A separate fixture runs the real collector's corrected old-frontend check
through its actual boto3/AWS CLI transports and the execution gate. It verifies
the accepted old manifest and every retained old file byte/hash, both asset KVS
entries and deployed distribution status. The newly approved old JS object and
KVS entry succeed. All ten fixture reads reconcile. A subsequent SDK request
for an unapproved object is held before forwarding. Synthetic credentials only;
no live credential/cache access or external network is available.

The accepted old frontend is still required for collect/close; the adopted
release is required for verify/activate/final. Existing cross-phase frontend,
drift and closure tests remain in place. No fresh cloud frontend validation or
readiness is claimed by these fixture results.

## Remaining unapproved read and start hold

`pendingScopedReads` no longer reports any of the three approved reads. It
reports only the newly observed **S3 GetBucketAcl**, for the same owned frontend
bucket (`GET /?acl`, no object/body). That read is **not granted** here. No other
provider operations are inferred to be allowed. Further unknown reads must
continue to stop.

The existing preflight now reports this known gap and still blocks `start`
before authentication, any window claim or network work. This avoids spending
live attempts on a request already known to be outside the approved scope.
A separate review is needed before any additional read permission or live
collection; any accepted permission implementation must be tested and repinned.
The three-read implementation does not authorize GetBucketAcl or a live trial.

## Verification, preservation and proposed command

- **74 execution/frontend tests passed:** 17 policy/accounting, 22 credential,
  nine path, 12 policy/read/diagnostic and 14 frontend cases.
- **11 isolated cases passed:** eight existing transport/byte/deadline cases,
  the actual context-entrypoint case and both approved-read/frontend fixtures.
- `npm run verify` passed offline: formatting, lint, application/infrastructure/
  tooling tests, frontend build and Go tests/vet. Ruff lint/format checks passed.
  The final empty-body framing delta passed its focused SDK tests and both
  actual SDK/preview fixtures after broader verification. The unchanged
  transition core's actual-DynamoDB evidence remains in the prior review.
- The clean tooling template was inspected locally: `authorized: false`, prior
  390, ceiling 1,799, original 3,300-second window and 240-second phase ceiling.
  No real `authorization-from-390.json`, `window-from-390.json` or
  `release-from-390` campaign exists.

All original stopped campaigns, claims, ledgers, private evidence, adopted and
rollback archives/frontend bytes, historical records and worktree registrations
are preserved. No `.build` removal, dependency installation, archive rebuild,
session replacement or cloud repair occurred. Fixture logs use fresh private
directories and raw diagnostic/request material is not published.

The [revised execution packet](project-environments-deployment-execution.md)
publishes this exact command for later review:

```sh
python3 -B scripts/environment-deploy/driver.py start --campaign .build/project-environments-execution/release-from-390 --authorization .build/project-environments-execution/authorization-from-390.json
```

**It was not run and remains locally held by pending GetBucketAcl.** The next
decision is a separate review of that observed read, followed by any required
implementation/pin update and explicit initial-collection authorization. Keep
390 prior attempts, the 1,799 ceiling, original 55-minute campaign deadline,
240-second collection ceiling and unchanged cumulative $1 allowance. No later
phase, runtime request, automatic restart or rollback authority is implied.
