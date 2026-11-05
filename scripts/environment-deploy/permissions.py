"""Check required exact reads without forwarding them."""

import json
from urllib.parse import quote

from artifacts import load_objects
from common import ROOT, Held
from wire import BUCKET_CONFIGURATION_READS, Policy


def pending_reads(scope, release):
    policy = Policy(scope, release, load_objects(scope, release), phase="collect")
    old = json.loads(
        (ROOT / "docs/development/project-environments-inventory.json").read_bytes()
    )["frontendRollback"]["manifest"]
    result = []
    host = scope["webBucket"] + ".s3.us-east-2.amazonaws.com"
    # Preflight the required operations against the separately pinned grants.
    for operation, key, target in [
        ("GetBucketPolicy", "", "/?policy"),
        ("GetBucketAcl", "", "/?acl"),
        *[
            (operation, "", "/?" + selector)
            for operation, selector in BUCKET_CONFIGURATION_READS
        ],
        *[
            ("GetObject", f["path"], "/" + quote(f["path"], safe="/-_.~"))
            for f in old["files"]
        ],
    ]:
        try:
            policy.request("GET", host, target, {}, b"")
        except Held:
            result.append(
                {
                    "service": "s3",
                    "operation": operation,
                    "bucket": scope["webBucket"],
                    "key": key,
                }
            )
    for entry in old["files"]:
        if not entry["path"].startswith("assets/"):
            continue
        key = scope["namespace"] + ":/" + entry["path"]
        if not policy.read(
            "cloudfront-keyvaluestore",
            "GetKey",
            {"KvsARN": scope["kvsArn"], "Key": key},
        ):
            result.append(
                {
                    "service": "cloudfront-keyvaluestore",
                    "operation": "GetKey",
                    "kvsArn": scope["kvsArn"],
                    "key": key,
                }
            )
    return result
