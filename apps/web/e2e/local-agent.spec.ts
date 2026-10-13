import { expect, test } from "@playwright/test";
// Secret-bearing fixture requests never enable traces/screenshots/video.
test("browser pairing, compiled foreground agent, sanitized evidence, revocation and project cleanup", async ({
  page,
  request,
}) => {
  const backend = process.env.VITE_API_URL!;
  const bearer = process.env.KURIER_BROWSER_FIXTURE_TOKEN!;
  const target = process.env.KURIER_BROWSER_LOCAL_TARGET!;
  if (!backend || !bearer || !target)
    throw Error("Run the Go integration,browser harness.");
  await request.post(backend + "/test-only/local-agent-reset");
  await page.route("https://cognito-idp.us-east-2.amazonaws.com/", (route) =>
    route.fulfill({
      contentType: "application/x-amz-json-1.1",
      body: JSON.stringify({
        AuthenticationResult: {
          AccessToken: bearer,
          IdToken: process.env.KURIER_BROWSER_FIXTURE_ID_TOKEN,
          RefreshToken: "test-only-refresh",
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
  const headers = { Authorization: `Bearer ${bearer}` };
  const created = await request.post(backend + "/api/v1/projects", {
    headers,
    data: { name: "Local agent browser fixture" },
  });
  expect(created.status()).toBe(201);
  const project = (await created.json()).data.project.projectId;
  const saved = await request.post(
    `${backend}/api/v1/projects/${project}/requests`,
    {
      headers,
      data: {
        name: "Local controlled request",
        method: "GET",
        url: target,
        headers: [
          {
            name: "Authorization",
            enabled: true,
            sensitive: true,
            bindingId: crypto.randomUUID(),
            secretWrite: {
              action: "set",
              value: "disposable-browser-local-secret",
            },
          },
        ],
      },
    },
  );
  expect(saved.status()).toBe(201);
  await page.getByRole("button", { name: "Load more projects" }).click();
  await page
    .getByRole("button", { name: "Local agent browser fixture", exact: true })
    .click();
  const panel = page.getByRole("region", { name: "Executions", exact: true });
  const pairing = page.getByRole("region", { name: "Local agent pairing" });
  await pairing
    .getByLabel("Exact HTTP(S) origins, one per line (up to 16)")
    .fill(target);
  await pairing
    .getByLabel(
      "I authorize this project and these exact destinations for the named device.",
    )
    .check();
  await pairing
    .getByRole("button", { name: "Create agent credential" })
    .click();
  const capability = await pairing
    .getByLabel("One-time agent token")
    .textContent();
  if (!capability) throw Error("Pairing token unavailable");
  await pairing.getByRole("button", { name: "Close token display" }).click();
  const req = (await saved.json()).data.request;
  await page
    .getByRole("button", { name: new RegExp("Local controlled request") })
    .click();
  await panel.getByLabel("Execution target").selectOption("local");
  await expect(
    panel.getByRole("button", { name: /Execute saved revision/ }),
  ).toBeDisabled();
  await panel
    .getByLabel(
      "I allow this execution on the selected device and its approved local/private services.",
    )
    .check();
  await panel
    .getByLabel(
      "I explicitly allow protected inputs over unencrypted HTTP for this execution or rerun.",
    )
    .check();
  await panel.getByRole("button", { name: /Execute saved revision/ }).click();
  await expect(
    panel.getByText("Execution durably accepted.", { exact: false }),
  ).toBeVisible();
  const history = await request.get(
    `${backend}/api/v1/projects/${project}/executions`,
    { headers },
  );
  const execution = (await history.json()).data.items.find(
    (item: { source: { requestId: string } }) =>
      item.source.requestId === req.requestId,
  ).executionId;
  const run = await request.post(backend + "/test-only/local-agent-run", {
    data: { Token: capability, ProjectID: project, ExecutionID: execution },
  });
  expect(run.status()).toBe(204);
  await panel.getByRole("button", { name: "Refresh execution status" }).click();
  await panel
    .getByRole("button", { name: "Inspect sanitized evidence" })
    .click();
  await expect(panel.getByText(/\[REDACTED\]/).first()).toBeVisible();
  const evidence = await request.get(
    `${backend}/api/v1/projects/${project}/executions/${execution}/evidence`,
    { headers },
  );
  expect(evidence.status()).toBe(200);
  const text = await evidence.text();
  expect(text.includes("disposable-browser-local-secret")).toBe(false);
  expect(text.includes(capability)).toBe(false);
  expect((await evidence.json()).data.evidence.target).toBe("local");
  expect(
    await page.evaluate(() =>
      JSON.stringify([
        Object.entries(localStorage),
        Object.entries(sessionStorage),
      ]),
    ),
  ).not.toContain("disposable-browser-local-secret");
  expect(
    (
      await page.evaluate(() =>
        JSON.stringify([
          Object.entries(localStorage),
          Object.entries(sessionStorage),
        ]),
      )
    ).includes(capability),
  ).toBe(false);
  await pairing.getByRole("button", { name: "Revoke local agent" }).click();
  await expect(pairing.getByText(/Revoked/)).toBeVisible();
  const denied = await request.get(backend + "/api/v1/local-agent/session", {
    headers: { Authorization: `Bearer ${capability}` },
  });
  expect(denied.status()).toBe(401);
  const detail = await request.get(`${backend}/api/v1/projects/${project}`, {
    headers,
  });
  const deleted = await request.delete(
    `${backend}/api/v1/projects/${project}`,
    { headers: { ...headers, "If-Match": detail.headers().etag } },
  );
  expect(deleted.status()).toBe(202);
  const deletionOperation = (await deleted.json()).data.deletionOperation;
  const inaccessible = await request.get(
    `${backend}/api/v1/projects/${project}/executions/${execution}/evidence`,
    { headers },
  );
  expect(inaccessible.status()).toBe(404);
  for (let i = 0; i < 20; i++) {
    await request.post(backend + "/test-only/cleanup", {
      data: { projectId: project },
    });
  }
  const operation = await request.get(
    `${backend}/api/v1/projects/${project}/deletion-operations/${deletionOperation.operationId}`,
    { headers },
  );
  expect(operation.status()).toBe(200);
  expect((await operation.json()).data.deletionOperation.state).toBe(
    "completed",
  );
});
