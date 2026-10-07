import { describe, expect, it, vi } from "vitest";
import { Api, ApiError, UncertainWrite } from "./api";
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
