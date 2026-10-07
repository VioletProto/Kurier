import { expect, test } from "@playwright/test";

test("browser session and local DynamoDB projects with verified fixture JWT", async ({
  page,
  request,
}) => {
  const token = process.env.KURIER_BROWSER_FIXTURE_TOKEN;
  const idToken = process.env.KURIER_BROWSER_FIXTURE_ID_TOKEN;
  const backend = process.env.VITE_API_URL;
  if (!token || !idToken || !backend)
    throw new Error(
      "Run the Go integration,browser harness; fixture tokens are generated only in memory.",
    );
  // Only Cognito is simulated. All project calls hit the real Go verifier and
  // DynamoDB Local through exact-origin CORS. Never record credential traces.
  await page.route(
    "https://cognito-idp.us-east-2.amazonaws.com/",
    async (route) => {
      await route.fulfill({
        contentType: "application/x-amz-json-1.1",
        body: JSON.stringify({
          AuthenticationResult: {
            AccessToken: token,
            IdToken: idToken,
            RefreshToken: "test-only-refresh",
            ExpiresIn: 900,
            TokenType: "Bearer",
          },
        }),
      });
    },
  );
  await page.goto("/");
  await page.getByLabel("Email", { exact: true }).fill("fixture@example.com");
  await page
    .getByLabel("Password", { exact: true })
    .fill("Test-only-password1!");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Your projects" }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Load more projects" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Load more projects" }).click();
  await expect(
    page.getByRole("button", { name: "Seed 00", exact: true }),
  ).toBeVisible();
  expect(
    await page.evaluate(() => ({
      local: localStorage.length,
      session: sessionStorage.length,
    })),
  ).toEqual({ local: 0, session: 0 });
  await page.getByLabel("New project name").fill("Browser project");
  await page
    .getByRole("button", { name: "Create project", exact: true })
    .click();
  const details = page.getByRole("region", { name: "Project details" });
  await expect(
    details.getByRole("heading", { name: "Browser project" }),
  ).toBeVisible();
  const id = (await details.locator("dd").first().textContent())!;
  await page
    .getByLabel("Project name", { exact: true })
    .fill("Browser renamed");
  await page.getByRole("button", { name: "Rename project" }).click();
  await expect(
    details.getByRole("heading", { name: "Browser renamed" }),
  ).toBeVisible();
  // A concurrent revision changes the server gate while the browser holds v1.
  const changed = await request.patch(backend + "/api/v1/projects/" + id, {
    headers: { Authorization: "Bearer " + token, "If-Match": '"1"' },
    data: { name: "Concurrent rename" },
  });
  expect(changed.status()).toBe(200);
  await page
    .getByLabel("Project name", { exact: true })
    .fill("Stale browser draft");
  await page.getByRole("button", { name: "Rename project" }).click();
  await expect(
    page.getByRole("button", { name: "Rename project" }),
  ).toBeDisabled();
  await page.getByRole("button", { name: "Refresh details" }).click();
  await expect(
    details.getByRole("heading", { name: "Concurrent rename" }),
  ).toBeVisible();
  await page
    .getByRole("button", { name: "Delete project", exact: true })
    .click();
  await page.getByRole("button", { name: "Confirm deletion" }).click();
  await expect(
    page.getByRole("region", { name: "Deletion status" }),
  ).toContainText("accepted");
  const cleanup = await request.post(backend + "/test-only/cleanup", {
    data: { projectId: id },
  });
  expect(cleanup.status()).toBe(200);
  await expect(
    page.getByRole("region", { name: "Deletion status" }),
  ).toContainText("completed");
  // Commit a POST but lose its acknowledgement; the UI must not replay it.
  let posts = 0;
  await page.route(backend + "/api/v1/projects", async (route) => {
    if (route.request().method() !== "POST") return route.continue();
    posts++;
    await route.fetch();
    await route.abort("failed");
  });
  await page.getByLabel("New project name").fill("Lost acknowledgement");
  await page
    .getByRole("button", { name: "Create project", exact: true })
    .click();
  await expect(
    page.getByRole("button", { name: "Create project", exact: true }),
  ).toBeDisabled();
  await expect(
    page.getByRole("alert").filter({ hasText: "uncertain" }),
  ).toBeVisible();
  expect(posts).toBe(1);
  await page.getByRole("button", { name: "Refresh projects" }).click();
  await expect(
    page.getByRole("button", { name: "Lost acknowledgement", exact: true }),
  ).toBeVisible();
  await page
    .getByRole("button", { name: "Lost acknowledgement", exact: true })
    .click();
  await expect(
    details.getByRole("heading", { name: "Lost acknowledgement" }),
  ).toBeVisible();
  const lostId = (await details.locator("dd").first().textContent())!;
  let deletes = 0;
  await page.route(backend + "/api/v1/projects/" + lostId, async (route) => {
    if (route.request().method() !== "DELETE") return route.continue();
    deletes++;
    if (deletes === 1) {
      await route.fetch();
      await route.abort("failed");
    } else await route.continue();
  });
  await page
    .getByRole("button", { name: "Delete project", exact: true })
    .click();
  await page.getByRole("button", { name: "Confirm deletion" }).click();
  await expect(
    page.getByRole("button", { name: "Check deletion outcome" }),
  ).toBeVisible();
  expect(deletes).toBe(1);
  await page.getByRole("button", { name: "Check deletion outcome" }).click();
  await expect(
    page.getByRole("region", { name: "Uncertain deletion" }),
  ).toHaveCount(0);
  expect(deletes).toBe(2);
  const lostCleanup = await request.post(backend + "/test-only/cleanup", {
    data: { projectId: lostId },
  });
  expect(lostCleanup.status()).toBe(200);
  await expect(
    page.getByRole("region", { name: "Deletion status" }).last(),
  ).toContainText("completed");
  await page.getByRole("button", { name: "Dark theme" }).click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await page.getByRole("button", { name: "Sign out", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Sign in", exact: true }),
  ).toBeVisible();
  await page.getByLabel("Email", { exact: true }).fill("fixture@example.com");
  await page
    .getByLabel("Password", { exact: true })
    .fill("Test-only-password1!");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Your projects" }),
  ).toBeVisible();
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "Sign in", exact: true }),
  ).toBeVisible();
  expect(
    await page.evaluate(() => ({
      local: localStorage.length,
      session: sessionStorage.length,
    })),
  ).toEqual({ local: 0, session: 0 });
});
