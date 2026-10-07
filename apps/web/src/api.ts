import { auth, SessionExpired } from "./auth";
export interface RequestField {
  name: string;
  value: string;
  enabled: boolean;
  sensitive: false;
}
export interface RequestBody {
  type: "text" | "json";
  text: string;
  sensitive: false;
}
export interface RequestConfiguration {
  name: string;
  method: "GET" | "POST" | "PUT" | "PATCH" | "DELETE";
  url: string;
  queryParameters: RequestField[];
  headers: RequestField[];
  body: RequestBody | null;
  operationRef: null;
}
export interface SavedRequest extends RequestConfiguration {
  requestId: string;
  projectId: string;
  revision: number;
  createdAt: string;
  updatedAt: string;
}
export interface RequestDetail {
  request: SavedRequest;
  etag: string;
}
export interface RequestPage {
  items: SavedRequest[];
  nextCursor: string | null;
}
export interface User {
  userId: string;
  displayName: string;
  createdAt: string;
}
export interface Project {
  projectId: string;
  name: string;
  version: number;
  createdAt: string;
  updatedAt: string;
}
export interface ProjectDetail {
  project: Project;
  etag: string;
}
export interface Page {
  items: Project[];
  nextCursor: string | null;
}
export interface Deletion {
  projectId: string;
  operationId: string;
  state: string;
  startedAt: string;
  completedAt: string | null;
  retryAfterSeconds: number;
}
export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
  ) {
    super(
      status === 412
        ? "This resource changed. Refresh its details before submitting again."
        : status === 404
          ? "This resource is no longer available."
          : status === 403
            ? "Your account cannot access this resource."
            : status === 409
              ? "Another change is in progress. Refresh before submitting again."
              : status === 429
                ? "Too many requests. Wait before trying again."
                : status === 413
                  ? "Complete saved configuration is limited to 64 KiB, including all fields and JSON overhead."
                  : status === 400 && code === "validation_failed"
                    ? "Check the request fields. Use public HTTP(S), public headers/query and valid text/JSON. Credentials and sensitive data cannot be saved."
                    : "The API could not complete this request.",
    );
  }
}
export class UncertainWrite extends Error {
  constructor() {
    super(
      "The result is uncertain. Refresh the project list or details before submitting another change.",
    );
  }
}
export function validateApiUrl(base: string): string {
  const url = new URL(base);
  const local =
    url.protocol === "http:" &&
    ["localhost", "127.0.0.1", "[::1]"].includes(url.hostname) &&
    !!url.port;
  const cloud =
    url.protocol === "https:" &&
    !url.port &&
    /^[a-z0-9]+\.execute-api\.us-east-2\.amazonaws\.com$/.test(url.hostname);
  if (
    (!local && !cloud) ||
    url.username ||
    url.password ||
    url.pathname !== "/" ||
    url.search ||
    url.hash
  )
    throw new Error(
      "The API URL must be a local HTTP origin with port or the development HTTPS API Gateway origin.",
    );
  return url.origin;
}
const base = validateApiUrl(
  import.meta.env.VITE_API_URL ?? "http://127.0.0.1:8080",
);

