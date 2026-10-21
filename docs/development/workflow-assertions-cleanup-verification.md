# Focused cleanup verification and tooling correction

The authorized read-only verification passed for project
`2d9723c2-facf-40e1-8a2b-130853d3c0f8`. The
[receipt](workflow-assertions-cleanup-verification.json) and
[attempt ledger](workflow-assertions-cleanup-verification-attempts.jsonl)
record independently collected current state. H–L remain held. No deployment,
stage writes, identity creation, target requests, deletion replay or manual
draining occurred; A–G were not repeated. Application code, IAM, quotas,
concurrency and the pinned release are unchanged.

## Current AWS evidence

The bounded observation ran on 2026-10-21 from 16:40:55.229Z to 16:41:05.634Z
(10:40:55–10:41:05 America/Denver). All checks passed. The meter charged **20
AWS attempts**, comprising 19 reads and one existing native CLI authentication
refresh (`CreateOAuth2Token`), within the 100-attempt/600-second allowance.
The attempt-to-outcome interval was 10.056205 seconds. API calls, resource writes,
stage writes, identities created and target sends were zero. The isolated
verification profile cannot enable resource writes through its phase file.
The meter was stopped and its temporary CA removed after evidence capture.

- Account `747336059622`, non-root `davian-admin`, region `us-east-2`.
- Control contains only the recognized project tombstone. Its completed deletion
  operation `4527b106-f0f9-4cc8-8749-bb19fe34a934` matches the retained identity.
  The currently read durable completion timestamp is
  `2026-10-21T16:04:56.266916214Z`; it is not a reconstructed historical readback.
- Protected has no children; no upload tickets remain. The complete S3 prefix
  listing is empty. Permanent residual-sweep registration remains, as designed.
  This establishes current settlement/drain consistency, not missing individual
  historical upload transitions.
- Fresh stage reads match retained active version **15**, savedRequests **8**,
  assertions **1**, all other capabilities **1**, generation
  `dev-api-initial-v1`.
- Complete, identical accounting projections before and after inspection have
  hash `739508d26d9b62807892fac3d4a4254a66eb4ea563a2864fb69ca58f340c013a`.
  Monthly admitted usage remains **20**. Reserved storage is now **0**, pins and
  inflight are zero, and quota version is **48**. Retained evidence had version
  36 and 16,308 reserved bytes across twelve published captures. Normal ticket
  cleanup subtracts each ticket's bytes and increments quota version once,
  making these current values consistent with legitimate cleanup. Daily usage
  remains 2, 2 and 14 for the three owners with usage; the fourth has no row.

These observations establish current consistency. They do **not** prove every
historical intermediate state or reconstruct the failed final accounting
readbacks. Historical collector sources, failure receipts and ledgers remain
unchanged. First-attempt AWS totals and all actual billing totals remain unknown.
The historical legacy-A 503 cause and precise earlier held browser operation
remain unestablished. The six cumulative disposable identities and prior second
project cleanup retain their earlier proofs; this task did not re-query Cognito.

## Collector corrections

The shared collector library uses one explicit, consistently read accounting
projection at both ends, including schema and version. Snapshots reject missing
schema/version, unknown projected fields, duplicate identities, mismatched
projection definitions and invalid checksums. It never fills missing historical
fields. Changed counters or versions produce inequality.

Subprocess deadlines are floored to positive safe integer milliseconds, with a
cleanup reserve. Exhausted deadlines fail before execution; zero cannot become
an unlimited subprocess timeout. The corrected actual AWS CLI adapter was tested
through the real isolated meter and used for this live read-only collection.
The old collector failures remain independently reproducible in regression tests.

## Narrow browser correction and local verification

Only an empty Cognito CORS OPTIONS request to
`https://cognito-idp.us-east-2.amazonaws.com/` from the exact pinned frontend
origin `https://d2kpq905qyh9zg.cloudfront.net`, requesting POST, is recognized.
The requested header subset must contain `content-type` and `x-amz-target` and
may additionally contain only `amz-sdk-invocation-id`, `amz-sdk-request`,
`cache-control` and `x-amz-user-agent`. Duplicates, foreign origins/paths/query
strings, bodies, credential headers and unrelated requested headers are rejected.
This does not authorize any Cognito POST or broaden AWS write permission.
Every forwarded OPTIONS is an AWS attempt. Held requests retain only fixed-enum
method/category/operation diagnostics and a safe reason; unknown names become
`unknown`, with no credentials, request headers or bodies persisted.

