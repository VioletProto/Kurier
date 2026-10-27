"""Read-only collector used for this review. Execution requires a newly authorized window.
The retained campaign ledger is stopped; this script cannot resume it.
"""

import datetime
import hashlib
import importlib.util
import json
import pathlib

import boto3
from botocore.config import Config

R = pathlib.Path(__file__).resolve().parents[1]
D = R / ".build/project-environments-release/44f1fb79d943501c95e06fcc1a6114111a7c1ea4"
s = importlib.util.spec_from_file_location(
    "m", R / "scripts/environment-inventory-meter.py"
)
m = importlib.util.module_from_spec(s)
s.loader.exec_module(m)
meter = m.Meter(D / "inventory-attempts.jsonl", resume=True)
meter.install()
session = boto3.Session(region_name="us-east-2")
cs = {}
r = json.loads((D / "inventory.json").read_text())
o = json.loads((R / ".sst/outputs.json").read_text())


def call(s, op, **kw):
    if s not in cs:
        cs[s] = session.client(
            s,
            config=Config(
                retries={"total_max_attempts": 2, "mode": "standard"},
                connect_timeout=10,
                read_timeout=30,
            ),
        )
    v = getattr(cs[s], op)(**kw)
    v.pop("ResponseMetadata", None)
    return v


def clean(v):
    if isinstance(v, dict):
        return {k: clean(x) for k, x in v.items()}
    if isinstance(v, list):
        return [clean(x) for x in v]
    if isinstance(v, datetime.datetime):
        return v.isoformat()
    return v


def sha(v):
    return hashlib.sha256(
        json.dumps(clean(v), sort_keys=True, separators=(",", ":")).encode()
    ).hexdigest()


try:
    r["bucketPolicies"] = {
        b: call("s3", "get_bucket_policy", Bucket=b)
        for b in [o["frontendHosting"]["webBucket"], o["evidenceBucket"]]
    }
    cf = r["cloudfront"]["Distribution"]["DistributionConfig"]
    name = cf["DefaultCacheBehavior"]["FunctionAssociations"]["Items"][0][
        "FunctionARN"
    ].split("/")[-1]
    desc = call("cloudfront", "describe_function", Name=name, Stage="LIVE")
    code = call("cloudfront", "get_function", Name=name, Stage="LIVE")
    data = code["FunctionCode"].read()
    code["FunctionCode"].close()
    f = D / "rollback/frontend/cloudfront-function.js"
    f.write_bytes(data)
    r["cloudfrontFunction"] = {
        "description": desc,
        "codeSha256": hashlib.sha256(data).hexdigest(),
        "bytes": len(data),
    }
    kvs = (
        desc["FunctionSummary"]["FunctionConfig"]
        .get("KeyValueStoreAssociations", {})
        .get("Items", [])
    )
    r["keyValueStores"] = []
    for store in kvs:
        arn = store["KeyValueStoreARN"]
        description = call(
            "cloudfront-keyvaluestore", "describe_key_value_store", KvsARN=arn
        )
        items = []
        token = None
        for _ in range(20):
            kw = {"KvsARN": arn, "MaxResults": 50}
            if token:
                kw["NextToken"] = token
            page = call("cloudfront-keyvaluestore", "list_keys", **kw)
            items += page.get("Items", [])
            token = page.get("NextToken")
            if not token:
                break
        if token:
            raise RuntimeError("kvs-page-bound")
        r["keyValueStores"].append(
            {"description": description, "entries": items, "entriesSha256": sha(items)}
        )
    for policy, k in [
        ("cache", "CachePolicyId"),
        ("headers", "ResponseHeadersPolicyId"),
    ]:
        pid = cf["DefaultCacheBehavior"].get(k)
        if pid:
            r["cloudfront" + policy.title() + "Policy"] = call(
                "cloudfront",
                "get_cache_policy"
                if policy == "cache"
                else "get_response_headers_policy",
                Id=pid,
            )
    r["ecr"] = call("ecr", "describe_repositories", repositoryNames=["sst-asset"])
    r["logGroups"] = call(
        "logs",
        "describe_log_groups",
        logGroupNamePrefix="/aws/lambda/kurier-dev-api-",
        limit=50,
    )
    if r["logGroups"].get("nextToken"):
        raise RuntimeError("log-pagination-required")
    r["stageQuotaFinal"] = call(
        "dynamodb",
        "get_item",
        TableName=o["controlTable"],
        Key={"PK": {"S": "STAGE#dev-api"}, "SK": {"S": "QUOTA"}},
        ConsistentRead=True,
    ).get("Item", {})
    r["stageFinal"] = call(
        "dynamodb",
        "get_item",
        TableName=o["controlTable"],
        Key={"PK": {"S": "STAGE#dev-api"}, "SK": {"S": "META"}},
        ConsistentRead=True,
    ).get("Item", {})
    r["stageUnchanged"] = r["stageBefore"] == r["stageFinal"]
    r["iamReadback"] = []
    for f in r["functions"]:
        role = f["configuration"]["Role"].split("/")[-1]
        trust = call("iam", "get_role", RoleName=role)["Role"][
            "AssumeRolePolicyDocument"
        ]
        inline = []
        for p in f["inlinePolicies"]:
            inline.append(
                call(
                    "iam", "get_role_policy", RoleName=role, PolicyName=p["PolicyName"]
                )
            )
        attached = call("iam", "list_attached_role_policies", RoleName=role)
        if attached.get("IsTruncated"):
            raise RuntimeError("iam-pagination-required")
        expected = [
            {"PolicyName": p["policy"]["PolicyName"], "PolicyArn": p["policy"]["Arn"]}
            for p in f["managedPolicies"]
        ]
        same = (
            trust == f["role"]["AssumeRolePolicyDocument"]
            and inline == f["inlinePolicies"]
            and attached["AttachedPolicies"] == expected
        )
        for p in f["managedPolicies"]:
            current = call("iam", "get_policy", PolicyArn=p["policy"]["Arn"])["Policy"]
            same = (
                same and current["DefaultVersionId"] == p["policy"]["DefaultVersionId"]
            )
        config = call(
            "lambda",
            "get_function_configuration",
            FunctionName=f["configuration"]["FunctionName"],
        )
        samecode = config["CodeSha256"] == f["configuration"]["CodeSha256"]
        sameenv = (
            sha(config.get("Environment", {}).get("Variables", {}))
            == f["environmentSha256"]
        )
        r["iamReadback"].append(
            {
                "logical": f["logical"],
                "iamUnchanged": same,
                "codeUnchanged": samecode,
                "environmentUnchanged": sameenv,
            }
        )
    r["physicalAttempts"] = meter.count
    r["supplementComplete"] = True
    print(
        json.dumps(
            {
                "supplementComplete": True,
                "physicalAttempts": meter.count,
                "readbacks": r["iamReadback"],
            }
        )
    )
except Exception as e:
    r["supplementComplete"] = False
    r["supplementStopType"] = type(e).__name__
    print(
        json.dumps(
            {
                "supplementComplete": False,
                "type": type(e).__name__,
                "physicalAttempts": meter.count,
            }
        )
    )
    raise
finally:
    (D / "inventory.json").write_text(json.dumps(clean(r), indent=2) + "\n")
    meter.finish()
