import { run } from "../workflow-assertions-hl/browser.mjs";
const project = {
  projectId: "44444444-4444-4444-8444-444444444444",
  name: "Local authoring project",
};
const workflow = {
  workflowId: "66666666-6666-4666-8666-666666666666",
  name: "Assertions H",
  revision: 0,
};
const state = {
  users: [
    { name: "fixture@example.invalid", password: "Local-only-password1!" },
  ],
  workflows: { H: workflow },
};
const c = {
  privateState: state,
  o: { hostedFrontendOrigin: "https://d2kpq905qyh9zg.cloudfront.net" },
  root: () => "/api/v1/projects/" + project.projectId,
  check: (v, s) => {
    if (!v) throw Error(s);
  },
  note: () => {},
};
try {
  for (const mode of ["author", "rename-before", "rename-after"]) {
    state.browserAction = { p: project, w: state.workflows.H, mode };
    await run(c);
  }
} catch {
  console.error(
    JSON.stringify({
      localAuthoringFailureAt: state.browserProgress,
      mode: state.browserAction.mode,
      expectedName: state.browserAction.w.name,
      expectedRevision: state.browserAction.w.revision,
    }),
  );
  process.exitCode = 1;
}
