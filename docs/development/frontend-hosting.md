# Frontend hosting implementation and deployment review

Davian accepted [contract `6b026a7`](../architecture/frontend-hosting-contract.md),
including the default AWS-hostname TLS policy, development-only Cognito sender
exception, release retention and cost allowances. Scoped implementation, local
verification, documentation synchronization, read-only AWS inventory and
existing-bootstrap-only SST preview were authorized. Davian then approved the
scoped development/demo deployment including correction `21c24be` and the
refreshed preview. The host is **https://d2kpq905qyh9zg.cloudfront.net**.
See the [deployment and genuine validation record](frontend-hosting-validation.md)
for performed checks, generated identifiers and Davian’s acceptance with disclosed
coverage limits, including untested hosted local-agent execution.
This is not authorization for a general-public signup launch.

## Implementation

`infra/frontend-hosting.ts` adds a StaticSite in existing `kurier/dev-api` using
the existing Ohio API and Cognito public client. It creates a dedicated retained,
protected private web bucket with public-access blocks, bucket-owner ownership,
SSE-S3 and a TLS-only, distribution-ARN-scoped CloudFront read policy. Evidence
and bootstrap buckets are never site origins. CloudFront uses the default AWS
certificate, PriceClass_100, static GET/HEAD, compression and HTTPS redirect.
It has no API behavior, custom domain, WAF subscription, Lambda@Edge or logs.
Its routing Function strips viewer credentials and rejects API/archive paths,
malformed paths and unknown file extensions. Missing hashed chunks return a no-store
404; application navigation uses the SPA shell without adding semantic
project routes.

The custom cache policy has min/default TTL zero, maximum one year, and no
cookies, authorization headers or query strings in its static cache key.
Nonhashed objects use `no-store, max-age=0, must-revalidate, stale-if-error=0`;
hashed assets use one-year immutable caching. Custom response headers enforce
the accepted CSP, HSTS, nosniff, DENY framing, no-referrer and disabled sensitive
browser permissions. Viewer-generated errors also carry safe headers.

The API uses `FrontendCORS` with exactly the generated CloudFront HTTPS origin
and its deliberate local origin. Existing `frontendOrigin` output and
`npm run dev:cloud` retain their local meaning. The new origin is exported
separately. Existing methods, request headers, ETag/Location/Retry-After exposure,
origin-less CLI authentication, no-store, JWT/ownership/scoped-capability checks
and Gateway-CORS omission are preserved. API routes, schema capabilities,
generation, quotas, runtime IAM and execution behavior are unchanged.

`apps/web/public-config.ts` validates the four allowed public values; hosted
Vite mode disables dotenv loading and rejects unsafe/missing values. The build
helper passes only those values and PATH/HOME to its subprocess, excluding AWS
credentials and SST/server secrets. No protected links enter the site. Source
maps and fixture login entrypoints are excluded. App startup has an external
script for accessible module-loading failures; history pagehide/restore clears
workspace/auth inputs and ends the memory session before stale UI can return.
No persistent auth/data cache or browser-localhost control is introduced.

## Read-only AWS and actual preview

The [sanitized preview record](frontend-hosting-preview.json) contains the
actual resource operations, changed-property names, public policies and
inventory fingerprints. Raw Pulumi state, encrypted environment records,
provider serialization and credentials are excluded from committed evidence.
The [public artifact manifest](frontend-hosting-artifact.json) lists every app
file's byte count, SHA-256 and cache metadata, the four public configuration
values, and the content-addressed release identity.

