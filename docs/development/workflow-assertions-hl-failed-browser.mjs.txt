import { chromium } from "@playwright/test";
export async function run(c) {
  const { p, w, mode } = c.privateState.browserAction;
  const { privateState: s, check, call, root, note, hash } = c;
  const browser = await chromium.launch({
    headless: true,
    proxy: { server: "http://127.0.0.1:18080" },
    args: [
      "--disable-background-networking",
      "--disable-component-update",
      "--disable-domain-reliability",
    ],
  });
  try {
    const context = await browser.newContext({
      ignoreHTTPSErrors: true,
      serviceWorkers: "block",
    });
    const page = await context.newPage();
    page.setDefaultTimeout(20000);
    const bad = [];
    page.on("response", (r) => {
      if (r.url().includes("/api/v1/") && r.status() >= 400)
        bad.push(r.status());
    });
    await page.goto(c.o.hostedFrontendOrigin, {
      waitUntil: "domcontentloaded",
    });
    await page.getByLabel("Email", { exact: true }).fill(s.users[0].name);
    await page
      .getByLabel("Password", { exact: true })
      .fill(s.users[0].password);
    await page.getByRole("button", { name: "Sign in", exact: true }).click();
    await page.getByRole("button", { name: p.name, exact: true }).click();
    const section = page.getByRole("region", {
      name: "Workflows",
      exact: true,
    });
    await section
      .getByRole("button", { name: "Load workflows", exact: true })
      .click();
    await section
      .getByRole("button", {
        name: w.name + " (revision " + w.revision + ")",
        exact: true,
      })
      .click();
    const tokens = [
      "0.10000000000000001",
      "0.1",
      null,
      "1e-308",
      "9007199254740991",
      "-0.0",
    ];
    const paths = [
      "$.precise",
      "$.short",
      "$.exponent",
      "$.min",
      "$.max",
      "$.zero",
    ];
    if (mode === "author")
      for (let pos = 0; pos < 2; pos++) {
        const step = section.getByRole("group", {
          name: "Step " + (pos + 1),
          exact: true,
        });
        const editor = step.getByRole("region", {
          name: "Step assertions",
          exact: true,
        });
        for (let i = 0; i < 8; i++) {
          await editor
            .getByRole("button", { name: "Add assertion", exact: true })
            .click();
          const a = editor.getByRole("group", {
            name: "Assertion " + (i + 1),
            exact: true,
          });
          if (i < 6) {
            await a
              .getByLabel("Assertion kind", { exact: true })
              .selectOption("jsonEquals");
            await a
              .getByLabel("Assertion JSON path", { exact: true })
              .fill(paths[i]);
            await a
              .getByLabel("Expected type", { exact: true })
              .selectOption("number");
            await a
              .getByLabel("Expected decimal", { exact: true })
              .fill(i === 2 ? (pos === 0 ? "1e-1" : "1E-01") : tokens[i]);
          } else if (i === 6) {
            await a
              .getByLabel("Assertion kind", { exact: true })
              .selectOption("jsonExists");
            await a
              .getByLabel("Assertion JSON path", { exact: true })
              .fill("$.exists");
          } else {
            await a
              .getByLabel("Assertion kind", { exact: true })
              .selectOption("statusEquals");
            await a.getByLabel("Expected status", { exact: true }).fill("200");
          }
        }
      }
    const expected = [
      ...tokens.slice(0, 2),
      "1e-1",
      ...tokens.slice(3),
      ...tokens.slice(0, 2),
      "1E-01",
      ...tokens.slice(3),
    ];
    if (mode !== "author")
      check(
        JSON.stringify(
          await section
            .getByLabel("Expected decimal", { exact: true })
            .allInputValues(),
        ) === JSON.stringify(expected),
        "Browser reopen decimal tokens changed",
      );
    const name =
      "Assertions H " +
      (mode === "author"
        ? "authored"
        : mode === "rename-before"
          ? "reopened"
          : "unrelated edit after H");
    await section.getByLabel("Workflow name", { exact: true }).fill(name);
    const response = page.waitForResponse(
      (r) =>
        r.url().endsWith(root() + "/workflows/" + w.workflowId) &&
        r.request().method() === "PATCH",
    );
    await section
      .getByRole("button", { name: "Save workflow", exact: true })
      .click();
    const r = await response;
    check(r.status() === 200, "Browser save uncertain or rejected");
    const d = await r.json();
    const out = d.data.workflow;
    check(out?.workflowId === w.workflowId, "Browser saved different workflow");
    s.workflows.H = out;
    check(
      JSON.stringify(
        out.steps.flatMap((st) =>
          st.assertions
            .filter((a) => a.expected?.type === "number")
            .map((a) => a.expected.decimal),
        ),
      ) === JSON.stringify(expected),
      "Browser/API serialization lost decimals",
    );
    const storage = await page.evaluate(async () => ({
      local: localStorage.length,
      session: sessionStorage.length,
      indexed: (await indexedDB.databases()).length,
      caches: (await caches.keys()).length,
    }));
    check(
      Object.values(storage).every((n) => n === 0),
      "Hosted browser persisted session material",
    );
    check(!bad.length, "Unexpected hosted API response");
    note("hosted-decimal-roundtrip", {
      mode,
      revision: out.revision,
      decimalTokens: expected,
      storageEmpty: true,
    });
    await context.close();
  } catch {
    throw Error("Hosted browser verification failed; no automatic replay");
  } finally {
    await browser.close();
  }
}
