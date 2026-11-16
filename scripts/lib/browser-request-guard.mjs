// Request objects stay in RAM. Never retain URLs, headers, bodies or SDK errors.
export function browserRequestGuard(context) {
  const pending = new Set();
  let failures = 0;
  let closeCancellations = 0;
  let closing = false;
  const relevant = (request) => {
    const url = new URL(request.url());
    return (
      url.pathname.startsWith("/api/v1/") ||
      url.hostname === "cognito-idp.us-east-2.amazonaws.com"
    );
  };
  context.on("request", (request) => {
    if (relevant(request)) pending.add(request);
  });
  context.on("requestfinished", (request) => pending.delete(request));
  context.on("requestfailed", (request) => {
    if (!relevant(request)) return;
    pending.delete(request);
    failures++;
    if (closing && request.failure()?.errorText === "net::ERR_ABORTED")
      closeCancellations++;
  });
  function assertSettled() {
    if (failures) throw Error("Browser transport failed; no automatic replay");
    if (pending.size) throw Error("Browser requests still in flight");
  }
  return {
    assertSettled,
    beginClose() {
      assertSettled();
      closing = true;
    },
    summary: () => ({
      pendingRequests: pending.size,
      transportFailures: failures,
      closeCancellations,
    }),
  };
}