export class Api {
  constructor(
    private token = auth.token,
    private transport: typeof fetch = (...args) => fetch(...args),
  ) {}
  private async request(
    path: string,
    method = "GET",
    body?: string,
    etag?: string,
    signal?: AbortSignal,
    refreshed = false,
  ): Promise<Response> {
    const token = await this.token(refreshed);
    let response: Response;
    try {
      response = await this.transport(base + path, {
        method,
        body,
        signal: signal
          ? AbortSignal.any([signal, AbortSignal.timeout(10000)])
          : AbortSignal.timeout(10000),
        credentials: "omit",
        cache: "no-store",
        redirect: "error",
        headers: {
          Authorization: `Bearer ${token}`,
          ...(body === undefined ? {} : { "Content-Type": "application/json" }),
          ...(etag ? { "If-Match": etag } : {}),
        },
      });
    } catch {
      if (method !== "GET") throw new UncertainWrite();
      throw new Error(
        "Cannot reach the API. Check your connection and API configuration.",
      );
    }
    if (response.status === 401) {
      // Retry only reads. A creation is never resubmitted, including on 401.
      if (method === "GET" && !refreshed)
        return this.request(path, method, body, etag, signal, true);
      throw new SessionExpired();
    }
    if (!response.ok) {
      if (method !== "GET" && response.status >= 500)
        throw new UncertainWrite();
      const payload = await response.json().catch(() => ({}));
      throw new ApiError(
        response.status,
        payload.error?.code ?? "invalid_response",
      );
    }
    return response;
  }
  private async detailResponse(response: Response): Promise<ProjectDetail> {
    try {
      const data = (await response.json()).data;
      const etag = response.headers.get("ETag");
      if (
        !etag ||
        !/^"(0|[1-9][0-9]*)"$/.test(etag) ||
        etag !== `"${data.project.version}"`
      )
        throw new Error();
      return { project: data.project, etag };
    } catch {
      throw new Error(
        "The API returned an incomplete project response. Refresh before submitting again.",
      );
    }
  }
  async me(signal?: AbortSignal): Promise<User> {
    return (
      await (
        await this.request(
          "/api/v1/users/me",
          "GET",
          undefined,
          undefined,
          signal,
        )
      ).json()
    ).data.user;
  }
  async list(cursor?: string, signal?: AbortSignal): Promise<Page> {
    return (
      await (
        await this.request(
          "/api/v1/projects?limit=25" +
            (cursor ? "&cursor=" + encodeURIComponent(cursor) : ""),
          "GET",
          undefined,
          undefined,
          signal,
        )
      ).json()
    ).data;
  }
  async detail(id: string, signal?: AbortSignal) {
    return this.detailResponse(
      await this.request(
        "/api/v1/projects/" + encodeURIComponent(id),
        "GET",
        undefined,
        undefined,
        signal,
      ),
    );
  }
  async create(name: string) {
    const response = await this.request(
      "/api/v1/projects",
      "POST",
      JSON.stringify({ name }),
    );
    try {
      return await this.detailResponse(response);
    } catch {
      throw new UncertainWrite();
    }
  }
  async rename(id: string, name: string, etag: string) {
    const response = await this.request(
      "/api/v1/projects/" + encodeURIComponent(id),
      "PATCH",
      JSON.stringify({ name }),
      etag,
    );
    try {
      return await this.detailResponse(response);
    } catch {
      throw new UncertainWrite();
    }
  }
  async delete(id: string, etag: string): Promise<Deletion> {
    const response = await this.request(
      "/api/v1/projects/" + encodeURIComponent(id),
      "DELETE",
      undefined,
      etag,
    );
    try {
      return (await response.json()).data.deletionOperation;
    } catch {
      throw new UncertainWrite();
    }
  }
  private requestPath(projectId: string, requestId?: string) {
    return (
      "/api/v1/projects/" +
      encodeURIComponent(projectId) +
      "/requests" +
      (requestId ? "/" + encodeURIComponent(requestId) : "")
    );
  }
  private async savedResponse(response: Response): Promise<RequestDetail> {
    const saved = (await response.json()).data?.request;
    const etag = response.headers.get("ETag");
    if (
      !saved ||
      typeof saved.requestId !== "string" ||
      typeof saved.projectId !== "string" ||
      typeof saved.name !== "string" ||
      !["GET", "POST", "PUT", "PATCH", "DELETE"].includes(saved.method) ||
      typeof saved.url !== "string" ||
      typeof saved.createdAt !== "string" ||
      typeof saved.updatedAt !== "string" ||
      !Array.isArray(saved.headers) ||
      !Array.isArray(saved.queryParameters) ||
      ![...saved.headers, ...saved.queryParameters].every(
        (field: RequestField) =>
          field &&
          typeof field.name === "string" &&
          typeof field.value === "string" &&
          typeof field.enabled === "boolean" &&
          field.sensitive === false,
      ) ||
      saved.operationRef !== null ||
      (saved.body !== null &&
        (!saved.body ||
          !["text", "json"].includes(saved.body.type) ||
          typeof saved.body.text !== "string" ||
          saved.body.sensitive !== false)) ||
      !Number.isSafeInteger(saved.revision) ||
      saved.revision < 0 ||
      etag !== `"${saved.revision}"`
    )
      throw new Error(
        "Incomplete request response. Refresh before submitting again.",
      );
    return { request: saved, etag };
  }
  async listRequests(
    projectId: string,
    cursor?: string,
    signal?: AbortSignal,
  ): Promise<RequestPage> {
    return (
      await (
        await this.request(
          this.requestPath(projectId) +
            "?limit=25" +
            (cursor ? "&cursor=" + encodeURIComponent(cursor) : ""),
          "GET",
          undefined,
          undefined,
          signal,
        )
      ).json()
    ).data;
  }
  async requestDetail(
    projectId: string,
    requestId: string,
    signal?: AbortSignal,
  ) {
    return this.savedResponse(
      await this.request(
        this.requestPath(projectId, requestId),
        "GET",
        undefined,
        undefined,
        signal,
      ),
    );
  }
  async createRequest(projectId: string, configuration: RequestConfiguration) {
    const response = await this.request(
      this.requestPath(projectId),
      "POST",
      JSON.stringify(configuration),
    );
    try {
      return await this.savedResponse(response);
    } catch {
      throw new UncertainWrite();
    }
  }
  async patchRequest(
    projectId: string,
    requestId: string,
    configuration: Partial<RequestConfiguration>,
    etag: string,
  ) {
    const response = await this.request(
      this.requestPath(projectId, requestId),
      "PATCH",
      JSON.stringify(configuration),
      etag,
    );
    try {
      return await this.savedResponse(response);
    } catch {
      throw new UncertainWrite();
    }
  }
  async deleteRequest(projectId: string, requestId: string, etag: string) {
    const response = await this.request(
      this.requestPath(projectId, requestId),
      "DELETE",
      undefined,
      etag,
    );
    if (response.status !== 204) throw new UncertainWrite();
  }
  async operation(op: Deletion, signal?: AbortSignal): Promise<Deletion> {
    return (
      await (
        await this.request(
          "/api/v1/projects/" +
            encodeURIComponent(op.projectId) +
            "/deletion-operations/" +
            encodeURIComponent(op.operationId),
          "GET",
          undefined,
          undefined,
          signal,
        )
      ).json()
    ).data.deletionOperation;
  }
}
export const api = new Api();
