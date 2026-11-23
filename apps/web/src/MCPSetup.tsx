import { useState } from "react";
import { apiOrigin } from "./api";

const release = "https://github.com/VioletProto/Kurier/releases/tag/mcp-v1.0.0";
const metadata =
  "https://api.github.com/repos/VioletProto/Kurier/releases/tags/mcp-v1.0.0";

function setupCommand(windows: boolean) {
  const asset = windows ? "kurier-mcp-windows-x64.exe" : "kurier-mcp-linux-x64";
  const url = release.replace("/tag/", "/download/") + "/" + asset;
  if (windows)
    return `& {
  $ErrorActionPreference = 'Stop'
  if ([Runtime.InteropServices.RuntimeInformation]::OSArchitecture -ne 'X64' -or $env:OS -ne 'Windows_NT') { throw 'Open PowerShell on a Windows x64 host.' }
  $dir = Join-Path $env:LOCALAPPDATA 'Kurier/mcp-v1.0.0'
  New-Item -ItemType Directory -Force $dir | Out-Null
  $exe = Join-Path $dir 'kurier-mcp.exe'
  $expected = ((Invoke-RestMethod '${metadata}').assets | Where-Object name -eq '${asset}').digest
  $tmp = Join-Path $dir ([guid]::NewGuid().ToString())
  try {
    Invoke-WebRequest -UseBasicParsing '${url}' -OutFile $tmp
    if (('sha256:' + (Get-FileHash $tmp -Algorithm SHA256).Hash) -ne $expected) { throw 'Download verification failed.' }
    Move-Item -Force $tmp $exe
  } finally { if (Test-Path $tmp) { Remove-Item -LiteralPath $tmp } }
  & $exe setup codex --api-origin '${apiOrigin}'
}`;
  return `(
  set -eu
  [ "$(uname -s)/$(uname -m)" = Linux/x86_64 ] || { echo 'Open a Linux x64 terminal.'; exit 1; }
  dir="$HOME/.local/share/kurier/mcp-v1.0.0"
  mkdir -p -m 700 "$dir"
  tmp=$(mktemp "$dir/download.XXXXXXXX")
  trap 'rm -f "$tmp"' EXIT
  expected=$(curl -fsSL '${metadata}' | python3 -c 'import json,sys; print(next(a["digest"] for a in json.load(sys.stdin)["assets"] if a["name"]=="${asset}"))')
  curl -fsSL '${url}' -o "$tmp"
  actual=$(sha256sum < "$tmp")
  [ "sha256:\${actual%% *}" = "$expected" ] || { echo 'Download verification failed.'; exit 1; }
  chmod 700 "$tmp"
  mv -f "$tmp" "$dir/kurier-mcp"
  "$dir/kurier-mcp" setup codex --api-origin '${apiOrigin}'
)`;
}

export function MCPSetup() {
  const [windows, setWindows] = useState(/Windows/i.test(navigator.userAgent));
  const [copied, setCopied] = useState("");
  const command = setupCommand(windows);
  return (
    <details>
      <summary>Connect your coding client once</summary>
      <p>
        Use the terminal where you run Codex CLI. Choose Linux for Codex inside
        WSL. The Linux installer uses curl and Python 3.
      </p>
      <label>
        Codex computer{" "}
        <select
          value={windows ? "windows" : "linux"}
          onChange={(e) => {
            setWindows(e.target.value === "windows");
            setCopied("");
          }}
        >
          <option value="linux">Linux x64</option>
          <option value="windows">Windows x64 (PowerShell)</option>
        </select>
      </label>
      {apiOrigin.startsWith("https://") ? (
        <>
          <p>
            Run this command to download and verify the{" "}
            <a href={release}>Kurier bridge</a> in your user account. Leave the
            terminal open.
          </p>
          <button
            type="button"
            onClick={async () => {
              try {
                await navigator.clipboard.writeText(command);
                setCopied("Copied.");
              } catch {
                setCopied("Select and copy the command below.");
              }
            }}
          >
            Copy setup command
          </button>{" "}
          <span role="status">{copied}</span>
          <details>
            <summary>View setup command</summary>
            <pre>{command}</pre>
          </details>
          <p>
            Refresh MCP status below, choose your projects and create a
            credential. Paste the token into the hidden terminal prompt. Run the
            registration command it prints, then open a fresh Codex session and
            ask “Which Kurier projects can I see?”
          </p>
        </>
      ) : (
        <p>Open the hosted HTTPS Kurier Settings to pair the bridge.</p>
      )}
    </details>
  );
}
