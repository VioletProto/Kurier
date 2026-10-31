# Preview denial and frontend phase correction — local review

[The machine review record](project-environments-preview-denial-review.json)
binds the findings, unchanged policy hashes, pending permissions and checks.

The retained failure is attributable to **S3 GetBucketPolicy for the owned
frontend bucket**, not the approved S3 Control tag read. The failed execution
gate condition is `s3-read-scope-held`. Local reproduction used the actual
`phase.mjs diff` entrypoint, pinned SST 4.17.1 and Pulumi AWS provider 7.40.0,
synthetic credentials and loopback responses. The request remained blocked
before forwarding. No AWS call, live preview rerun, renewal or live phase ran.

Tooling commit **19de0cc077c0652f61ccf9c70ad4e3744505fc07** also corrects frontend
phase expectations, adds private first-denial labels, and carries accounting
forward to **390/1,799**. Release source remains
**44f1fb79d943501c95e06fcc1a6114111a7c1ea4**. The remaining **1,409 attempts**
and unchanged cumulative **$1 procedural allowance** grant no live continuation;
billing remains unmeasured. All three stopped campaigns and consumed claims
remain preserved.

## Attribution and comparison with review 8

The original [390-call result](project-environments-collection-from-303.md)
reported the broad gate stop without attributing its operation. Subsequent local
inspection found the retained provider error in
`collect-diff/work/sst/log/pulumi.log` and its retained event log: operation
**S3 GetBucketPolicy**, the exact owned frontend bucket, and synthetic gate
status **599**. Only those reviewed conclusions are published. Raw diagnostics,
resource state, credential-bearing material and private capture hashes remain private.

Review 8's frozen read-only policy accepts S3 GET/HEAD on its known buckets.
That covers `GET /?policy` on the owned frontend bucket. The execution policy
instead allows root reads with no query or `location`, plus separately scoped
lists. A `policy` query falls through to `s3-read-scope-held`. Real SDK wire
serialization tests reproduce this old/new classification difference.

The approved S3 Control `ListTagsForResource` read is a separate operation:
exact account-prefixed us-east-2 host, exact encoded frontend-bucket ARN path,
empty body, reviewed headers and signing scope. Both policies call the same
`s3control_tags.allowed` helper. Its bytes are identical to successful review-8
tooling `1fd9812f8b974d527ba77e71ac78f7db63db276e`. Real pinned SDK serialization
passes both gates for that exact request; changed resource, query or body remains
rejected. No S3 Control change is justified by this failure.

The offline reproduction uses a minimal existing-bucket read to exercise the
same pinned provider bucket Read path through the actual preview entrypoint.
It reproduces the retained GetBucketPolicy/599 failure, the precise policy
condition and zero forwards of that denied read. All admitted synthetic reads
reconcile with loopback receipts; no writes or unmatched attempts occur. This
does not constitute a complete release preview or prove that no additional
provider reads will be required after the first denial is addressed.

**The execution wire policy, execution scope, old preview policy and S3 Control
helper are unchanged.** No missing read was silently allowed. The original
stopped campaign was not replayed or reclassified as successful.

## Frontend checks before and after deployment

The old collector always compared the deployed frontend against the adopted
release. Its recorded `frontendReady: false` therefore did not establish whether
the accepted old frontend was intact. It remains a historical observation;
this correction does not retroactively verify the old deployed frontend.

| Phase                   | Required frontend                                     |
| ----------------------- | ----------------------------------------------------- |
| collect, close          | Accepted old manifest and exact old file bytes        |
| verify, activate, final | Adopted release manifest and exact adopted file bytes |

The accepted old release ID is
`bd511e3f9979d2ef7dd6b73d183dcf7620a423b62659ac89c64e4a1a08981cde`,
from the preserved inventory/rollback manifest. The adopted ID is
`38325083b003b3f320752dbfa0357c377d57d2b66c1175e06c88ba3638210236`.
Checks compare the full marker, every expected file's length/hash, asset KVS
entries and deployed distribution status. Missing/denied/malformed/different
content fails closed. A phase cannot choose an arbitrary manifest from its input.

Collection and verification reject failed frontend readiness. Bundles name the
fixed target and expected release ID; offline stage planning checks both against
the requested action. Closure now requires the old frontend and rechecks it
before its CAS. Activation still requires the adopted frontend, backend readers
and its prewrite reread. The transport-free transition core remains guarded;
no live close, deploy, verify, activate or final action was exercised.

## Exact permission proposals — none granted

These are **operator execution-gate read permissions**, not IAM mutations or
deployment authority. The existing region/account/resource boundaries stay fixed.

