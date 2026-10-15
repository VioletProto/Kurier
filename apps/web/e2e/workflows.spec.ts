import { expect, test } from "@playwright/test";
test("cloud workflow editor freezes revisions, configures Bearer rendering and submits once through Go/DDB Local", async ({
  page,
}) => {
  const token = process.env.KURIER_BROWSER_FIXTURE_TOKEN;
  if (!token) throw Error("Run Go integration,browser harness");
  await page.route("https://cognito-idp.us-east-2.amazonaws.com/", (route) =>
    route.fulfill({
      contentType: "application/x-amz-json-1.1",
      body: JSON.stringify({
        AuthenticationResult: {
          AccessToken: token,
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
  await page.getByRole("button", { name: "Load more projects" }).click();
  await page
    .getByRole("button", { name: "Workflow browser fixture", exact: true })
    .click();
  const panel = page.getByRole("region", { name: "Workflows" });
  await panel.getByText("Load workflows", { exact: true }).click();
  await panel.getByLabel("Workflow name").fill("Browser login workflow");
  await panel.getByRole("button", { name: "Add step", exact: true }).click();
  let first = panel.getByRole("group", { name: "Step 1", exact: true });
  await first
    .getByLabel("Saved request")
    .selectOption({ label: "Workflow login (revision 0)" });
  await first.getByRole("button", { name: "Add output", exact: true }).click();
  await first.getByLabel("Output name").fill("token");
  await first.getByLabel("JSON path").fill("$.token");
  await panel.getByRole("button", { name: "Add step", exact: true }).click();
  let second = panel.getByRole("group", { name: "Step 2", exact: true });
  await second
    .getByLabel("Saved request")
    .selectOption({ label: "Workflow consumer (revision 0)" });
  await second
    .getByRole("button", { name: "Add binding", exact: true })
    .click();
  await second.getByLabel("Public literal prefix").fill("Bearer ");
  await second.getByRole("button", { name: "Move up", exact: true }).click();
  await expect(
    panel.getByRole("button", { name: "Save workflow", exact: true }),
  ).toBeDisabled();
  first = panel.getByRole("group", { name: "Step 1", exact: true });
  await first.getByRole("button", { name: "Move down", exact: true }).click();
  await panel
    .getByRole("button", { name: "Save workflow", exact: true })
    .click();
  await expect(
    panel.getByRole("button", {
      name: "Browser login workflow (revision 0)",
      exact: true,
    }),
  ).toBeVisible();
  second = panel.getByRole("group", { name: "Step 2", exact: true });
  await expect(second.getByLabel("Public literal prefix")).toHaveValue(
    "Bearer ",
  );
  const admitted = page.waitForResponse(
    (r) => r.request().method() === "POST" && r.url().endsWith("/runs"),
  );
  await panel
    .getByRole("button", { name: "Submit saved workflow", exact: true })
    .click();
  expect((await admitted).status()).toBe(202);
  await expect(
    panel.getByRole("region", { name: "Workflow run" }),
  ).toContainText("queued");
  await expect(
    panel.getByRole("region", { name: "Workflow run" }),
  ).toContainText("value never displayed");
  await page.reload();
  await expect(
    page.getByRole("button", { name: "Sign in", exact: true }),
  ).toBeVisible();
});
