import { describe, expect, it, vi } from "vitest";
import { Api, ApiError, UncertainWrite, validateApiUrl } from "./api";
import { SessionExpired } from "./auth";
const project = {
  projectId: "opaque",
  name: "Demo",
  version: 0,
  createdAt: "2026-10-07T00:00:00Z",
  updatedAt: "2026-10-07T00:00:00Z",
};
const ok = () =>
  new Response(JSON.stringify({ data: { project } }), {
    status: 201,
    headers: { ETag: '"0"', Location: "/api/v1/projects/opaque" },
  });
describe("local API client", () => {
  it("sends an access token and quoted If-Match and reads the exposed ETag", async () => {
    const transport = vi.fn().mockResolvedValue(ok());
    const api = new Api(vi.fn().mockResolvedValue("access-fixture"), transport);
    expect((await api.rename("opaque", "New", '"0"')).etag).toBe('"0"');
    expect(transport.mock.calls[0][1]).toMatchObject({
      method: "PATCH",
      credentials: "omit",
      redirect: "error",
      headers: {
        Authorization: "Bearer access-fixture",
        "If-Match": '"0"',
        "Content-Type": "application/json",
      },
    });
  });
  it.each([
    new TypeError("lost ack"),
    new Response("{}", { status: 503 }),
    new Response("{}", { status: 201 }),
  ])("never retries uncertain creation", async (result) => {
    const transport = vi.fn();
    if (result instanceof Error) transport.mockRejectedValue(result);
    else transport.mockResolvedValue(result);
    await expect(
      new Api(vi.fn().mockResolvedValue("access-fixture"), transport).create(
        "Demo",
      ),
    ).rejects.toBeInstanceOf(UncertainWrite);
    expect(transport).toHaveBeenCalledTimes(1);
  });
  it("does not retry a stale rename", async () => {
    const transport = vi
      .fn()
      .mockResolvedValue(
        new Response(
          JSON.stringify({ error: { code: "precondition_failed" } }),
          { status: 412 },
        ),
      );
    await expect(
      new Api(vi.fn().mockResolvedValue("access-fixture"), transport).rename(
        "opaque",
        "New",
        '"0"',
      ),
    ).rejects.toBeInstanceOf(ApiError);
    expect(transport).toHaveBeenCalledTimes(1);
  });
  it("refreshes once on a rejected read token and keeps users/me", async () => {
    const token = vi.fn().mockResolvedValue("access-fixture");
    const transport = vi
      .fn()
      .mockResolvedValueOnce(new Response("{}", { status: 401 }))
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ data: { user: { userId: "user" } } })),
      );
    await new Api(token, transport).me();
    expect(token.mock.calls).toEqual([[false], [true]]);
    expect(transport.mock.calls[0][0]).toContain("/api/v1/users/me");
  });
  it("never resubmits a creation on 401", async () => {
    const transport = vi
      .fn()
      .mockResolvedValue(new Response("{}", { status: 401 }));
    await expect(
      new Api(vi.fn().mockResolvedValue("access-fixture"), transport).create(
        "Demo",
      ),
    ).rejects.toBeInstanceOf(SessionExpired);
    expect(transport).toHaveBeenCalledTimes(1);
  });
  it("encodes opaque pagination cursors", async () => {
    const transport = vi
      .fn()
      .mockResolvedValue(
        new Response(JSON.stringify({ data: { items: [], nextCursor: null } })),
      );
    expect(
      await new Api(
        vi.fn().mockResolvedValue("access-fixture"),
        transport,
      ).list("a+b/c="),
    ).toEqual({ items: [], nextCursor: null });
    expect(transport.mock.calls[0][0]).toContain("cursor=a%2Bb%2Fc%3D");
  });
});

describe("API origin validation", () => {
  it.each([
    "http://localhost:8080",
    "http://127.0.0.1:8080",
    "https://abc123.execute-api.us-east-2.amazonaws.com/",
  ])("accepts scoped origin %s", (url) => {
    expect(validateApiUrl(url)).toBe(new URL(url).origin);
  });
  it.each([
    "http://api.example.com",
    "https://api.example.com",
    "https://abc.execute-api.us-east-1.amazonaws.com",
    "https://user:pass@abc.execute-api.us-east-2.amazonaws.com",
    "https://abc.execute-api.us-east-2.amazonaws.com/path",
    "https://abc.execute-api.us-east-2.amazonaws.com?token=secret",
    "https://abc.execute-api.us-east-2.amazonaws.com#fragment",
  ])("rejects unsafe URL %s", (url) => {
    expect(() => validateApiUrl(url)).toThrow();
  });
});

describe("saved request client outcomes", () => {
  const configuration = {
    name: "Public",
    method: "GET" as const,
    url: "https://example.com",
    headers: [],
    queryParameters: [],
    body: null,
    operationRef: null,
  };
  it("treats incomplete acknowledged creation as uncertain without retry", async () => {
    const transport = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          data: { request: { requestId: "r", revision: 0 } },
        }),
        { status: 201, headers: { ETag: '"0"' } },
      ),
    );
    await expect(
      new Api(vi.fn().mockResolvedValue("fixture"), transport).createRequest(
        "p",
        configuration,
      ),
    ).rejects.toBeInstanceOf(UncertainWrite);
    expect(transport).toHaveBeenCalledTimes(1);
  });
  it("sends explicit clearing and the request revision, and never replays stale writes", async () => {
    const transport = vi
      .fn()
      .mockResolvedValue(
        new Response(
          JSON.stringify({ error: { code: "precondition_failed" } }),
          { status: 412 },
        ),
      );
    await expect(
      new Api(vi.fn().mockResolvedValue("fixture"), transport).patchRequest(
        "p",
        "r",
        { headers: [], body: null },
        '"7"',
      ),
    ).rejects.toBeInstanceOf(ApiError);
    expect(transport).toHaveBeenCalledTimes(1);
    expect(transport.mock.calls[0][1]).toMatchObject({
      method: "PATCH",
      body: JSON.stringify({ headers: [], body: null }),
      headers: { "If-Match": '"7"' },
    });
  });
});