Owned frontend bucket: `kurier-dev-api-web-fc2927a49a0b246240e5a17774`.
Existing KVS: `arn:aws:cloudfront::747336059622:key-value-store/0e7ac6e9-b4b6-4a9e-b673-4d3838436ecd`.

| Proposed read      | Exact resource/selector                               | Purpose                                |
| ------------------ | ----------------------------------------------------- | -------------------------------------- |
| S3 GetBucketPolicy | Owned frontend bucket, `GET /?policy`, no object/body | Provider existing-bucket preview read  |
| S3 GetObject       | Same bucket, `assets/index-CD_8s4hz.js`               | Fresh accepted-old-frontend byte check |
| KVS GetKey         | Existing scoped KVS, `76ef:/assets/index-CD_8s4hz.js` | Accepted old JS asset routing check    |

No other bucket, region, key, object version, ARN spelling, KVS entry, query,
body, mutation or generic bucket-read permission is proposed. Preserve all
current signing, framing, identity, attempt/deadline and no-retry checks. These
are the **known minimum gaps**; unknown later provider reads must still stop.
The old JS object and KVS entry are absent from the current adopted-release
upload/read sets, so the correct pre-deployment check cannot complete under the
unchanged scope. Skipping those bytes or comparing to the new frontend would
conceal that permission gap.

`check` and `template` report these exact `pendingScopedReads`. `start` refuses
before authentication, any claim creation or network work while the list is
nonempty. Editing an authorization JSON cannot bypass the policy calculation.
Approval of a future campaign alone is insufficient: permission adoption needs
a separately approved, tested policy/pin correction and a new clean tooling pin.
This revision intentionally grants no read permission.

## Private denial diagnostics

The proxy now retains the first request denial as mode-0600 `denial.json` within
its mode-0700 child directory. It contains only a fixed operation label, a
whitelisted condition label, `admitted` and `beforeForward` booleans. Unrecognized
operations/conditions become fixed `unclassified` labels. It reads no header or
body for diagnosis and records no raw host/path/query value, ARN, credential,
secret, exception text, request/response bytes or hashes. Repeated denials do
not overwrite the first record. The file remains outside provider mounts.

The actual preview reproduction verifies
`s3/GetBucketPolicy`, `s3-read-scope-held`, `admitted: false`,
`beforeForward: true`, mode 0600, no denied upstream forward, and no write.
Canary tests ensure headers, bodies, query values and unknown exception values
cannot enter the diagnostic. Recording failure remains terminal; no diagnostic
fallback forwards the request or opens a flow capture.

## Validation, preservation and proposed command

- **68 execution/frontend tests:** 17 policy/accounting, 22 credential, nine
  path, six policy/diagnostic and 14 frontend cases passed.
- **Ten isolated cases:** eight transport/accounting/byte/deadline cases, the
  actual context-entrypoint case and the pinned-provider preview denial passed.
- **90 transition tests passed with actual cached DynamoDB Local, zero skipped**,
  including old-frontend failure and prewrite drift making zero writes. The
  associated isolated Go integration suites and ten archive checks also passed.
- `npm run verify` passed with external networking/dependency downloads disabled:
  formatting, lint, tests, frontend build, Go tests/vet. Ruff lint/format passed.
  The final diagnostic delta was checked with its focused unit and actual
  preview-entrypoint reproduction after the broader checks.
- Local source/artifact/attestation/three-parent checks passed. The clean commit's
  template remained `authorized: false`, with three pending reads. No real
  `release-from-390`, authorization or window claim was created.

Original ledgers, claims, private provider logs/state, all adopted/rollback bytes,
frontend manifests, archive hashes, historical records and worktree registrations
remain retained. Each fixture run uses a fresh private directory. No `.build`
removal, worktree pruning, dependency install, archive rebuild, cache replacement,
cloud repair or live continuation occurred.

The revised [execution packet](project-environments-deployment-execution.md)
publishes the future command for review:

```sh
python3 -B scripts/environment-deploy/driver.py start --campaign .build/project-environments-execution/release-from-390 --authorization .build/project-environments-execution/authorization-from-390.json
```

**Do not execute it under this request. This tooling will refuse it while the
read permissions remain pending.** After explicit permission approval, review
and pin the exact implementation before separately authorizing initial collection.
Carry all **390** prior attempts, the **1,799** ceiling, original **55-minute**
campaign deadline, **240-second** collection ceiling and unchanged cumulative
**$1** allowance forward. No later phase, runtime target or rollback authority
is implied. Fresh complete diff, ZIP agreement and deployment readiness remain
unestablished.
