import { readFileSync } from "node:fs";

// Development only: keep the browser on HTTPS and proxy solely to loopback.
export function sessionDevelopmentConfig(env: NodeJS.ProcessEnv) {
  if (env.VITE_KURIER_SESSION_BROKER !== "true") return {};
  const target = new URL(
    env.KURIER_SESSION_PROXY_TARGET ?? "http://127.0.0.1:8081",
  );
  if (
    target.protocol !== "http:" ||
    !["127.0.0.1", "[::1]"].includes(target.hostname) ||
    !target.port ||
    target.pathname !== "/" ||
    target.username ||
    target.password ||
    target.search ||
    target.hash
  )
    throw Error(
      "Session development proxy requires an explicit loopback HTTP port.",
    );
  if (!env.KURIER_LOCAL_HTTPS_CERT || !env.KURIER_LOCAL_HTTPS_KEY)
    throw Error(
      "Session development requires a trusted local HTTPS certificate and key.",
    );
  return {
    server: {
      https: {
        cert: readFileSync(env.KURIER_LOCAL_HTTPS_CERT),
        key: readFileSync(env.KURIER_LOCAL_HTTPS_KEY),
      },
      proxy: {
        "/api/v1/browser-session": {
          target: target.origin,
          changeOrigin: false,
        },
      },
    },
  };
}
