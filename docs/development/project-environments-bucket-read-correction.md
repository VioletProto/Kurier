# Approved frontend bucket configuration reads

Tooling: **9eafa58928b5a2486f30bd7531f18c324d4fe534**. Release source remains
**44f1fb79d943501c95e06fcc1a6114111a7c1ea4**.
[Machine results](project-environments-bucket-read-correction.json).

Implemented exactly the ten reads approved from the
[ACL review](project-environments-acl-read-correction.md) at
`69f31efcfe42097353d24c33b54d93c5dabd1c17`. The old review is preserved unchanged
and separately hash-pinned as the approval basis. The prior four additional-read
grants remain; no other operation or mutation is added.

Each new grant is restricted to frontend bucket
`kurier-dev-api-web-fc2927a49a0b246240e5a17774`, account `747336059622`, region
`us-east-2`, host
`kurier-dev-api-web-fc2927a49a0b246240e5a17774.s3.us-east-2.amazonaws.com`,
canonical root GET, exactly one empty selector and an empty body. Both `?selector`
and the SDK's equivalent `?selector=` form are accepted. The raw query must match;
extra separators, duplicate/extra/aliased selectors and nonempty values fail.

| Granted operation                | Exact root request     |
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

Existing signing, TLS/identity verification, framing, deadlines, durable admission,
shared accounting, session limits and artifact-byte checks are unchanged. Wrong
expected owner, other buckets/regions/hosts, object paths, range/conditional/payer
headers, body/framing changes and writes remain held. No wildcard match, IAM
permission change or live execution authority is introduced.

The actual `phase.mjs diff` entrypoint with pinned SST 4.17.1 and provider 7.40.0
now exits **0** through the amended **production proxy**, without loading or
patching in the test-only observer. The synthetic origin supplies absent/default
bucket-configuration responses only after production admission. All **22 local
fixture forwards** reconcile with durable attempt outcomes; twelve bucket reads
(policy, ACL and the ten above) each occur once. No mutations, unmatched attempts
or denial occur. A separate actual SDK child attempts unapproved GetBucketTagging:
only its STS fixture call forwards, and the bucket read stops before admission.
The old-frontend SDK fixture still passes and then blocks an unexpected object.

`check` reports `pendingScopedReads: []`. **No further gap was observed in this
minimal existing-bucket fixture.** This is not a full real-release preview and does
not establish current cloud configuration, conditional configured-resource reads,
complete resource diff, archive agreement, quiescence or readiness. Further unknown
operations still stop before forwarding; no automatic widening is allowed.

Verification passed: **80** execution/frontend tests (17 policy/accounting,
22 credential, nine path, 18 policy/SDK/diagnostic and 14 frontend); **12** isolated
transport/preview/context cases; repository `npm run verify` (format, lint, tests,
build, Go tests and vet); Ruff lint/format; retained artifact/evidence/source pins.
Every new operation has actual SDK serialization positives and negatives for
changed bucket, owner, method, body, selector, path and framing. Wildcard scope
entries and unreviewed bucket reads/writes fail. Dependency downloads and external
networking were disabled. An initial repository verification harness failed to
open its read-only `/dev/null`; the rerun used private proc/dev mounts. That failed
run and successful verification evidence are retained. Historical DynamoDB
transition tests were not repeated because their core is unchanged.

Accounting remains **390/1,799**, at most **1,409** additional physical attempts,
unchanged cumulative **$1** procedural allowance, billing unmeasured. Synthetic
fixture attempts do not alter live accounting. No AWS call, renewal, live
collection, IAM change or later phase ran. All stopped campaigns, consumed claims,
ledgers, private evidence, adopted ZIPs and rollback bytes are preserved. No
release-from-390 campaign, authorization file or window claim was created.

The next decision is separate authorization for **one initial collection** with
this exact clean tooling pin and unchanged source. Preserve the original 55-minute
shared deadline, 240-second collection ceiling, at most one reviewed existing-session
renewal if needed, mandatory STS verification, no background/mid-phase renewal,
and stop after collection. Closed-stage deadlines and all later-phase holds remain.
Prepare and review the false bound template using the execution packet's local
commands only after that decision; do not alter history or reuse a consumed claim.

Exact proposed command, **not executed or authorized by this local task**:

```sh
python3 -B scripts/environment-deploy/driver.py start --campaign .build/project-environments-execution/release-from-390 --authorization .build/project-environments-execution/authorization-from-390.json
```
