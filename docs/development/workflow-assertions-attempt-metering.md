# Replacement attempt metering

Davian accepted replacement packet `e702f434eab1837cecc5ccfab100b861067b39e5`
and conditionally authorized its rollout after complete operator attempt metering
passes local verification. This tooling changes no application source, release
manifest, pinned ZIP, frontend artifact, AWS resource, quota, concurrency or IAM.
The accepted activator still supplies the close/activation guards and separate
fresh approval records. Meter phase selection does not replace those guards.

## Coverage and limits

`scripts/attempt-meter/run.py` launches every live operator child in a Linux
network namespace with no external route. A Unix-socket relay is its only network
escape, leading to a single TLS-inspecting request proxy. CLI, Go/Pulumi, Node/SST,
SDK pagination, SDK retries, native fetch, Chromium and their children therefore
either pass the meter or fail without reaching an external network. Node's
preload explicitly replaces SDK-created agents, which otherwise bypass its
environment proxy support. Proxy upstream certificate verification remains on.
HTTP/2 streams are individually metered; unproxied UDP/QUIC cannot escape.

Each request is synchronously appended and fsynced **before forwarding**. Each
page, retry, failed request and authentication request is a separate attempt.
All operator AWS HTTP attempts, including writes and token refresh, conservatively
consume the 1,500-read allowance; this avoids undercounting reads through an
operation-classification mistake. Application API requests consume the 500-call
allowance, including browser/anonymous/MCP/cleanup requests. Status polls also
consume the 240-poll allowance and require two seconds between requests to the
same status resource. The 120-minute monotonic deadline starts at the first
admitted request, including live preflight. Static frontend reads have a further
200-request bound. Counters cannot be reopened or reset in an existing directory.

Only the fixed application API and frontend origins and AWS service origins are
accepted. Direct fixture requests, unknown origins and insecure requests stop
execution. Default inspection holds resource writes; its sole authentication
exception is native CLI token refresh. Explicit phases allow only the reviewed
operation classes. IAM, organization, quota and free-tier writes remain blocked.
An uncertain write response or lost acknowledgment latches a stop before a
client retry can pass. Client AWS retry count is additionally set to one.
The proxy performs no HTTP replay. Remaining guards and the coordinator stop on
unexpected outcomes; unused allowance never authorizes a replay or reopening.

The ledger stores sequence/time, operation name, category, protocol, counters
and response status only. It does not store hosts/URLs, query values, bodies,
headers, credentials, raw evidence or proxy flows. CA private material is kept
in the ignored mode-0700 runtime directory and removed on normal server exit.
TLS key logging is disabled. Fixture passwords and access/refresh/MCP tokens
remain in coordinator/browser memory. The official AWS CLI may update **its one
existing protected login-cache file** to retain rotated operator credentials;
task artifacts never copy them. The rest of the home filesystem is read-only.

These are operator transport counts. The application's internal service calls
and scheduled maintenance are covered by the bounded campaign's cost allowance,
not claimed as measured operator AWS reads. Billing is delayed: the additional
$1 remains a procedural allowance, not an enforced AWS spending cap or a verified
bill. Historical AWS-read and billing totals remain unknown. Historical two
admitted steps and two audited sends remain consumed.

## Local verification

- Ten budget tests pass: before-forward persistence, retry/page counting,
  read-only default, latched stops, failed-read attempts, uncertain-write replay
  rejection, poll spacing, deadline, exact 500/240/1,500 ceilings and secret-free
  receipts/direct-target denial.
- The controlled-origin transport regression runs actual AWS CLI, Go AWS SDK and
  Node AWS SDK clients. Each retries one 503 and reads two pages: **nine attempts**
  reach the local origin and exactly nine AWS attempts are recorded. Native fetch
  plus Chromium generate **three application API attempts**, including HTTP/2.
  Direct socket bypass returns `ENETUNREACH` inside the namespace.
- The local origin accepts one write then loses the acknowledgment. It observes
  exactly **one** write; a subsequent client attempt is blocked by the latched
  meter. No request material or private canary appears in the ledger.
- All upstream traffic in these tests is mapped to localhost. They make **zero
  AWS, Cognito or live target calls** and do not establish deployed behavior.
- Six release-pin checks pass against the unchanged archives and manifest.
  Repository format/build/lint/tests and Python lint/format checks pass.

Tool-only dependencies are pinned in `scripts/attempt-meter/requirements.txt`;
they are installed in an ignored virtual environment, not application packages.
Run budget tests with `python3 -m unittest discover -s scripts/attempt-meter -p
test_budget.py`; run transport verification using that environment's Python and
`scripts/attempt-meter/test_transport.py`. Transport verification requires local
socket/network-namespace permission and installed SDK/browser fixtures.

## Campaign placement and cleanup capacity

Use the same two approved identities and projects, with A–G in the first owner's
project and H–L in the second owner's project. Check foreign ownership in both
directions. No extra identity, project, request mode or campaign step is added.
Any single deterministic two-step reserve belongs to the second project.

`TestIntegrationReplacementCampaignCleanupTicks` uses the actual local workflow
submission, processing and project drain methods, DynamoDB Local and controlled
object/HTTP fixtures. Its conservative maximum shape includes the reserve:
**26 replacement admissions, 22 sends**, four MCP evidence reads and revocation.
The two projects drain in **19 and 15 scheduler passes**, respectively, leaving
one minute in the reviewed 20-minute cleanup reserve. Stored objects disappear,
usage remains 26 and cleanup sends no HTTP. This is local component capacity
evidence; it does not prove a live scheduler meets its timing obligation.

The cumulative campaign remains 28 maximum admissions and 28 absolute sends;
the reviewed sequence permits only 24 cumulative sends with reserve, including
the two historical sends. A–L alone leaves two admitted reserve steps. Stop new
admissions by minute 100 and earlier if the remaining window/read/API allowance
cannot cover the next phase and normal cleanup. Retain at least 200 AWS attempts
and 60 application API calls for cleanup; observe project drains concurrently
with bounded reads. No manual drain, durable-row repair, automatic reopening,
broad live fault injection or PITR exercise is authorized.

Fresh live preflight, deployment, activation and runtime/cleanup results must
be published separately from these local results and previous operator reports.
