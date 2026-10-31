# Resource-scoped S3 Control tag-read amendment — review only

**Local evidence establishes one required bucket ARN for the observed stop.**
The amendment permits only its canonical GET ListTagsForResource request through
the already recognized account/region endpoint. It does not grant a generic
S3 Control REST permission or change IAM. No AWS call, refresh, probe or preview
ran during this preparation. Accounting remains **263/500**, with **237** attempts
of arithmetic headroom and the same cumulative **$1 procedural allowance**;
billing and remaining dollar balance are unmeasured.

Release source remains `44f1fb79d943501c95e06fcc1a6114111a7c1ea4`.
Review tooling: `1fd9812f8b974d527ba77e71ac78f7db63db276e`. The historical review-7 result and every
original ledger, claim and release/rollback artifact are retained unchanged.

## Establishing resource scope locally

The operation-name receipt alone did not establish an ARN. This investigation
instead combined retained errors and inventory/state identifiers with the
pinned provider's actual bucket branch and its SDK serialization contract:

| Evidence                                                                | Locally established finding                                                                                                                                                                     |
| ----------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Retained review-7 Pulumi diagnostics, inspected privately               | Two tag-read diagnostics identify `FrontendWebAssets` and the same bucket identifier already disclosed in inventory. Their raw text is not published.                                           |
| Inventoried bucket policy and retained legacy Bucket / BucketV2 outputs | The bucket identifier, ID and ARN agree: `arn:aws:s3:::kurier-dev-api-web-fc2927a49a0b246240e5a17774`. Its bucket region is `us-east-2`.                                                        |
| Pinned provider `service/s3.(*servicePackage).ListTags`, Bucket branch  | Constructs an ARN with AWS-client partition, service `s3`, empty region/account, and the bucket identifier as Resource. The result is the legacy global bucket ARN, without a `bucket/` prefix. |
| That branch's S3 Control call chain                                     | Supplies the separately obtained account ID and constructed ARN to `service/s3control.listTags`, then SDK `(*Client).ListTagsForResource`.                                                      |
| Pinned SDK serializer and installed model                               | GET `/v20180820/tags/{resourceArn+}`; required URI ResourceArn and `x-amz-account-id` header; no modeled query or request-body members.                                                         |
| Offline model serialization with synthetic credentials and mocked send  | Produces the canonical encoded path below, GET and an empty body. This is an offline wire-contract fixture, not replay of the historical request.                                               |

The one reviewed ARN is therefore:

```text
arn:aws:s3:::kurier-dev-api-web-fc2927a49a0b246240e5a17774
```

The global S3 ARN has no embedded account or region. Those restrictions must be
checked independently at endpoint, account header and signing scope. The other
three inventoried bucket names do **not** establish permission for this stopped
read and are denied, including SST state and asset buckets. No wildcard, object
ARN, regional/account-qualified variant or unrelated inventoried ARN is added.

The provider executable is retained locally with SHA-256
`741bfe58a4f6100dfa839beb98d53e0f27c9b63998d53a293cbbfb7a6b23a797`.
It embeds provider AWS 7.40.0, SDK Go v2 core 1.43.0 / S3 Control 1.73.0 /
Smithy Go 1.27.3. Full exact-version Go source was unavailable locally; the
critical construction was traced without executing the provider, using its ELF
Go function table and disassembly:

- Bucket branch `ListTags`: entry `0x1bd143a0`; `Bucket` literal comparisons at
  `0x1bd14472`/`0x1bd1447e`; S3ControlClient call at `0x1bd144c5`.
- The ARN temporary is zeroed, partition restored from AWSClient, service `s3`
  assigned at `0x1bd14558`, identifier copied at `0x1bd14573`–`0x1bd1458b`;
  region/account remain zero. String assembly calls `runtime.concatstrings`
  at `0x1bd146ef` with the `arn:` and colon literals.
- Function value at `0x3aac6498` resolves through `0x22c24ef8` to
  `s3control.listTags` at `0x1bbf57a0`, which calls the named SDK operation.
- SDK HTTP binding serializer entry `321301504`, end `321302400`, uses URIValue
  binding for ResourceArn. Installed botocore 1.43.108 independently describes
  the same operation members/method/path; its offline serializer fixture checks
  exact encoded path and headers before any send.

Adjacent cached Smithy/ARN sources helped interpret the ABI but are not claimed
to be byte-identical pinned source. The historical raw request ARN was not
captured; this exact scope is derived from the pinned branch's deterministic
construction, the diagnosed bucket identifier and retained inventory/state.
No new private resource-value capture is necessary for this proposal. No raw
operation/hostname receipt or its hash is included in public evidence.

## Narrow request gate

The dedicated `s3control_tags.py` helper is called only in the S3 Control branch
before admission. Its allowed request is:

```text
Endpoint: https://747336059622.s3-control.us-east-2.amazonaws.com:443
Method: GET
Path: /v20180820/tags/arn%3Aaws%3As3%3A%3A%3Akurier-dev-api-web-fc2927a49a0b246240e5a17774
Query: absent
Body: exactly empty
Account header: x-amz-account-id: 747336059622
Signing scope: <date>/us-east-2/s3/aws4_request
```

Path comparison is exact, without URL decoding or normalization. Raw colons,
lowercase escapes, double encoding, encoded aliases, extra/trailing slashes,
object suffixes, dot segments, fragments, any query delimiter/parameter and
other ARNs are denied. Endpoint aliases, wrong account/region, trailing dots,
non-TLS and non-443 traffic remain blocked. All mutation methods and other
S3 Control operation paths remain blocked before forwarding.

