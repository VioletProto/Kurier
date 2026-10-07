// Opt-in developer browser checks. Uses the genuine memory-only session;
// returns safe IDs/statuses only. Never logs tokens, auth payloads or passwords.
import { api, ApiError, type Deletion, type ProjectDetail } from "./api";
const pause = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));
async function expectDenied(action: () => Promise<unknown>, status: number) {
  try {
    await action();
  } catch (error) {
    if (error instanceof ApiError && error.status === status) return status;
    throw new Error(
      "Validation received an unexpected response; refresh before another write.",
    );
  }
  throw new Error(`Expected ${status}; check failed.`);
}
async function awaitDeletion(op: Deletion) {
  for (let n = 0; n < 45; n++) {
    const current = await api.operation(op);
    if (current.state === "completed") return current;
    await pause(2000);
  }
  throw new Error(
    "Deletion remains pending after 90 seconds; no completion claimed.",
  );
}
export async function validateCloudProjects() {
  const user = await api.me();
  const created: ProjectDetail[] = [];
  for (let n = 0; n < 26; n++) {
    created.push(await api.create(`Cloud browser check ${n + 1}`));
    await pause(150);
  }
  let pagination = false;
  for (let attempt = 0; attempt < 30; attempt++) {
    const first = await api.list();
    if (first.nextCursor) {
      const second = await api.list(first.nextCursor);
      const ids = new Set(
        [...first.items, ...second.items].map((p) => p.projectId),
      );
      if (created.every((p) => ids.has(p.project.projectId))) {
        pagination = true;
        break;
      }
    }
    await pause(1000);
  }
  if (!pagination)
    throw new Error(
      "Cloud pagination did not converge; created projects remain available for inspection.",
    );
  const probe = created[0];
  const detail = await api.detail(probe.project.projectId);
  const renamed = await api.rename(
    detail.project.projectId,
    "Cloud isolation probe",
    detail.etag,
  );
  const staleRename = await expectDenied(
    () => api.rename(detail.project.projectId, "Stale write", detail.etag),
    412,
  );
  const staleDelete = await expectDenied(
    () => api.delete(detail.project.projectId, detail.etag),
    412,
  );
  const operations: Deletion[] = [];
  for (const p of created.slice(1)) {
    operations.push(await api.delete(p.project.projectId, p.etag));
    await pause(150);
  }
  const completed = await awaitDeletion(operations[0]);
  for (const op of operations) await awaitDeletion(op);
  const hiddenAfterDelete = await expectDenied(
    () => api.detail(operations[0].projectId),
    404,
  );
  const repeat = await api.delete(
    created[1].project.projectId,
    created[1].etag,
  );
  if (
    repeat.operationId !== completed.operationId ||
    repeat.state !== "completed"
  )
    throw new Error("Repeat DELETE contract failed.");
  return {
    currentUserId: user.userId,
    currentUserLookup: "passed",
    pagination: "passed",
    createDetailRename: "passed",
    staleRename,
    staleDelete,
    hiddenAfterDelete,
    scheduledDeletionsCompleted: operations.length,
    repeatDelete: "passed",
    isolationProjectId: renamed.project.projectId,
    isolationOwnerId: user.userId,
  };
}
export async function validateForeignProject(
  projectId: string,
  ownerId: string,
) {
  const user = await api.me();
  if (user.userId === ownerId)
    throw new Error("Sign in as a different user first.");
  const read = await expectDenied(() => api.detail(projectId), 404);
  const rename = await expectDenied(
    () => api.rename(projectId, "Denied", '"1"'),
    404,
  );
  const remove = await expectDenied(() => api.delete(projectId, '"1"'), 404);
  const operation = await expectDenied(
    () =>
      api.operation({
        projectId,
        operationId: "unknown-operation",
        state: "accepted",
        startedAt: "",
        completedAt: null,
        retryAfterSeconds: 2,
      }),
    404,
  );
  let cursor: string | undefined;
  do {
    const page = await api.list(cursor);
    if (page.items.some((p) => p.projectId === projectId))
      throw new Error("Foreign project exposed in list.");
    cursor = page.nextCursor ?? undefined;
  } while (cursor);
  return {
    crossUserIsolation: "passed",
    read,
    rename,
    remove,
    operation,
    ownerDifferent: true,
  };
}
export async function cleanupIsolationProbe(
  projectId: string,
  ownerId: string,
) {
  const user = await api.me();
  if (user.userId !== ownerId)
    throw new Error("Sign in as the original owner first.");
  const detail = await api.detail(projectId);
  if (
    detail.project.name !== "Cloud isolation probe" ||
    detail.project.version !== 1
  )
    throw new Error("Owner probe was unexpectedly changed.");
  const op = await api.delete(projectId, detail.etag);
  const completed = await awaitDeletion(op);
  return { ownerProbeUnchanged: true, finalDeletion: completed.state };
}

