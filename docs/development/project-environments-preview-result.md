# Authorized environment preview — stopped

Davian authorized one read-only preview using reviewed tooling
`843428a3875e6d5ac3544420c392731d6a3991ee` and unchanged release source
`44f1fb79d943501c95e06fcc1a6114111a7c1ea4`. This is the actual result of that
single invocation, continuing [PR #18](https://github.com/VioletProto/Kurier/pull/18)
and the [reviewed meter packet](project-environments-preview-meter.md).

**Stopped on `endpoint-held`; no restart, policy widening, refresh or repair.**
The shared meter rejected an unrecognized endpoint before forwarding its request,
and the supervisor terminated the sandbox. The launcher returned1 and reported
sandbox exit−15. This is not a successful or complete SST preview and provides
no deployment approval.

## Command, bounds and retained evidence

Exactly this command ran once from `/home/king/Neumont/Kurier`:

```sh
python3 -B scripts/preview-meter/run.py preview \
  --review-commit 843428a3875e6d5ac3544420c392731d6a3991ee \
  --source /tmp/kurier-environments-release-source \
  --output /home/king/Neumont/Kurier/.build/project-environments-preview/review-1
```

Local checks first verified the clean tooling/source heads, reviewed code
checksums, SST/platform/provider pins and prepared/rollback artifact bytes.
Existing cached credentials passed the reviewed local validity check. No
credential refresh was attempted or permitted. The policy and launcher were
not edited to execute or recover this attempt.

| Window/accounting    | Actual result                                                                                |
| -------------------- | -------------------------------------------------------------------------------------------- |
| Shared proxy created | 2026-10-28 02:47:34.091105 UTC (October27 20:47:34 MDT)                                      |
| Stop latched         | 2026-10-28 02:47:55.955451 UTC                                                               |
| Elapsed              | 21.864seconds of the authorized3600seconds                                                   |
| Prior AWS attempts   | **135**, retained from the inventory campaign                                                |
| This invocation      | **30/365** additional physical read attempts, numbered136–165                                |
| Cumulative           | **165/500** physical attempts                                                                |
| Outcomes             | All30 retained: **23 HTTP200, 7 HTTP404**; no unmatched attempt or transport-failure receipt |
| Stop                 | `endpoint-held`; rejected endpoint/operation not forwarded and not charged as an AWS call    |

The arithmetic remainder is335 calls. It is **not** permission to reopen this
window, rerun this command or reset the clock. All live calls stopped. The
reviewed launcher's `check` action still describes its frozen135-call parent;
that output is not current campaign accounting and must not erase these30 calls.
Any future preparation must retain cumulative165 and the stopped ledger below.

[Machine result and classified partial diff](project-environments-preview-result.json).
[Frozen attempt/outcome ledger](project-environments-preview-attempts.jsonl)
SHA-256: `e65635f9b3e21f406cb70dcd0ba7ae71f952e203e54d30f6474da591abe525eb`.
All30 forwarded operations were on the unchanged read allowlist: SSM
GetParameter2, S3 GET15/HEAD2, STS GetCallerIdentity7, Cognito DescribeUserPool1/
DescribeUserPoolClient1 and DynamoDB DescribeTable2. The404 responses are
retained as observed; they are not reported as successful reads or silently retried
with mutation/repair commands. SDK/provider retries, if any, remain in these
physical counts rather than inferred from logical resources.

The meter records a fixed endpoint-held reason but **does not retain the rejected
hostname or operation**. Private output and meter stderr are empty; available
SST/Pulumi diagnostics do not conclusively attribute that stop. No endpoint is
claimed to be a harmless read, telemetry or a specific AWS service. This is an
attribution gap requiring review, not permission to enlarge the allowlist.

Raw state and the907,184-byte Pulumi event log remain in the ignored private
run directory. Only selected resource names/types/operations and file hashes are
published. No inputs/outputs, parameter values, credentials, signed URLs or raw
provider diagnostics are included. No post-stop AWS readback was performed;
current live stage/accounting/IAM cannot be asserted from the earlier inventory
or this interrupted plan.

## Classified partial resource diff

The local Pulumi log contains1101 JSON events, including109 resource pre-events
and175 resource-output events. There is **no summary event**, and SST's final
JSON stdout is empty. Output events include duplicates; they are not counted as
additional proposed resources.

| Classification             | Observed preview event evidence                                        | Limit                                                                                                                                                                                                          |
| -------------------------- | ---------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| New route components       | **8 create pre-events**, all `sst:aws:ApiGatewayV2LambdaRoute`         | Local component registrations only; these are not AWS API route creations or deployment results.                                                                                                               |
| Existing entries           | **100 same pre-events**                                                | Provisional entries observed before interruption; not proof the complete release or all IAM/resources are unchanged.                                                                                           |
| Existing bucket reference  | **1 read pre-event**, `FrontendWebAssets`, type `aws:s3/bucket:Bucket` | A read/reference, not a proposed new bucket.                                                                                                                                                                   |
| AWS add/update/delete plan | No completed plan or summary                                           | The expected24 route/integration/Lambda-permission additions, four code-object/four Lambda changes and three frontend changes were not completely evaluated. No complete IAM diff or allowed-resource verdict. |

The eight component IDs were independently matched to pinned source routes using
the installed platform's existing naming function, not event-order guesses:

| Component suffix after `DevelopmentHttpApiRoute` | Pinned source operation                                             |
| ------------------------------------------------ | ------------------------------------------------------------------- |
| Bateco                                           | POST `/api/v1/projects/{projectId}/environments`                    |
| Bcuohn                                           | GET `/api/v1/projects/{projectId}/environments`                     |
| Bbrzwo                                           | GET `/api/v1/projects/{projectId}/environments/{environmentId}`     |
| Vxffaw                                           | PATCH `/api/v1/projects/{projectId}/environments/{environmentId}`   |
| Rdfcms                                           | DELETE `/api/v1/projects/{projectId}/environments/{environmentId}`  |
| Kotabw                                           | OPTIONS `/api/v1/projects/{projectId}/environments`                 |
| Sndcdb                                           | OPTIONS `/api/v1/projects/{projectId}/environments/{environmentId}` |
| Kbebwh                                           | POST `/api/v1/local-agent/capabilities`                             |

These observations are classified in the machine result; the full sanitized
pre-event list is retained there. Source-derived expectations in the earlier
packet remain expectations, not a substitute for the missing complete preview.

## Packaging and preservation

The preview rebuilt the hosted frontend locally. All five file sizes/hashes,
public configuration and release ID match the prepared release:
`38325083b003b3f320752dbfa0357c377d57d2b66c1175e06c88ba3638210236`.
`matchesPreparedFrontend:true` is retained in the local packaging receipt and
published machine result. **No new frontend hash requires acceptance.**

No Lambda ZIP was generated before the stop, and the scratch artifact directory
contains no package files. An empty package list means packaging did not finish,
not that new packages match the prepared ZIPs. There are no new ZIP hashes to
review. The four prepared ZIPs/bootstraps, foreground clients, frontend and
retrieved rollback bytes were rechecked locally and remain unchanged. Release
manifest SHA-256 remains
`94d658f4cdae67103c755e3cf5578a50252f8f780c2df82c46c2c2cf18f8c6c5`.
The old preparation and meter checksums still pass. The pinned source checkout
remains clean. No production artifacts were replaced or uploaded.

## Holds, uncertainty and next decision

No deployment, stage closure, migration, activation, identity/project creation
or live target request occurred. The unchanged policy denied mutations throughout;
no forwarded request was outside its read classifications. This does not establish
that another actor made no concurrent changes to AWS. OpenAPI production scope
remains separate.

The **same cumulative $1 procedural allowance** is retained, not replenished.
No account billing or price request was made, and no unused dollar balance or
measured spend is claimed. Ordinary read/transfer fees and delayed/shared costs
remain uncertain.

Local publication checks include format/gofmt, lint/tests/build/Go verification,
ledger sequence/outcome accounting, sanitized event classification, local links
and artifact/checksum preservation. They do not complete the interrupted AWS
preview or deployed runtime validation.

Next decision is review of the endpoint-attribution gap, the stopped165-call
ledger and incomplete diff. Any instrumentation/fixture follow-up, subsequent
preview tooling/policy review or new window must be separately scoped. No changes
to the read policy, repairs, credential refresh or automatic restart were made
or proposed for execution by this result. All existing deployment/runtime holds
remain.
