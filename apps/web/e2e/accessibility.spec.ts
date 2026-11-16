import { expect, test, type Page } from "@playwright/test";

// Local fixtures only; no real credentials, AWS or target execution.
async function fixture(page: Page) {
  const writes: string[] = [];
  await page.route("**/*", async (route) => {
    if (new URL(route.request().url()).origin !== "http://localhost:5175") {
      writes.push("blocked external request");
      await route.abort();
    } else await route.fallback();
  });
  await page.route("**/src/auth.ts", (route) =>
    route.fulfill({
      contentType: "application/javascript",
      body: `export const authConfigured = true;
        export const sessionBrokerEnabled = false;
        export const sessionChangedEvent = 'fixture-session-change';
        export class SessionExpired extends Error {}
        export const authMessage = () => 'Fixture error';
        export const auth = { login: async (email) => { if (email === 'fail@example.com') throw Error(); return 'done'; }, token: async () => 'fixture', logout: async () => {} };`,
    }),
  );
  const project = {
    projectId: "ui-fixture",
    name: "Workspace fixture",
    version: 0,
    createdAt: "2026-11-13T00:00:00Z",
    updatedAt: "2026-11-13T00:00:00Z",
  };
  await page.route("**/api/v1/**", async (route) => {
    const path = new URL(route.request().url()).pathname;
    if (route.request().method() !== "GET") writes.push(path);
    const data = path.endsWith("/users/me")
      ? { user: { userId: "fixture", displayName: "UI fixture" } }
      : path.endsWith("/projects/ui-fixture")
        ? { project }
        : {
            items: path.endsWith("/projects") ? [project] : [],
            nextCursor: null,
          };
    await route.fulfill({
      json: { data },
      headers: { ETag: '"0"', "Access-Control-Expose-Headers": "ETag" },
    });
  });
  await page.goto("/");
  await expect(
    page.getByRole("heading", { name: "Sign in", exact: true }),
  ).toBeVisible();
  return writes;
}

test("skip link and account-mode changes provide keyboard focus", async ({
  page,
}) => {
  await fixture(page);
  await page.keyboard.press("Tab");
  await expect(
    page.getByRole("link", { name: "Skip to workspace" }),
  ).toBeFocused();
  await page.keyboard.press("Enter");
  await expect(
    page.getByRole("region", { name: "Workspace", exact: true }),
  ).toBeFocused();
  await page.keyboard.press("Tab");
  await expect(page.getByLabel("Email", { exact: true })).toBeFocused();
  for (const name of [
    "Reset password",
    "Create account",
    "Verify email",
    "Sign in",
  ]) {
    const button = page.getByRole("button", { name, exact: true });
    await button.focus();
    await button.press("Enter");
    await expect(
      page.getByRole("heading", { name, exact: true }),
    ).toBeFocused();
    await page.keyboard.press("Tab");
    await expect(page.getByLabel("Email", { exact: true })).toBeFocused();
  }
});

