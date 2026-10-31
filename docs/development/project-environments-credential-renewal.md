# Credential renewal correction — local review only

The existing login mechanism supports bounded renewal between approved phases.
It cannot issue a single 55-minute credential. The selected correction preserves
one original campaign deadline and the original closed-stage deadline while
limiting each phase to four minutes. No AWS call, refresh, authentication-setting
change, collection or later phase ran during this correction. Actual usage stays
**301/1,799**, with **1,498** remaining attempts and the unchanged cumulative
**$1 procedural allowance**; billing remains unmeasured.

## Pinned source and retained metadata

The existing [SDK pins](../../scripts/preview-meter/authentication-sdk.json)
identify botocore **1.43.108**, including `credentials.py` SHA-256
`38f95561c553c7e76d9da6d23d30a779dcfb0403a254cb98562e92e12cc0b0e8`
and Sign-In model `data/signin/2023-01-01/service-2.json` SHA-256
`06c2ba0d3c74b024aba1ffecdff7bb81cf8a31d7841ae1af1574f726ccc839f6`.
Inspection used those local sources and the retained stopped-campaign record;
no remote documentation or AWS observation was needed.

`CreateOAuth2Token` documents `expiresIn` as a maximum of **900 seconds** for
authorization-code redemption and refresh. Its input supports `clientId`,
`grantType`, `code`, `redirectUri`, `codeVerifier` and `refreshToken`. It has no
duration input. Changing a CLI duration setting cannot make this operation return
55-minute credentials. No supported longer-lived credential mechanism was found
within the existing approved login session, and no configuration was changed.

`LoginCredentialFetcher.refresh_credentials()` loads the existing DPoP key and
cache. If remaining access-token validity is greater than **300 seconds**, it
returns cached credentials. Otherwise it sends one `refresh_token` grant using
the same client, refresh token and DPoP binding. It calculates expiration from
the returned `expiresIn`, rotates the access and refresh tokens, and saves the
same cache. Existing loader checks retain the client/DPoP binding and account.
The existing metered transport restricts requests to this refresh and exact STS,
with retries/redirects suppressed and durable before-forward accounting.

`LoginProvider` ordinarily exposes SDK `RefreshableCredentials`. Our isolated
children receive static environment credentials and cannot load the host cache
or independently renew. The host must authenticate at a controlled boundary;
enabling a background provider would bypass the reviewed session/renewal policy.

The retained [stop record](project-environments-collection-stop.json) shows a
successful refresh at ordinal 300 and mandatory identity verification at 301,
followed by failure to cover the original 55-minute window. The approximately
900-second lifetime matches the model. Local cache inspection was limited to
field names, expiration/account conclusions and file metadata; no credentials,
refresh grant, key, cache contents or secret fingerprints are published.
The cache contains no separate refresh-token expiration field. Future refresh
acceptance or a remaining renewable session lifetime cannot be proved offline.
An expired/revoked grant stops; interactive login or a replacement session is
outside this design.

## Selected bounded design

Tooling commit **3e380db5f1ef7dba6587abb003d953bc79436871** contains
[the credential boundary](../../scripts/environment-deploy/phase_credentials.py)
and the revised ledger/driver. The unchanged release source is
**44f1fb79d943501c95e06fcc1a6114111a7c1ea4**.

1. A separately authorized new campaign carries forward **301**, capped at
   **1,799**, and starts one **3,300-second monotonic window** before any possible
   authentication call. New `window-from-301` and campaign paths must be absent.
   The stopped parent, original authorization and consumed `window-from-299`
   claim remain intact. Local checks bind the public stop record and read its
   quiescent SQLite ledger without opening it for writes or replaying it.
2. Validate the exact phase approval, then consume its single-use phase claim.
   Its deadline is the minimum of `now + 240 seconds`, the original campaign
   deadline and the original closed-stage deadline if present. Authentication,
   offline planning, child setup and all child work consume that phase window.
   The original closed-stage deadline starts before close-phase authentication
   and remains bounded by the campaign deadline. Renewal never changes either.
