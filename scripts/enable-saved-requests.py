#!/usr/bin/env python3
"""Explicit scoped capability migration; preview by default, no credential output."""
import argparse
import json
import os
import subprocess

parser = argparse.ArgumentParser()
parser.add_argument("--apply", action="store_true")
parser.add_argument("--local", action="store_true")
args = parser.parse_args()
region = "us-east-2"
table = "kurier-local-control" if args.local else "kurier-dev-api-ControlTable-bdxbaoxb"
stage = "local" if args.local else "dev-api"
base = ["aws", "--region", region]
if args.local:
    base += ["--endpoint-url", "http://127.0.0.1:8000"]


def aws(*parts):
    env = dict(os.environ)
    if args.local:
        env.update(AWS_ACCESS_KEY_ID="local", AWS_SECRET_ACCESS_KEY="local", AWS_SESSION_TOKEN="")
    result = subprocess.run(base + list(parts), capture_output=True, text=True, env=env)
    if result.returncode:
        raise SystemExit("AWS operation failed; no migration completion claimed.")
    return json.loads(result.stdout or "{}")


if not args.local:
    identity = aws("sts", "get-caller-identity")
    if identity["Account"] != "747336059622" or identity["Arn"].endswith(":root"):
        raise SystemExit("Expected intended account and non-root identity.")
    for name in (
        "kurier-dev-api-UsersProjectsApiFunction-werhekow",
        "kurier-dev-api-EmptyProjectCleanupFunction-bcmuzwku",
    ):
        fn = aws("lambda", "get-function-configuration", "--function-name", name)
        if args.apply and (fn.get("LastUpdateStatus") != "Successful" or
                fn.get("Environment", {}).get("Variables", {}).get("KURIER_SAVED_REQUESTS_SCHEMA_VERSION") != "1"):
            raise SystemExit("Deploy both saved-request handlers successfully before activation.")
key = {"PK": {"S": "STAGE#" + stage}, "SK": {"S": "META"}}
item = aws("dynamodb", "get-item", "--table-name", table, "--key", json.dumps(key), "--consistent-read")["Item"]
if item.get("state") != {"S": "active"} or item.get("kind") != {"S": "stage"} or item.get("schemaVersion") != {"N": "1"}:
    raise SystemExit("Active known stage required; never reactivate recovery.")
if item.get("savedRequestsSchemaVersion") == {"N": "1"}:
    print("Saved-request capability already active; no mutation.")
    raise SystemExit(0)
if item.get("localEmptyProjectsOnly") != {"BOOL": True} or "savedRequestsSchemaVersion" in item:
    raise SystemExit("Expected legacy empty-project capability; no mutation.")
print("Plan: update only STAGE#" + stage + "/META: savedRequestsSchemaVersion=1; remove localEmptyProjectsOnly; retain state, generation and all data.")
if args.apply:
    values = {":active": {"S": "active"}, ":kind": {"S": "stage"}, ":schema": {"N": "1"}, ":yes": {"BOOL": True}, ":one": {"N": "1"}, ":v": item["version"], ":next": {"N": str(int(item["version"]["N"]) + 1)}, ":gen": item["recoveryGeneration"]}
    aws("dynamodb", "update-item", "--table-name", table, "--key", json.dumps(key),
        "--update-expression", "SET savedRequestsSchemaVersion = :one, #v = :next REMOVE localEmptyProjectsOnly",
        "--condition-expression", "#state = :active AND #kind = :kind AND schemaVersion = :schema AND #v = :v AND recoveryGeneration = :gen AND localEmptyProjectsOnly = :yes AND attribute_not_exists(savedRequestsSchemaVersion)",
        "--expression-attribute-names", json.dumps({"#state": "state", "#kind": "kind", "#v": "version"}),
        "--expression-attribute-values", json.dumps(values))
    current = aws("dynamodb", "get-item", "--table-name", table, "--key", json.dumps(key), "--consistent-read")["Item"]
    if current.get("savedRequestsSchemaVersion") != {"N": "1"} or "localEmptyProjectsOnly" in current or current["recoveryGeneration"] != item["recoveryGeneration"]:
        raise SystemExit("Activation readback failed.")
    print("Scoped saved-request capability activation verified; recovery generation unchanged.")
