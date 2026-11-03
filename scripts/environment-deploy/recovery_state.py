"""Fixed, read-only SST metadata inspection. No object body or credential logging."""

import datetime
import json
import sys
from pathlib import Path

import boto3
from botocore.config import Config
from botocore.exceptions import ClientError


def inspect(client, bucket, cutoff):
    cutoff = datetime.datetime.fromisoformat(cutoff)
    if cutoff.tzinfo is None:
        raise ValueError("recovery-cutoff-held")
    app = "app/kurier/dev-api.json"
    lock = "lock/kurier/dev-api.json"
    header = client.head_object(Bucket=bucket, Key=app)
    if header["LastModified"] > cutoff:
        raise ValueError("recovery-state-mutation-held")
    lock_header = None
    try:
        lock_header = client.head_object(Bucket=bucket, Key=lock)
    except ClientError as error:
        if error.response["Error"]["Code"] not in {"404", "NoSuchKey", "NotFound"}:
            raise
    else:
        obj = client.get_object(Bucket=bucket, Key=lock)
        if (
            lock_header["LastModified"] > cutoff
            or obj["LastModified"] != lock_header["LastModified"]
            or obj["ETag"] != lock_header["ETag"]
        ):
            raise ValueError("recovery-state-mutation-held")
        raw = obj["Body"].read(65537)
        obj["Body"].close()
        if len(raw) > 65536:
            raise ValueError("recovery-lock-size-held")
        value = json.loads(raw)
        if not isinstance(value, dict) or (
            value and value.get("created") != "0001-01-01T00:00:00Z"
        ):
            raise ValueError("recovery-live-deploy-lock-held")
    counts = {}
    for prefix in [
        app,
        lock,
        *[
            f"{kind}/kurier/dev-api/"
            for kind in ["snapshot", "eventlog", "summary", "update"]
        ],
    ]:
        token = None
        count = 0
        for _ in range(10):
            page = client.list_objects_v2(
                Bucket=bucket,
                Prefix=prefix,
                MaxKeys=100,
                **({"ContinuationToken": token} if token else {}),
            )
            for obj in page.get("Contents", []):
                if not obj["Key"].startswith(prefix) or (
                    prefix in {app, lock} and obj["Key"] != prefix
                ):
                    raise ValueError("recovery-state-key-held")
                if obj["LastModified"] > cutoff:
                    raise ValueError("recovery-state-mutation-held")
                if prefix == lock and (
                    lock_header is None
                    or obj["ETag"] != lock_header["ETag"]
                    or obj["LastModified"] != lock_header["LastModified"]
                ):
                    raise ValueError("recovery-state-head-list-drift")
                if prefix == app and (
                    obj["ETag"] != header["ETag"]
                    or obj["Size"] != header["ContentLength"]
                    or obj["LastModified"] != header["LastModified"]
                ):
                    raise ValueError("recovery-state-head-list-drift")
                count += 1
            if not page.get("IsTruncated"):
                break
            token = page.get("NextContinuationToken")
            if not token:
                raise ValueError("recovery-state-pagination-held")
        else:
            raise ValueError("recovery-state-pagination-held")
        counts[prefix] = count
    if counts[app] != 1:
        raise ValueError("recovery-state-app-held")
    if counts[lock] != (1 if lock_header else 0):
        raise ValueError("recovery-state-lock-held")
    return {
        "noCurrentDeployLock": True,
        "noObservedStateChangesSinceOriginalCollection": True,
        "appLastModified": header["LastModified"].isoformat(),
        "scopedObjectCounts": counts,
        "metadataOnly": True,
    }


if __name__ == "__main__":
    try:
        context = json.loads(Path(sys.argv[1]).read_bytes())
        scope = json.loads(
            (
                Path(__file__).resolve().parents[2]
                / "docs/development/project-environments-execution-scope.json"
            ).read_bytes()
        )
        client = boto3.client(
            "s3",
            region_name="us-east-2",
            config=Config(
                retries={"total_max_attempts": 1}, connect_timeout=10, read_timeout=30
            ),
        )
        try:
            print(
                json.dumps(
                    inspect(client, scope["stateBucket"], context["stateCutoffUtc"])
                )
            )
        finally:
            client.close()
    except Exception:  # noqa: BLE001 - never expose credentials or raw errors
        raise SystemExit(
            "Read-only recovery state inspection held; no retry."
        ) from None
