# Frontend bucket ACL correction and consolidated read proposal

Tooling: **2ad78b89dcf070bc1253c4cab122be4572d826e3**. Release source remains
**44f1fb79d943501c95e06fcc1a6114111a7c1ea4**.
[Machine record](project-environments-acl-read-correction.json).

Only the newly approved **GetBucketAcl** is added: exact owned frontend bucket
`kurier-dev-api-web-fc2927a49a0b246240e5a17774`, canonical root `GET /?acl`,
empty body. The SDK's equivalent empty selector `?acl=` is accepted. No other
bucket, ACL mutation or generic read grant is added. Scope and approval pins
bind the prior three reads and this fourth read separately. Signing, account,
framing, identity, durable accounting, deadlines and session controls remain.
The S3 Control resource-scoped tag read is unchanged.

The actual `phase.mjs diff` entrypoint with SST 4.17.1 and provider 7.40.0
now passes policy and ACL reads against loopback fixtures, then stops before
forwarding **GetBucketCors**. A separate test-only observer continues that same
entrypoint's minimal existing-bucket fixture by returning synthetic absent/default
configuration responses inline for rejected reads. It neither admits nor forwards
those reads. This enumerates the following ten gaps in one proposal; **none is
granted**. All ten fail actual SDK serialization policy tests and appear in
`pendingScopedReads`; `start` stops before authentication or claim creation.

Every proposed row is limited to the same bucket above at
`kurier-dev-api-web-fc2927a49a0b246240e5a17774.s3.us-east-2.amazonaws.com`,
account `747336059622`, region `us-east-2`: root GET, exactly one empty selector,
empty body, existing account/signing/framing controls, no other selectors,
versions, paths, ranges, buckets, operations or mutations.

| Proposed operation (unapproved)  | Exact root request     |
| -------------------------------- | ---------------------- |
| GetBucketCors                    | `GET /?cors`           |
| GetBucketWebsite                 | `GET /?website`        |
| GetBucketVersioning              | `GET /?versioning`     |
| GetBucketAccelerateConfiguration | `GET /?accelerate`     |
| GetBucketRequestPayment          | `GET /?requestPayment` |
| GetBucketLogging                 | `GET /?logging`        |
| GetBucketLifecycleConfiguration  | `GET /?lifecycle`      |
| GetBucketReplication             | `GET /?replication`    |
| GetBucketEncryption              | `GET /?encryption`     |
| GetObjectLockConfiguration       | `GET /?object-lock`    |

The installed botocore S3 model validates these wire shapes and is now pinned.
The installed provider binary identifies embedded S3 SDK **v1.106.0**; exact
cached source for that version is unavailable, so nearby source is not treated
as proof. The actual pinned provider observations supply the operation inventory.
The fixture observer requires a loopback-only network namespace and explicit
fixture configuration. Only the test harness selects it; the live launcher still
loads the production proxy and has no observer selector.

The synthetic minimal bucket preview exits successfully after these responses.
It does not establish the real release's complete diff, generated archive
agreement, current bucket configuration, conditional configured-resource reads,
or absence of further gaps elsewhere in a full release preview. Unexpected reads
still stop. Synthetic configuration is never adopted as a real baseline.

Verification: 77 execution/frontend tests; 12 isolated cases including real
preview, old-frontend SDK, context mapping and read-gap continuation; repository
`npm run verify` (format/lint/tests/build/Go tests and vet); Python Ruff lint and
format. Dependency fetching was disabled; all credentials/responses in fixtures
were synthetic. ACL negatives cover other bucket/owner, writes, body/framing,
extra/duplicate/aliased selectors, paths and versions. All ten proposed reads
remain blocked. Private evidence is retained; no raw request or credential data
is published. The historical DynamoDB transition proof was not repeated because
its core is unchanged.

Accounting stays **390/1,799**, at most **1,409** additional attempts, unchanged
cumulative **$1** procedural allowance (billing unmeasured). No AWS calls,
renewal, live preview, IAM change or later phase occurred. All stopped campaigns,
claims, ledgers, adopted ZIPs, rollback bytes and prior records remain intact.
No release-from-390 campaign, authorization or claim was created.

Proposed command only, currently held by the ten gaps:

```sh
python3 -B scripts/environment-deploy/driver.py start --campaign .build/project-environments-execution/release-from-390 --authorization .build/project-environments-execution/authorization-from-390.json
```

The next decision is whether to approve local implementation of exactly the ten
resource-scoped reads in the table. Approval must be followed by tested, pinned
tooling; a new initial-collection authorization is separately required before
preparing an approved bound authorization and running the command once. Retain
the 55-minute shared window, 240-second collection ceiling and original closed-stage
deadline rules. No live start or later phase is authorized by this correction.
