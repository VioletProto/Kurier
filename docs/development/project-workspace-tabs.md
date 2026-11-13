# Project workspace tabs

[Trello delivery card](https://trello.com/c/cF4IzJMO).

The selected project opens on **Requests & evidence**, followed by Workflows,
Environments, OpenAPI, Local agent pairing, MCP access and Project settings.
Request execution and evidence remain together in their existing request flow.
Local agent pairing has its own tab; its current status is shared with execution
controls, including revocation and project/gate resets. Project creation, selection and sign-out remain outside the tabs.

Tabs use a single keyboard stop with Left/Right (including wraparound), Home
and End navigation. Tab moves into the selected panel. Active tabs have a
filled background and underline; keyboard focus retains the shared outline.
Inactive panels are hidden from interaction and accessibility navigation but
remain mounted, preserving drafts, pending operations and execution polling in
memory. Changing projects or signing out retains the existing reset behavior;
drafts are not persisted to browser storage.

The desktop layout uses a bounded project sidebar and the remaining viewport
width for the selected project. Below 900px it stacks into one column. Tabs
wrap, fields can shrink and long evidence/token text wraps within its panel.

## Local verification

- Repository formatting check, frontend lint and production build passed.
- All 96 frontend unit/component tests passed.
- Chromium UI fixtures at 375, 1366 and 1920px passed: keyboard navigation,
  request and rename draft preservation, one visible panel, no page-wide
  horizontal scrolling and desktop content width above 65% of the viewport.
  Fixtures include long URL/body values, theme switching, no writes on tab
  changes and empty local/session storage.
- The race-enabled Go/DynamoDB Local browser integration passed with all eleven
  Chromium scenarios: project/request CRUD and conflict handling, protected
  inputs, local-agent execution, pinning/evidence, workflows/assertions,
  environments, OpenAPI, MCP and the three layout fixtures. Cognito is simulated.

The standalone layout fixture mocks authentication and API responses. It does
not establish genuine Cognito, hosted browser, AWS or cross-engine behavior.
The later authorized deployment and its distinct hosted coverage are recorded below.

## Development deployment — 2026-11-13

Davian authorized deploy, test and merge after the local implementation. UI source
`5dc7d38` is live at **https://d2kpq905qyh9zg.cloudfront.net** with release
`82d4268bc460a2ea560e20ed13ba77f8c4d547924efded8536e05efdc71f23f7`.

Verified non-root `davian-admin` in account `747336059622`, region `us-east-2`.
The full SST preview included rebuilt backend packages, so deployment used SST
4.17.1's supported `--target FrontendWeb` option. Its reviewed plan and actual
updates were limited to the existing static asset upload, KV file keys and
waited cache invalidation. No Lambda, IAM, schema activation or new resource
was deployed. Existing bootstrap v5, the complete stage marker, Free/ACTIVE plan,
concurrency limits and all five deployed Lambda code/configuration fingerprints
matched before and after. Earlier previews rejected missing/incorrect public
configuration before planning; no deployment occurred on those attempts.

Actual post-deployment checks passed:

- Public manifest and all five files matched deployed byte counts/SHA-256 and
  cache metadata. The broker-enabled public configuration was preserved.
- Existing credential-free CloudFront checks returned JSON/no-store 401 for a
  missing session and 403 for foreign origin, missing header and navigation.
- Genuine signed-out Chromium navigation and reload rendered sign-in.
- The deployed HTML/JS/CSS passed keyboard, draft-preservation, one-visible-panel,
  theme and no-horizontal-overflow checks at 375/1366/1920px with intercepted
  synthetic session/project responses. No product mutation reached the backend.
  This is deployed UI coverage, **not genuine authenticated Cognito/product
  validation**. No real account, credential, target request, HAR or screenshot
  was created. Chromium's initial network-idle navigation timed out; the
  successful run used direct networking with QUIC disabled and document-load
  waiting. No application change was needed.
- Full `npm run verify` passed (96 frontend tests, 16 infrastructure tests,
  tooling, formatting, lint, production build and configured Go tests/vet).
  The restricted runner's tooling-test failure did not recur outside the sandbox.

Run `node scripts/test-project-workspace-hosted.mjs` after a future scoped
release to repeat the static-byte, signed-out and synthetic-workspace checks.
The [machine receipt](project-workspace-tabs-hosted.json) retains fixed outcomes
and preservation hashes; raw deployment state/logs remain ignored.

[PR #21](https://github.com/VioletProto/Kurier/pull/21) integrates the UI and
this deployment record. Real authenticated sign-in, hosted execution and
cross-engine validation were not repeated for this UI-only deployment.

## Local-agent tab follow-up

Davian requested separating Local agent pairing after the initial deployment.
Pairing controls now occupy their own mounted project tab. Pairing draft inputs
survive tab switches; requests continue to select paired agents and require
explicit per-execution consent. Revocation updates the execution target controls.
The local browser regression covers pairing in the new tab, executing from
Requests & evidence, revoking in the pairing tab and disabled local execution
after revocation. The layout fixture checks all seven tabs at the three widths.
Formatting, frontend lint/build, all 96 frontend tests and the race-enabled
Go/DynamoDB Local browser suite (11 scenarios) passed.
This follow-up has not been deployed. The earlier hosted receipt describes the
initial six-tab release.
