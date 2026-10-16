import { createHash } from "node:crypto";
import { execFileSync } from "node:child_process";
import {
  cpSync,
  existsSync,
  lstatSync,
  mkdirSync,
  readFileSync,
  readdirSync,
  writeFileSync,
} from "node:fs";
import path from "node:path";
import { hostedPublicConfig } from "../apps/web/public-config.ts";

export const htmlCache =
  "no-store, max-age=0, must-revalidate, stale-if-error=0";
export const assetCache = "public, max-age=31536000, immutable";
export function sha256(value: string | Buffer) {
  return createHash("sha256").update(value).digest("hex");
}

export function inspectArtifact(
  directory: string,
  environment: Record<string, unknown>,
) {
  const config = hostedPublicConfig(environment);
  const files: {
    path: string;
    bytes: number;
    sha256: string;
    cacheControl: string;
  }[] = [];
  function visit(relative = "") {
    for (const entry of readdirSync(path.join(directory, relative))) {
      const name = path.posix.join(relative, entry);
      const full = path.join(directory, name);
      const stat = lstatSync(full);
      if (stat.isSymbolicLink())
        throw Error("Artifact symlinks are forbidden.");
      if (stat.isDirectory()) {
        visit(name);
        continue;
      }
      if (
        !["index.html", "startup.js", "404.html"].includes(name) &&
        !/^assets\/[A-Za-z0-9_-]+-[A-Za-z0-9_-]{8,}\.(js|css)$/.test(name)
      )
        throw Error("Unexpected public artifact file.");
      if (files.length >= 128 || stat.size > 8 * 1024 * 1024)
        throw Error("Public artifact exceeds bounds.");
      const bytes = readFileSync(full);
      if (
        /KURIER_BROWSER_FIXTURE|KURIER_CURSOR_KEY|AWS_SECRET_ACCESS_KEY|BEGIN (?:RSA |EC )?PRIVATE KEY/.test(
          bytes.toString(),
        )
      )
        throw Error("Unsafe artifact content.");
      files.push({
        path: name,
        bytes: bytes.length,
        sha256: sha256(bytes),
        cacheControl: name.startsWith("assets/") ? assetCache : htmlCache,
      });
    }
  }
  visit();
  files.sort((a, b) => a.path.localeCompare(b.path));
  const html = readFileSync(path.join(directory, "index.html"), "utf8");
  if (
    !files.some((f) => f.path === "startup.js") ||
    !files.some((f) => f.path.endsWith(".css")) ||
    !files.some((f) => f.path.startsWith("assets/") && f.path.endsWith(".js"))
  )
    throw Error("Incomplete hosted artifact.");
  for (const match of html.matchAll(/(?:src|href)="([^"]+)"/g)) {
    if (
      !match[1].startsWith("/") ||
      !files.some((f) => "/" + f.path === match[1])
    )
      throw Error("Artifact references an unavailable asset.");
  }
  if (files.reduce((sum, f) => sum + f.bytes, 0) > 20 * 1024 * 1024)
    throw Error("Public artifact exceeds total bounds.");
  const payload = { schemaVersion: 1, publicConfig: config, files };
  return { ...payload, releaseId: sha256(JSON.stringify(payload)) };
}

export function buildFrontendArtifact(environment: Record<string, unknown>) {
  const config = hostedPublicConfig(environment);
  const root = process.cwd();
  const archiveRoot = path.join(root, ".build/frontend-releases");
  const rollback = process.env.KURIER_FRONTEND_ROLLBACK_RELEASE;
  let source: string;
  if (rollback) {
    if (!/^[a-f0-9]{64}$/.test(rollback))
      throw Error("Invalid rollback release.");
    source = path.join(archiveRoot, rollback, "app");
  } else {
    // Do not pass AWS, SST secrets, arbitrary VITE values or dotenv files to Vite.
    execFileSync(
      "npm",
      ["run", "build", "--workspace", "@kurier/web", "--", "--mode", "hosted"],
      {
        cwd: root,
        env: { PATH: process.env.PATH, HOME: process.env.HOME, ...config },
        stdio: "pipe",
      },
    );
    source = path.join(root, "apps/web/dist");
  }
  const manifest = inspectArtifact(source, config);
  if (rollback && manifest.releaseId !== rollback)
    throw Error("Rollback artifact/config hash mismatch.");
  const release = path.join(archiveRoot, manifest.releaseId);
  const app = path.join(release, "app");
  const site = path.join(release, "site");
  if (!existsSync(app)) {
    mkdirSync(app, { recursive: true });
    cpSync(source, app, { recursive: true });
  }
  if (JSON.stringify(inspectArtifact(app, config)) !== JSON.stringify(manifest))
    throw Error("Retained release differs from artifact.");
  mkdirSync(site, { recursive: true });
  cpSync(app, site, { recursive: true });
  const archive = path.join(site, "_releases", manifest.releaseId);
  mkdirSync(archive, { recursive: true });
  cpSync(path.join(app, "index.html"), path.join(archive, "index.html"));
  const json = JSON.stringify(manifest, null, 2) + "\n";
  writeFileSync(path.join(site, "artifact.json"), json);
  writeFileSync(path.join(archive, "artifact.json"), json);
  writeFileSync(path.join(release, "artifact.json"), json);
  mkdirSync(path.join(root, ".sst"), { recursive: true });
  writeFileSync(path.join(root, ".sst/frontend-hosting-artifact.json"), json);
  return { directory: site, manifest };
}

// Planning only: no object is removed automatically or by the maintenance Lambda.
export function planFrontendRetention(
  releases: {
    manifest: ReturnType<typeof inspectArtifact>;
    uploadedAt: string;
  }[],
  current: string,
  previous: string,
  now = Date.now(),
) {
  if (
    ![current, previous].every(
      (id) =>
        /^[a-f0-9]{64}$/.test(id) &&
        releases.some((r) => r.manifest.releaseId === id),
    )
  )
    throw Error("Both known-good releases must be inventoried.");
  const keep = new Set<string>();
  const expired: typeof releases = [];
  for (const release of releases) {
    const { releaseId, ...payload } = release.manifest;
    if (
      payload.schemaVersion !== 1 ||
      sha256(JSON.stringify(payload)) !== releaseId ||
      !Number.isFinite(Date.parse(release.uploadedAt))
    )
      throw Error("Invalid release manifest or timestamp.");
    hostedPublicConfig(payload.publicConfig);
    for (const f of payload.files)
      if (
        !/^[a-f0-9]{64}$/.test(f.sha256) ||
        (f.path.startsWith("assets/") &&
          !/^assets\/[A-Za-z0-9_-]+-[A-Za-z0-9_-]{8,}\.(js|css)$/.test(f.path))
      )
        throw Error("Invalid retained asset.");
    if (
      releaseId === current ||
      releaseId === previous ||
      Date.parse(release.uploadedAt) >= now - 30 * 86400000
    ) {
      for (const f of payload.files) keep.add(f.path);
    } else expired.push(release);
  }
  return [
    ...new Set(
      expired.flatMap(({ manifest }) => [
        `_releases/${manifest.releaseId}/index.html`,
        `_releases/${manifest.releaseId}/artifact.json`,
        ...manifest.files
          .filter((f) => f.path.startsWith("assets/") && !keep.has(f.path))
          .map((f) => f.path),
      ]),
    ),
  ].sort();
}
