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
  const requests = page.getByRole("region", { name: "Saved requests" });
  await expect(
    requests.getByText(/64 KiB applies to the complete saved configuration/),
  ).toBeVisible();
  await requests
    .getByRole("button", { name: "New request", exact: true })
    .click();
  await requests
    .getByLabel("Request name", { exact: true })
    .fill("Public browser request");
  await requests
    .getByLabel("Public request URL")
    .fill("https://example.com/public");
  await requests.getByLabel("Method", { exact: true }).selectOption("POST");
  await requests
    .getByRole("button", { name: "Add header", exact: true })
    .click();
  await requests.getByLabel("Header name 1").fill("Accept");
  await requests.getByLabel("Header value 1").fill("application/json");
  await requests.getByLabel("Body type").selectOption("json");
  await requests.getByLabel("Public request body").fill('{ "public": true }');
  await requests
    .getByRole("checkbox", { name: /All fields contain public data/ })
    .check();
  await requests
    .getByRole("button", { name: "Create request", exact: true })
    .click();
  await expect(
    requests.getByRole("heading", { name: "Edit request (revision 0)" }),
  ).toBeVisible();
  await expect(
    details.getByRole("button", { name: "Delete project", exact: true }),
  ).toBeDisabled();
  await requests.getByRole("button", { name: "Remove header 1" }).click();
  await requests.getByLabel("Body type").selectOption("none");
  await requests
    .getByRole("checkbox", { name: /All fields contain public data/ })
    .check();
  await requests.getByRole("button", { name: "Save request changes" }).click();
  await expect(
    requests.getByRole("heading", { name: "Edit request (revision 1)" }),
  ).toBeVisible();
  await requests
    .getByRole("button", { name: "Refresh request details" })
    .click();
  await expect(requests.getByLabel("Body type")).toHaveValue("none");
  await expect(requests.getByLabel("Header name 1")).toHaveCount(0);
  // Commit a request edit while deliberately losing its response. Do not replay.
  let requestWrites = 0;
  await page.route(
    backend + "/api/v1/projects/" + id + "/requests/*",
    async (route) => {
      if (route.request().method() !== "PATCH") return route.continue();
      requestWrites++;
      await route.fetch();
      await route.abort("failed");
    },
  );
  await requests
    .getByLabel("Request name", { exact: true })
    .fill("Committed request edit");
  await requests
    .getByRole("checkbox", { name: /All fields contain public data/ })
    .check();
  await requests.getByRole("button", { name: "Save request changes" }).click();
  await expect(
    requests.getByRole("button", { name: "Save request changes" }),
  ).toBeDisabled();
  await expect(
    requests.getByLabel("Request name", { exact: true }),
  ).toHaveValue("Committed request edit");
  expect(requestWrites).toBe(1);
  await page.unroute(backend + "/api/v1/projects/" + id + "/requests/*");
  await requests
    .getByRole("button", { name: "Refresh request details" })
    .click();
  await expect(
    requests.getByRole("heading", { name: "Edit request (revision 2)" }),
  ).toBeVisible();
  await requests
    .getByRole("button", { name: "Delete request", exact: true })
    .click();
  await requests
    .getByRole("button", { name: "Confirm request deletion" })
    .click();
  await expect(
    requests.getByText("Request deleted.", { exact: true }),
  ).toBeVisible();
  // Keep one live request plus the individual tombstone for project cleanup.
  await requests
    .getByRole("button", { name: "New request", exact: true })
    .click();
  await requests
    .getByLabel("Request name", { exact: true })
    .fill("Cascade browser request");
  await requests
    .getByLabel("Public request URL")
    .fill("https://example.com/public");
  // Genuine Chromium-to-Go/Local protected authoring; no traces/screenshots.
  const disposable = "Bearer " + crypto.randomUUID();
  const responseChecks: Promise<boolean>[] = [];
  page.on("response", (response) => {
    if (response.url().startsWith(backend + "/api/"))
      responseChecks.push(
        response
          .text()
          .then((text) => !text.includes(disposable))
          .catch(() => true),
      );
  });
  await requests
    .getByRole("button", { name: "Add header", exact: true })
    .click();
  await requests.getByLabel("Header name 1").fill("Authorization");
  await requests
    .getByRole("checkbox", { name: "Protected", exact: true })
    .check();
  await requests.getByLabel("Header value 1").fill(disposable);
  await requests
    .getByRole("checkbox", { name: /All fields contain public data/ })
    .check();
  await requests
    .getByRole("button", { name: "Create request", exact: true })
    .click();
  await expect(
    requests.getByRole("heading", { name: "Edit request (revision 0)" }),
  ).toBeVisible();
  await expect
    .poll(
      async () =>
        (await requests.getByLabel("Header value 1").inputValue()) === "",
    )
    .toBe(true);
  await requests
    .getByLabel("Request name", { exact: true })
    .fill("Protected cascade request");
  await requests
    .getByRole("checkbox", { name: /All fields contain public data/ })
    .check();
  await requests.getByRole("button", { name: "Save request changes" }).click();
  await expect(
    requests.getByRole("heading", { name: "Edit request (revision 1)" }),
  ).toBeVisible();
  await requests
    .getByRole("button", { name: "Refresh project secrets" })
    .click();
  await expect(requests.getByText(/revision 0/)).toBeVisible();
  const storageSafe = await page.evaluate(async (value) => {
    const text = JSON.stringify({ ...localStorage, ...sessionStorage });
    return (
      !text.includes(value) &&
      localStorage.length === 0 &&
      sessionStorage.length === 0 &&
      (await indexedDB.databases()).length === 0 &&
      (await caches.keys()).length === 0
    );
  }, disposable);
  if (!storageSafe || (await Promise.all(responseChecks)).some((v) => !v))
    throw new Error("Protected value detected in forbidden browser surface");
  await details
    .getByRole("button", { name: "Refresh details", exact: true })
    .click();
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