export async function validateCloudSavedRequests() {
  const user = await api.me();
  const project = await api.create("Saved-request browser validation");
  const projectId = project.project.projectId;
  const configuration: import("./api").RequestConfiguration = {
    name: "Public browser request",
    method: "POST",
    url: "https://example.com/public",
    queryParameters: [
      { name: "page", value: "1", enabled: true, sensitive: false },
    ],
    headers: [
      {
        name: "Accept",
        value: "application/json",
        enabled: true,
        sensitive: false,
      },
      { name: "Accept", value: "text/plain", enabled: false, sensitive: false },
    ],
    body: { type: "json", text: ' { "public" : true } ', sensitive: false },
    operationRef: null,
  };
  const requests: import("./api").RequestDetail[] = [];
  for (let n = 0; n < 26; n++) {
    requests.push(
      await api.createRequest(projectId, {
        ...configuration,
        name: `Public browser request ${n + 1}`,
      }),
    );
    await pause(200);
  }
  const probe = requests[0];
  const detail = await api.requestDetail(projectId, probe.request.requestId);
  if (
    detail.request.headers.length !== 2 ||
    detail.request.body?.text !== configuration.body!.text
  )
    throw new Error("Ordered fields/body text changed.");
  const changed = await api.patchRequest(
    projectId,
    detail.request.requestId,
    { name: "Saved-request isolation probe", headers: [], body: null },
    detail.etag,
  );
  if (
    changed.request.headers.length ||
    changed.request.body !== null ||
    changed.request.method !== "POST" ||
    changed.request.queryParameters.length !== 1
  )
    throw new Error("PATCH replacement/clearing failed.");
  const stalePatch = await expectDenied(
    () =>
      api.patchRequest(
        projectId,
        detail.request.requestId,
        { name: "Stale" },
        detail.etag,
      ),
    412,
  );
  const staleDelete = await expectDenied(
    () => api.deleteRequest(projectId, detail.request.requestId, detail.etag),
    412,
  );
  const credentialRejected = await expectDenied(
    () =>
      api.patchRequest(
        projectId,
        changed.request.requestId,
        {
          headers: [
            {
              name: "Authorization",
              value: "fixture-only",
              enabled: false,
              sensitive: false,
            },
          ],
        },
        changed.etag,
      ),
    400,
  );
  // Exercise the actual Gateway/Lambda limit boundary, not only Local validation.
  const boundary = {
    ...configuration,
    name: "Complete configuration limit",
    headers: [],
    queryParameters: [],
    body: { type: "text" as const, text: "", sensitive: false as const },
  };
  boundary.body.text = "x".repeat(
    65536 - new TextEncoder().encode(JSON.stringify(boundary)).length,
  );
  const large = await api.createRequest(projectId, boundary);
  const oversizedRejected = await expectDenied(
    () =>
      api.patchRequest(
        projectId,
        large.request.requestId,
        {
          headers: [
            {
              name: "Accept",
              value: "application/json",
              enabled: true,
              sensitive: false,
            },
          ],
        },
        large.etag,
      ),
    413,
  );
  const verified = await api.requestDetail(projectId, large.request.requestId);
  if (verified.request.revision !== 0)
    throw new Error("Rejected oversize changed revision.");
  await api.deleteRequest(projectId, large.request.requestId, large.etag);
  await api.deleteRequest(
    projectId,
    requests[1].request.requestId,
    requests[1].etag,
  );
  const individuallyDeleted = await expectDenied(
    () => api.requestDetail(projectId, requests[1].request.requestId),
    404,
  );
  requests.push(
    await api.createRequest(projectId, {
      ...configuration,
      name: "Pagination continuation",
    }),
  );
  let pagination = false;
  for (let attempt = 0; attempt < 30; attempt++) {
    const first = await api.listRequests(projectId);
    if (first.nextCursor) {
      const second = await api.listRequests(projectId, first.nextCursor);
      const ids = new Set(
        [...first.items, ...second.items].map((r) => r.requestId),
      );
      if (
        requests
          .filter((_, n) => n !== 1)
          .every((r) => ids.has(r.request.requestId))
      ) {
        pagination = true;
        break;
      }
    }
    await pause(1000);
  }
  if (!pagination)
    throw new Error(
      "Request pagination did not converge; validation project remains for inspection.",
    );
  const gate = await api.detail(projectId);
  const staleProjectDelete = await expectDenied(
    () => api.delete(projectId, project.etag),
    412,
  );
  return {
    currentUserId: user.userId,
    projectId,
    requestId: changed.request.requestId,
    requestRevision: changed.request.revision,
    projectVersion: gate.project.version,
    publicCreateDetailPatch: "passed",
    patchClearingAndHeaderReplacement: "passed",
    stalePatch,
    staleDelete,
    credentialRejected,
    complete64KiBCreate: "passed",
    oversizedRejected,
    individuallyDeleted,
    pagination: "passed",
    staleProjectDelete,
    cleanup: "pending owner review/isolation",
  };
}
export async function validateForeignSavedRequest(
  projectId: string,
  requestId: string,
  ownerId: string,
) {
  const user = await api.me();
  if (user.userId === ownerId)
    throw new Error("Sign in as a different user first.");
  const publicConfig: import("./api").RequestConfiguration = {
    name: "Denied",
    method: "GET",
    url: "https://example.com",
    headers: [],
    queryParameters: [],
    body: null,
    operationRef: null,
  };
  return {
    ownerDifferent: true,
    list: await expectDenied(() => api.listRequests(projectId), 404),
    create: await expectDenied(
      () => api.createRequest(projectId, publicConfig),
      404,
    ),
    detail: await expectDenied(
      () => api.requestDetail(projectId, requestId),
      404,
    ),
    patch: await expectDenied(
      () => api.patchRequest(projectId, requestId, { name: "Denied" }, '"1"'),
      404,
    ),
    delete: await expectDenied(
      () => api.deleteRequest(projectId, requestId, '"1"'),
      404,
    ),
  };
}
export async function cleanupSavedRequestValidation(
  projectId: string,
  requestId: string,
  ownerId: string,
) {
  const user = await api.me();
  if (user.userId !== ownerId)
    throw new Error("Sign in as original owner first.");
  const request = await api.requestDetail(projectId, requestId);
  if (
    request.request.name !== "Saved-request isolation probe" ||
    request.request.revision !== 1 ||
    request.request.body !== null ||
    request.request.headers.length
  )
    throw new Error("Owner probe changed unexpectedly.");
  const project = await api.detail(projectId);
  const operation = await api.delete(projectId, project.etag);
  const hidden = await expectDenied(
    () => api.requestDetail(projectId, requestId),
    404,
  );
  // >20 known records need multiple scheduled ticks. No operator invocation.
  for (let n = 0; n < 90; n++) {
    const current = await api.operation(operation);
    if (current.state === "completed")
      return {
        ownerProbeUnchanged: true,
        hiddenAfterAcceptance: hidden,
        scheduledRequestCleanup: current.state,
        projectId,
      };
    await pause(2000);
  }
  throw new Error(
    "Scheduled request cleanup remains pending after three minutes.",
  );
}
