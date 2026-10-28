"""Bounded endpoint metadata, independent of endpoint authorization.

Unknown labels may contain credentials. Emit only reviewed static names; do not
truncate, hash or log arbitrary authority text, userinfo or URL components.
"""

from policy import BUCKETS, JSON_READS, QUERY_READS, REST

# This catalogue controls diagnostic disclosure ONLY, never service()/classify().
# Extra conventional endpoint names help explain a denial without permitting it.
NAMES = (
    set(JSON_READS)
    | set(QUERY_READS)
    | REST
    | {"acm", "route53", "monitoring", "cognito-identity", "resource-explorer-2"}
)
KNOWN_HOSTS = {f"{name}.amazonaws.com" for name in NAMES}
KNOWN_HOSTS |= {
    f"{name}.{region}.{suffix}"
    for name in NAMES
    for region in ["us-east-1", "us-east-2", "us-west-1", "us-west-2"]
    for suffix in ["amazonaws.com", "api.aws"]
}
KNOWN_HOSTS |= {
    f"{bucket}.s3{region}.amazonaws.com"
    for bucket in BUCKETS
    for region in ["", ".us-east-2"]
}
KNOWN_HOSTS |= {
    "s3.amazonaws.com",
    "s3.us-east-2.amazonaws.com",
    "cloudfront-kvs.global.api.aws",
    "747336059622.cloudfront-kvs.global.api.aws",
    "cloudfront-keyvaluestore.global.api.aws",
    "api.ecr.us-east-2.amazonaws.com",
    "8dnymkcoa0.execute-api.us-east-2.amazonaws.com",
    "d2kpq905qyh9zg.cloudfront.net",
    "registry.npmjs.org",
    "api.pulumi.com",
}
VERSIONS = {"HTTP/1.0", "HTTP/1.1", "HTTP/2.0", "HTTP/3"}
METHODS = {"CONNECT", "GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}


def endpoint_metadata(host, port, scheme, version, method, phase):
    hostname = "<redacted-host>"
    trailing_dot = False
    if isinstance(host, str) and 0 < len(host) <= 253 and host.isascii():
        candidate = host.lower()
        if candidate.endswith("."):
            trailing_dot = True
            candidate = candidate[:-1]
        if candidate in KNOWN_HOSTS:
            # Select a catalogue entry; never retain arbitrary input fragments.
            hostname = next(h for h in KNOWN_HOSTS if h == candidate)
    return {
        "hostname": hostname,
        "hostnameStatus": "known-static"
        if hostname != "<redacted-host>"
        else "redacted",
        "trailingDot": trailing_dot,
        "port": port if type(port) is int and 1 <= port <= 65535 else None,
        "scheme": scheme
        if isinstance(scheme, str) and scheme in {"http", "https"}
        else "unspecified",
        "httpVersion": version
        if isinstance(version, str) and version in VERSIONS
        else "other",
        "method": method if isinstance(method, str) and method in METHODS else "OTHER",
        "phase": phase
        if isinstance(phase, str) and phase in {"connect", "request"}
        else "other",
    }
