"""Read-only collector used for this review. Execution requires a newly authorized window.
The retained campaign ledger is stopped; this script cannot resume it.
"""

import base64
import datetime
import hashlib
import importlib.util
import io
import json
import pathlib
import urllib.parse
import urllib.request
import zipfile

import boto3
from botocore.config import Config

ROOT = pathlib.Path(__file__).resolve().parents[1]
DEST = (
    ROOT
    / ".build/project-environments-release/44f1fb79d943501c95e06fcc1a6114111a7c1ea4"
)
DEST.mkdir(parents=True, exist_ok=True)
spec = importlib.util.spec_from_file_location(
    "meter", ROOT / "scripts/environment-inventory-meter.py"
)
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)
meter = m.Meter(DEST / "inventory-attempts.jsonl", resume=True)
meter.install(allow_refresh=False)
session = boto3.Session(region_name="us-east-2")
clients = {}


def call(service, op, **args):
    if service not in clients:
        clients[service] = session.client(
            service,
            config=Config(
                retries={"total_max_attempts": 2, "mode": "standard"},
                connect_timeout=10,
                read_timeout=30,
            ),
        )
    return getattr(clients[service], op)(**args)


def clean(v):
    if isinstance(v, dict):
        return {k: clean(x) for k, x in v.items() if k != "ResponseMetadata"}
    if isinstance(v, list):
        return [clean(x) for x in v]
    if isinstance(v, datetime.datetime):
        return v.isoformat()
    return v


def sha(v):
    return hashlib.sha256(
        v
        if isinstance(v, bytes)
        else json.dumps(clean(v), sort_keys=True, separators=(",", ":")).encode()
    ).hexdigest()


def save(name, v):
    (DEST / name).write_text(json.dumps(clean(v), indent=2) + "\n")


def pages(service, op, result, **args):
    rows = []
    for _ in range(100):
        r = call(service, op, **args)
        rows += r.get(result, [])
        token = (
            r.get("NextToken")
            or r.get("NextMarker")
            or (r.get("Marker") if r.get("IsTruncated") else None)
        )
        if not token:
            return rows
        args[
            "Marker"
            if "IsTruncated" in r
            else "NextToken"
            if "NextToken" in r
            else "Marker"
        ] = token
    raise RuntimeError("pagination-bound")


def s3bytes(bucket, key):
    r = call("s3", "get_object", Bucket=bucket, Key=key)
    data = r["Body"].read(64 * 1024 * 1024 + 1)
    r["Body"].close()
    if len(data) > 64 * 1024 * 1024:
        raise RuntimeError("download-bound")
    return data


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, *a, **k):
        return None


