# Separate S3 Control endpoint proposal — not applied

Review-6 captured and privately reviewed the exact destination
`747336059622.s3-control.us-east-2.amazonaws.com`. The
[execution result](project-environments-refresh-preview-review-6.md) distinguishes
that observed host from the unobserved HTTP operation. The pinned provider's S3
tag-reading helper constructs an S3 Control client. This is a supported provider
endpoint, but no particular read API or its parameters were observed before the
CONNECT hold.

**This proposal grants no S3 Control AWS operation.** It is the smallest
endpoint-scoping change supported by the current evidence. It would allow the
local proxy to recognize this exact CONNECT target, then hold every S3 Control
HTTP request before admission/forwarding. It will not by itself produce a
completed preview. No endpoint or HTTP rule below has been applied.

Proposed policy delta, for review only:

```python
# Inside service(), after the existing lowercase conversion:
if host == "747336059622.s3-control.us-east-2.amazonaws.com":
    return "s3control"

# Inside classify(), immediately after svc = service(host):
if svc == "s3control":
    raise Stop("s3control-operation-held")
```

Scope and limits:

- CONNECT remains port 443 only; request processing remains HTTPS/443 only.
  The host is exact: no other account, region, base host, alias, trailing dot,
  dual-stack variant, suffix match or wildcard.
- Do **not** add s3control to REST, JSON_READS or QUERY_READS. GET/HEAD are not
  generally safe permission to read an entire service; all methods remain denied.
  No raw headers, body, path or query are authorized for capture by this proposal.
- The reviewed proxy uses lazy upstream connection creation and upstream_cert=false.
  The endpoint-only step is local proxy CONNECT/TLS processing. The explicit HTTP
  hold runs before budget admission or upstream send. This must be demonstrated
  with offline tests before any later executable amendment is approved.
- No operation is chosen from a dependency's list of supported methods. In
  particular, an SDK model containing GetPublicAccessBlock or tagging APIs does
  not prove either was requested by this CONNECT. A source-linked or separately
  reviewed, bounded operation-level observation is needed before proposing any
  actual AWS read allowance. That diagnostic scope is not defined or executed here.
- Keep the existing public hostname sanitizer and all authentication, identity,
  shared-deadline, accounting, failure and descendant-stop controls intact.

Required next decisions are whether to accept this endpoint-only scope and how
to establish the necessary read operation without publishing sensitive request
material. Implementation, offline acceptance tests, 231-based accounting and any
new live window require separate authorization/review. Current arithmetic
headroom is 269 calls, with the same cumulative $1 procedural allowance and
unmeasured billing. No future execution command is authorized by this document.

The current policy remains byte-identical at SHA-256
`ed237fbf3fe24ccd06e5ed08a73df641499e5c19c9342c56f14e93198764a560`.
All deployment/runtime holds remain.
