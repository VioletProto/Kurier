#!/usr/bin/env python3
"""Preview/activate schema 2 only after both scoped development handlers are ready."""
import argparse
import json
import subprocess
from pathlib import Path

parser = argparse.ArgumentParser()
parser.add_argument("--apply", action="store_true")
args = parser.parse_args()
outputs = json.loads(Path(".sst/outputs.json").read_text())
if outputs.get("stage") != "dev-api" or outputs.get("region") != "us-east-2":
    raise SystemExit("Expected dev-api outputs.")

def aws(*parts):
    result = subprocess.run(["aws", "--region", "us-east-2", *parts], capture_output=True, text=True)
    if result.returncode:
        raise SystemExit("AWS check/update failed; activation not claimed.")
    return json.loads(result.stdout or "{}")

identity = aws("sts", "get-caller-identity")
if identity["Account"] != "747336059622" or identity["Arn"].endswith(":root"):
    raise SystemExit("Expected intended account and non-root identity.")
for name in (outputs["apiFunction"], outputs["cleanupFunction"]):
    fn = aws("lambda", "get-function-configuration", "--function-name", name)
    env = fn.get("Environment", {}).get("Variables", {})
    if (fn.get("LastUpdateStatus") != "Successful" or env.get("KURIER_SAVED_REQUESTS_SCHEMA_VERSION") != "2"
            or env.get("KURIER_PROTECTED_SECRETS_SCHEMA_VERSION") != "1"
            or env.get("KURIER_PROTECTED_TABLE") != outputs["protectedTable"]):
        raise SystemExit("Deploy both compatible handlers before capability activation.")
table = aws("dynamodb", "describe-table", "--table-name", outputs["protectedTable"])["Table"]
keymeta = aws("kms", "describe-key", "--key-id", outputs["protectedKeyArn"])["KeyMetadata"]
if table["TableStatus"] != "ACTIVE" or keymeta["KeyState"] != "Enabled" or keymeta["KeyManager"] != "CUSTOMER":
    raise SystemExit("Expected active Protected and enabled customer key.")
key = {"PK": {"S": "STAGE#dev-api"}, "SK": {"S": "META"}}
item = aws("dynamodb", "get-item", "--table-name", outputs["controlTable"], "--key", json.dumps(key), "--consistent-read")["Item"]
if item.get("savedRequestsSchemaVersion") == {"N": "2"} and item.get("protectedSecretsSchemaVersion") == {"N": "1"}:
    print("Protected capability already active; no mutation.")
    raise SystemExit(0)
if (item.get("state") != {"S": "active"} or item.get("kind") != {"S": "stage"}
        or item.get("schemaVersion") != {"N": "1"} or item.get("savedRequestsSchemaVersion") != {"N": "1"}
        or "protectedSecretsSchemaVersion" in item):
    raise SystemExit("Expected active known schema-1 stage; no recovery reset.")
print("Plan: conditionally advance savedRequestsSchemaVersion=2, protectedSecretsSchemaVersion=1; retain state/generation/data.")
if args.apply:
    values = {":active": {"S": "active"}, ":kind": {"S": "stage"}, ":one": {"N": "1"}, ":two": {"N": "2"}, ":v": item["version"], ":next": {"N": str(int(item["version"]["N"]) + 1)}, ":gen": item["recoveryGeneration"]}
    aws("dynamodb", "update-item", "--table-name", outputs["controlTable"], "--key", json.dumps(key),
        "--update-expression", "SET savedRequestsSchemaVersion = :two, protectedSecretsSchemaVersion = :one, #v = :next",
        "--condition-expression", "#state = :active AND #kind = :kind AND schemaVersion = :one AND #v = :v AND recoveryGeneration = :gen AND savedRequestsSchemaVersion = :one AND attribute_not_exists(protectedSecretsSchemaVersion)",
        "--expression-attribute-names", json.dumps({"#state": "state", "#kind": "kind", "#v": "version"}), "--expression-attribute-values", json.dumps(values))
    current = aws("dynamodb", "get-item", "--table-name", outputs["controlTable"], "--key", json.dumps(key), "--consistent-read")["Item"]
    if current.get("savedRequestsSchemaVersion") != {"N": "2"} or current.get("protectedSecretsSchemaVersion") != {"N": "1"} or current["recoveryGeneration"] != item["recoveryGeneration"]:
        raise SystemExit("Activation readback failed.")
    print("Protected capability activation verified; recovery generation unchanged.")
