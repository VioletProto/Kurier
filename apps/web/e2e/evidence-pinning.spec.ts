import { expect, test } from "@playwright/test";

test("owner pin/unpin, immutable reopen, expired confirmation and deletion through Go and DynamoDB Local", async ({
  page,
  request,
}) => {
  const backend = process.env.VITE_API_URL!;
  const token = process.env.KURIER_BROWSER_FIXTURE_TOKEN!;
  const execution = process.env.KURIER_BROWSER_PIN_EXECUTION!;
  const project = process.env.KURIER_BROWSER_PIN_PROJECT!;
  if (!backend || !token || !execution || !project)
    throw Error("Run the Go integration,browser harness.");
  const path = `${backend}/api/v1/projects/${project}/executions/${execution}`;
  const headers = { Authorization: `Bearer ${token}` };
  await page.route("https://cognito-idp.us-east-2.amazonaws.com/", (route) =>
    route.fulfill({
      contentType: "application/x-amz-json-1.1",
      body: JSON.stringify({
        AuthenticationResult: {
          AccessToken: token,
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
  await expect(
    page.getByRole("heading", { name: "Your projects" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Load more projects" }).click();
  await page
    .getByRole("button", { name: "Pinned browser evidence", exact: true })
    .click();
  const panel = page.getByRole("region", { name: "Executions" });
  await expect(
    panel.getByRole("button", { name: `Pin evidence ${execution}` }),
  ).toBeEnabled();
  const before = await request.get(path + "/evidence", { headers });
  expect(before.status()).toBe(200);
  const immutable = await before.text();
  await panel
    .getByRole("button", { name: `Pin evidence ${execution}` })
    .click();
  await expect(
    panel.getByRole("button", { name: `Unpin evidence ${execution}` }),
  ).toBeEnabled();
  await expect(
    panel.getByText(/Protected from automatic expiry while pinned/),
  ).toBeVisible();
  const first = await request.put(path + "/pin", { headers });
  expect(first.status()).toBe(200);
  const retention = (await first.json()).data.retention;
  const repeat = await request.put(path + "/pin", { headers });
  expect((await repeat.json()).data.retention).toEqual(retention);
  const after = await request.get(path + "/evidence", { headers });
  expect(await after.text()).toBe(immutable);
  for (const method of ["PUT", "DELETE"]) {
    const denied = await request.fetch(path + "/pin", {
      method,
      headers: {
        Authorization: `Bearer ${process.env.KURIER_BROWSER_BOB_TOKEN}`,
      },
    });
    expect(denied.status()).toBe(404);
  }
  await request.post(backend + "/test-only/pin-expire");
  await panel
    .getByRole("button", { name: "Refresh execution history" })
    .click();
  await panel
    .getByRole("button", { name: `Unpin evidence ${execution}` })
    .click();
  await expect(
    panel.getByRole("dialog", { name: "Confirm expired evidence unpin" }),
  ).toBeVisible();
  await panel
    .getByRole("button", { name: "Confirm unpin expired evidence" })
    .click();
  await expect(
    panel.getByText(/Automatic expiry applies immediately/),
  ).toBeVisible();
  expect((await request.get(path + "/evidence", { headers })).status()).toBe(
    404,
  );
  const repeatedUnpin = await request.delete(path + "/pin", { headers });
  expect(repeatedUnpin.status()).toBe(200);
  expect((await repeatedUnpin.json()).data.retention.effectiveStatus).toBe(
    "expired",
  );
  const expiredPin = await request.put(path + "/pin", { headers });
  expect(expiredPin.status()).toBe(409);
  const current = await request.get(`${backend}/api/v1/projects/${project}`, {
    headers,
  });
  expect(
    (
      await request.delete(`${backend}/api/v1/projects/${project}`, {
        headers: { ...headers, "If-Match": current.headers().etag },
      })
    ).status(),
  ).toBe(202);
  for (let n = 0; n < 20; n++) {
    const r = await request.post(backend + "/test-only/cleanup", {
      data: { projectId: project },
    });
    if (r.status() === 200) break;
    if (n === 19) throw Error("Fixture deletion did not settle");
  }
  expect((await request.delete(path + "/pin", { headers })).status()).toBe(404);
  expect(
    await page.evaluate(() => ({
      local: localStorage.length,
      session: sessionStorage.length,
    })),
  ).toEqual({ local: 0, session: 0 });
});