`scripts/preview-frontend-hosting.mjs --diff` verifies account `747336059622`,
non-root identity, installed/pinned SST 4.17.1, existing bootstrap **version 5**,
existing Ohio state/asset buckets and ECR registry, and existing stage
passphrase metadata/state before invoking only `sst diff --stage dev-api --json`.
It never invokes deploy, refresh, activation, account upgrade or bootstrap
creation. The [pinned SST bootstrap source](https://github.com/sst/sst/blob/v4.17.1/pkg/project/provider/aws.go)
has five steps; a version-5 parameter bypasses all mutation steps. The
[pinned diff implementation](https://github.com/sst/sst/blob/v4.17.1/pkg/project/run.go)
uses Pulumi preview; existing passphrase/state checks prevent first-run creation.
No domain/us-east-1 provider is added. Bootstrap parameter version/value and
stage/accounting are read again after preview; no bootstrap infrastructure or
product resources were deployed.

Inventory confirms active Free plan (recorded expiry 2026-12-29), account and
unreserved concurrency ten, no function reserved/provisioned concurrency,
worker SQS-only configured mapping/max2/batch1/standard pollers, enabled minute
maintenance, API 10 RPS/burst20 and absent Gateway CORS. Existing Cognito remains
Essentials/default sender/public SRP/one-day rotating refresh/ten-second grace.
Stage remains active/version11/schema7/MCP1 with generation
`dev-api-initial-v1`. No marker activation, counter reset or recovery action is
part of this change. Four consumers' runtime inline/attached/trust policy
fingerprints are recorded; preview has no IAM resource changes.

The actual plan adds the static bucket and controls/policy, cache/header
policies, distribution, routing Function/KeyValueStore, SST components and
deployment bookkeeping. It replaces four **code-object artifacts** in the
existing bootstrap asset bucket and updates four existing Lambda packages;
this does not replace the bootstrap bucket or any Lambda resource. Only the
API adds an environment variable, `KURIER_HOSTED_FRONTEND_ORIGIN`. Worker,
controlled fixture and maintenance have no planned environment/configuration
changes. API/maintenance share the updated API binary. Worker/fixture source
is unchanged; comparison found identical `resource.enc`, same Go 1.27.1 compiler
and changed VCS build metadata (`916ea23` live versus `6b026a7` worktree preview).
Their package bytes differ; do not claim binary equality. No Go dependency,
worker permission, trigger, concurrency, timeout or memory change is requested.

First-create preview limitations were explicit in the JSON: generated names,
policy IDs, distribution ARN and API origin environment are unknown until AWS
resolves them. StaticSite registers its AssetFiles/Invalidation providers inside
applies after the new bucket read resolves, so they are **deferred**, absent
from this initial diff. The review packet names their exact configured behavior
instead of claiming an exhaustive resolved first-create plan. Verify these
resources and all final IDs/configuration in the authorized deployment/readback;
stop for any change outside the reviewed additions and code updates. No
resource/route/schema deletion, IAM expansion or data-table change is expected.

## Deployment permissions and resource boundaries

No new IAM role/policy is defined. Existing non-root deployment access is reused;
browser and CloudFront receive no runtime role or API/evidence access. New S3
policy allows only CloudFront service GetObject on the new web bucket with this
distribution's SourceArn, plus the insecure-transport deny. A new customer key,
secret, network or compute resource is unnecessary.

Deployment needs the reviewed S3 bucket/control/policy and asset PUT/read/list
operations on the new web bucket, existing SST state operations and the four
existing function artifact prefixes; CloudFront distribution, cache/header
policy, Function publish, KeyValueStore keys and invalidation operations; and
existing scoped Lambda code/API-environment updates. Global/list/create actions
that AWS cannot scope to a pre-existing ARN remain deployment-principal actions,
never browser/runtime grants. No broader IAM administration, paid plan,
quota/concurrency or bootstrap-infrastructure permission change is proposed.
Old code-object replacements can delete superseded deployment ZIP objects in
the existing asset bucket; they do not touch evidence or application data.

The rollback archive correction leaves the app manifest/release identity unchanged:
`acb951896da16807815751c0509b1f25647656d8a0963932c7fc1f3a8ccac67e`.
The refreshed guarded preview adds no resources/IAM changes beyond the original
hosting proposal; its `releaseUpload` section lists the complete private archive
and shared hashed keys. All generated upload app bytes were checked against the
manifest locally. The deployment record separates actual remote recovery from the earlier fixture proof.

## Upload order, retention and rollback

The artifact builder validates root asset references, rejects symlinks, maps,
unexpected files and known secret/fixture markers, and bounds the app to 128
files, 8 MiB/file and 20 MiB total. These are deployment artifact checks, not API
input limits. Each release has an ignored local archive under
`.build/frontend-releases/<release-id>/app`, a generated site tree, and manifest.
The S3 site tree adds public `artifact.json` and private-origin
`_releases/<id>/{index.html,startup.js,404.html,artifact.json}` archives.
Every non-content-addressed app file is copied into its release prefix; hashed
JS/CSS remain shared at their manifest-listed root keys. CloudFront denies viewer
access to `_releases`; none of these archives contains protected data.

The [pinned uploader](https://github.com/sst/sst/blob/v4.17.1/pkg/server/resource/aws-bucket-files.go)
separates non-HTML and HTML batches and finishes the non-HTML batch first.
Thus hashed JS/CSS/startup/manifest uploads complete before the new shell and
404/archive HTML. Routing KV updates can happen alongside uploads, but retain
their stable namespace and root `/assets` routing; old hashed files remain.
`assets.purge=false` preserves old objects and KV entries. The uploader normally
deletes objects on provider deletion even with purge false; the scoped resource
transformation sets its `retainOnDelete=true`, and the bucket itself is protected
and retained. Invalidation depends on completed uploads and KV updates, uses
one `/*` wildcard path, and waits. Distribution deployment also waits.
Publication across edges is not atomic; old/new shells remain compatible with
the current API while propagation completes.

Before a future authorized release, keep current and previous operator-verified
known-good release IDs and manifests. Do not expire the whole bucket by age.
The retention planner:

```sh
node scripts/frontend-retention.mjs <current-id> <previous-known-good-id>
```

only reads this scoped web bucket and lists deletion candidates. It preserves
every release within 30 days, both known-good releases regardless of age, and
its complete nonhashed archive plus all hashed assets referenced by any
preserved release. Expired release candidates include every archived nonhashed
file and its manifest; shared hashed assets are candidates only when no retained
release references them. Incomplete/malformed
inventory fails closed. It never deletes anything; an operator must review
candidates and separately authorize deliberate cleanup. The existing KV store now
also has retained per-file hashed keys: the planner lists their corresponding
`kvDeleteCandidates`. Under separate cleanup approval, remove only expired-file
KV keys, invalidate and wait, then delete the reviewed S3 candidates. Leaving a
KV key for a deleted object would admit a request to S3's error path. Never
remove retained/current/previous keys. Existing scheduled
application maintenance gets no web-bucket permissions or new duty.

Concrete rollback, after deployment authorization:

1. Choose the prior **verified** manifest and confirm the same API/pool/client
   public configuration. Retain the current API/CORS-capable binary and all
   database/capability/accounting state.
2. Use the retained local `app` archive. If unavailable, run
   `node scripts/restore-frontend-artifact.mjs <id>` with deployed scoped outputs
   and read-only origin credentials. It retrieves `_releases/<id>/artifact.json`,
   every nonhashed file exclusively from `_releases/<id>/<path>`, and shared
   hashed assets from their root manifest keys. It verifies the selected release
   identity, current public configuration, every size/hash and complete artifact
   before atomically publishing `.build/frontend-releases/<id>/app`. It never
   uses the mutable root startup/404/shell, rebuilds, uploads or invalidates.
   Missing/corrupted bytes, malformed manifests or configuration mismatch stop
   recovery and leave no selectable local release. Existing local releases are
   refused rather than overwritten; inspect/quarantine an invalid local archive
   deliberately before retrying. Keep the independently verified release ID:
   hashes establish identity/integrity, not operator approval of a release.
3. Set `KURIER_FRONTEND_ROLLBACK_RELEASE=<64-hex-id>` for
   `node scripts/build-frontend-artifact.mjs` and then the guarded `--diff`.
   The helper validates the release/config/content hash and uses the preserved
   bytes rather than rebuilding. Review that no backend/config/schema changes
   accompany rollback. Current and old hashed objects coexist.
4. Obtain approval for the concrete rollback deployment; run only the scoped
   deployment using that release setting. Wait for distribution/invalidation
   completion; verify manifest, shell, old chunk availability and hosted auth/API
   smoke checks. Clear the rollback environment afterward.

Initial release has no previous hosted release: failure keeps localhost
development available, and approval must identify whether to correct the host
or disable its distribution. Do not remove the stack, revert data, close/reset
stage markers or replay executions to undo a frontend release.

## Refreshed costs and usage assumptions

No standing compute/networking, domain, SES sender, customer key or secret is
added. Actual artifact size is approximately 0.4 MB uncompressed, with one
JS/CSS pair and three nonhashed app files; the manifest supplies exact sizes.
Generated site upload adds four small archive files (all three nonhashed app
files plus the manifest) and a public manifest: ten uploaded objects per release.
Ten releases are roughly 100 object uploads before skips/retries, substantially
inside the accepted 2,000 PUT/LIST light allowance. The routing Function is
below its 10 KiB code limit; KV probes/misses and administrative operations
remain separately metered. No viewer-response Function is added.

Retain the accepted conservative monthly model: 1,000 visits/20k objects/1 GB
transfer/0.2 GB incremental S3/10 releases, versus 10,000 visits/200k objects/
10 GB/1 GB S3/30 releases. The contract's itemized gross estimates remain
**$0.2266 / $1.4730**, or approximately **$0.12 / $0.37** with otherwise unused
CloudFront transfer/request/function/KV-read allowances, before S3 allowances or
credits. Actual account-wide remaining allowances were not metered here.
Retain **$0.25–$2/month hosting** and **$1–$3 validation** allowances, not caps.
Extra new Essentials users screen at $0.015/MAU before allowances. API/polling/
execution traffic remains existing usage and can rise with public access.

Use ordinary pay-as-you-go CloudFront; do not enroll in its separate flat-rate
Free bundle or upgrade the account's AWS Free plan. Source rates and all usage
assumptions are [itemized in the accepted contract](../architecture/frontend-hosting-contract.md#incremental-cost-model).
The plan/credit expiry remains an operational constraint, not permission to
convert the account. No bill or hard spending ceiling was verified.

## Validation and gaps

Completed during this implementation:

- Infrastructure fixture tests cover strict public config, artifact hashes and
  exclusions, signed/scoped bucket policy, cache/header/method settings, retained
  uploader, path exclusions and 30-day/current/previous retention planning.
- A remote-only recovery regression creates A/B with different shell, startup
  and 404 bytes, overwrites live roots with B, removes A's entire local archive,
  reconstructs A using only simulated retained origin objects, verifies every
  manifest hash, and prepares rollback with zero build calls. Removing or
  corrupting each required object (including manifest/shared assets) fails
  closed without a partial release. Retention covers all archived nonhashed
  files and preserves shared assets referenced by retained releases.
- Go tests cover both exact origins, all five preflight methods/required
  headers, readable application errors, foreign/suffix/null/duplicate origins,
  unsafe configuration and preserved local-only adapter behavior.
- Frontend tests cover pagehide/history restoration clearing along with all
  existing auth/request/evidence/workflow/agent/MCP tests.
- Actual DynamoDB Local race suite and the existing Chromium-to-Go/DynamoDB
  feature harness passed. First invocations omitted the required explicit test
  endpoint and failed before meaningful integration; corrected runs passed.
- The production-asset Chromium fixture passed enforced CSP, signed-out deep
  links/root assets, keyboard/mobile layout, simulated A→B→A cache/rollback,
  missing assets/API 404, empty browser persistence and failed-module startup.
  It blocked AWS requests and observed zero; it is a local simulation, not a
  CloudFront or Cognito result.
- Guarded actual AWS inventory and SST previews passed with existing bootstrap
  and stage preserved. No AWS deployment/activation was performed.

Final `npm run verify` passed formatting/gofmt, lint, **58 frontend** and
**15 infrastructure** tests, TypeScript/Vite production build and tests/vet for
all seven Go modules. The history race regression rejects a sign-in callback
that finishes after pagehide. The guarded artifact rollback selector also
reproduced the retained release identity without rebuilding. The final preview
fingerprint and submission are recorded with the task commit/Trello.
A supplementary standalone whole-SST `tsc` check encounters existing installed
SST/Pulumi type errors and the historical root `@pulumi/pulumi` import; it is not
a configured passing repository gate. SST preview/bundling and project tests
remain separate checks, not a claim that vendor source type-checks cleanly.

The [deployment validation record](frontend-hosting-validation.md) supersedes
this packet's historical deployment hold and local-only status. On 2026-10-19
Davian accepted the development/demo slice and reported successful signup,
login/sign-out, password reset, request execution with redacted values, project
deletion and MCP. Hosted local-agent execution was explicitly not performed;
remaining planned cases were accepted without individual verification. The
record distinguishes these operator reports from automated/AWS checks and does
not claim elapsed refresh/expiry, full product/cross-engine/accessibility coverage
or independent physical cleanup of participant data. General-public signup/SES
readiness, abuse/load testing and metered costs remain separately scoped.

## Documentation synchronization

The native proposal's CloudFront/auth paragraphs and API contract's auth sender
rule/hosting continuation were updated in place under revision guards and read
back. They record acceptance, four public fields, exact CORS, memory-only auth,
development-demo sender exception, static-only CDN, retention/rollback,
local-agent/stdio boundaries, existing-bootstrap-only preview and deployment
hold. The deployment continuation was subsequently synchronized in place with
revision guards and verified readback: live URL, complete archive/remote rollback,
automated checks and participant coverage. The closeout synchronization records
Davian’s acceptance, reported successes and the deferred hosted local-agent check.
Unrelated content and milestone structure were preserved. Trello distinguishes
reported passes, acceptance without individual verification and unperformed work.

## Live missing-asset correction

A genuine unknown hashed-file probe exposed a 502: SST 4.17.1's default origin
is `placeholder.sst.dev`, so CloudFront's custom-error subrequest could not fetch
`/404.html` through the viewer-request origin switch. Known assets and the SPA
already worked. The correction updates only the existing routing Function and
existing KV resource: retain a per-file key for every published hashed JS/CSS
file, reject unknown hashed files with a no-store 404 before origin routing,
and preserve historical keys with `purge=false`. The namespace matches pinned
SST's app/stage/site MD5 convention. No new resource, IAM, origin, compute or
network was added. KV entry updates and asset uploads complete before waited
invalidation. Fixture coverage checks missing-file denial, preserved keys and
current-file admission; real HTTP coverage checks the 404 after deployment.

[AWS custom-error response routing](https://docs.aws.amazon.com/cloudfront/latest/APIReference/API_CustomErrorResponse.html)
requires the error page's actual origin behavior; it does not repeat the viewer
Function's origin override for that subrequest. Retained keys and objects must
therefore be cleaned up together in the deliberate order above. This guard adds
one KV lookup per hashed-file request; conservative accepted hosting/validation
allowances remain unchanged, and actual metered charges remain unverified.
