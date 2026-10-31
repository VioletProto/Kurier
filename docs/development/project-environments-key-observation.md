# Authorized key observation — empty bindings authenticated

**The one authorized observation succeeded and stopped.** All four retained
review-8 `resource.enc` members authenticated with the existing SST key and
matched the required empty binding map. No credentials or unrelated resource
bindings occur in those decrypted maps. This is a safe validation conclusion;
no key, plaintext, full Lambda environment or key fingerprint is published or
retained. Core dumps and SDK logging were disabled for the live process.

Actual usage is **299/500**: 296 prior plus exactly three additional physical
attempts. Same cumulative **$1 procedural allowance**; billing remains unmeasured.
Candidate adoption is **still pending review**. Both manifests, all retained
ZIPs/bootstraps/frontend/clients, original proofs and rollback bytes are unchanged.
No deployment, closure, activation, runtime request or automatic continuation ran.

## Offline implementation and checks before execution

Clean observer tooling was committed as
`7b77598082529a678bf8962b6371c43673ac25af`. Its SDK transport is separate from
the unchanged read-only preview policy. One synchronous process and one durable
ledger span existing-session authentication, identity verification and the exact
Lambda read. The 600-second original deadline begins before the first possible
live call and also bounds offline validation.

The actual prepared request destination is checked before admission/forwarding.
The only allowed sequences are STS → exact Lambda read, or one existing-session
refresh → STS → exact Lambda read. The identity must match the inventoried
non-root identity and account 747336059622 before Lambda work. Region us-east-2,
TLS certificate verification, no proxies/default credential chain and one SDK
attempt are enforced. The physical HTTP pool disables retries/redirect following.
A second refresh, extra STS/read, query, alias/qualifier, other function/region,
mutation, body or unexpected destination stops before forwarding. The final
allowed response can be parsed at ordinal 299; a fourth attempt cannot forward.

The only Lambda target is the inventoried
`kurier-dev-api-UsersProjectsApiFunction-werhekow`, using GetFunctionConfiguration.
No parameter, state object, other function, alternate key or other resource was
fetched. The CLI's existing same-identity login cache is the only authentication
cache that may be refreshed; no IAM/application identity was created.

**17 observer tests passed with external networking disabled**, using the real
pinned SDK serializer and mocked physical sends/synthetic keys. Coverage includes
cached and refreshed sessions, exact wire destination, prior/shared count,
identity gating, retry/redirect suppression, failed/uncertain transport accounting,
a blocking-transport deadline interrupt, exhausted limits, unknown operations,
missing/invalid key, validation failure, cleared response/key buffers, sanitized
diagnostics, ledger failure and exclusive claim/output replay rejection.
`npm run verify` passed offline (format, lint, tests, builds and Go test/vet),
as did Ruff and the local original-evidence/artifact preflight. No AWS call was
made until these checks passed. The archive checker's prior synthetic GCM/content
regressions remain relevant; the following observation supplies the real proof.

## Historical command and actual accounting

The exact once-only command was:

```sh
python3 -B scripts/observe-environment-bindings.py observe \
  --review-commit 7b77598082529a678bf8962b6371c43673ac25af \
  --output /home/king/Neumont/Kurier/.build/project-environments-key-observation/review-1
```

**Historical evidence; do not rerun.** The consumed `window-from-296.json` claim
and exclusive output files are retained. A fresh output path cannot reset or
reuse this baseline. The source remains
`44f1fb79d943501c95e06fcc1a6114111a7c1ea4`.

| Observation                                      | Result                                                     |
| ------------------------------------------------ | ---------------------------------------------------------- |
| Window began                                     | 2026-10-29 19:38:12.007784 UTC                             |
| Final validation receipt                         | 2026-10-29 19:38:13.280835 UTC                             |
| Elapsed through final receipt                    | 1.273 seconds of 600 allowed                               |
| Existing-session refresh                         | Ordinal 297, HTTP 200, one attempt                         |
| Required STS verification                        | Ordinal 298, HTTP 200, same account/identity verified      |
| Exact GetFunctionConfiguration                   | Ordinal 299, HTTP 200, one attempt                         |
| New / cumulative                                 | **3/3; 299/500**                                           |
| Denials / failed transports / unmatched attempts | 0 / 0 / 0                                                  |
| Real encrypted binding validation                | All four authenticated and matched expected empty bindings |
| Launcher exit                                    | 0                                                          |

