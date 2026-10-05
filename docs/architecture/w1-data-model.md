# W1 PostgreSQL data model

Status: proposed for Davian Hernandez review, 2026-10-05. This is a logical
schema specification, not SQL migrations. Names are internal snake_case;
HTTP fields remain lower camelCase and opaque string IDs. See the
[system design](w1-system-design.md) and [review decisions](w1-review-decisions.md).

## ERD

The diagram shows durable ownership and principal relationships. Nullable live
associations are outside frozen evidence. Every project child has a direct
`project_id` FK even when that edge is omitted for diagram readability.

```mermaid
erDiagram
  users ||--o{ projects : owns
  users ||--o{ local_agent_credentials : pairs
  users ||--o{ mcp_credentials : authorizes
  users ||--o{ mcp_audit_records : invokes
  projects ||--o{ request_definitions : contains
  projects ||--o{ environments : contains
  projects ||--o{ protected_secrets : protects
  projects ||--o{ openapi_imports : imports
  projects ||--o{ workflows : contains
  projects ||--o{ executions : owns_history
  projects ||--o{ workflow_runs : owns_runs
  projects ||--o{ mcp_audit_records : scopes
  openapi_imports ||--o{ openapi_operations : describes
  openapi_operations o|--o{ request_definitions : associates
  request_definitions o|--o{ executions : live_association
  environments o|--o{ executions : live_association
  executions ||--|| execution_jobs : schedules
  executions ||--o| execution_snapshots : captures_once
  executions ||--|| execution_retention : controls_lifetime
  executions ||--o{ execution_events : reports
  execution_jobs ||--o{ job_secret_bindings : freezes_protected_values
  execution_jobs ||--o| execution_outbox : publishes_cloud_job
  workflows ||--|{ workflow_steps : orders
  workflows o|--o{ workflow_runs : live_association
  workflow_runs ||--|{ workflow_run_steps : freezes_steps
  workflow_run_steps ||--o{ workflow_step_secret_bindings : freezes_secrets
  workflow_runs ||--o{ workflow_runtime_bindings : protects_extractions
  workflow_run_steps o|--o| executions : executes
  local_agent_credentials o|--o{ execution_jobs : leases

  users {
    uuid id PK
    text cognito_issuer UK
    text cognito_sub UK
    text display_name
    timestamptz created_at
  }
  projects {
    uuid id PK
    uuid user_id FK
    text name
    text state
  }
  request_definitions {
    uuid id PK
    uuid project_id FK
    uuid operation_id FK
    bigint revision
    jsonb configuration
  }
  environments {
    uuid id PK
    uuid project_id FK
    bigint revision
    jsonb variables
  }
  protected_secrets {
    uuid id PK
    uuid project_id FK
    uuid request_id FK
    uuid environment_id FK
    bytea ciphertext
    bytea wrapped_data_key
  }
  openapi_imports {
    uuid id PK
    uuid project_id FK
    jsonb sanitized_document
    text sha256
  }
  openapi_operations {
    uuid id PK
    uuid project_id FK
    uuid import_id FK
    text method
    text path_template
    jsonb schema_bundle
  }
  executions {
    uuid id PK
    uuid project_id FK
    uuid live_request_id FK
    uuid live_environment_id FK
    uuid run_step_id FK
    timestamptz submitted_at
  }
  execution_jobs {
    uuid id PK
    uuid project_id FK
    uuid user_id FK
    uuid execution_id FK,UK
    jsonb frozen_plan
    text status
    bigint fence
    timestamptz dispatch_intent_at
  }
  job_secret_bindings {
    uuid job_id PK,FK
    text binding_path PK
    uuid project_id FK
    uuid source_secret_id
    bytea ciphertext
    bytea wrapped_data_key
  }
  execution_snapshots {
    uuid execution_id PK,FK
    uuid project_id FK
    jsonb sanitized_request
    jsonb sanitized_response
    jsonb replay_configuration
    integer http_status
    text outcome
  }
  execution_retention {
    uuid execution_id PK,FK
    uuid project_id FK
    boolean pinned
    timestamptz expires_at
  }
  execution_events {
    uuid execution_id PK,FK
    bigint sequence PK
    uuid project_id FK
    text event_type
    timestamptz occurred_at
  }
  execution_outbox {
    uuid job_id PK,FK
    uuid project_id FK
    timestamptz published_at
    integer publish_attempts
  }
  workflows {
    uuid id PK
    uuid project_id FK
    bigint revision
    text name
  }
  workflow_steps {
    uuid id PK
    uuid project_id FK
    uuid workflow_id FK
    uuid request_id FK
    integer position
    jsonb rules
  }
  workflow_runs {
    uuid id PK
    uuid project_id FK
    uuid live_workflow_id FK
    jsonb frozen_configuration
    text status
  }
  workflow_run_steps {
    uuid id PK
    uuid project_id FK
    uuid run_id FK
    integer position
    jsonb frozen_plan
    text status
  }
  workflow_runtime_bindings {
    uuid run_id PK,FK
    text variable_name PK
    uuid project_id FK
    bytea ciphertext
    bytea wrapped_data_key
  }
  workflow_step_secret_bindings {
    uuid run_step_id PK,FK
    text binding_path PK
    uuid project_id FK
    uuid source_secret_id
    bytea ciphertext
    bytea wrapped_data_key
  }
  local_agent_credentials {
    uuid id PK
    uuid user_id FK
    bytea token_hash UK
    timestamptz expires_at
    timestamptz revoked_at
  }
  mcp_credentials {
    uuid id PK
    uuid user_id FK
    bytea token_hash UK
    timestamptz expires_at
    timestamptz revoked_at
  }
  mcp_audit_records {
    uuid id PK
    uuid user_id FK
    uuid project_id FK
    uuid credential_id FK
    text tool_name
    text outcome
    timestamptz occurred_at
  }
```

