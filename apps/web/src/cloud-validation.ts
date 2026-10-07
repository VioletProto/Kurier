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
