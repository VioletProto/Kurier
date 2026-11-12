import { expect, test, type BrowserContext } from "@playwright/test";

test("restores a first-party HttpOnly session across reload, tabs and reopening, then signs out", async ({
  browser,
}) => {
  let active = true;
  let productRequests = 0;
  const expiry = new Date(Date.now() + 3600000).toISOString();
  const metadata = {
    sessionBinding: "fixture-binding",
    csrfToken: "fixture-csrf",
    sessionExpiresAt: expiry,
  };
  async function routes(context: BrowserContext) {
    await context.route("**/api/v1/browser-session**", async (route) => {
      const request = route.request();
      const headers = request.headers();
      expect(headers["x-kurier-session"]).toBe("1");
      const validCookie = headers.cookie?.includes(
        "__Host-kurier-session=fixture-cookie",
      );
      if (!active || !validCookie) {
        await route.fulfill({
          status: 401,
          json: { error: { code: "unauthenticated" } },
          headers: { "Cache-Control": "no-store" },
        });
        return;
      }
      if (request.method() !== "GET") {
        expect(headers["x-kurier-csrf"]).toBe("fixture-csrf");
        expect(headers["x-kurier-session-binding"]).toBe("fixture-binding");
      }
      if (request.method() === "DELETE") {
        active = false;
        await route.fulfill({
          json: { data: { signedOut: true, providerRevocation: "confirmed" } },
          headers: {
            "Set-Cookie":
              "__Host-kurier-session=; Secure; HttpOnly; SameSite=Strict; Path=/; Max-Age=0",
            "Cache-Control": "no-store",
          },
        });
      } else {
        await route.fulfill({
          json: {
            data:
              request.method() === "GET"
                ? metadata
                : {
                    ...metadata,
                    accessToken: "fixture-access",
                    accessExpiresAt: expiry,
                  },
          },
          headers: { "Cache-Control": "no-store" },
        });
      }
    });
    await context.route(
      "https://fixture.execute-api.us-east-2.amazonaws.com/**",
      async (route) => {
        const request = route.request();
        const cors = {
          "Access-Control-Allow-Origin": "https://localhost:5176",
          "Access-Control-Allow-Headers": "Authorization,Content-Type",
          "Access-Control-Allow-Methods": "GET,POST,DELETE",
        };
        if (request.method() === "OPTIONS") {
          await route.fulfill({ status: 204, headers: cors });
          return;
        }
        expect(request.headers().authorization).toBe("Bearer fixture-access");
        expect(request.headers().cookie).toBeUndefined();
        productRequests++;
        await route.fulfill({
          json: {
            data: request.url().includes("/users/me")
              ? {
                  user: {
                    userId: "fixture-user",
                    displayName: "",
                    createdAt: "2026-11-11T00:00:00Z",
                  },
                }
              : { items: [], nextCursor: null },
          },
          headers: cors,
        });
      },
    );
  }
  const context = await browser.newContext({ ignoreHTTPSErrors: true });
  await routes(context);
  await context.addCookies([
    {
      name: "__Host-kurier-session",
      value: "fixture-cookie",
      domain: "localhost",
      path: "/",
      secure: true,
      httpOnly: true,
      sameSite: "Strict",
      expires: Date.now() / 1000 + 3600,
    },
  ]);
  const page = await context.newPage();
  await page.goto("https://localhost:5176");
  await expect(
    page.getByRole("heading", { name: "Your projects" }),
  ).toBeVisible();
  expect(await page.evaluate(() => document.cookie)).toBe("");
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "Your projects" }),
  ).toBeVisible();
  const sibling = await context.newPage();
  await sibling.goto("https://localhost:5176");
  await expect(
    sibling.getByRole("heading", { name: "Your projects" }),
  ).toBeVisible();
  expect(
    await sibling.evaluate(() => ({
      local: localStorage.length,
      session: sessionStorage.length,
    })),
  ).toEqual({ local: 0, session: 0 });
  const state = await context.storageState();
  await context.close();
  const reopened = await browser.newContext({
    ignoreHTTPSErrors: true,
    storageState: state,
  });
  await routes(reopened);
  const restored = await reopened.newPage();
  await restored.goto("https://localhost:5176");
  await expect(
    restored.getByRole("heading", { name: "Your projects" }),
  ).toBeVisible();
  expect(productRequests).toBeGreaterThan(0);
  await restored.getByRole("button", { name: "Sign out", exact: true }).click();
  await expect(
    restored.getByRole("heading", { name: "Sign in", exact: true }),
  ).toBeVisible();
  await restored.reload();
  await expect(
    restored.getByRole("heading", { name: "Sign in", exact: true }),
  ).toBeVisible();
  expect(
    (await reopened.cookies()).some(
      (cookie) => cookie.name === "__Host-kurier-session",
    ),
  ).toBe(false);
  await reopened.close();
});