All attempts were durably recorded before forwarding; every unique ordinal has
one HTTP outcome. Every receipt falls within the same original deadline. No
retry, pagination, alternate key search, repair or post-result AWS inspection
occurred. The remaining **201 attempts** are arithmetic headroom, not authority.
Earlier uncertain remote refresh outcomes remain uncertain; their ledgers/claims
are unchanged. Frozen observer tooling retains its 296 parent; these receipts
establish actual 299 and must seed any separately reviewed future accounting.

## Sensitive response handling and artifact conclusions

GetFunctionConfiguration necessarily returns a configuration response that the
SDK briefly parses in private memory. The observer checks exact returned function
name/ARN, extracts only SST_KEY, clears the variable/environment/response maps,
base64-decodes the existing key, and passes it directly to the offline archive
checker. It verifies GCM authentication and exact expected content for each ZIP.
The mutable key buffer is overwritten and the process exits after returning only
boolean conclusions. There is no claim of guaranteed erasure of every Python,
SDK or cryptographic-library memory copy; no such material was persisted, logged,
passed to a subprocess or placed in a core dump.

The observation directory contains only a sanitized attempt ledger and boolean/
accounting result. Its parent contains the consumed sanitized campaign claim;
the launch log contains the same sanitized result. No key file, environment dump,
plaintext, credential-bearing response, private capture or key digest exists in
these observation outputs. Only these reviewed non-sensitive records/hashes are
published. The public ledger uses an explicit field projection.

This resolves the binding-content uncertainty in the earlier readiness packet.
It supports the intended empty links from pinned source and the conclusion that
the added member has no credential/unrelated binding content. Combined with the
previous exact-member and clean-bootstrap checks, no unexpected archive file or
added binding is identified. The Go applications' non-use of the SST SDK loader
and their explicit KURIER environment path are unchanged.

The exact proposed ZIPs remain:

- API/cleanup: `58f2bbf364042ac209d4f9b3a4d6ca583da18914fb5851f0c166d34b9c757373`.
- Worker/controlled endpoint: `82af7beeda4b619afa6831b342c693bb1fb3e8d08e5e6c0416ce253526f59098`.

Original prepared manifest SHA-256 remains
`94d658f4cdae67103c755e3cf5578a50252f8f780c2df82c46c2c2cf18f8c6c5`;
proposed SST manifest remains
`25ef3cd0c8f0c09037607c564bfb8964c1ef8e8e14a3aa0ae8757894918895a9`.
Its historical unvalidated/pending-adoption fields were **not edited**. This
separate attestation binds the existing manifest/archive hashes and records the
new verified fact. It does not silently promote the manifest, enable the transition
core or resolve the other deployment-readiness blockers.

## Publication and next decision

[Machine result](project-environments-key-observation.json),
[sanitized attempt projection](project-environments-key-observation-attempts.jsonl)
and [manifest](project-environments-key-observation.sha256) record the proof.
Original and review-8 evidence hashes, both manifests, all release/rollback bytes
and clean release source were rechecked locally after execution. No raw diagnostic
capture or its hash is included. Code/tooling and test receipts are reviewable
on PR #18.

The next decision is whether to adopt the exact candidate artifacts using this
attestation. The separately reviewed live metered deployment/stage adapter,
upload-boundary byte gate, fresh approved reader/state/IAM/accounting baseline,
viable execution allowance and explicit release/runtime authorization remain
required by [the readiness packet](project-environments-final-readiness.md).
No further AWS command is proposed or authorized here. Deployment, closure,
migration, activation, identity/project creation and live targets remain held;
production OpenAPI scope stays separate. No successful observation implies
continuation.
