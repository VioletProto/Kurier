"""One source-established bucket tag read; never a generic S3 Control allowance."""

import re
from urllib.parse import quote

ACCOUNT = "747336059622"
REGION = "us-east-2"
HOST = ACCOUNT + ".s3-control." + REGION + ".amazonaws.com"
BUCKET = "kurier-dev-api-web-fc2927a49a0b246240e5a17774"
# The pinned provider Bucket branch leaves ARN region/account empty.
ARN = "arn:aws:s3:::" + BUCKET
PATH = "/v20180820/tags/" + quote(ARN, safe="/")
EMPTY_SHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
REQUIRED = {"authorization", "x-amz-account-id", "x-amz-date", "x-amz-security-token"}
ALLOWED = REQUIRED | {
    "host",
    "user-agent",
    "x-amz-user-agent",
    "amz-sdk-invocation-id",
    "amz-sdk-request",
    "x-amz-content-sha256",
    "x-amz-api-version",
    "content-length",
    "content-type",
    "accept",
    "accept-encoding",
    "connection",
}


def normalized_headers(headers):
    fields = headers.fields if hasattr(headers, "fields") else headers.items()
    result = {}
    for index, (key, value) in enumerate(fields):
        if index >= 32:
            return None
        key = key.decode("ascii") if isinstance(key, bytes) else key
        value = value.decode("ascii") if isinstance(value, bytes) else value
        if not isinstance(key, str) or not isinstance(value, str):
            return None
        key = key.lower()
        if key not in ALLOWED or key in result or len(value) > 16384:
            return None
        if not value.isascii() or any(ord(c) < 32 or ord(c) == 127 for c in value):
            return None
        result[key] = value
    return result


def request_metadata_valid(value):
    seen = set()
    for part in value.split("; "):
        key, separator, item = part.partition("=")
        if separator != "=" or key in seen:
            return False
        seen.add(key)
        if key in {"attempt", "max"}:
            if not re.fullmatch(r"[1-9][0-9]{0,2}", item):
                return False
        elif key == "ttl":
            if not re.fullmatch(r"[0-9]{8}T[0-9]{6}Z", item):
                return False
        else:
            return False
    return "attempt" in seen


def target_allowed(method, host, target):
    return (
        method == "GET"
        and isinstance(host, str)
        and host.lower() == HOST
        and target == PATH
    )


def allowed(method, host, target, headers, body):
    # Exact wire path comparison: no decoding/normalization, aliases, fragments,
    # query parameters, objects, other ARN spellings or double encoding.
    if not target_allowed(method, host, target) or body != b"":
        return False
    try:
        h = normalized_headers(headers)
        if h is None or not REQUIRED <= h.keys() or h["x-amz-account-id"] != ACCOUNT:
            return False
        if "host" in h and h["host"].lower() != HOST:
            return False
        if (
            not re.fullmatch(r"[0-9]{8}T[0-9]{6}Z", h["x-amz-date"])
            or not h["x-amz-security-token"]
        ):
            return False
        auth = re.fullmatch(
            r"AWS4-HMAC-SHA256 Credential=[A-Z0-9]{20}/([0-9]{8})/us-east-2/s3/aws4_request, "
            r"SignedHeaders=([a-z0-9;-]{1,1024}), Signature=[0-9a-f]{64}",
            h["authorization"],
        )
        if not auth or auth[1] != h["x-amz-date"][:8]:
            return False
        signed = auth[2].split(";")
        required_signed = {
            "host",
            "x-amz-account-id",
            "x-amz-date",
            "x-amz-security-token",
        }
        if signed != sorted(set(signed)) or not required_signed <= set(signed):
            return False
        if not set(signed) <= (h.keys() | {"host"}) - {"authorization", "user-agent"}:
            return False
        if "content-length" in h and h["content-length"] != "0":
            return False
        if "content-type" in h and h["content-type"] not in {
            "application/xml",
            "application/xml; charset=utf-8",
        }:
            return False
        if "x-amz-content-sha256" in h and h["x-amz-content-sha256"] not in {
            EMPTY_SHA256,
            "UNSIGNED-PAYLOAD",
        }:
            return False
        if "x-amz-api-version" in h and h["x-amz-api-version"] != "2018-08-20":
            return False
        if "accept" in h and h["accept"] not in {"application/xml", "*/*"}:
            return False
        if "accept-encoding" in h and h["accept-encoding"].lower() not in {
            "gzip",
            "identity",
        }:
            return False
        if "connection" in h and h["connection"].lower() not in {"close", "keep-alive"}:
            return False
        if "amz-sdk-invocation-id" in h and not re.fullmatch(
            r"[0-9a-fA-F]{8}(?:-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}",
            h["amz-sdk-invocation-id"],
        ):
            return False
        return "amz-sdk-request" not in h or request_metadata_valid(
            h["amz-sdk-request"]
        )
    except Exception:  # noqa: BLE001 - reject malformed objects without leaking values
        return False
