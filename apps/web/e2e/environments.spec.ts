import { expect, test } from "@playwright/test";

test("environment editor and compiled agent preserve typed frozen values, current secrets and versioned evidence", async ({
  page,
  request,
}) => {
  const token = process.env.KURIER_BROWSER_FIXTURE_TOKEN,
    idToken = process.env.KURIER_BROWSER_FIXTURE_ID_TOKEN,
    backend = process.env.VITE_API_URL,
    target = process.env.KURIER_BROWSER_LOCAL_TARGET;
  if (!token || !idToken || !backend || !target)
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
  await page.getByLabel("New project name").fill("Environment browser fixture");
  await page
    .getByRole("button", { name: "Create project", exact: true })
    .click();
  const details = page.getByRole("region", { name: "Project details" });
  await expect(
    details.getByRole("heading", {
      name: "Environment browser fixture",
      exact: true,
    }),
  ).toBeVisible();
  const project = (await details.locator("dd").first().textContent())!;
  try {
    const panel = page.getByRole("region", {
      name: "Project environments",
      exact: true,
    });
    await panel
      .getByRole("button", { name: "Open environments", exact: true })
      .click();
    await panel
      .getByLabel("Environment name", { exact: true })
      .fill("Development");
    await panel
      .getByRole("button", { name: "Add variable", exact: true })
      .click();
    await panel.getByLabel("Variable name 1", { exact: true }).fill("ratio");
    await panel
      .getByLabel("Variable type 1", { exact: true })
      .selectOption("number");
    await panel
      .getByLabel("Variable value 1", { exact: true })
      .fill("0.10000000000000001");
    await panel
      .getByRole("button", { name: "Add variable", exact: true })
      .click();
    await panel.getByLabel("Variable name 2", { exact: true }).fill("token");
    await panel.getByLabel("Protected variable 2", { exact: true }).check();
    await panel
      .getByLabel("Variable value 2", { exact: true })
      .fill("environment-browser-before-canary");
    await panel
      .getByRole("button", { name: "Create environment", exact: true })
      .click();
    await expect(
      panel.getByRole("group", {
        name: "Edit environment · revision 0",
        exact: true,
      }),
    ).toBeVisible();
    await expect(
      panel.getByLabel("Variable value 1", { exact: true }),
    ).toHaveValue("0.10000000000000001");
    await expect(
      panel.getByLabel("Variable value 2", { exact: true }),
    ).toHaveCount(0);
    await panel
      .getByRole("button", { name: "Refresh environments", exact: true })
      .click();
    await expect(
      panel.getByLabel("Variable value 1", { exact: true }),
    ).toHaveValue("0.10000000000000001");
    const envs = await request.get(
      `${backend}/api/v1/projects/${project}/environments`,
      { headers },
    );
    expect(envs.status()).toBe(200);
    const summary = (await envs.json()).data.items[0];
    const requestPanel = page.getByRole("region", {
      name: "Saved requests",
      exact: true,
    });
    await requestPanel
      .getByRole("button", { name: "New request", exact: true })
      .click();
    await requestPanel
      .getByLabel("Request name", { exact: true })
      .fill("Environment request");
    await requestPanel
      .getByLabel("Public request URL", { exact: true })
      .fill(target);
    await requestPanel
      .getByLabel("Method", { exact: true })
      .selectOption("POST");
    await requestPanel
      .getByLabel("Enable environment templates", { exact: true })
      .check();
    await requestPanel
      .getByRole("button", { name: "Add header", exact: true })
      .click();
    await requestPanel
      .getByLabel("Header name 1", { exact: true })
      .fill("Authorization");
    await requestPanel
      .getByLabel("Header value 1", { exact: true })
      .fill("Bearer {{token}}");
    await requestPanel
      .getByLabel("Body type", { exact: true })
      .selectOption("json");
    await requestPanel
      .getByLabel("Public request body", { exact: true })
      .fill('{"ratio":"{{ratio}}"}');
    await requestPanel
      .getByLabel(
        "All fields contain public data unless explicitly protected; the URL contains no credentials or secrets.",
      )
      .check();
    await requestPanel
      .getByRole("button", { name: "Create request", exact: true })
      .click();
    await expect(
      requestPanel.getByRole("button", {
        name: "Save request changes",
        exact: true,
      }),
    ).toBeVisible();
    const executions = page.getByRole("region", {
      name: "Executions",
      exact: true,
    });
    const pairing = page.getByRole("region", {
      name: "Local agent pairing",
      exact: true,
    });
    await pairing
      .getByLabel("Exact HTTP(S) origins, one per line (up to 16)")
      .fill(target);
    await pairing
      .getByLabel(
        "I authorize this project and these exact destinations for the named device.",
      )
      .check();
    await pairing
      .getByRole("button", { name: "Create agent credential", exact: true })
      .click();
    const agentToken = (await pairing
      .getByLabel("One-time agent token")
      .textContent())!;
    await pairing
      .getByRole("button", { name: "Close token display", exact: true })
      .click();
    // The compiled CLI declares its capabilities before admission, with no job.
    expect(
      (
        await request.post(backend + "/test-only/local-agent-run", {
          data: { Token: agentToken, ProjectID: project, ExecutionID: "" },
        })
      ).status(),
    ).toBe(204);
    await executions.getByLabel("Execution target").selectOption("local");
    await executions
      .getByLabel(
        "I allow this execution on the selected device and its approved local/private services.",
      )
      .check();
    await executions
      .getByLabel(
        "I explicitly allow protected inputs over unencrypted HTTP for this execution or rerun.",
      )
      .check();
    const selector = executions.getByRole("group", {
      name: "Execution environment",
      exact: true,
    });
    await selector
      .getByRole("button", { name: "Refresh environment choices", exact: true })
      .click();
    await selector
      .getByLabel("Execution environment", { exact: true })
      .selectOption(summary.environmentId);
    await executions
      .getByRole("button", { name: "Execute saved revision 0", exact: true })
      .click();
    await expect(
      executions.getByText("Execution durably accepted.", { exact: false }),
    ).toBeVisible();
    const history = await request.get(
      `${backend}/api/v1/projects/${project}/executions`,
      { headers },
    );
    const execution = (await history.json()).data.items[0].executionId;
    expect(
      (
        await request.post(backend + "/test-only/local-agent-run", {
          data: {
            Token: agentToken,
            ProjectID: project,
            ExecutionID: execution,
          },
        })
      ).status(),
    ).toBe(204);
    const evidenceResponse = await request.get(
      `${backend}/api/v1/projects/${project}/executions/${execution}/evidence`,
      { headers },
    );
    expect(evidenceResponse.status()).toBe(200);
    const evidence = (await evidenceResponse.json()).data.evidence;
    expect(evidence.schemaVersion).toBe(3);
    expect(evidence.environmentContext).toEqual({
      environmentId: summary.environmentId,
      revision: 0,
    });
    expect(evidence.request.body.text).toContain("0.10000000000000001");
    expect(JSON.stringify(evidence)).not.toContain(
      "environment-browser-before-canary",
    );
    await panel.getByLabel("Variable value 1", { exact: true }).fill("2");
    await panel
      .getByRole("button", { name: "Save environment changes", exact: true })
      .click();
    await expect(
      panel.getByRole("group", {
        name: "Edit environment · revision 1",
        exact: true,
      }),
    ).toBeVisible();
    const detailResponse = await request.get(
      `${backend}/api/v1/projects/${project}/environments/${summary.environmentId}`,
      { headers },
    );
    const env = (await detailResponse.json()).data.environment;
    const secretId = env.variables[1].secretRef.secretId;
    expect(
      (
        await request.patch(
          `${backend}/api/v1/projects/${project}/secrets/${secretId}`,
          {
            headers: { ...headers, "If-Match": '"0"' },
            data: { value: "environment-browser-after-canary" },
          },
        )
      ).status(),
    ).toBe(200);
    const currentAgent = await request.get(
      `${backend}/api/v1/local-agents/current`,
      { headers },
    );
    const agent = (await currentAgent.json()).data.agent;
    const rerun = await request.post(
      `${backend}/api/v1/projects/${project}/executions/${execution}/rerun`,
      {
        headers: { ...headers, "Idempotency-Key": crypto.randomUUID() },
        data: {
          target: "local",
          local: {
            localAgentId: agent.localAgentId,
            allowPrivateNetwork: true,
          },
          allowInsecureSecrets: true,
        },
      },
    );
    expect(rerun.status()).toBe(202);
    const rerunId = (await rerun.json()).data.execution.executionId;
    expect(
      (
        await request.post(backend + "/test-only/local-agent-run", {
          data: { Token: agentToken, ProjectID: project, ExecutionID: rerunId },
        })
      ).status(),
    ).toBe(204);
    const again = await request.get(
      `${backend}/api/v1/projects/${project}/executions/${rerunId}/evidence`,
      { headers },
    );
    const frozen = (await again.json()).data.evidence;
    expect(frozen.environmentContext.revision).toBe(0);
    expect(frozen.request.body.text).toContain("0.10000000000000001");
    expect(JSON.stringify(frozen)).not.toContain(
      "environment-browser-after-canary",
    );
    expect(
      await page.evaluate(() => ({
        local: localStorage.length,
        session: sessionStorage.length,
      })),
    ).toEqual({ local: 0, session: 0 });
    await panel
      .getByRole("button", { name: "Delete environment", exact: true })
      .click();
    await panel
      .getByRole("button", {
        name: "Confirm environment deletion",
        exact: true,
      })
      .click();
    await expect(
      panel.getByRole("group", { name: "Create environment", exact: true }),
    ).toBeVisible();
    const projectDetail = await request.get(
      `${backend}/api/v1/projects/${project}`,
      { headers },
    );
    expect(
      (
        await request.delete(`${backend}/api/v1/projects/${project}`, {
          headers: { ...headers, "If-Match": projectDetail.headers().etag },
        })
      ).status(),
    ).toBe(202);
    for (let i = 0; i < 20; i++) {
      const result = await request.post(backend + "/test-only/cleanup", {
        data: { projectId: project },
      });
      if (result.status() === 200) break;
    }
  } finally {
    // Release only this test's fixture resources, including after a failed UI step.
    const current = await request.get(
      `${backend}/api/v1/local-agents/current`,
      { headers },
    );
    if (current.status() === 200) {
      const agent = (await current.json()).data.agent;
      if (
        agent?.scopes?.some(
          (scope: { projectId: string }) => scope.projectId === project,
        )
      ) {
        await request.delete(
          `${backend}/api/v1/local-agents/${agent.localAgentId}`,
          { headers },
        );
      }
    }
    const currentProject = await request.get(
      `${backend}/api/v1/projects/${project}`,
      { headers },
    );
    if (currentProject.status() === 200) {
      await request.delete(`${backend}/api/v1/projects/${project}`, {
        headers: { ...headers, "If-Match": currentProject.headers().etag },
      });
      for (let i = 0; i < 20; i++) {
        const r = await request.post(backend + "/test-only/cleanup", {
          data: { projectId: project },
        });
        if (r.status() === 200) break;
      }
    }
  }
});
