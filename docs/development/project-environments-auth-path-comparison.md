# Offline comparison: default SDK/CLI refresh versus custom helper

**The current default SDK/CLI and custom helper do not select the same
CreateOAuth2Token hostname.** They agree on region and currently effective proxy
and TLS settings, but differ in operation endpoint, configuration inheritance,
timeouts and retry configuration. This is an offline configuration finding,
**not proof of historical causation**. No endpoint, cache, network or policy
setting was changed. No network request, refresh, preview, DNS/TCP/TLS probe or live
credential-provider call ran. Only GitHub publication uses external access.

## Confirmed current comparison

The earlier successful invocation used
boto3.Session(region_name=us-east-2).get_credentials().get_frozen_credentials().
This comparison reproduces its client region/configuration with the installed
SDK while supplying only synthetic parameters, blocking credential-provider
access, and interrupting CreateOAuth2Token at **before-call**. The custom column
executes only the hash-pinned helper's extracted client factory, never its
ExistingCache/authenticate/refresh routine. The CLI column resolves through the
installed CLI-vendored SDK in a separate process; no CLI login/export-credentials
command was executed. Separating processes prevents CLI import aliases from
changing which SDK is measured in another column.

| Configuration                            | Current default SDK path                       | Current CLI-vendored SDK                  | Custom helper                                                      |
| ---------------------------------------- | ---------------------------------------------- | ----------------------------------------- | ------------------------------------------------------------------ |
| SDK version                              | botocore 1.43.108                              | CLI 2.37.9, vendored botocore 2.0.0dev155 | botocore 1.43.108                                                  |
| Client base hostname                     | signin.us-east-2.amazonaws.com                 | same                                      | same                                                               |
| **CreateOAuth2Token operation hostname** | **us-east-2.signin.aws.amazon.com**            | **us-east-2.signin.aws.amazon.com**       | **signin.us-east-2.amazonaws.com**                                 |
| Client/operation region                  | us-east-2                                      | us-east-2                                 | us-east-2                                                          |
| Endpoint selection                       | SDK operation resolver                         | SDK operation resolver                    | Explicit endpoint override                                         |
| Profile/config files                     | Inherited                                      | Inherited                                 | Disabled for SDK session; existing login cache selected separately |
| Proxy configuration                      | Inherited/default                              | Inherited/default                         | Explicit empty mapping                                             |
| Effective proxy now                      | None                                           | None                                      | None                                                               |
| TLS verification                         | Required                                       | Required                                  | Required                                                           |
| CA anchors                               | Same public bundle hash                        | Same                                      | Same                                                               |
| TLS version limits                       | Minimum TLS 1.2, maximum supported             | Same                                      | Same                                                               |
| TLS options/cipher configuration         | Same                                           | Same                                      | Same                                                               |
| Client certificate                       | None configured                                | None                                      | None                                                               |
| Expected TLS peer/SNI                    | Operation hostname                             | Operation hostname                        | Different operation hostname                                       |
| Connect/read timeout seconds             | 60/60                                          | 60/60                                     | 10/30                                                              |
| Retry mode/configured attempt fields     | legacy; no explicit max/total in client config | standard; max_attempts=3 in client config | legacy; total_max_attempts=1                                       |

The before-call hostname is measured after the operation endpoint rules run;
looking only at client.meta.endpoint_url would show identical base names and
miss the divergence. **That was the limitation in the earlier preparation
check.** This comparison corrects that check; it does not change the running
helper or claim a verified recovery.

The installed service model marks CreateOAuth2Token as IsControlPlane=false.
With no custom Endpoint, the commercial us-east-2 operation rule selects
us-east-2.signin.aws.amazon.com. The explicit Endpoint branch instead returns
the override, so the helper forces signin.us-east-2.amazonaws.com. The source
rule and the captured before-call results explain the **current selection
mechanism**, not why a historical connection timed out. No assertion is made
that the forced hostname is universally invalid or that the native one is
currently reachable. The prior diagnostic contacted only the forced hostname;
it supplies no connectivity evidence for the native operation hostname.

Certificate requirement, CA bundle SHA-256
`9a9c1b798f6ae057ecb84545704e38c89e10f3db5519b24cd4647a1bb2dbf299`,
TLS limits/options and cipher configuration match in all three current clients.
Their SSLContext.check_hostname property is false, but hostname assertions are
not disabled: inspected urllib3 code performs hostname matching when context
checking is delegated and CERT_REQUIRED applies. This is configuration/source
evidence only, not peer certificate or handshake verification. The expected
SNI/hostname-check target changes with the operation authority. No proxy is
effective in this environment; inheritance remains a difference if settings
change. Historical effective proxy/TLS snapshots were not retained.

## Existing login-session metadata and refresh behavior

Read-only metadata inspection found default profile region **us-east-2**, a
configured login_session with no ARN region component, required cache fields
present and **no separate region metadata** in the cache/access-token structure.
There is no default endpoint override or services section, nor any inspected
region/profile/config-file/cache-directory/proxy/endpoint/CA environment
override. Credential/session values, cache filenames/hashes, tokens and keys are
not output. Cache bytes were unchanged during the metadata read; no comparison
path writes the cache.