for (const width of [320, 683, 1366]) {
  test(`workspace keyboard entry, reflow and sign-out at ${width}px`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 900 });
    const writes = await fixture(page);
    await page.getByLabel("Email", { exact: true }).fill("fixture@example.com");
    await page.getByLabel("Password", { exact: true }).fill("fixture");
    await page.getByLabel("Password", { exact: true }).press("Enter");
    const workspace = page.getByRole("region", {
      name: "Workspace",
      exact: true,
    });
    await expect(workspace).toBeFocused();
    const project = page.getByRole("button", {
      name: "Workspace fixture",
      exact: true,
    });
    await project.focus();
    await project.press("Enter");
    const requests = page.getByRole("tab", { name: "Requests & evidence" });
    await expect(requests).toBeFocused();
    for (let i = 0; i < 7; i++) {
      const selected = page.getByRole("tab", { selected: true });
      await expect(selected).toBeFocused();
      await page.keyboard.press("Tab");
      await expect(page.getByRole("tabpanel")).toBeFocused();
      expect(
        await page.evaluate(
          () => document.documentElement.scrollWidth <= innerWidth,
        ),
      ).toBe(true);
      await page.keyboard.press("Shift+Tab");
      await expect(selected).toBeFocused();
      await page.keyboard.press("ArrowRight");
    }
    await expect(requests).toBeFocused();
    const out = page.getByRole("button", { name: "Sign out", exact: true });
    await out.focus();
    await out.press("Enter");
    await expect(workspace).toBeFocused();
    await expect(
      page.getByRole("heading", { name: "Sign in", exact: true }),
    ).toBeVisible();
    await expect(
      page.getByText("Workspace fixture", { exact: true }),
    ).toHaveCount(0);
    await expect(page.getByLabel("Password", { exact: true })).toHaveValue("");
    expect(
      await page.evaluate(() => localStorage.length + sessionStorage.length),
    ).toBe(0);
    expect(writes).toEqual([]);
  });
}

test("loading, unavailable projects and revision conflicts expose a recovery path", async ({
  page,
}) => {
  const writes = await fixture(page);
  let release!: () => void;
  const paused = new Promise<void>((resolve) => {
    release = resolve;
  });
  await page.route(
    (url) => url.pathname === "/api/v1/projects",
    async (route) => {
      await paused;
      await route.fulfill({
        status: 503,
        json: { error: { code: "unavailable" } },
      });
    },
    { times: 1 },
  );
  await page.getByLabel("Email", { exact: true }).fill("fixture@example.com");
  await page.getByLabel("Password", { exact: true }).fill("fixture");
  await page.getByLabel("Password", { exact: true }).press("Enter");
  await expect(
    page.getByRole("status").filter({ hasText: "Loading…" }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Create project", exact: true }),
  ).toBeDisabled();
  release();
  await expect(
    page
      .getByRole("alert")
      .filter({ hasText: "The API could not complete this request." }),
  ).toBeVisible();
  const refresh = page.getByRole("button", {
    name: "Refresh projects",
    exact: true,
  });
  await refresh.focus();
  await refresh.press("Enter");
  await page
    .getByRole("button", { name: "Workspace fixture", exact: true })
    .click();
  await page.getByRole("tab", { name: "Project settings" }).click();
  await page.route("**/api/v1/projects/ui-fixture", async (route) => {
    if (route.request().method() === "PATCH") {
      await route.fulfill({
        status: 412,
        json: { error: { code: "version_conflict" } },
      });
    } else await route.fallback();
  });
  await page.getByLabel("Project name", { exact: true }).fill("Changed name");
  await page.getByLabel("Project name", { exact: true }).press("Enter");
  await expect(
    page.getByRole("status").filter({ hasText: "Refresh details to review" }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Rename project", exact: true }),
  ).toBeDisabled();
  const details = page.getByRole("button", {
    name: "Refresh details",
    exact: true,
  });
  await details.focus();
  await details.press("Enter");
  await expect(
    page.getByRole("button", { name: "Rename project", exact: true }),
  ).toBeEnabled();
  expect(writes).toEqual([]);
});

test("failed sign-in leaves an announced error and editable controls", async ({
  page,
}) => {
  const writes = await fixture(page);
  await page.getByLabel("Email", { exact: true }).fill("fail@example.com");
  await page.getByLabel("Password", { exact: true }).fill("fixture");
  await page.getByLabel("Password", { exact: true }).press("Enter");
  await expect(page.getByRole("alert")).toHaveText("Fixture error");
  await expect(page.getByLabel("Email", { exact: true })).toBeEnabled();
  await expect(page.getByLabel("Password", { exact: true })).toBeEnabled();
  await expect(
    page.getByRole("button", { name: "Sign in", exact: true }),
  ).toBeEnabled();
  expect(writes).toEqual([]);
});
