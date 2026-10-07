#!/usr/bin/env python3
"""Provision once, never print or overwrite the development signing key."""
import base64
import json
import os
import secrets
import subprocess

account = os.environ.get("KURIER_AWS_ACCOUNT_ID")
identity = json.loads(subprocess.check_output(["aws", "sts", "get-caller-identity", "--output", "json"]))
if account != identity["Account"] or identity["Arn"].endswith(":root"):
    raise SystemExit("Wrong account or root identity; refusing secure configuration")
name = "/kurier/dev-api/cursor-key"
existing = subprocess.run(["aws", "ssm", "get-parameter", "--name", name, "--region", "us-east-2", "--query", "Parameter.Type", "--output", "text"], capture_output=True, text=True)
if existing.returncode == 0:
    if existing.stdout.strip() != "SecureString":
        raise SystemExit("Existing parameter is not a SecureString")
    print("Existing secure cursor key retained")
elif "ParameterNotFound" in existing.stderr:
    payload = {"Name": name, "Type": "SecureString", "Tier": "Standard", "Value": base64.b64encode(secrets.token_bytes(32)).decode(), "Description": "Stable Kurier development API cursor signing key; do not rotate during active pagination"}
    fd = os.memfd_create("kurier-secure-config", os.MFD_CLOEXEC)
    try:
        os.write(fd, json.dumps(payload).encode())
        os.lseek(fd, 0, os.SEEK_SET)
        result = subprocess.run(["aws", "ssm", "put-parameter", "--region", "us-east-2", "--cli-input-json", f"file:///proc/self/fd/{fd}"], pass_fds=(fd,), capture_output=True, text=True)
    finally:
        os.close(fd)
    if result.returncode != 0:
        raise SystemExit("Secure cursor key creation failed: " + result.stderr.replace(payload["Value"], "[REDACTED]"))
    print("Secure cursor key created; value never printed")
else:
    raise SystemExit("Cannot inspect secure parameter; refusing replacement")
