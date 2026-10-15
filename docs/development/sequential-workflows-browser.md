# Sequential workflow Cognito/browser checks

These are numbered checks for Davian against genuine Cognito and the deployed
`dev-api` API. They are **pending operator execution**. Automated Chromium tests
use fixture authentication and do not satisfy this checklist. The SDK AWS suite
uses a separate disposable owner/project, which is deleted after its checks.

Start the current branch frontend with `npm run dev:cloud`, open
<http://localhost:5173>, and sign in with your existing Cognito account. No
frontend hosting or Cognito/account settings changed. Keep bearer credentials
out of screenshots, exported network captures and messages. The fixture token
is generated only by the controlled endpoint; never type it into a saved request.

1. Create an owned project named **Workflow browser acceptance**. In its saved
   requests, create these two **GET** requests with no body or query rows:

   | Request                 | Exact URL                                                                                                                                |
   | ----------------------- | ---------------------------------------------------------------------------------------------------------------------------------------- |
   | Browser token producer  | `https://8dnymkcoa0.execute-api.us-east-2.amazonaws.com/execution-fixture/workflow-token?fixture=14ee6f5b-0f56-43c8-817f-2ec599498f00`   |
   | Browser Bearer consumer | `https://8dnymkcoa0.execute-api.us-east-2.amazonaws.com/execution-fixture/workflow-consume?fixture=14ee6f5b-0f56-43c8-817f-2ec599498f00` |

   Add an enabled public `Authorization` header to the **consumer** with an
   **empty value**. Save both requests. Expected: both save successfully;
   no token or secret reference is entered or revealed.

2. Select **Load workflows**, then **New workflow**. Name it **Browser Bearer
   sequence** and add two ordered steps: producer first, consumer second.
   In step 1 add extraction name `token`, path `$.token`, type `string`.
   In step 2 add a binding from step 1's `token`, destination `header`, target
   index `0` (Authorization); prefix exactly `Bearer ` including the trailing
   space; suffix empty. Save. Expected: revision 1, correct order/references;
   no script/template fields. Leave HTTP-secret consent unchecked: both URLs
   use HTTPS.

3. Select **Submit saved workflow** once. Expected: one new run goes
   queued/running → completed, both steps completed with HTTP 200. Open each
   step's evidence. Producer `token` is redacted; consumer has redacted
   Authorization and raw/rendered reflection values, with public
   `authorized: true`. The raw token and `Bearer <token>` must not appear in
   evidence or browser storage. A consumer HTTP 401 is a failure, not a pass.
   Refresh/reopen evidence: the sanitized response is stable. Refresh the
   project before unrelated mutations if the UI reports a stale project version.
   If admission reports uncertainty, use **Reconcile original submission**;
   it retains the original key. Do not submit again with a fresh key to repair
   a missing response or obtain a pass.

4. Pin the first step. Expected: **Protected from automatic expiry while
   pinned**, with no response-byte changes. Refresh and verify the pin remains.
   Unpin and verify normal retention returns. This browser check does not
   simulate thirty days or establish pin/expiry race behavior; the AWS suite
   separately ages disposable timestamps and uses genuine scheduled cleanup.

5. Select the saved workflow and **Delete workflow definition**. Expected:
   definition disappears while the completed run and both evidence views remain.
   Open the retained completed run and select **Rerun entire workflow** once.
   Expected: a new run ID, both steps completed/HTTP 200, frozen order and Bearer
   behavior preserved despite the absent live definition. Original evidence
   remains unchanged. This is a deliberate new run; a refresh must not create
   another run.

6. Create a GET request **Browser known failure** using exactly
   `https://8dnymkcoa0.execute-api.us-east-2.amazonaws.com/execution-fixture/workflow-failure?fixture=14ee6f5b-0f56-43c8-817f-2ec599498f01`.
   Create a two-step workflow with that request first and the saved producer
   second, with **no extractions or bindings**. Submit once. Expected: run
   failed, step 1 HTTP 500/known outcome, step 2 skipped with no execution ID;
   no automatic successor HTTP.

7. Create a GET request **Browser unknown outcome** using exactly
   `https://8dnymkcoa0.execute-api.us-east-2.amazonaws.com/execution-fixture/workflow-delay?fixture=14ee6f5b-0f56-43c8-817f-2ec599498f02`.
   Create a two-step workflow with it first and the producer second, with no
   extractions or bindings. Submit once. Expected: failed/unknown certainty
   after the response-header deadline (about ten seconds), an unknown-outcome
   warning, and step 2 skipped. Wait a minute and refresh: no new run or HTTP
   retry. Do not select rerun for this check. The target deliberately waits
   twelve seconds; unknown does not mean it received no request.

8. In a separate browser profile, sign in as a **different genuine Cognito
   user** at localhost:5173. Open DevTools Console and use this development-only
   snippet, replacing the three safe IDs from the first owner's successful run.
   It imports the existing app client and uses its in-memory session without
   printing a token. Each operation is attempted once; stop on any unexpected
   result. Expected: every line reports 404 and first-owner data stays intact.
   If no second account is available, report this check unperformed.

   ```js
   const { api } = await import("/src/api.ts");
   const project = "FIRST_OWNER_PROJECT_ID";
   const run = "FIRST_OWNER_RUN_ID";
   const execution = "FIRST_OWNER_SUCCESSFUL_STEP_EXECUTION_ID";
   for (const [name, invoke] of [
     ["definitions", () => api.workflows(project)],
     ["runs", () => api.workflowRuns(project)],
     ["run", () => api.workflowRun(project, run)],
     [
       "rerun",
       () => api.submitWorkflow(project, run, crypto.randomUUID(), false),
     ],
     ["evidence", () => api.executionEvidence(project, execution)],
     ["pin", () => api.pinExecution(project, execution, true)],
     ["unpin", () => api.pinExecution(project, execution, false)],
   ]) {
     let status;
     try {
       await invoke();
       status = "UNEXPECTED_SUCCESS";
     } catch (error) {
       status = error.status ?? "UNEXPECTED_ERROR";
     }
     console.log(name, status);
     if (status !== 404) throw Error("Stop: isolation check failed");
   }
   ```

9. Return to the original owner and pin one successful step again. Refresh the
   project, delete **Workflow browser acceptance**, and record only the safe
   project/deletion-operation IDs. Expected: accepted deletion immediately
   hides workflow/run/evidence/pin operations (404), then the minute-scheduled
   cleanup reports completed and overrides the pin. Report the IDs so an
   independent Control/Protected/S3 cleanup readback can be performed. Do not
   delete your Cognito user or reset any quota/counter.

Report the numbered results, safe project/run/operation IDs and any public error
codes. Never report raw tokens. With exactly one submission in checks 3, 5, 6
and 7, expected target sends for these fixture IDs are producer **2**, consumer
**2**, known-failure **1**, timeout **1**. Ten-second unknown and deliberate
rerun behavior must be interpreted separately. Extra deliberate submissions
change expected counts; report them rather than retrying to obtain a pass.

The controlled endpoint logs only an opaque fixture UUID, mode, authorization
boolean and AWS request ID. A bounded private CloudWatch audit can count sends
and check containment after your report. The numbered checks do not yet claim
operator acceptance, browser-secret auditing, cross-owner denial or cleanup
completion.
