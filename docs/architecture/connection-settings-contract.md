# Kurier connection settings

Davian requested account-level Settings for MCP and local-agent pairing on
2026-11-16, and selected **all current and future projects** for the shared local
agent. This continuation supersedes the project-tab UI and fixed-project default
for newly issued agent credentials. Existing credentials retain their permissions.

## Account navigation

The signed-in workspace has Projects and Settings navigation. Settings contains
one MCP connection panel and one local-agent pairing panel. Selecting a project
never replaces/hides a second MCP panel: there is no connection UI in a project.
Project sections are Requests & evidence, Workflows, Environments, OpenAPI and
Project settings. Requests still select cloud/local execution with fresh consent.

Project drafts and in-flight work remain mounted when Settings is open. Settings
unmounts when leaving it, clearing one-time tokens and pending response handlers.
Sign-out/session replacement removes both views and their shared agent state.
A late initial status read cannot overwrite a newly paired or revoked agent.

## One local agent for all projects

New pairing uses the existing owner-authenticated POST `/api/v1/local-agents`,
with its UUID Idempotency-Key and one-time token response:

```json
{
  "name": "My local agent",
  "projectScope": "all_owned_projects",
  "scopes": [],
  "destinations": [
    { "origin": "http://localhost:3000", "addressClass": "loopback" }
  ]
}
```

Require 1–16 unique, normalized exact origins. Reject unknown modes, nonempty
project scopes mixed with automatic mode, and invalid/private-address-class
combinations. Pairing can happen before the first project exists. Explicit
consent covers all current/future **owned** projects and these destinations;
origin, address-class, local operator confirmation and per-execution/insecure
secret consent remain required. No wildcard origins, network-wide discovery,
MCP execution authority, additional credential slot or automatic credential renewal.

The credential remains immutable and expires thirty days after issuance. Adding
or changing destinations requires revoke/re-pair; adding projects does not.
Deleting the last project does not revoke the account credential. New projects
use it automatically, without adding per-project credential/link rows.

Automatic credentials use credential-row schema2 with `projectScope` and
`destinations`; fixed credentials retain schema1 and their existing `scopes`.
The account scope is returned to the browser and authenticated agent session.
Missing `projectScope` retains legacy semantics; unknown modes fail closed.
No automatic widening or migration of an existing credential is permitted.

## Account polling and release guards

The updated CLI uses POST `/api/v1/local-agent/poll` with an empty body for an
automatic credential. Authentication remains the separate opaque agent bearer,
never a Cognito or MCP credential. Legacy credentials cannot use account polling
and retain the project-specific poll routes. The new API Gateway route reuses
the existing integration and invocation grant; no new IAM policy, queue or index.

Automatic queued jobs use GSI2 `DPK=AGENT#<credentialId>`; legacy jobs retain
`AGENT#<credentialId>#PROJECT#<projectId>`. Existing due obligations, immutable
plans, one unresolved job per credential, grant bounds and retained result ACKs
are unchanged. Discovery remains bounded to five pages of twenty candidates;
index entries confer no execution authority. Strong reads verify the candidate,
owner, active project, credential, source, exact destination, stage and recovery
generation before the grant and again before releasing executable bytes.
Account polling with no queued work records bounded account presence without
requiring an arbitrary project. The CLI polls one account queue every ten seconds
with existing backoff/jitter, independently checks exact destination consent and
never replays ambiguous target HTTP. Result upload retains project ownership,
credential/fence/nonce checks and conditioned publication/ACKs.

## Compatibility and delivery

Existing project-scoped credentials and older agents retain their behavior.
Settings displays their limited scope and explains revoke/re-pair for account
scope. An old CLI rejects the unfamiliar automatic-session fields rather than
silently expanding access. Install the updated agent before using the new
all-project pairing. An old API rejects the new pairing input. Deploy the new
API/cleanup code and route together with the updated frontend before offering the new flow;
credential rows cannot be safely processed by an older backend after issuance.
No stage capability activation or existing-data rewrite is required by this
implementation. AWS deployment and hosted checks are separate from local tests.
MCP's existing selected/all-project modes and read-only authorization are unchanged.
