# Guided MCP connection setup

Research for [Kurier card #50](https://trello.com/c/mdWalEux), 2026-11-19.
Expanded to native Windows on 2026-11-21. [Delivery card #51](https://trello.com/c/6zSxoiHw).

Recommend versioned, prebuilt Linux x64 and native Windows x64 bridge assets with one interactive `kurier-mcp setup codex` command, linked from **Settings → MCP connection**. The user approves the download and local installation, chooses and confirms project scope in Kurier, then approves private credential storage and any Codex registration change in the terminal. The command reuses the hidden token prompt and registers the stdio launcher through Codex. Keep the [current guide](../development/read-only-mcp.md) available for source builds, unsupported platforms and manual registration.

## Options compared

| Option                                | What the user still does                                                                                                                             | Assessment                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| ------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Current guide                         | Build the bridge, approve scope in Settings, paste into `pair`, register with `codex mcp add`, launch a fresh Codex session, and make a scoped read. | Supports the existing safeguards and provides a fallback. Requires a checkout, Go and separate commands with absolute paths.                                                                                                                                                                                                                                                                                                                           |
| Prebuilt bridge and one setup command | Approve installation, approve scope, invoke setup, approve the local changes and paste once, then launch Codex and read.                             | Removes the local build and separate registration command. Keeps the existing credential and API contracts. The Linux trial measured the current guide and guided workflow.                                                                                                                                                                                                                                                                            |
| Codex-supported registration UI       | Install and pair the bridge, then use the custom MCP entry in Codex settings or the CLI.                                                             | Codex supports a command and arguments, but the registration surface does not acquire the bridge or perform Kurier pairing. The published desktop deep links cover settings and known-marketplace plugin installation. They do not document arbitrary stdio registration from a web page. [Codex MCP](https://learn.chatgpt.com/docs/extend/mcp?surface=cli), [desktop commands](https://learn.chatgpt.com/docs/app/commands#deep-links).              |
| Automatic browser handoff             | Install a local helper or use a supported client integration, then authorize a browser-to-local exchange.                                            | Could remove the paste, but needs a new trust and recovery design. An opaque, session-bound, single-use exchange would need short expiry, replay protection, origin binding and explicit local approval. Remote Streamable HTTP with OAuth would require a new MCP transport and authorization surface. Neither is needed to remove the immediate build and registration friction. [Codex MCP](https://learn.chatgpt.com/docs/extend/mcp?surface=cli). |

The selected design retains the separate 30-day read credential, explicit selected-project or all-current-and-future-owned consent, live scope changes, revocation, audit-before-release and sanitized evidence projections. Browser authentication stays with the browser. Linux uses the existing owner-only credential file. The Windows proposal uses a per-user Credential Manager entry. URLs, command arguments, environment, client configuration and logs carry no token. [Accepted MCP contract](../architecture/read-only-mcp-contract.md), [project scope delivery](../development/mcp-project-scopes.md), [Settings contract](../architecture/connection-settings-contract.md).

## Prototype and measurement

Both paths started in a signed-in Chromium fixture with **Settings** ready to
open. Each trial used a fresh browser context, a fresh temporary Codex home,
no bridge binary or credential file, and a fresh MCP issuance. The browser
used the repository's actual Settings UI and Go API with DynamoDB Local 3.3.0.
Cognito sign-in was simulated. The Go API also served HTTPS under a private
test CA for the real bridge. Each trial used a separate fixture issuance rate
window after an initial run reached the five-per-hour account limit.

| Path             | Manual sequence from Settings                                                                                                                                                                                                                                         | Seconds to first scoped Codex read | Median  |
| ---------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------- | ------- |
| Current guide    | Build with Go. Refresh status, choose projects, consent and create the credential. Run `pair`, paste at the hidden prompt, and run `codex mcp add`. Open a fresh Codex session and call `list_projects`.                                                              | 1.706, 1.624, 1.628                | 1.628 s |
| Prebuilt + setup | Approve acquiring and placing the bridge. Refresh status, choose projects, consent and create the credential. Run setup, approve private storage and the announced Codex change, and paste at the hidden prompt. Open a fresh Codex session and call `list_projects`. | 1.032, 1.022, 1.027                | 1.027 s |

Elapsed time ran from the Settings click to a verified `list_projects` result in
the installed Codex client. The baseline used a warm Go build cache. The guided
trial copied a previously built executable locally and used a disposable wrapper
around the real `pair` and Codex CLI. That copy represented a user-approved
artifact installation. The prototype used Codex app-server to check saved
configuration, discover seven tools and make real tool calls without a model
turn. These are automated comparisons with three trials per path. Real release
download, signature verification, human reading/typing time, initial toolchain
installation, interactive TUI navigation and hosted latency are outside the
timing. The concrete reduction is two terminal setup actions, the local build
and separate registration, in exchange for deliberate artifact installation.

| Case                           | Observed in the isolated fixture                                                                                                                                                                                                                                                                                                                                                                                  |
| ------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Selected or all owned projects | Issuance required explicit read consent. The all-owned mode also required future-project consent. Selected access returned exactly one chosen project. All-owned access matched the owner catalog and discovered a project created after pairing, in the same Codex session. A foreign project read was denied. The all-owned session also retrieved published fixture evidence.                                  |
| Credential containment         | No token appeared in the browser URL or Web Storage, terminal output, Codex output or inspected Codex home files. It was passed to the real bridge only through the hidden PTY prompt and persisted by the existing pair command with a 0600 file in a 0700 directory. Codex received only launcher arguments and the credential-file path. Closing the one-time display removed the token from the UI.           |
| Existing registrations         | The wrapper rejected a different launcher with the same name before pairing or writing. A matching entry was reused with byte-identical Codex configuration. A separate disposable CLI check confirmed that bare `codex mcp add` replaces a same-name entry.                                                                                                                                                      |
| Cancellation and interruption  | Declining the proposed setup or pressing Ctrl-C in the hidden prompt left no credential or registration. The wrapper handled the interrupt and restored terminal settings. An injected stop after successful pairing left only the private file. Retrying with the still-visible token finished registration and read successfully. Resume directly from a valid persisted credential is a follow-up improvement. |
| Revocation and expiry          | Browser revocation and a separate 31-day test clock advance each caused new pair attempts and fresh Codex reads to fail. Failed pairing kept the existing local file and configuration byte-for-byte intact.                                                                                                                                                                                                      |

The focused DynamoDB Local integration tests passed for audit races, visibility
and narrowing, sanitized evidence, revocation and expiry boundaries. The
repository's installed-Codex regression also passed seven-tool coverage,
summary/complete evidence, oversize denial, scope denial and revocation
against its HTTPS API fixture. Those regressions can be rerun with
an in-memory DynamoDB Local instance on port 8000 and an installed Codex CLI.

```sh
KURIER_DYNAMODB_TEST_ENDPOINT=http://127.0.0.1:8000 go -C services/api test -tags=integration -run '^TestIntegrationMCP(SevenToolsAndContainment|ReadAuditRaces|IssuanceRevocationAndGeneration|ConnectionReadRaces|ConnectionDiscoveryAndLiveNarrowing|ConcurrentIssuanceAndExpiryBoundary)$' -count=1 ./execution
KURIER_CODEX_INTEGRATION=1 go -C services/mcp test -tags=codex -run '^TestInstalledCodexMCPIntegration$' -count=1 .
```

This spike used disposable fixture configuration and simulated account
credentials. Hosted services were outside the experiment.

## Client and platform compatibility

| Surface                                     | Result or adoption boundary                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
| ------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Codex CLI on Linux x86-64                   | Installed `codex-cli 0.170.0` was the prototype target. `mcp add`, `get` and `list --json` operate on the selected client configuration. A fresh local session reads from the installed executable and private credential.                                                                                                                                                                                                                                                                                                                                                                                                                        |
| Codex IDE extension and ChatGPT desktop app | OpenAI documents shared MCP configuration for clients on the same Codex host. Validate IDE launch and desktop launch independently before advertising them. The desktop app is available on macOS and Windows, and in preview on Linux. Remote SSH, containers and WSL can have different binaries, homes and policies. Install and register inside the actual execution environment. [Codex MCP](https://learn.chatgpt.com/docs/extend/mcp?surface=cli), [Linux desktop preview](https://learn.chatgpt.com/docs/linux/linux-app), [WSL](https://learn.chatgpt.com/docs/windows/wsl).                                                             |
| macOS and Linux ARM64                       | The Unix bridge is a candidate. Verify execution, hidden TTY input, private-file access, credential path resolution, and platform behavior with the release artifact before advertising availability. macOS also requires a distribution decision for signing/notarization and quarantine.                                                                                                                                                                                                                                                                                                                                                        |
| Native Windows                              | A native Codex CLI launched the probe and called its synthetic tool. The shipped bridge still fails closed for Windows credential persistence and stdio. A native desktop session needs its own qualification. See the experiment below. [Windows bridge implementation](../../services/mcp/storage_windows.go), [Windows desktop app](https://learn.chatgpt.com/docs/windows/windows-app#windows-subsystem-for-linux-wsl).                                                                                                                                                                                                                       |
| Claude Code, Cursor and VS Code Copilot     | All document a local stdio path. Claude Code offers its own add command, Cursor offers an install link with a client prompt, and Copilot offers a guided add/trust flow. Their client setup and MCP negotiation with Kurier were not exercised in this spike. A Cursor link could carry a launcher description only. Pairing and a locally installed bridge would still be required. [Claude Code MCP](https://code.claude.com/docs/en/mcp), [Cursor install links](https://cursor.com/docs/mcp/install-links), [VS Code MCP setup](https://code.visualstudio.com/docs/agent-customization/mcp-servers#add-a-server-with-a-launch-command-stdio). |
| ChatGPT web, hosted Work or Codex cloud     | A local host registration does not connect a managed browser/cloud session. Treat remote MCP hosting as a separate design. [Codex MCP](https://learn.chatgpt.com/docs/extend/mcp?surface=cli).                                                                                                                                                                                                                                                                                                                                                                                                                                                    |

## Linux and native Windows

Use the OS and CPU architecture of the machine or environment that launches
`serve`. The browser can suggest a download and let the user override it.
The installer or bootstrap checks the local target against the release
manifest before installation. An x64 Linux host selects the Linux executable.
An x64 native Windows host selects the `.exe`. A Windows browser connecting
to Codex inside WSL2, SSH, or a Linux container selects the Linux target when
the local bootstrap runs there. Reject unsupported targets without
installing or changing Codex settings. [Codex on WSL2](https://learn.chatgpt.com/docs/windows/wsl).

The [platform probe](../../spikes/mcp-bridge-platform) checked Linux and Windows
x64 selection and rejected unsupported OS and architecture inputs. The
installed Codex CLI 0.170.0 registered and launched the selected fixture and
called its synthetic `list_projects` tool on Linux and native Windows.
Linux used a pollable pipe. Windows used synchronous byte-mode stdout with
`PIPE_NOWAIT`, then ended the stream on a short write. Both fixtures stopped
with an unread stdout pipe instead of hanging. A Windows process read a
synthetic canary from Credential Manager after a separate process wrote it.
The probe deleted the disposable entry and verified it was absent. The first
Windows run completed those checks, then failed to remove an open Codex
directory. After the harness waited for Codex shutdown, both native runner
jobs passed. [GitHub Actions run](https://github.com/VioletProto/Kurier/actions/runs/44929671452).

The Windows transport test establishes feasibility for the observed Codex pipe
type. Windows nonblocking byte-mode writes can be partial. The production
adapter needs to terminate or invalidate that stream on a partial frame and
verify deadline behavior under actual Codex backpressure across the largest
supported MCP responses. Hidden console entry, interrupted prompt restoration,
real Settings issuance through the Windows release build, and WSL2 and desktop
client acceptance still need separate tests. The prototype never used a
real token or contacted Kurier's hosted API. [Windows pipe modes](https://learn.microsoft.com/en-us/windows/win32/ipc/named-pipe-type-read-and-wait-modes), [Credential Manager](https://learn.microsoft.com/en-us/windows/win32/api/wincred/ns-wincred-credentialw).

## Follow-up scope and estimate

1. Publish pinned Linux x86-64 and Windows x86-64 artifacts with trusted provenance, version and digest/signature verification. Document the recommended user-owned executable location, deliberate updates, rollback and removal. Settings can link to the release and show a token-free command with the trusted origin for that deployment. Keep approval for installing the bridge separate from project authorization and client configuration. [GitHub Releases](https://docs.github.com/en/repositories/releasing-projects-on-github/about-releases).
2. Implement `setup codex` in the Go bridge. Resolve the actual executable and canonical private credential path, display safe proposed changes, and fail closed for unsupported clients/configuration, cancelled approval, or a conflicting or disabled registration. Reuse matching entries and keep existing user options. Validate the capability using the existing read path before writing the private file or registering a launcher. A restart with the same valid private file should be able to finish registration after fresh approval without requiring a new token. An invalid or lost credential must send users back to Settings to revoke/reissue as appropriate.
3. Before enabling automatic registration, verify concurrency with the actual installed client. The tested CLI add operation can replace a same-name entry and offers no demonstrated conditional insertion. A preflight check alone leaves a window for another writer. Prove a conflict-safe write on supported versions or defer that write to the user through Codex's own registration UI. Read back the effective launcher and confirm `list_projects` through a fresh client. Treat partial completion as recoverable, and keep browser Revoke authoritative for a discarded file or abandoned setup.
4. Verify on a clean supported machine with the release download and a real operator, an existing registration and a forced failure after pairing. Include an end-to-end hosted first read and revocation before promoting the guided path. Keep Cognito, backend contracts, scope and MCP transport out of the implementation scope. Review additional client and OS support separately.

The original Linux estimate was **16–24 active hours**. Budget an additional
**12–20 hours** for native Windows console pairing and Credential Manager,
the guarded stdio adapter, the second artifact, and clean Windows release
validation. The two-platform estimate is **28–44 active hours**. Allow roughly
2–4 hours per release for both artifact and client smoke tests. WSL2, macOS
and desktop client qualification are separate estimates.
