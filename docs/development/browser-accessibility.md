# Browser and accessibility validation

The [browser/accessibility follow-up](https://trello.com/c/ckyXtYHL) now separates
repeatable local checks from genuine hosted acceptance. The first bounded slice
was verified on 2026-11-16. The demonstration browser remains Vivaldi based on
Davian's earlier reported use; automated Chromium is a separate proxy, not a
Vivaldi result or a declaration of Firefox/WebKit support.

## Completed local keyboard slice

Keyboard checks reproduced missing focus at the workspace skip destination and
screen transitions. The workspace region now accepts programmatic focus without
adding a Tab stop. Entering or leaving sign-in, including session restoration
and retry transitions, focuses that region. Changing account forms focuses their
new heading; opening a project focuses its first tab. Existing tab Arrow keys,
Home/End behavior and draft preservation remain intact.

The local Chromium tests cover:

- Keyboard activation of the skip link and all account-form navigation choices;
  focus reaches the destination and Tab continues into its controls.
- Sign-in, project entry, all seven workspace tabs, forward/reverse Tab order and
  sign-out, including clearing private project content and password fields.
- Reflow without document horizontal overflow at 320, 683 and 1366 CSS pixels;
  existing draft-preservation tests also cover 375, 1366 and 1920 pixels.
- Loading status and disabled creation, readable project-list failure with
  keyboard refresh recovery, and stale-rename conflict with refresh recovery.
- Failed sign-in with an alert and editable fields; empty browser web storage.
- Existing HTTPS broker regression tests for password-manager focus return and
  session restoration across reload, tabs, reopening and sign-out.

Environment: Linux, Node 26.13.1, Playwright 1.63.0, its installed Chromium.
Authentication and API responses are local fixtures, including the intentional
failed rename. The new fixture blocks unmatched external HTTP requests. Browser
trace, screenshots and video are disabled. No real credentials, Cognito, AWS
mutation, product data, target execution or deployment was used.

Validation commands from the repository root:

```sh
npm run verify
cd apps/web
npx playwright test e2e/accessibility.spec.ts e2e/workspace.spec.ts
```

The browser pair passed nine tests. The existing local HTTPS broker configuration
passed its two tests using a disposable localhost certificate as in CI.
`npm run verify` passed formatting/gofmt, ESLint, frontend/infrastructure/tooling
tests, TypeScript/Vite production build and all Go modules' tests/vet. The first
sandboxed verification stopped at a denied Git subprocess; the rerun outside
the sandbox passed. CI now runs the keyboard/reflow pair in the existing browser
job, in addition to its broker checks. A local pass does not establish a CI run.

## Remaining hosted and browser work

This change has not been deployed. The parent card remains open. Record each
future result individually, including browser/version, operating system, source
release, scenario, outcome and cleanup, without credential-bearing artifacts.

- Genuine signed-in hosted Vivaldi critical product paths: requests/evidence,
  workflows, project changes, MCP and local-agent management. Use controlled
  disposable inputs and the existing consent boundaries for target execution.
- Actual browser zoom at 200% and 400%, keyboard focus visibility/reading order,
  and smaller-screen usability while signed in. A 683px viewport approximates
  the reflow width of 1366px at 200%; it is not an actual browser-zoom result.
- Select Firefox/WebKit support deliberately, then record those engines'
  results. Do not equate Chromium with Vivaldi or Safari/WebKit certification.
- Genuine session/history privacy, loading/error/conflict states across the
  wider product, and assistive-technology review. DOM roles/focus assertions
  are not a screen-reader audit or WCAG conformance certification.

Historical hosted results remain in
[frontend hosting validation](frontend-hosting-validation.md); this record does
not upgrade their coverage or repeat the accepted hosting deployment.
