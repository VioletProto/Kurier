# Session persistence, connection setup and desktop delivery

Requested by Davian on 2026-11-11. The cards define planned product outcomes.
Prerequisite spikes will select designs, contract amendments and verification
plans before dependent implementation.

## Backlog and dependencies

| Outcome                                                                         | Delivery card                                                           | Prerequisite                                                 |
| ------------------------------------------------------------------------------- | ----------------------------------------------------------------------- | ------------------------------------------------------------ |
| Keep users signed in after browser reload                                       | [Session persistence](https://trello.com/c/p4yuaybE)                    | [Session design](https://trello.com/c/JR5tQXNA)              |
| Offer manual project selection or automatic exposure through one MCP connection | [MCP connection modes](https://trello.com/c/RaD7xqgS)                   | [Connection and scope design](https://trello.com/c/mgcmRSIa) |
| Install, configure, start and pair an agent after user approval                 | [Guided agent setup](https://trello.com/c/j5di83sr)                     | [Agent onboarding design](https://trello.com/c/kqa7Vggt)     |
| Determine desktop app feasibility and distribution                              | [Desktop, GitHub Releases and AUR spike](https://trello.com/c/atEniUxB) | Session and agent findings inform the recommendation         |

All seven cards start in To Do with incomplete checklists. The configured Doing
WIP limit is three. Davian owns the work. Estimates and supported platforms will
be recorded with spike findings. Schedule these improvements against the actual
submission date and hours worked. The [tabbed workspace](https://trello.com/c/cF4IzJMO)
remains related UI refinement. The [MCP stored-results extension](https://trello.com/c/jQR3w1Zt)
remains a separate MVP item.

## Browser session persistence

The current [authentication baseline](../architecture/w1-review-decisions.md#authentication-and-email)
uses Cognito SRP, 15-minute access tokens, a one-day rotating refresh session and
browser memory storage. Reload currently requires sign-in. Restore sign-in on
reload while the refresh session is valid, with an explicit loading state,
reliable sign-out and clear expiry handling.

The spike must choose credential storage and browser-close behavior, comparing
browser persistence with a backend session/cookie approach against actual
hosting origins and auth code. Document XSS/CSRF defenses, cookie/CORS and route
changes if applicable, refresh rotation across tabs, network failure, revocation
lag and migration. Persistent sign-in requires an amendment to the memory-only
policy. A storage mechanism remains to be selected.

Verify reload, concurrent tabs, rotation races, expired/revoked sessions,
sign-out, offline recovery and the selected reopening policy. Existing natural
refresh observations describe the current implementation.

The [2026-11-11 research packet](../spikes/browser-session-persistence.md) records
Davian's selection of backend sessions and reopening until sign-out or fixed
one-day expiry. It proposes a same-origin refresh broker with encrypted server-side
refresh credentials, an HttpOnly cookie and memory-only access tokens. Detailed
contract/hosting/storage choices are now clarified in the
[broker contract](../architecture/browser-session-contract.md): defer the session
count cap, reissue cookies from authenticated retries with the original
browser-generated proof, and remove CloudFront's global error replacement.
Davian authorized proceeding with scoped implementation using existing workflows.
The research distinguishes local checks from future browser/Cognito/AWS gates
and estimates 24–40 active implementation hours plus hosted/elapsed validation.

## MCP connection and project exposure

The [current MCP contract](../architecture/read-only-mcp-contract.md) uses a
foreground stdio bridge and separate expiring read-only credentials with
immutable explicit scopes of up to ten projects. Offer manual project selection
or explicit consent to expose current and future owned projects through one
client connection.

Define scope mode representation, issuance/lifecycle, switching and narrowing
modes, discovery, pagination/cursor binding, limits and compatibility with
existing credentials and clients. Automatic exposure must check current
ownership, deletion, revocation/expiry, recovery generation, audit-before-data
and sanitization on each read. Specify how new projects become discoverable and
whether exclusions are supported. Evaluate the existing transport first.

Affected interfaces include browser credential management, bridge project
discovery, authorization/cursors and MCP client setup. Coordinate with the
separate stored-contract-results tool extension. Verify new-project discovery,
scope narrowing, multiple connections, foreign IDs and old-client behavior.

## Guided local-agent setup

The [current agent contract](../architecture/local-agent-execution-contract.md)
uses a foreground executable, explicit pairing and optional private POSIX
credential persistence. Handle installation, configuration, launch and pairing
after user permission without terminal commands, token copying or configuration
editing in the normal flow.

Evaluate installer/launcher and installed-helper handoffs, including desktop
integration. Determine what supported browsers and OSes permit. Define artifact
verification, permission prompts, install location, one-time expiring handoff,
origin/replay defenses, credential storage, startup, updates, rollback, uninstall
and failure recovery. Keep tokens out of URLs and logs. Explain installation
consent and project/destination execution consent separately.

Specify pairing/launcher interfaces, credential lifecycle and compatibility
with installed agents. Preserve distinct agent authority, explicit destination
scope and uncertain-send settlement. Verify clean/existing installs, rejected
permission, interrupted/offline setup, pairing and revoke/uninstall behavior.

## Desktop and distribution feasibility

Assess an installable web app and a desktop wrapper using the existing React
frontend and hosted API. Record supported platforms, frontend reuse, footprint,
maintenance, native permissions, system-browser authentication, callback
validation, credential storage and agent/MCP integration. Produce a minimal
local proof of concept when the spike is executed, then a recommendation,
estimate and implementation scope if selected.

Assess GitHub Releases artifacts, checksums/signing, reproducible builds,
versioning, upgrades, rollback and uninstall. Evaluate AUR source versus binary
packaging, PKGBUILD/.SRCINFO, licensing, clean-chroot checks and maintenance.
Desktop technology, platforms and package format remain open decisions. AUR
publication is a future delivery step.

## Documentation and acceptance

The native proposal and API Contracts receive dated planning continuations.
Affected repository contracts link here. When a spike produces a selected
design, synchronize exact interfaces, migration, acceptance status and tests
into its contract and cards. Record implementation, local tests, installed
client checks and hosted checks against delivery work. This planning update
contains no runtime implementation or deployment results.
