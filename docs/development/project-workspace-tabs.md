# Project workspace tabs

[Trello delivery card](https://trello.com/c/cF4IzJMO).

The selected project opens on **Requests & evidence**, followed by Workflows,
Environments, OpenAPI, MCP access and Project settings. Request execution,
local-agent controls and evidence remain together in their existing request
flow. Project creation, selection and sign-out remain outside the tabs.

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
- The race-enabled Go/DynamoDB Local browser integration passed with all ten
  Chromium scenarios: project/request CRUD and conflict handling, protected
  inputs, local-agent execution, pinning/evidence, workflows/assertions,
  environments, OpenAPI, MCP and the three layout fixtures. Cognito is simulated.

The standalone layout fixture mocks authentication and API responses. It does
not establish genuine Cognito, hosted browser, AWS or cross-engine behavior.
No deployment or AWS changes are part of this UI task.