test("disposable-secret audit tolerates unrelated storage and detects real leaks", async ({
  page,
}) => {
  await page.goto("/");
  const flags = await page.evaluate(async () => {
    const { auditSecretPersistence } =
      await import("/src/secret-persistence-audit.ts");
    const value = "Bearer " + crypto.randomUUID();
    localStorage.setItem("unrelated-preference", "dark");
    sessionStorage.setItem("unrelated-panel", "open");
    const db = await new Promise<IDBDatabase>((resolve, reject) => {
      const open = indexedDB.open("disposable-audit-fixture", 1);
      open.onupgradeneeded = () => open.result.createObjectStore("rows");
      open.onsuccess = () => resolve(open.result);
      open.onerror = () => reject(new Error("Fixture setup failed"));
    });
    const put = (data: unknown) =>
      new Promise<void>((resolve, reject) => {
        const tx = db.transaction("rows", "readwrite");
        tx.objectStore("rows").put(data, "row");
        tx.oncomplete = () => resolve();
        tx.onerror = () => reject(new Error("Fixture setup failed"));
      });
    await put({ preference: "public" });
    const cache = await caches.open("disposable-audit-fixture");
    const url = location.origin + "/disposable-audit-fixture";
    await cache.put(url, new Response("public"));
    const unrelated = await auditSecretPersistence([value]);
    localStorage.setItem("disposable-value", value);
    const web = await auditSecretPersistence([value]);
    localStorage.removeItem("disposable-value");
    await put({ bytes: new TextEncoder().encode(value) });
    const indexed = await auditSecretPersistence([value]);
    await put({ preference: "public" });
    await cache.put(url, new Response(value));
    const cached = await auditSecretPersistence([value]);
    const safeOutput = !JSON.stringify([
      unrelated,
      web,
      indexed,
      cached,
    ]).includes(value);
    const unrelatedPreserved =
      localStorage.getItem("unrelated-preference") === "dark";
    db.close();
    return {
      unrelatedAllowed: unrelated.storageSafe && unrelated.auditComplete,
      webDetected: !web.storageSafe && web.auditComplete,
      indexedDetected: !indexed.storageSafe && indexed.auditComplete,
      cacheDetected: !cached.storageSafe && cached.auditComplete,
      safeOutput,
      unrelatedPreserved,
    };
  });
  expect(flags).toEqual({
    unrelatedAllowed: true,
    webDetected: true,
    indexedDetected: true,
    cacheDetected: true,
    safeOutput: true,
    unrelatedPreserved: true,
  });
});