report = {
    "checkedAt": datetime.datetime.now(datetime.UTC).isoformat(),
    "releaseSourceCommit": "44f1fb79d943501c95e06fcc1a6114111a7c1ea4",
    "previewPerformed": False,
    "refreshCompletion": {
        "credentialsReady": True,
        "physicalAttempts": 1,
        "httpStatusRetained": False,
    },
}
try:
    out = json.loads((ROOT / ".sst/outputs.json").read_text())
    identity = call("sts", "get_caller_identity")
    if identity["Account"] != "747336059622" or identity["Arn"].endswith(":root"):
        raise RuntimeError("identity-mismatch")
    report["identity"] = clean(identity)
    stagekey = {"PK": {"S": "STAGE#dev-api"}, "SK": {"S": "META"}}

    def stage():
        return call(
            "dynamodb",
            "get_item",
            TableName=out["controlTable"],
            Key=stagekey,
            ConsistentRead=True,
        ).get("Item", {})

    report["stageBefore"] = stage()
    report["accountSettings"] = clean(call("lambda", "get_account_settings"))
    report["plan"] = clean(call("freetier", "get_account_plan_state"))
    bootstrap = call("ssm", "get_parameter", Name="/sst/bootstrap")["Parameter"]
    report["bootstrap"] = {
        "value": json.loads(bootstrap["Value"]),
        "version": bootstrap["Version"],
    }
    report["passphraseMetadata"] = clean(
        call(
            "ssm",
            "describe_parameters",
            ParameterFilters=[
                {
                    "Key": "Name",
                    "Option": "Equals",
                    "Values": ["/sst/passphrase/kurier/dev-api"],
                }
            ],
        )
    )
    report["tables"] = {}
    projection = [
        "PK",
        "SK",
        "kind",
        "schemaVersion",
        "state",
        "version",
        "revision",
        "projectId",
        "ownerId",
        "jobId",
        "executionId",
        "runId",
        "position",
        "writeSettled",
        "bytes",
        "recoveryGeneration",
        "activeJob",
        "count",
        "month",
        "pinnedCount",
        "pinnedBytes",
        "inflightCount",
        "storageReservedBytes",
        "deletionEpoch",
        "assertionSemanticsVersion",
        "templateSemanticsVersion",
        "scalarType",
    ]
    for table in [out["controlTable"], out["protectedTable"]]:
        meta = []
        cursor = None
        for _ in range(100):
            args = {
                "TableName": table,
                "ConsistentRead": True,
                "Limit": 100,
                "ProjectionExpression": ",".join(
                    "#p" + str(i) for i in range(len(projection))
                ),
                "ExpressionAttributeNames": {
                    "#p" + str(i): p for i, p in enumerate(projection)
                },
            }
            if cursor:
                args["ExclusiveStartKey"] = cursor
            r = call("dynamodb", "scan", **args)
            meta += r.get("Items", [])
            cursor = r.get("LastEvaluatedKey")
            if not cursor:
                break
        if cursor:
            raise RuntimeError("scan-bound")
        report["tables"][table] = {
            "description": clean(call("dynamodb", "describe_table", TableName=table)),
            "ttl": clean(call("dynamodb", "describe_time_to_live", TableName=table)),
            "backup": clean(
                call("dynamodb", "describe_continuous_backups", TableName=table)
            ),
            "metadata": meta,
        }
    report["functions"] = []
    old = json.loads(
        (ROOT / "docs/development/workflow-assertions-release.json").read_text()
    )
    for logical, key in [
        ("UsersProjectsApi", "apiFunction"),
        ("EmptyProjectCleanup", "cleanupFunction"),
        ("CloudExecutionWorker", "workerFunction"),
        ("ExecutionControlledEndpoint", "controlledEndpointFunction"),
    ]:
        fn = call("lambda", "get_function", FunctionName=out[key])
        cfg = fn["Configuration"]
        env = cfg.get("Environment", {}).get("Variables", {})
        url = fn["Code"]["Location"]
        parsed = urllib.parse.urlsplit(url)
        if (
            parsed.scheme != "https"
            or not parsed.hostname.endswith(".amazonaws.com")
            or "s3" not in parsed.hostname
        ):
            raise RuntimeError("code-download-origin")
        attempt = meter.admit("s3-presigned", "GetLambdaCodeObject")
        try:
            with urllib.request.build_opener(NoRedirect).open(
                url, timeout=30
            ) as response:
                data = response.read(64 * 1024 * 1024 + 1)
                meter.record(event="outcome", attempt=attempt, status=response.status)
        except Exception:
            meter.record(event="outcome", attempt=attempt, transportError=True)
            raise
        if (
            len(data) > 64 * 1024 * 1024
            or base64.b64encode(hashlib.sha256(data).digest()).decode()
            != cfg["CodeSha256"]
        ):
            raise RuntimeError("lambda-code-digest-mismatch")
        dest = DEST / "rollback" / logical
        dest.mkdir(parents=True, exist_ok=True)
        (dest / "code.zip").write_bytes(data)
        z = zipfile.ZipFile(io.BytesIO(data))
        boot = z.read("bootstrap")
        (dest / "bootstrap").write_bytes(boot)
        role = cfg["Role"].split("/")[-1]
        trust = call("iam", "get_role", RoleName=role)["Role"]
        inline = []
        managed = []
        for name in pages("iam", "list_role_policies", "PolicyNames", RoleName=role):
            inline.append(
                clean(call("iam", "get_role_policy", RoleName=role, PolicyName=name))
            )
        for p in pages(
            "iam", "list_attached_role_policies", "AttachedPolicies", RoleName=role
        ):
            policy = call("iam", "get_policy", PolicyArn=p["PolicyArn"])["Policy"]
            version = call(
                "iam",
                "get_policy_version",
                PolicyArn=p["PolicyArn"],
                VersionId=policy["DefaultVersionId"],
            )["PolicyVersion"]
            managed.append({"policy": clean(policy), "version": clean(version)})
        config = {
            k: clean(v)
            for k, v in cfg.items()
            if k not in ["Environment", "ResponseMetadata"]
        }
        expected = next(p for p in old["preview"]["packages"] if p["name"] == logical)[
            "sha256"
        ]
        item = {
            "logical": logical,
            "configuration": config,
            "environmentSha256": sha(env),
            "capabilityEnvironment": {
                k: v for k, v in env.items() if k.endswith("SCHEMA_VERSION")
            },
            "publicConfigurationMatches": env.get(
                "KURIER_COGNITO_CLIENT_ID", out["cognitoClientId"]
            )
            == out["cognitoClientId"]
            and env.get("KURIER_COGNITO_ISSUER", out["cognitoIssuer"])
            == out["cognitoIssuer"],
            "rollback": {
                "zipBytes": len(data),
                "zipSha256": sha(data),
                "bootstrapBytes": len(boot),
                "bootstrapSha256": sha(boot),
                "matchesRetainedAssertionsRelease": sha(data) == expected,
            },
            "role": clean(trust),
            "inlinePolicies": inline,
            "managedPolicies": managed,
            "concurrency": clean(
                call("lambda", "get_function_concurrency", FunctionName=out[key])
            ),
            "provisioned": clean(
                call(
                    "lambda",
                    "list_provisioned_concurrency_configs",
                    FunctionName=out[key],
                )
            ),
            "invokePolicy": clean(call("lambda", "get_policy", FunctionName=out[key]))
            if logical
            in [
                "UsersProjectsApi",
                "EmptyProjectCleanup",
                "ExecutionControlledEndpoint",
            ]
            else None,
        }
        report["functions"].append(item)
        save("inventory.json", report)
    report["workerMappings"] = pages(
        "lambda",
        "list_event_source_mappings",
        "EventSourceMappings",
        FunctionName=out["workerFunction"],
    )
    report["queues"] = {
        q: clean(
            call("sqs", "get_queue_attributes", QueueUrl=out[q], AttributeNames=["All"])
        )
        for q in ["executionQueueUrl", "executionDlqUrl"]
    }
    report["schedule"] = clean(
        call("events", "describe_rule", Name=out["cleanupSchedule"])
    )
    report["scheduleTargets"] = clean(
        call("events", "list_targets_by_rule", Rule=out["cleanupSchedule"])
    )
    api = urllib.parse.urlsplit(out["apiUrl"]).hostname.split(".")[0]
    report["gateway"] = {
        "api": clean(call("apigatewayv2", "get_api", ApiId=api)),
        "stage": clean(
            call("apigatewayv2", "get_stage", ApiId=api, StageName="$default")
        ),
        "routes": pages("apigatewayv2", "get_routes", "Items", ApiId=api),
        "integrations": pages("apigatewayv2", "get_integrations", "Items", ApiId=api),
    }
    pool = call(
        "cognito-idp", "describe_user_pool", UserPoolId=out["cognitoUserPoolId"]
    )["UserPool"]
    client = call(
        "cognito-idp",
        "describe_user_pool_client",
        UserPoolId=out["cognitoUserPoolId"],
        ClientId=out["cognitoClientId"],
    )["UserPoolClient"]
    if client.get("ClientSecret"):
        raise RuntimeError("unexpected-client-secret")
    report["cognito"] = {
        "pool": {
            k: clean(pool[k])
            for k in [
                "Id",
                "UserPoolTier",
                "EmailConfiguration",
                "Policies",
                "DeletionProtection",
            ]
            if k in pool
        },
        "client": clean(client),
    }
    report["kms"] = {
        "key": clean(call("kms", "describe_key", KeyId=out["protectedKeyArn"])),
        "policy": clean(
            call(
                "kms",
                "get_key_policy",
                KeyId=out["protectedKeyArn"],
                PolicyName="default",
            )
        ),
        "rotation": clean(
            call("kms", "get_key_rotation_status", KeyId=out["protectedKeyArn"])
        ),
    }
    cf = call(
        "cloudfront", "get_distribution", Id=out["frontendHosting"]["distributionId"]
    )
    report["cloudfront"] = clean(cf)
    webbucket = out["frontendHosting"]["webBucket"]
    report["buckets"] = {}
    for bucket in [
        webbucket,
        out["evidenceBucket"],
        report["bootstrap"]["value"]["state"],
        report["bootstrap"]["value"]["asset"],
    ]:
        report["buckets"][bucket] = {
            "location": clean(call("s3", "get_bucket_location", Bucket=bucket)),
            "publicAccess": clean(call("s3", "get_public_access_block", Bucket=bucket)),
            "versioning": clean(call("s3", "get_bucket_versioning", Bucket=bucket)),
            "encryption": clean(call("s3", "get_bucket_encryption", Bucket=bucket)),
        }
    manifest = s3bytes(webbucket, "artifact.json")
    current = json.loads(manifest)
    front = DEST / "rollback/frontend"
    front.mkdir(parents=True, exist_ok=True)
    (front / "artifact.json").write_bytes(manifest)
    report["frontendRollback"] = {
        "manifest": current,
        "manifestSha256": sha(manifest),
        "files": [],
    }
    for f in current["files"]:
        data = s3bytes(webbucket, f["path"])
        archivekey = (
            f["path"]
            if f["path"].startswith("assets/")
            else "_releases/" + current["releaseId"] + "/" + f["path"]
        )
        archived = s3bytes(webbucket, archivekey) if archivekey != f["path"] else data
        if sha(data) != f["sha256"] or len(data) != f["bytes"] or data != archived:
            raise RuntimeError("frontend-rollback-mismatch")
        file = front / "app" / f["path"]
        file.parent.mkdir(parents=True, exist_ok=True)
        file.write_bytes(data)
        report["frontendRollback"]["files"].append(
            {
                "path": f["path"],
                "bytes": len(data),
                "sha256": sha(data),
                "archiveVerified": True,
            }
        )
    arc = s3bytes(webbucket, "_releases/" + current["releaseId"] + "/artifact.json")
    if arc != manifest:
        raise RuntimeError("archive-manifest-mismatch")
    report["frontendRollback"]["archiveManifestVerified"] = True
    # List metadata only, with explicit pagination, never download evidence/state values.
    report["objectMetadata"] = {}
    for bucket, prefix in [
        (out["evidenceBucket"], "dev-api/projects/"),
        (report["bootstrap"]["value"]["state"], ""),
    ]:
        items = []
        token = None
        for _ in range(100):
            args = {"Bucket": bucket, "Prefix": prefix, "MaxKeys": 1000}
            if token:
                args["ContinuationToken"] = token
            r = call("s3", "list_objects_v2", **args)
            items += clean(r.get("Contents", []))
            token = r.get("NextContinuationToken")
            if not token:
                break
        if token:
            raise RuntimeError("s3-list-bound")
        report["objectMetadata"][bucket] = {
            "count": len(items),
            "sha256": sha(items),
            "keys": [x["Key"] for x in items],
        }
    report["stageAfter"] = stage()
    report["stageUnchanged"] = report["stageBefore"] == report["stageAfter"]
    report["physicalAttempts"] = meter.count
    save("inventory.json", report)
    print(
        json.dumps(
            {
                "inventoryComplete": True,
                "physicalAttempts": meter.count,
                "stageUnchanged": report["stageUnchanged"],
            }
        )
    )
except Exception as error:  # noqa: BLE001 - fixed diagnostic; never print SDK request material
    report["inventoryComplete"] = False
    report["stopType"] = type(error).__name__
    report["physicalAttempts"] = meter.count
    save("inventory.json", report)
    print(
        json.dumps(
            {
                "inventoryComplete": False,
                "stopType": type(error).__name__,
                "physicalAttempts": meter.count,
            }
        )
    )
    raise SystemExit(1)
finally:
    meter.finish()