Headers are allowlisted, ASCII/control checked, bounded and unique ignoring
case. Required: Authorization, `x-amz-account-id`, `x-amz-date`, and nonempty
`x-amz-security-token`. Host must match when explicitly present (HTTP/2 may
supply authority separately). SigV4 structure must use the reviewed region and
S3 signing service, date matching the request header, and sorted unique signed
headers including host, account, date and session token. Signature syntax is
checked; AWS remains responsible for cryptographic verification.

Optional headers are limited to Host, User-Agent / X-Amz-User-Agent,
Amz-Sdk-Invocation-Id / Amz-Sdk-Request, X-Amz-Content-Sha256,
X-Amz-Api-Version, Content-Length / Content-Type, Accept / Accept-Encoding and
Connection. Length is zero; API version is 2018-08-20; content type XML;
payload hash empty-SHA256 or UNSIGNED-PAYLOAD with independently empty body;
transport values, UUID and attempt/max/ttl metadata have narrow checks.
Unexpected parameters/headers, duplicate headers, transfer encodings and
nonempty bodies fail closed. No header values, credentials or bodies are
recorded by this gate. An unfamiliar legitimate SDK variant will also stop;
there is no automatic policy repair.

There is no S3 Control entry in the generic REST, JSON or Query allowlists.
A regression compares the complete policy with the held parent and allows only
the dedicated helper import/conditional. Authentication transport, corrected
Sign-In endpoint, same-session restrictions, TLS verification and mandatory STS
identity gate are unchanged. The supervisor, attempt admission, retry accounting,
shared deadline, claim validation, redirect/error handling and descendant
shutdown are unchanged. Any unexpected request still terminates the campaign.

## Local verification and preservation

**90 unit tests and 26 offline process/transport scenarios passed**, with
external networking disabled. The new scope tests cover modeled serialization,
exact resource/endpoint, methods, body, query, header duplication/selectors,
SigV4 account/region/date binding, optional metadata and ambiguous encodings.
Production-supervisor fixtures forwarded the approved read only to a local
origin after a durable typed receipt; a simulated 503 retry consumed a second
attempt. Both hit their bounded call limit and stopped root/child/grandchild.
Correct-ARN requests with a wrong account header or a body produced zero
upstream requests and stopped descendants. Existing shared SST/Pulumi/Go/Node,
pagination, deadline, redirect, failed transport, write denial, ledger failure,
shared-handoff and single-use tests also passed. The fixture temporary directory
now lives through every scenario, preserving its Go client until teardown.
`npm run verify` passed offline (format, lint, tests, builds and Go test/vet);
Ruff lint and format checks passed. These are local fixtures, not AWS acceptance
of the ARN/header profile or a completed preview.

The accounting chain now verifies **74** frozen non-capture proof files through
review 7 and requires the original 231-based run's ordinals 232–263, successful
identity gate and terminal S3 Control hold. Original failed claims and both
uncertain historical remote refresh outcomes remain intact. Synthetic fixture
calls never enter real ledgers. No window-from-263 claim or review-8 directory
has been created.

`valid_parent()` rehashed prepared ZIPs, bootstraps, clients, frontend files,
retained rollback ZIPs/bootstraps and CloudFront function against the existing
manifest. The clean detached release source is unchanged. Release manifest
SHA-256 remains `94d658f4cdae67103c755e3cf5578a50252f8f780c2df82c46c2c2cf18f8c6c5`.
No release packaging ran; there are no release/rollback artifact differences.
The new policy/helper/accounting tooling hashes are in the
[machine packet](project-environments-listtags-resource-review.json) and
[public file manifest](project-environments-listtags-resource-review.sha256). No IAM/resource changes or new deployment diff are
claimed. The review-7 error-time 8 create/97 same summary remains incomplete.

## Exact proposed execution — do not execute without review and authorization

In the original workspace containing the preserved private proofs and pinned
local dependencies, begin from this clean exact tooling commit:

```sh
cd /home/king/Neumont/Kurier
git switch --detach 1fd9812f8b974d527ba77e71ac78f7db63db276e
python3 -B scripts/preview-meter/run.py refresh-preview \
  --review-commit 1fd9812f8b974d527ba77e71ac78f7db63db276e \
  --source /tmp/kurier-environments-release-source \
  --output /home/king/Neumont/Kurier/.build/project-environments-preview/review-8
```

No diagnostic-capture flag is needed. The proposed single new window is
**60 minutes, at most 237 additional physical AWS attempts**, starting before
any authentication call, carrying prior **263**, ceiling **500** and the same
cumulative $1 procedural allowance. Existing-session refresh if needed and
mandatory STS identity verification consume that budget before provider calls.
Retries, pagination and failed transports consume attempts; a denied request
is not forwarded or charged as an AWS attempt. No repeated refresh, automatic
restart, repair or automatic continuation is proposed.

The next decision is whether to authorize this one exact resource-scoped read
and a new bounded campaign using the above reviewed tooling/command. No live
work is authorized by this packet. IAM changes, deployment, stage closure,
migration, activation, identity/project creation, target requests and production
OpenAPI work remain held/separate. Stop after preview success or failure and
publish sanitized totals, any completed diff and artifact comparisons.
