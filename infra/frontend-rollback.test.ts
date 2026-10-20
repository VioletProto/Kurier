import {
  existsSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  readdirSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import os from "node:os";
import path from "node:path";
import { execFileSync } from "node:child_process";
import { afterEach, expect, it, vi } from "vitest";
import {
  buildFrontendArtifact,
  inspectArtifact,
  planFrontendRetention,
  restoreFrontendArtifact,
  retainedObjectKey,
  sha256,
} from "./frontend-artifact";

vi.mock("node:child_process", () => ({ execFileSync: vi.fn() }));
const config = {
  VITE_API_URL: "https://abc.execute-api.us-east-2.amazonaws.com",
  VITE_COGNITO_USER_POOL_ID: "us-east-2_Example",
  VITE_COGNITO_CLIENT_ID: "publicclient",
  VITE_KURIER_STAGE: "dev-api",
};
let root: string;
afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllEnvs();
  if (root) rmSync(root, { recursive: true, force: true });
});

it("recovers exact A from remote retained objects after B overwrites all nonhashed roots, without rebuilding", () => {
  root = mkdtempSync(path.join(os.tmpdir(), "kurier-rollback-"));
  vi.spyOn(process, "cwd").mockReturnValue(root);
  const dist = path.join(root, "apps/web/dist");
  mkdirSync(path.join(dist, "assets"), { recursive: true });
  const remote = new Map<string, Buffer>();
  function release(label: string) {
    writeFileSync(
      path.join(dist, "index.html"),
      `<html>${label}<script src="/startup.js"></script><script src="/assets/app-12345678.js"></script><link href="/assets/app-12345678.css"></html>`,
    );
    writeFileSync(path.join(dist, "startup.js"), `console.log('${label}');`);
    writeFileSync(path.join(dist, "404.html"), `<html>Missing ${label}</html>`);
    writeFileSync(
      path.join(dist, "assets/app-12345678.js"),
      "console.log('shared');",
    );
    writeFileSync(
      path.join(dist, "assets/app-12345678.css"),
      "body{color:black}",
    );
    const built = buildFrontendArtifact(config);
    function upload(relative = "") {
      for (const entry of readdirSync(path.join(built.directory, relative), {
        withFileTypes: true,
      })) {
        const key = path.posix.join(relative, entry.name);
        if (entry.isDirectory()) upload(key);
        else remote.set(key, readFileSync(path.join(built.directory, key)));
      }
    }
    upload();
    return built.manifest;
  }
  const a = release("A");
  const b = release("B");
  expect(a.releaseId).not.toBe(b.releaseId);
  expect(remote.get("startup.js")?.toString()).toContain("B");
  expect(
    remote.get(`_releases/${a.releaseId}/startup.js`)?.toString(),
  ).toContain("A");
  const localA = path.join(root, ".build/frontend-releases", a.releaseId);
  rmSync(localA, { recursive: true });
  vi.mocked(execFileSync).mockClear();
  const read = (key: string) => {
    const bytes = remote.get(key);
    if (!bytes) throw Error("Missing historical object.");
    return bytes;
  };
  // Every required retained object, including the manifest, must exist and match.
  const keys = [
    `_releases/${a.releaseId}/artifact.json`,
    ...a.files.map((f) => retainedObjectKey(a.releaseId, f.path)),
  ];
  for (const key of keys) {
    const original = read(key);
    remote.delete(key);
    expect(() => restoreFrontendArtifact(a.releaseId, config, read)).toThrow();
    expect(existsSync(localA)).toBe(false);
    const corrupted = Buffer.from(original);
    corrupted[0] ^= 1;
    remote.set(key, corrupted);
    expect(() => restoreFrontendArtifact(a.releaseId, config, read)).toThrow();
    expect(existsSync(localA)).toBe(false);
    remote.set(key, original);
  }
  expect(restoreFrontendArtifact(a.releaseId, config, read)).toEqual(a);
  for (const file of a.files) {
    const bytes = readFileSync(path.join(localA, "app", file.path));
    expect(bytes.length).toBe(file.bytes);
    expect(sha256(bytes)).toBe(file.sha256);
  }
  expect(inspectArtifact(path.join(localA, "app"), config)).toEqual(a);
  vi.stubEnv("KURIER_FRONTEND_ROLLBACK_RELEASE", a.releaseId);
  expect(buildFrontendArtifact(config).manifest).toEqual(a);
  expect(execFileSync).not.toHaveBeenCalled();
  expect(
    readdirSync(path.join(root, ".build/frontend-releases")).some((n) =>
      n.startsWith(".restore-"),
    ),
  ).toBe(false);
  const expired = planFrontendRetention(
    [
      { manifest: a, uploadedAt: "2025-01-01" },
      { manifest: b, uploadedAt: "2025-01-01" },
    ],
    b.releaseId,
    b.releaseId,
    Date.parse("2026-10-16"),
  );
  expect(expired).toEqual(
    ["404.html", "artifact.json", "index.html", "startup.js"]
      .map((n) => `_releases/${a.releaseId}/${n}`)
      .sort(),
  );
  expect(
    planFrontendRetention(
      [
        { manifest: a, uploadedAt: "2025-01-01" },
        { manifest: b, uploadedAt: "2025-01-01" },
      ],
      b.releaseId,
      a.releaseId,
    ),
  ).toEqual([]);
});