The actual unchanged pinned frontend ran in Playwright through the same isolated
TLS proxy, without request interception or disabled CORS. A local provider
fixture exercised InitiateAuth → RespondToAuthChallenge → rotating refresh →
RevokeToken, including browser reload after sign-out and empty browser storage.
The meter counted **four OPTIONS and four POST attempts**, plus eight local API
attempts. AdminCreateUser was held before forwarding, and credential canaries
were absent from retained diagnostics. This uses simulated provider responses;
the fixture checks SRP challenge/proof shape but does not cryptographically
validate the password proof. It is not genuine AWS Cognito, signup or email
validation. No live identity or target was involved.

Local verification also covers SDK retries/pagination; real coordinator
503-to-200 polling; redirect rejection; cleanup reserve and cleanup-only scopes;
immutable read-only verification limits; network bypass denial; lost-ACK
non-replay; negative preflight cases; collector historical failures and corrected
integer timeouts. Repository formatting, lint, tests, builds and Go verification
passed before publishing this packet. The [local result summary](workflow-assertions-tooling-verification.json)
separates these fixtures from genuine AWS evidence. Reproduction commands:

```sh
npm run verify
python3 -m unittest discover -s scripts/attempt-meter -p 'test_*.py'
.build/attempt-meter-venv/bin/ruff format --check scripts/attempt-meter
.build/attempt-meter-venv/bin/ruff check --isolated --select E4,E7,E9,F scripts/attempt-meter
.build/attempt-meter-venv/bin/python scripts/attempt-meter/test_transport.py
.build/attempt-meter-venv/bin/python scripts/attempt-meter/test_transport.py preflight
.build/attempt-meter-venv/bin/python scripts/attempt-meter/test_transport.py cleanup
.build/attempt-meter-venv/bin/python scripts/attempt-meter/test_verification.py
.build/attempt-meter-venv/bin/python scripts/attempt-meter/test_browser_auth.py
```

Transport tests require the existing local meter environment/network namespace
permissions and retained pinned frontend files. They route only to local fixtures.

## Pins, remaining scope and Trello

The release manifest remains
`920667777eb69cabfb7800910451062a18e6ff06ce22ad4fb543f96c78321dd6`, source
`68d9f218225c7092a1e4fca0b66a21421d862a22`. API/cleanup ZIP hash remains
`f4be7af179049c70f479a45dd26e4ae24a19c3a9e8016a865318fa7a96965c32`;
worker/fixture ZIP hash remains
`15e7e6db438de3c19fe962c95be0d3c9d52d17f81b21a0b58cd4603974021379`.
Frontend release hash remains
`bd511e3f9979d2ef7dd6b73d183dcf7620a423b62659ac89c64e4a1a08981cde`.
Historical final receipt hash remains
`f8b20b3c63a0dff4e772ce6bec200415639770f94cefaa1d63e451ba9de829df`.
The new ledger hash is
`c7984a58bb150835436fc141bc82ce47f8e60f249c0f3613ca56b2813baca098`.

Carry forward **18 consumed steps, 16 sends and six identities/projects**.
Cleanup verification and local browser correction are complete; remaining live
validation can now be scoped specifically to H–L in a separate authorization.
No identity allowance, admission authorization or billing headroom is inferred.
Broad fault injection and PITR remain excluded.

Trello synchronization is unresolved: repeated bounded card/checklist reads of
[the card](https://trello.com/c/FkUYPLYV) returned no response, including a final
15-second attempt. No remote update or completion is claimed. The pending update
is: current first-project cleanup verified; collector and narrow browser meter
corrections locally verified; historical gaps retained; H–L held/incomplete;
no redeployment or A–G replay. Preserve existing incomplete runtime checklist
items when synchronization becomes available.
