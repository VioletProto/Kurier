import { expect, test } from "@playwright/test";

// Local UI fixtures only: no Cognito, AWS, credentials or target requests.
for (const width of [375, 1366, 1920]) {
  test(`workspace tabs preserve drafts and fit ${width}px`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 1000 });
    await page.route("**/src/auth.ts", (route) =>
      route.fulfill({
        contentType: "application/javascript",
        body: `export const authConfigured = true;
        export const sessionBrokerEnabled = false;
        export const sessionChangedEvent = 'fixture-session-change';
        export class SessionExpired extends Error {}
        export const authMessage = () => 'Fixture error';
        export const auth = { login: async () => 'done', token: async () => 'fixture', logout: async () => {} };`,
      }),
    );
    const project = {
      projectId: "ui-fixture",
      name: "Workspace fixture",
      version: 0,
      createdAt: "2026-11-13T00:00:00Z",
      updatedAt: "2026-11-13T00:00:00Z",
    };
    const writes: string[] = [];
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
    await page.getByLabel("Email", { exact: true }).fill("fixture@example.com");
    await page.getByLabel("Password", { exact: true }).fill("fixture");
    await page.getByRole("button", { name: "Sign in", exact: true }).click();
    await page.getByRole("button", { name: project.name, exact: true }).click();
    const tabs = page.getByRole("tab");
    await expect(tabs).toHaveCount(7);
    const requests = page.getByRole("tab", { name: "Requests & evidence" });
    await expect(requests).toHaveAttribute("aria-selected", "true");
    await page
      .getByRole("button", { name: "New request", exact: true })
      .click();
    await page
      .getByLabel("Request name", { exact: true })
      .fill("Unsaved request");
    await page
      .getByLabel("Public request URL")
      .fill("https://example.com/" + "x".repeat(400));
    await page.getByLabel("Body type").selectOption("json");
    await page
      .getByLabel("Public request body")
      .fill(JSON.stringify({ text: "x".repeat(500) }));
    await expect(
      page.getByRole("region", { name: "Local agent pairing" }),
    ).toBeHidden();
    await page.getByRole("tab", { name: "Local agent pairing" }).click();
    await page
      .getByLabel("Agent name", { exact: true })
      .fill("Unsaved agent name");
    await requests.click();
    await expect(
      page.getByRole("region", { name: "Local agent pairing" }),
    ).toBeHidden();
    await page.getByRole("tab", { name: "Local agent pairing" }).click();
    await expect(page.getByLabel("Agent name", { exact: true })).toHaveValue(
      "Unsaved agent name",
    );
    await requests.click();
    await requests.focus();
    await page.keyboard.press("ArrowRight");
    const workflows = page.getByRole("tab", { name: "Workflows", exact: true });
    await expect(workflows).toBeFocused();
    await expect(workflows).toHaveAttribute("aria-selected", "true");
    await expect(page.getByLabel("Request name", { exact: true })).toBeHidden();
    await page.keyboard.press("End");
    const settings = page.getByRole("tab", { name: "Project settings" });
    await expect(settings).toBeFocused();
    await page
      .getByLabel("Project name", { exact: true })
      .fill("Unsaved rename");
    await settings.focus();
    await page.keyboard.press("ArrowRight");
    await expect(requests).toBeFocused();
    await expect(page.getByLabel("Request name", { exact: true })).toHaveValue(
      "Unsaved request",
    );
    await page.keyboard.press("ArrowLeft");
    await expect(settings).toBeFocused();
    await expect(page.getByLabel("Project name", { exact: true })).toHaveValue(
      "Unsaved rename",
    );
    await page.keyboard.press("Home");
    await expect(requests).toBeFocused();
    await page.keyboard.press("Tab");
    await expect(page.getByRole("tabpanel")).toBeFocused();
    for (const tab of await tabs.all()) {
      await tab.click();
      await expect(page.getByRole("tabpanel")).toHaveCount(1);
      expect(
        await page.evaluate(
          () => document.documentElement.scrollWidth <= window.innerWidth,
        ),
      ).toBe(true);
    }
    if (width >= 1366) {
      const bounds = await page
        .getByRole("region", { name: "Project details" })
        .boundingBox();
      expect(bounds!.width).toBeGreaterThan(width * 0.65);
    }
    await page.getByRole("button", { name: "Dark theme" }).click();
    await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
    expect(writes).toEqual([]);
    expect(
      await page.evaluate(() => localStorage.length + sessionStorage.length),
    ).toBe(0);
  });
}
