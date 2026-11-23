import assert from "node:assert/strict";
import { execFileSync, spawn } from "node:child_process";
import { randomUUID } from "node:crypto";
import { mkdirSync, mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { createInterface } from "node:readline";
import { once } from "node:events";
import { setTimeout as delay } from "node:timers/promises";

function artifactFor(platform, architecture) {
  if (architecture !== "x64") throw new Error("Unsupported architecture");
  if (platform === "linux") return "kurier-platform-linux";
  if (platform === "win32") return "kurier-platform-windows.exe";
  throw new Error("Unsupported platform");
}

assert.equal(artifactFor("linux", "x64"), "kurier-platform-linux");
assert.equal(artifactFor("win32", "x64"), "kurier-platform-windows.exe");
assert.throws(() => artifactFor("darwin", "x64"), /Unsupported platform/);
assert.throws(() => artifactFor("win32", "arm64"), /Unsupported architecture/);

const bridge = join(process.cwd(), artifactFor(process.platform, process.arch));
const platform = process.platform === "win32" ? "windows" : "linux";
const root = mkdtempSync(join(tmpdir(), "kurier-platform-spike-"));
const target = "Kurier/PlatformSpike/" + randomUUID();
const codexHome = join(root, "codex");
mkdirSync(codexHome);
const env = { ...process.env, CODEX_HOME: codexHome };
const codexJs = process.env.KURIER_CODEX_JS;
const codex = codexJs ? process.execPath : "codex";
const codexArgs = (...args) => (codexJs ? [codexJs, ...args] : args);
const options = { cwd: root, env, encoding: "utf8", timeout: 15000 };

async function readThroughCodex() {
  const child = spawn(codex, codexArgs("app-server", "--stdio"), {
    cwd: root,
    env,
    stdio: ["pipe", "pipe", "inherit"],
  });
  child.stdin.on("error", () => {});
  const lines = createInterface({ input: child.stdout });
  const timeout = setTimeout(() => lines.close(), 20000);
  const responses = lines[Symbol.asyncIterator]();
  let id = 0;
  function send(message) {
    child.stdin.write(JSON.stringify(message) + "\n");
  }
  async function call(method, params) {
    send({ id: ++id, method, params });
    while (true) {
      const line = await responses.next();
      assert(!line.done, "Codex closed before answering " + method);
      const response = JSON.parse(line.value);
      if (response.id === id) {
        assert(
          !response.error,
          "Codex rejected " + method + ": " + (response.error?.message ?? ""),
        );
        return response.result;
      }
    }
  }

  try {
    await call("initialize", {
      clientInfo: { name: "kurier-platform-spike", version: "1" },
      capabilities: { experimentalApi: true },
    });
    send({ method: "initialized", params: {} });
    const { thread } = await call("thread/start", {
      cwd: root,
      ephemeral: true,
      approvalPolicy: "never",
      sandbox: "read-only",
    });
    const { data } = await call("mcpServerStatus/list", {
      threadId: thread.id,
      serverName: "kurier-platform-probe",
    });
    assert(data[0]?.tools.list_projects, "Codex did not discover the fixture");
    const { structuredContent } = await call("mcpServer/tool/call", {
      threadId: thread.id,
      server: "kurier-platform-probe",
      tool: "list_projects",
      arguments: {},
    });
    assert.equal(structuredContent.project, "synthetic-project");
    assert.equal(structuredContent.platform, platform + "/amd64");
    assert.match(
      structuredContent.writer,
      /^(pollable-pipe|nonblocking-byte-pipe)$/,
    );
    console.log(
      "Codex synthetic scoped read:",
      structuredContent.platform,
      structuredContent.writer,
    );
  } finally {
    clearTimeout(timeout);
    if (child.exitCode === null && child.signalCode === null) {
      const exited = once(child, "close");
      child.stdin.end();
      if (
        !(await Promise.race([
          exited.then(() => true),
          delay(3000, false, { ref: false }),
        ]))
      ) {
        if (platform === "windows") {
          execFileSync(
            "taskkill.exe",
            ["/PID", String(child.pid), "/T", "/F"],
            {
              ...options,
              stdio: "ignore",
            },
          );
        } else {
          child.kill();
        }
      }
      await exited;
    }
    lines.close();
  }
}

async function checkBackpressure() {
  const child = spawn(bridge, ["pressure", target], {
    stdio: ["ignore", "pipe", "pipe"],
  });
  // Intentionally leave stdout unread and only collect the safe fixture result.
  let diagnostics = "";
  child.stderr.setEncoding("utf8").on("data", (chunk) => {
    diagnostics += chunk;
  });
  const timeout = setTimeout(() => child.kill(), 5000);
  try {
    const [code] = await once(child, "exit");
    assert.equal(code, 0, "backpressure fixture failed: " + diagnostics);
    assert.match(diagnostics, /withheld blocked output/);
    console.log(diagnostics.trim());
  } finally {
    clearTimeout(timeout);
    child.stdout.destroy();
  }
}

try {
  console.log("Platform selector:", platform, process.arch);
  console.log(execFileSync(codex, codexArgs("--version"), options).trim());
  execFileSync(bridge, ["seed", target], options);
  execFileSync(
    codex,
    codexArgs(
      "mcp",
      "add",
      "kurier-platform-probe",
      "--",
      bridge,
      "serve",
      target,
    ),
    options,
  );
  await readThroughCodex();
  await checkBackpressure();
} finally {
  try {
    execFileSync(bridge, ["cleanup", target], options);
    execFileSync(bridge, ["missing", target], options);
    if (platform === "windows")
      console.log(
        "Disposable Credential Manager entry deleted and verified absent.",
      );
  } finally {
    rmSync(root, { recursive: true, force: true, maxRetries: 5 });
  }
}