3. Lock the original cache and require the inventoried non-root IAM identity.
   Before initial authentication, file metadata must match the stopped parent's
   retained checkpoint. Later boundaries must match this campaign's previous
   successful checkpoint. Changed inode/size/mtime, another configured session,
   or a cache validation failure stops before accepting replacement credentials.
4. Invoke the pinned fetcher once. It either reuses credentials above its
   five-minute threshold or performs one metered renewal at/below the threshold.
   Always verify the same account and exact non-root identity through mandatory
   metered STS before releasing credentials to child work. At most one refresh
   per phase, at most six across the six phases; each phase needs its own approval.
   Initial collection alone can consume at most one refresh and one host STS.
5. Require validity through the remaining phase deadline plus **five seconds**.
   A short refresh response, denial, identity mismatch, transport uncertainty,
   exhausted call budget, cache change, phase/campaign/closed-window expiry or
   interrupted authentication latches the campaign stopped. The alarm bounds
   authentication; host/proxy checks enforce the same lease for descendants.
   No mid-phase renewal, automatic restart, second refresh, write replay,
   reauthentication or deadline reset occurs.

The 240-second lease plus five-second margin fits below the SDK's existing
300-second reuse threshold without overriding SDK internals or inventing a
duration parameter. Both reader collection and SST diff share one collection
lease; the boundary does not renew between those children. The maximum six host
refreshes and six host STS observations all consume the same cumulative budget.
Additional child/provider STS calls, errors and failed forwards also count.
Renewal permission in the policy does not approve any later phase.

Four minutes is a proposed strict phase ceiling, **not proven live capacity**.
If SST deployment or collection takes longer, it stops without retry. A timeout
after closure leaves the stage closed for separate inspection/recovery. Review
latency still consumes the original 55-minute window and, while closed, the
original 30-minute subwindow. This design cannot guarantee that all phases,
propagation and human reviews fit those windows; no capacity claim or automatic
reopen is implied.

## Offline validation and review command

**22 credential tests pass**, using the real pinned SDK serializer/fetcher,
synthetic DPoP/cache material and mocked physical sends, with sockets and DNS
forbidden. They cover the supported lifetime/renewal threshold, expired reuse
versus renewal, real cache-loader rotation in place, short issued credentials,
renewal denial/uncertainty, identity/account mismatch, external cache changes,
shared accounting/exhaustion, per-phase/campaign renewal bounds, approval order,
phase replay, deadline continuity and interruption, and sanitized ledger output.

**17 policy/accounting tests** also pass. **Eight isolated transport cases** pass
using real pinned SST/Pulumi and Go/Node/Python children with synthetic
credentials and loopback responses. The new phase-deadline case stops all
descendants without forwarding or extending the original campaign deadline.
Historical fixture logs are preserved; each transport run now retains logs in
a separate private directory. No real AWS credential or service is used by
these fixtures.

`npm run verify` passed with external networking and dependency downloads
disabled: format, lint, application/infrastructure/tooling tests, frontend build
and Go tests/vet. Ruff lint/format passed. The artifact/source/attestation and
stopped-parent local check passed. The clean code commit's template was read
locally and remained `authorized: false`; no new campaign or claim was created.

The revised [execution packet](project-environments-deployment-execution.md)
contains the exact clean-checkout and private template instructions. After
separate approval of this policy and a new initial collection, its proposed
single start command is:

```sh
python3 -B scripts/environment-deploy/driver.py start --campaign .build/project-environments-execution/release-from-301 --authorization .build/project-environments-execution/authorization-from-301.json
```

**Do not run it under this local correction request.** The original ceiling
approval remains accepted, but the old collection authorization was consumed.
No approved authorization file was created for the command above. No new live
baseline, quiescence, configuration/IAM, complete resource diff, generated-artifact
agreement, propagation or runtime result is claimed. Each later phase, runtime
campaign and rollback still requires separate approval.