`cognito_issuer` and `cognito_sub` form **one composite unique key**, not two
individually unique columns. ERD `UK` labels denote membership in that key.
An execution has one job and retention row, enforced by creation in one
transaction; a final snapshot exists only once terminal. A workflow contains
two or more steps (the diagram's one-or-more edge cannot express that minimum).

## Common keys and ownership constraints

Use application-generated random UUIDs, `timestamptz`, nonnegative `bigint`
revision/counter fields, bounded `text`, and explicit CHECK constraints for
statuses/targets/methods. Avoid PostgreSQL enum types initially so adding a
reviewed status does not require enum-specific rollback handling. All rows
with `id` have primary keys and timestamps unless identified as join rows.

`projects.user_id REFERENCES users(id) ON DELETE CASCADE`. Every project-owned
table has `project_id NOT NULL REFERENCES projects(id) ON DELETE CASCADE`.
Give referenced project-child tables `UNIQUE(project_id, id)`. Use composite
FKs `(project_id, parent_id)` for associations to prove same-project membership,
including job/execution, environment, workflow step/request, import/operation,
run/step, and secrets. Nullable live links use `ON DELETE SET NULL (parent_id)`
so `project_id` stays intact. Account-scoped credentials reference users with
cascade, and never infer ownership from a job message.

PostgreSQL supports composite constraints and
[column-specific SET NULL](https://www.postgresql.org/docs/17/ddl-constraints.html).
An FK does not create an index on its referencing columns: index each FK used
by deletion or joins. Owner checks are mandatory in API queries even when
composite FKs validate writes. Proposed MVP uses explicit ownership queries
and scoped DB grants; PostgreSQL RLS can be reviewed later, with connection
pool identity handling proved before relying on it.

## Table descriptions

| Table                       | Required fields and constraints beyond common keys                                                                                                                                                                                                                                                                                                                        | Lifecycle                                                                                                                                                                                               |
| --------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `users`                     | `cognito_issuer`, `cognito_sub`, `display_name`; UNIQUE(issuer, sub). No login password, token, or email-based identity key.                                                                                                                                                                                                                                              | Upsert only after verified Cognito identity. Account deletion is not an MVP route.                                                                                                                      |
| `projects`                  | `user_id`, name 1–120 characters, state active/deleting, revision. Name need not be unique.                                                                                                                                                                                                                                                                               | Root ownership/deletion boundary.                                                                                                                                                                       |
| `request_definitions`       | Name, method CHECK GET/POST/PUT/PATCH/DELETE, revision, `configuration jsonb`, nullable operation FK. Configuration contains URL template, ordered query/header arrays, JSON/text body descriptor, sensitive path policy, and secret references only.                                                                                                                     | Mutable saved configuration; unaffected by execution cleanup.                                                                                                                                           |
| `environments`              | Name, revision, `variables jsonb` ordered array; unique enabled variable names within each environment validated on write; secret references replace protected values.                                                                                                                                                                                                    | Mutable; absence means no environment rather than implicit default.                                                                                                                                     |
| `protected_secrets`         | Stable ID, nullable owning request/environment links with at most one set, binding locator, value revision, ciphertext, nonce/tag, wrapped data key, key ARN, algorithm/version, revoked_at. No plaintext or plaintext fingerprint.                                                                                                                                       | Project-level secrets may have neither child link. Request/environment-scoped rows cascade on child deletion; project secrets survive child deletion. Unique binding locator within the selected scope. |
| `executions`                | Submitted timestamp, nullable live request/environment/run-step links, nullable `rerun_of_id` SET NULL within project. Captured source IDs/names belong in frozen plan/snapshot, not these live links. Unique non-null run-step association.                                                                                                                              | History anchor owned directly by project. Child-definition deletion clears live associations only.                                                                                                      |
| `execution_jobs`            | UNIQUE execution_id; target cloud/local; local credential ID when local; status queued/running/completed/failed; frozen_plan, plan schema version/hash, owner-scoped idempotency key/hash, attempt_count, lease_owner/deadline, fence, dispatch_intent_at, started_at/completed_at.                                                                                       | Operational mutable state; plan immutable after enqueue; lease/status updated by narrow procedures. Terminal state cannot reopen.                                                                       |
| `job_secret_bindings`       | PK(job_id, binding_path), source secret ID/revision, encrypted value copied at submission, crypto metadata as above. Source ID is a logical reference, not a cascade FK.                                                                                                                                                                                                  | Runtime-only protected payload; deleted at terminalization or source revocation. Never selected by evidence readers.                                                                                    |
| `execution_snapshots`       | PK execution_id; evidence version, frozen source IDs/names, target, sanitized resolved request/response, error, timing, assertions, validation results, schema hash/version, redaction version, replay_configuration, sanitized evidence hash. outcome completed/failed; http_status NULL or 100–599; duration_ms >= 0; completed_at NOT NULL >= started_at when present. | Insert once. No UPDATE permission or mutable retention fields. Source identifiers inside the payload are historical strings, not live FKs.                                                              |
| `execution_retention`       | PK execution_id; pinned default false, pinned_at/by nullable, expires_at nullable until terminal; version for concurrency. At terminal CHECK: pinned implies expires_at NULL, otherwise completed_at + 30 days maintained by procedure.                                                                                                                                   | Mutable retention, independent of snapshot. Pin/unpin never edits evidence.                                                                                                                             |
| `execution_events`          | PK(execution_id, sequence), event type CHECK contract's four event names, occurred_at, minimum safe status/httpStatus payload. Sequence increases under execution row lock.                                                                                                                                                                                               | Append only; retained with execution. No raw configuration, responses, or secrets.                                                                                                                      |
| `execution_outbox`          | PK job_id, next_publish_at, attempt count, published_at. Message generated from trusted relational identifiers, not arbitrary stored JSON.                                                                                                                                                                                                                                | Dispatcher updates metadata; cascade with execution/project. Only cloud jobs have an outbox row.                                                                                                        |
| `openapi_imports`           | Detected version, parser version, sanitized_document or future artifact reference, sanitized document hash, status ready/failed, warnings and unsupported features JSON.                                                                                                                                                                                                  | Immutable successful import; reimport creates new ID. A hash covers sanitized content, not secret-bearing uploaded bytes.                                                                               |
| `openapi_operations`        | Import FK, method/path_template, optional document operationId, schema_bundle JSONB/hash; UNIQUE(import_id, method, path_template). Document operationId uniqueness checked when provided.                                                                                                                                                                                | Immutable operation version. A request's operation link may change; a queued execution captures the schema bundle.                                                                                      |
| `workflows`                 | Name, revision, stop_on_failure default true.                                                                                                                                                                                                                                                                                                                             | Mutable definition, direct project child.                                                                                                                                                               |
| `workflow_steps`            | Workflow FK, request FK, positive position, rules JSONB for extraction/expected status/property assertions; UNIQUE(workflow_id, position) deferrable for reordering.                                                                                                                                                                                                      | Request FK RESTRICT; API deletion first removes affected step rows and marks the workflow invalid if fewer than two remain. Workflow deletion cascades definition steps.                                |
| `workflow_runs`             | Nullable live workflow FK, frozen configuration/revision, status queued/running/completed/failed, failed_step_position, timing, explicit history expires_at.                                                                                                                                                                                                              | Immutable configuration, mutable orchestration. Project-owned; workflow deletion clears live link.                                                                                                      |
| `workflow_run_steps`        | Run FK, positive position, frozen non-secret plan, status pending/running/completed/failed/skipped, safe assertion/extraction summaries; UNIQUE(run_id, position).                                                                                                                                                                                                        | Frozen steps independent of live definitions. Execution association through executions.run_step_id; no circular mandatory FK.                                                                           |
| `workflow_runtime_bindings` | PK(run_id, variable_name), encrypted current value and taint flag/crypto metadata. Non-secret extracted values may also use this store for predictable protection.                                                                                                                                                                                                        | Deleted at terminal run, failed lease, or project deletion. Never in run-history outputs.                                                                                                               |
| `local_agent_credentials`   | User FK, opaque credential ID, SHA-256 digest of 256-bit random token, name, created/expires/revoked/last_seen timestamps. Token type prefix supports routing, not authority. UNIQUE token_hash.                                                                                                                                                                          | Raw token returned once. Partial UNIQUE(user_id) WHERE revoked_at IS NULL; expired predecessor revoked transactionally before replacement.                                                              |
| `mcp_credentials`           | Same random-token hash model, user FK, expiry/revocation, scope read:evidence only.                                                                                                                                                                                                                                                                                       | Separate from agent execution tokens; proposed stdio authentication route needed. Multiple clients allowed.                                                                                             |
| `mcp_audit_records`         | User FK; nullable project and credential FKs; tool name, bounded correlation ID, safe target ID/type, outcome allowed/denied/error, timestamp, duration. No raw tool arguments or returned evidence.                                                                                                                                                                      | Append only. Project-scoped rows cascade at project deletion; unscoped listing/denied calls retain no project content. Credential deletion SET NULL.                                                    |

`workflow_step_secret_bindings` has PK `(run_step_id, binding_path)`, direct
project/run-step cascade FKs, logical source secret ID/revision, and the same
envelope crypto columns as job bindings. At run submission it freezes protected
values for **every** step, before any step job is created. When a step becomes
eligible, create its execution/job and transfer/re-encrypt bindings atomically;
skipped steps get no execution. Destroy unused bindings on run termination,
source revocation, or project deletion. Index `(project_id, source_secret_id)`.
This prevents later-step secrets from silently changing during a run.

`executions.run_step_id` uses same-project SET NULL on step deletion so run
summary cleanup cannot remove pinned execution evidence. The job's live local
credential association is nullable and SET NULL on credential deletion; capture
its original ID in the frozen plan and require a live valid credential at claim.
The API ordinarily revokes credentials rather than physically deleting them.
Revocation destroys pending bindings and fences leases in the same transaction.

Jobs carry a redundant trusted `user_id` to enforce UNIQUE(user_id,
idempotency_key) for non-null submission keys. Add UNIQUE(user_id, id) to
projects and an additional `(user_id, project_id)` FK to jobs referencing that
key; a message cannot change either value. Project ownership transfer is not
an MVP operation. Deletion procedures examine captured source identifiers in
pending job/run-step plans before clearing live links, so a deleted request or
environment also invalidates unstarted frozen work without rewriting history.

Each binding ciphertext is rewrapped/re-encrypted for its **destination scope**;
do not blindly copy ciphertext bound to a different encryption context.
Bind AEAD/KMS context to app, stage, project ID, binding ID, and purpose. The
data model's crypto columns denote one envelope, never stored plaintext keys.
Scopes are non-secret identifiers; labels and user-supplied values stay out of
KMS context and CloudTrail. Runtime DB roles cannot enumerate the secret table
through generic list APIs; execution-scoped retrieval verifies owner/job/lease.

## JSON versus relational storage

Relational columns own identity, authorization, joins, status, ordering,
retention, and deletion. JSONB owns versioned variable-shaped request fields,
assertion rules, sanitized response structures, and OpenAPI schema bundles.
Use arrays for headers/query so duplicates and order survive; a JSON object
would collapse duplicate keys. Represent raw sanitized JSON/text bodies as a
text string inside a typed body descriptor when preserving whitespace/number
spelling matters; JSONB parsed views are optional, not exact-wire substitutes.
[PostgreSQL JSONB](https://www.postgresql.org/docs/17/datatype-json.html)
normalizes object representation and is not a byte-for-byte archive.

All JSON has a schema version, validated shape, nesting/size caps, and no raw
secret values. Version hashes cover canonical **sanitized** content only.
Redacted snapshots preserve exact non-secret content within configured limits;
omit unsupported/oversized bodies with explicit omission metadata rather than
claiming complete capture. Response validation runs on supported bounded
in-memory content and stores only sanitized violation details.

Schema associations and limits/redaction policy are frozen before execution.
An import edit/delete cannot change old contract-validation results. Reject
remote `$ref` fetching initially; local document references only, with cycle,
depth, and resource limits. Examples containing credentials are removed before
storage or generated request creation. The library-selection card must define
supported OpenAPI versions and dialects; this model does not imply universal
OpenAPI 3.x support.

## Indexes and transactional constraints

- Projects: `(user_id, created_at DESC, id DESC)` for keyset pagination.
- Definitions/environments/imports/workflows: `(project_id, created_at DESC, id DESC)`;
  composite parent references indexed. Unique environment name per project is
  optional UX, not required for resolution by ID.
- Executions: `(project_id, submitted_at DESC, id DESC)` and
  `(project_id, live_request_id, submitted_at DESC, id DESC)`; separate frozen
  source request ID column/index if history must be grouped after deletion.
- Snapshots: `(project_id, outcome, completed_at DESC, execution_id DESC)` for
  failed-history/MCP filters; partial failed index if volume warrants it.
- Jobs: partial `(status, lease_deadline)` for queued/running reconciliation;
  `(target, status, created_at)` for agent claims; UNIQUE scoped submission
  idempotency key where present. Include project FK index independently.
- Outbox: partial `(next_publish_at, job_id) WHERE published_at IS NULL`.
- Retention: partial `(expires_at, execution_id) WHERE pinned = false`.
- Protected bindings: `(project_id, source_secret_id)` for revocation;
  protected secret owner/scope indexes and uniqueness on locator. No ciphertext
  indexes, plaintext fingerprints, or speculative JSONB GIN indexes.
- Workflow steps/run steps: unique parent/position, FK request/run indexes;
  run history `(project_id, created_at DESC, id DESC)`.
- Audit: `(user_id, occurred_at DESC, id DESC)` and project FK index.
- Credentials: unique token digest, user index, one active agent constraint.

Cross-table CHECK constraints cannot inspect other rows. Procedures/transactions
enforce terminal snapshot/job agreement, one retention row per execution,
expiry calculation, same owner for agent assignment, secret locator validity,
monotonic transitions, and minimum two valid workflow steps. Definition writes
use revision compare-and-swap; submission reads/locks a consistent revision
set. Workflow reorder/delete is atomic. Proposed owner-scoped idempotency keys
live on jobs until history deletion; after expiration replay is not guaranteed.

## Immutability, retention, and deletion

Revoke UPDATE on execution_snapshots and add a defensive reject-update trigger.
Worker inserts only through finalization with fencing checks. Grant deletion
only to reviewed project-deletion/retention procedures and the migration owner;
immutability prevents edits, not authorized removal. Execution identity and
frozen plans are also write-protected after submission. Mutable live links,
jobs, run progress, and retention occupy separate tables.

Hourly cleanup locks eligible retention rows in batches of 500 using
`FOR UPDATE SKIP LOCKED`, rechecks pinned/expiry under the same lock, and
deletes the execution anchor (cascading snapshots, jobs, bindings, outbox,
events, and retention). Pin/unpin acquires that lock too. A cleanup winner
makes a concurrent pin return not_found; a pin winner blocks cleanup. UI
reads filter expired unpinned records immediately even if deletion awaits the
next batch. Queue/running jobs have no result expiry; stalled-job finalization
gives failed results a completedAt and normal 30-day expiry. Saved definitions
and secrets are never execution-cleanup targets.

| Deletion trigger                              | Preserve                                                                                                            | Remove or revoke                                                                                                                                                                                           |
| --------------------------------------------- | ------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Request deletion                              | Project-owned execution snapshots and frozen replay plans; run history. Clear live request links.                   | Request-scoped protected secrets, queued bindings referencing them; unstarted jobs tied to the deleted request fail clearly. Remove live workflow steps using request; validate workflows before new runs. |
| Environment deletion                          | Execution snapshots' captured environment ID/name/non-secret context and run history. Clear live environment links. | Environment variables and protected secrets; fail queued jobs requiring that environment. Reruns never silently substitute another environment.                                                            |
| Workflow deletion                             | Frozen runs/steps and executions; clear live workflow link.                                                         | Definition and definition steps. Proposed unstarted run fails with workflow_deleted; in-progress run follows its frozen plan unless project/secret revocation intervenes.                                  |
| Import deletion, if a later route is approved | Captured schema bundle/hash and validation results in executions.                                                   | Import/operation definitions; clear live request association. No delete route exists yet.                                                                                                                  |
| History cleanup                               | Request/environment/workflow definitions; project secrets; workflow step ordering.                                  | Expired unpinned execution anchor and evidence. Run-step live execution link becomes NULL, with safe summary indicating evidenceExpired. No duplicate body in workflow history.                            |
| Project deletion                              | Account-scoped user and credentials usable for other projects.                                                      | Every project child, pinned/unpinned evidence, secrets/bindings, imports/operations, workflow definitions/runs, events, outbox, and project-scoped audit rows. Fence workers and deny stale submissions.   |

For workflow history, propose unpinned run summaries expire 30 days after run
completedAt too. A run with any pinned step execution retains its safe run/step
summaries until the last pin is removed or project deletion. Expired unpinned
step bodies still disappear; references must report expiration. This prevents
workflow storage from bypassing execution retention and requires an explicit
contract addition. Do not cascade execution deletion to a whole workflow run
or preserve expired response copies inside run summaries.

Source IDs inside snapshots and replay references are immutable metadata even
after deletion. Project ownership remains enforced by executions.project_id,
so nulling a live request/environment FK never creates ownerless history.
No FK from a snapshot to a mutable definition can block project deletion.

## Migration ownership and rollout

Propose versioned SQL under `services/api/migrations/`, maintained by the
backend owner Davian Hernandez and reviewed in PRs. A separate one-shot
migration command/task uses a DDL role, records version/checksum/applied_at,
and obtains a PostgreSQL advisory lock. Run once before compatible API/worker
rollout; do not run schema changes on every service startup. The spike's
startup migration proves a mechanism only; do not reuse its table or master
DB user as product policy.

Runtime roles are separate API, worker, cleanup, and audit capabilities;
no runtime CREATE/ALTER/DROP. Use expand/backfill/contract changes, checksum
validation, transactional DDL where supported, and reviewed data rollbacks.
Introduce tables as slices need them, starting users/projects, rather than
migrating this entire future model immediately. Local Compose and CI use the
same migrations. Tests must exercise same-project composite FKs, cross-user
authorization, immutable updates, cleanup/pin races, deletion cascades, and
schema upgrades on PostgreSQL 17. Backups and a restore rehearsal precede
destructive schema changes; production recovery remains untested here.
