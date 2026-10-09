# Accepted architecture

## Proposed cloud execution continuation

The [single saved-request cloud execution review packet](cloud-execution-evidence-contract.md)
is **Proposed**, not an accepted contract or implemented behavior. It records
execution/evidence routes, schemas, redaction policies, protected-input freezing
and preparation results for review. Davian found the design acceptable subject
to clarifications; source-bound submission/rerun idempotency and evidence delivery
encoding, prepublication API/Lambda size checks and escaping-test requirements
are now explicit and await clarification review. Publishing the packet does
not authorize dependent implementation or AWS deployment while review is pending.

## Accepted W1 baseline

The concrete [W1 system design](w1-system-design.md),
[DynamoDB model](w1-data-model.md), and
[decision sheet](w1-review-decisions.md) were accepted by Davian Hernandez at
commit `1fba942`, including their documented defaults. ADR 0002 is Accepted.
They distinguish repository foundations and completed spike evidence from
planned product behavior. Acceptance does not authorize deployment or
implementation. The [synchronization record](w1-acceptance-sync.md) records
external updates and genuinely new contract choices still proposed for review.

Kurier's architecture is a planning baseline. It should evolve when spikes,
security review, or vertical-slice delivery provide better evidence.

## Components

- A React and TypeScript web application presents request authoring, execution
  status, and sanitized evidence.
- A Go API handles authenticated application requests and persists durable
  metadata in DynamoDB through accepted Lambda handlers.
- A Go execution worker consumes queued jobs, performs remote API requests, and
  writes immutable, deliberately redacted evidence.
- A Go local agent will execute requests that require access to a developer's
  machine or private network. A future MCP interface will expose only
  deliberately sanitized evidence to coding agents.

The revised W1 topology uses CloudFront for web delivery, Go Lambda handlers
behind HTTP API Gateway, DynamoDB metadata/protected envelopes, SQS execution
wakeups, and private S3 sanitized evidence. It is recorded in
[accepted ADR 0002](../decisions/0002-serverless-persistence-topology.md), not
deployed or runtime-tested. SST 4 with TypeScript is the
selected infrastructure framework following the compute/frontend and database
viability spikes. GitHub Actions is the accepted CI/CD entry point.

## Trust and data boundaries

Request credentials are sensitive by default. Authorization headers, cookies,
credential headers, and user-designated secrets must be redacted before logs,
persistence, evidence artifacts, or MCP responses are produced. Evidence should
be immutable and traceable to an execution without retaining unneeded secrets.

The local agent is a separate trust boundary: it should bind locally by
default, require explicit authorization, and reveal the minimum information
needed by the hosted platform.

## Restore, limits and dispatch boundaries

Davian selected the MVP recovery exception: restoring an earlier point may
lose later changes and restore later-deleted projects/items. Show actual restore
timestamp(s) and this warning; deletion does not survive every restore. No
independently preserved anti-resurrection deletion journal. Normal access denial,
upload fencing, tombstones and orphan cleanup remain. Before reopening reconcile
Control, Protected and S3; missing evidence is unavailable, not fabricated.
Restored queued/running work and OUT must not automatically replay HTTP; require
deliberate new submissions/reruns under the accepted recovery gate.

Accepted size defaults are 64 KiB complete saved request configuration, 64 KiB
complete frozen execution configuration and independent 64 KiB resolved body;
2 MiB response wire-read and decompressed bounds, and 4 MiB complete encoded
evidence/API/local-upload caps. All configuration fields/serialization count.
No large request-body S3 subsystem in MVP. Accepted fast SQS notification after
durable commit improves cloud dispatch opportunity; durable OUT and scheduled
delivery recover failure/uncertainty without extra jobs or automatic HTTP replay.

## Current state

The separately authorized [users/projects local slice](users-projects-contract.md)
now implements seven routes, verified-identity ownership, conditional DynamoDB
Local writes and resumable empty-project cleanup. See [setup/tests](../development/local-ownership.md).
It is a local HTTP adapter, not a Lambda deployment or full cross-store cascade.
Actual Local integration tests do not establish Cognito/IAM/GSI propagation or
AWS recovery semantics. Other product components remain planned.

Local process foundations and isolated, removable infrastructure-spike
definitions remain; no deployment was performed by the ownership task. PostgreSQL is
available through Docker Compose, while an isolated Fargate spike proved
private RDS PostgreSQL connectivity, secure credential delivery, TLS, and a
versioned migration. Product services are not connected to PostgreSQL yet.
Queueing, evidence storage, public custom-email readiness and redaction remain
future work. Development Cognito authentication is covered below; fixture token
verification alone is not cloud authentication.

## Cognito browser/local projects continuation

The [development guide](../development/cognito-local-projects.md) records React
auth/project integration, local CORS and real browser-to-Go/DynamoDB tests using
a test-only Cognito fixture. The `dev-auth` SST stage contains authentication
only. Davian authorized Cognito default email for development; custom SES
public-signup readiness remains future work. Davian accepted Essentials with one-day rotating refresh after AWS's
Lite/rotation feature conflict was verified. See the guide
for current cloud/email validation gaps; fixture tests are not real Cognito.

## Saved requests continuation

Davian accepted the [saved-request contract](saved-requests-contract.md) on
2026-10-07, including partial PATCH clearing/whole-collection replacement.
The authorized public-only request slice implements five additional routes and
React authoring in the existing development API/Control resources. Credentials
and user-designated secrets fail closed until encrypted storage/reference
eligibility exists. Complete normalized saved configuration remains <=64 KiB.
Request revision CAS and project gate CAS protect writes/deletion.

The explicit savedRequestsSchemaVersion=1 stage capability replaces the legacy
empty-project-only marker without resetting recovery generation. Scheduled
cleanup drains recognized requests/tombstones in resumable bounded transactions,
preserves unknown children and retains the strong whole-partition completion
proof. Local and actual AWS integration passed; genuine browser status is
recorded separately in [the development guide](../development/saved-requests.md).
Execution, Protected, workflows and full cross-store cleanup remain planned.

## Protected saved-request continuation

The [protected saved-request contract](protected-request-secrets-contract.md)
is accepted, including project-owned lifecycle, explicit shared replacement and
revocation, stable binding identity, envelope encryption, limits and two-table
cleanup. Preserve cannot introduce a binding. Defined URL checks cover named
credential patterns and unsupported modes, not arbitrary embedded secrets.

The authorized development slice provisions Protected and the annual-rotation
stage KMS key and extends existing API/cleanup Lambdas. Request revisions,
project gates and recovery generation remain authoritative. See
[actual validation and remaining gaps](../development/protected-request-secrets.md).
Execution, outbound HTTP, evidence, workflows and local-agent delivery remain
outside this slice. Earlier public-only descriptions record their historical
baseline; this continuation supersedes those secret-storage limits.
