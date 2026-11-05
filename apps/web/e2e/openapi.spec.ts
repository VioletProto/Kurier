import { expect, test } from "@playwright/test";

test("OpenAPI preview completes inputs, publishes requests and exports sanitized JSON and YAML", async ({
  page,
  request,
}) => {
  const token = process.env.KURIER_BROWSER_FIXTURE_TOKEN,
    idToken = process.env.KURIER_BROWSER_FIXTURE_ID_TOKEN,
    backend = process.env.VITE_API_URL;
  if (!token || !idToken || !backend)
    throw Error("Run Go integration,browser fixture harness");
  const headers = { Authorization: `Bearer ${token}` };
  await page.route("https://cognito-idp.us-east-2.amazonaws.com/", (route) =>
    route.fulfill({
      contentType: "application/x-amz-json-1.1",
      body: JSON.stringify({
        AuthenticationResult: {
          AccessToken: token,
          IdToken: idToken,
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
  await page.getByLabel("New project name").fill("OpenAPI browser fixture");
  await page
    .getByRole("button", { name: "Create project", exact: true })
    .click();
  const details = page.getByRole("region", { name: "Project details" });
  await expect(
    details.getByRole("heading", {
      name: "OpenAPI browser fixture",
      exact: true,
    }),
  ).toBeVisible();
  const project = (await details.locator("dd").first().textContent())!;
  try {
    const panel = page.getByRole("region", {
      name: "OpenAPI imports",
      exact: true,
    });
    await panel.getByRole("button", { name: "Open OpenAPI imports" }).click();
    const source = `openapi: 3.0.4
info:
  title: browser-synthetic-private-canary
  version: "1"
servers:
  - url: https://example.com
paths:
  /items/{id}:
    get:
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
      responses:
        '200':
          description: browser-synthetic-private-canary
          content:
            application/json:
              schema:
                type: object
                required: [ratio]
                properties:
                  ratio:
                    type: number
                    example: 0.10000000000000001
`;
    await panel.getByLabel("OpenAPI source", { exact: true }).fill(source);
    await panel
      .getByRole("checkbox", { name: /I confirm the remaining/ })
      .check();
    await panel
      .getByRole("button", { name: "Prepare import", exact: true })
      .click();
    await expect(
      panel.getByRole("button", { name: "Publish generated requests" }),
    ).toBeDisabled();
    await expect(
      panel.getByLabel("OpenAPI source", { exact: true }),
    ).toHaveValue("");
    await panel
      .getByLabel("Operation inputs (JSON)")
      .fill('{"parameters":{"p0":"widget"}}');
    await panel
      .getByRole("checkbox", { name: /These operation inputs/ })
      .check();
    await panel.getByRole("button", { name: "Save operation inputs" }).click();
    await expect(
      panel.getByRole("button", { name: "Publish generated requests" }),
    ).toBeEnabled();
    await panel
      .getByRole("button", { name: "Publish generated requests" })
      .click();
    await expect(
      panel.getByRole("heading", { name: /Import: ready/ }),
    ).toBeVisible();
    const list = await request.get(
      `${backend}/api/v1/projects/${project}/requests`,
      { headers },
    );
    expect(list.status()).toBe(200);
    const generated = (await list.json()).data.items;
    expect(generated).toHaveLength(1);
    expect(generated[0].url).toBe("https://example.com/items/widget");
    expect(generated[0].operationRef).toBeTruthy();
    const importId = generated[0].operationRef.importId;
    for (const encoding of ["json", "yaml"] as const) {
      const exported = await request.get(
        `${backend}/api/v1/projects/${project}/openapi-imports/${importId}/export?format=${encoding}`,
        { headers },
      );
      expect(exported.status()).toBe(200);
      const text = await exported.text();
      expect(text).not.toContain("browser-synthetic-private-canary");
      expect(text).toContain("0.10000000000000001");
      const download = page.waitForEvent("download");
      await panel
        .getByRole("button", {
          name: `Export sanitized ${encoding.toUpperCase()}`,
        })
        .click();
      expect((await download).suggestedFilename()).toBe(
        `openapi-sanitized.${encoding}`,
      );
    }
  } finally {
    const current = await request.get(`${backend}/api/v1/projects/${project}`, {
      headers,
    });
    if (current.status() === 200) {
      expect(
        (
          await request.delete(`${backend}/api/v1/projects/${project}`, {
            headers: { ...headers, "If-Match": current.headers().etag },
          })
        ).status(),
      ).toBe(202);
      for (let n = 0; n < 10; n++) {
        const cleanup = await request.post(backend + "/test-only/cleanup", {
          data: { projectId: project },
        });
        if (cleanup.status() === 200) break;
      }
    }
  }
});
