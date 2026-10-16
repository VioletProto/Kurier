import { expect, test } from "@playwright/test";
// Real browser -> Go -> DynamoDB Local. No traces/screenshots/video or token logs.
test("read-only MCP browser consent, one-time credential, scope denial, revoke and cleanup", async ({
  page,
  request,
}) => {
  const backend = process.env.VITE_API_URL!;
  const bearer = process.env.KURIER_BROWSER_FIXTURE_TOKEN!;
  if (!backend || !bearer)
    throw Error("Run the Go integration,browser harness");
  await request.post(backend + "/test-only/local-agent-reset");
  const headers = { Authorization: `Bearer ${bearer}` };
  const created = await request.post(backend + "/api/v1/projects", {
    headers,
    data: { name: "MCP browser fixture" },
  });
  expect(created.status()).toBe(201);
  const project = (await created.json()).data.project.projectId;
  await page.route("https://cognito-idp.us-east-2.amazonaws.com/", (route) =>
    route.fulfill({
      contentType: "application/x-amz-json-1.1",
      body: JSON.stringify({
        AuthenticationResult: {
          AccessToken: bearer,
          IdToken: process.env.KURIER_BROWSER_FIXTURE_ID_TOKEN,
          RefreshToken: "fixture",
          ExpiresIn: 900,
          TokenType: "Bearer",
        },
      }),
    }),
  );
  await page.goto("/");
  await page.getByLabel("Email", { exact: true }).fill("fixture@example.com");
  await page
    .getByLabel("Password", { exact: true })
    .fill("Test-only-password1!");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await page
    .getByRole("button", { name: "MCP browser fixture", exact: true })
    .click();
  const panel = page.getByRole("region", { name: "Read-only MCP access" });
  await expect(
    panel.getByRole("button", { name: "Create MCP credential" }),
  ).toBeDisabled();
  await panel.getByRole("button", { name: "Refresh MCP status" }).click();
  await panel.getByRole("checkbox").check();
  await panel.getByRole("button", { name: "Create MCP credential" }).click();
  const token = await panel.getByLabel("One-time MCP token").innerText();
  expect(token.length).toBe(43);
  const stored = await page.evaluate(
    (value) =>
      Object.values(localStorage).some((v) => v.includes(value)) ||
      Object.values(sessionStorage).some((v) => v.includes(value)),
    token,
  );
  expect(stored).toBe(false);
  const read = await request.post(backend + "/api/v1/mcp/invocations", {
    headers: { Authorization: `Bearer ${token}` },
    data: { tool: "list_projects", input: {} },
  });
  expect(read.status()).toBe(200);
  const readData = (await read.json()).data;
  expect(readData.items.length).toBe(1);
  expect(readData.items[0].projectId).toBe(project);
  const foreign = await request.post(backend + "/api/v1/mcp/invocations", {
    headers: { Authorization: `Bearer ${token}` },
    data: {
      tool: "get_execution",
      input: {
        projectId: crypto.randomUUID(),
        executionId: crypto.randomUUID(),
      },
    },
  });
  expect(foreign.status()).toBe(404);
  await panel.getByRole("button", { name: "Close MCP token display" }).click();
  await expect(panel.getByLabel("One-time MCP token")).toHaveCount(0);
  await panel.getByRole("button", { name: "Revoke MCP access" }).click();
  await expect(panel.getByText(/Revoked/)).toBeVisible();
  const denied = await request.post(backend + "/api/v1/mcp/invocations", {
    headers: { Authorization: `Bearer ${token}` },
    data: { tool: "list_projects", input: {} },
  });
  expect(denied.status()).toBe(401);
  const detail = await request.get(`${backend}/api/v1/projects/${project}`, {
    headers,
  });
  const removed = await request.delete(
    `${backend}/api/v1/projects/${project}`,
    { headers: { ...headers, "If-Match": detail.headers().etag } },
  );
  expect(removed.status()).toBe(202);
  for (let n = 0; n < 20; n++) {
    const cleanup = await request.post(backend + "/test-only/cleanup", {
      data: { projectId: project },
    });
    if (cleanup.status() === 200) break;
  }
  const after = await request.get(`${backend}/api/v1/projects/${project}`, {
    headers,
  });
  expect(after.status()).toBe(404);
  await page.reload();
  await expect(
    page.getByRole("button", { name: "Sign in", exact: true }),
  ).toBeVisible();
});