Installed default LoginProvider passes its Session.create_client to
LoginCredentialFetcher. That fetcher creates a Sign-In client without a region
or endpoint override, so session/profile resolution applies; cache loading does
not supply a different region. The custom factory forces us-east-2 and its
literal endpoint while preserving the same SDK fetcher. Current region agrees;
a separate hidden cache region was not found.

Source inspection confirms these refresh behaviors without executing them:

- Default LoginProvider loads cached credentials and returns
  RefreshableCredentials. Frozen-credential access can trigger refresh; installed
  defaults have 900-second advisory and 600-second mandatory windows.
- Both installed fetchers skip the token exchange when cached access validity is
  greater than 300 seconds; otherwise the shared existing-session refresh-token
  operation/DPoP flow is used. The DPoP handler consumes the already-resolved URI
  to create a header; it does not choose another hostname or region. No real
  DPoP key/header or token was used by this comparison.
- The helper invokes that fetcher directly, imposes its one-attempt transport
  policy and shared deadline, requires at least 120 seconds validity afterward,
  checks the same account/identity, and requires STS verification before provider
  work. Those checks were not executed in this task.
- The default credential chain can select earlier providers, whereas the helper
  only selects the existing login cache. This comparison intentionally blocks
  Session.get_credentials, so it does not assert which provider would be selected
  live now. The successful historical ledger does record a token attempt.
- The SDK loader supports AWS_LOGIN_CACHE_DIRECTORY; the helper fixes the usual
  home cache path. No directory override is set now. Both use the session-derived
  SHA-256 cache key. SDK cache persistence and the helper's locked/fsynced existing
  file writer differ; neither writer ran here.
- Native SDK, CLI and helper retry/timeouts differ as shown above. These are
  configured/source facts; no retry or timing behavior was exercised against a
  service. The historical successful tool reported 0.373 seconds, so this finding
  does not justify increasing a timeout or restoring automatic retries.

The current installed rules/models are captured with hashes in the
[machine comparison](project-environments-auth-path-comparison.json). Actual
historical refresh hostname/address and all historical effective configuration
were not captured. The current reconstruction must not be substituted for those
missing records. **Both failed remote refresh/credential outcomes stay uncertain.**

## Offline method and verification

[Reproducer](../../scripts/preview-meter/compare_auth_paths.py) and
[regressions](../../scripts/preview-meter/test_compare_auth_paths.py) run with
socket connects/sends/DNS functions, SDK send and credential providers blocked.
The SDK constructs a synthetic operation then stops at before-call, before
transport. The whole run also uses a namespace without external routes. No
actual refresh, credential-chain call, request/response body, URL, header or
credential value is published. Only allowlisted hostnames, metadata, public
CA/cipher/source hashes and bounded config values are emitted. Child SDK errors
are suppressed rather than relayed with possible request material.

Reproduce without network access (requires the already-installed SDK/CLI and
bubblewrap; no dependency installation):

```sh
bwrap --unshare-net --ro-bind / / --bind /home/king /home/king --bind /tmp /tmp --proc /proc --dev /dev python3 -B scripts/preview-meter/test_compare_auth_paths.py
```

The namespace and blocked SDK/socket boundaries prevent a transport even if
endpoint resolution unexpectedly changes. This command runs synthetic resolution
only; it cannot authorize or initiate refresh/preview.

Three comparison/disclosure regressions and full repository format/lint/tests/
build/Go checks passed offline. Repository downloads were disabled. Final
source, retained receipt/hash and formatting checks pass before publication.
No existing authentication, endpoint, network, cache or policy source/config
was edited; only comparison/review artifacts are added.

## Smallest supported next action — review only

**Review the authentication operation-endpoint divergence and an authentication-only
alignment proposal before any further live work.** The candidate, not applied,
is to let the SDK resolve CreateOAuth2Token rather than force the client base
endpoint, and to review the corresponding exact authentication hostname
allowlist plus a before-call operation-host regression. Preserve region us-east-2,
TLS verification, explicit no-proxy/current one-attempt policy, existing-session
identity restrictions, counters/deadline and the byte-identical SST read-only
policy. Do not substitute the unmetered default credential chain or widen retries.

This is a narrowly scoped contract/configuration review, **not a proven fix**.
Any endpoint/allowlist edit needs separate authorization; none was made here.
Any later credential-free comparison probe of the native hostname also needs a
new explicit destination/counter scope; the previous probe authorization covered
only the forced hostname. No probe or refresh is proposed for automatic execution.

Actual usage remains **168/500**, zero added attempts, 332 arithmetic headroom.
The parent checker remains the frozen 167 baseline; the preserved diagnostic
receipt accounts for the additional charged connection. All inventory/preview/
refresh/diagnostic ledgers, failed 165/166 claims and diagnostic 167 claim remain
unchanged; no new 168 claim/window exists. Same cumulative **$1 procedural
allowance**, billing and unused dollar balance unmeasured.

Release source remains `44f1fb79d943501c95e06fcc1a6114111a7c1ea4`. Release and
rollback bytes, original manifests and endpoint policy remain unchanged. No
packaging/resource diff was generated. Deployment, stage closure, migration,
activation, identity/project creation, live targets, refresh, preview, external
probes and repairs remain held. OpenAPI production scope remains separate.
