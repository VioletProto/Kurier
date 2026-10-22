// SDK-created agents ignore Node's environment proxy setting. Force the same
// metered CONNECT transport; the isolated network still blocks any bypass.
import http from "node:http";
import https from "node:https";
import { createRequire, syncBuiltinESMExports } from "node:module";
const require = createRequire(
  new URL("../../.sst/platform/package.json", import.meta.url),
);
const { HttpsProxyAgent } = require("https-proxy-agent");
const { HttpProxyAgent } = require("http-proxy-agent");
if (!process.env.KURIER_ATTEMPT_METER_ACTIVE || !process.env.HTTPS_PROXY)
  throw Error(
    "Metered Node transport requires explicit isolated meter context",
  );
for (const [module, Agent] of [
  [http, HttpProxyAgent],
  [https, HttpsProxyAgent],
]) {
  const original = module.request;
  const agent = new Agent(process.env.HTTPS_PROXY);
  module.request = function (...args) {
    const target =
      typeof args[0] === "string" || args[0] instanceof URL
        ? new URL(args[0])
        : args[0];
    const hostname = target?.hostname ?? target?.host;
    // Pulumi's local RPC stays inside the isolated namespace. It cannot reach
    // the host's loopback or any external service and needs no cloud allowance.
    if (["localhost", "127.0.0.1", "::1", "[::1]"].includes(hostname))
      return original.apply(this, args);
    if (typeof args[0] === "string" || args[0] instanceof URL) {
      if (typeof args[1] === "function" || args[1] === undefined)
        args.splice(1, 0, { agent });
      else args[1] = { ...args[1], agent };
    } else args[0] = { ...args[0], agent };
    return original.apply(this, args);
  };
  module.get = function (...args) {
    const request = module.request(...args);
    request.end();
    return request;
  };
}
syncBuiltinESMExports();
