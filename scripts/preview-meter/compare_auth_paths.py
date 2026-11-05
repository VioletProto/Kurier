"""Offline Sign-In operation resolution only. Never fetch credentials or send."""

import argparse
import ast
import configparser
import contextlib
import hashlib
import json
import os
import re
import socket
import subprocess
import sys
import time
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch
from urllib.parse import urlsplit

ROOT = Path(__file__).resolve().parents[2]
ACCEPTED_HOSTS = {
    "signin.us-east-2.amazonaws.com",
    "us-east-2.signin.aws.amazon.com",
    "signin.us-east-1.amazonaws.com",
    "us-east-1.signin.aws.amazon.com",
}
MODES = {"sdk-default", "custom", "cli-default"}


class Captured(BaseException):
    pass


class OfflineViolation(BaseException):
    pass


def blocked(*_, **__):
    raise OfflineViolation()


def sha(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def hostname(url):
    value = urlsplit(url)
    if (
        value.scheme != "https"
        or value.port not in (None, 443)
        or value.username is not None
        or value.password is not None
        or value.query
        or value.fragment
        or value.hostname not in ACCEPTED_HOSTS
    ):
        raise OfflineViolation()
    return value.hostname


def safe_region(value):
    return (
        value
        if isinstance(value, str)
        and re.fullmatch(r"[a-z]{2}(?:-gov)?-[a-z]+-[1-9]", value)
        else None
    )


def metadata():
    # Read only metadata; never emit/cache/hash credential contents or session ARN.
    cfg = configparser.RawConfigParser()
    cfg.read(Path.home() / ".aws/config")
    name = cfg.get("default", "login_session")
    cache = (
        Path.home()
        / ".aws/login/cache"
        / (hashlib.sha256(name.encode()).hexdigest() + ".json")
    )
    raw = cache.read_bytes()
    data = json.loads(raw)
    regions = []
    for container in [data, data.get("accessToken", {})]:
        for key, value in container.items():
            if "region" in key.lower():
                regions.append(safe_region(value))
    value = {
        "profileRegion": safe_region(cfg.get("default", "region", fallback=None)),
        "loginSessionConfigured": bool(name),
        "loginSessionArnRegionPresent": bool(name.split(":")[3]),
        "cacheRegionMetadata": regions,
        "requiredCacheFieldsPresent": all(
            k in data for k in ["accessToken", "refreshToken", "clientId", "dpopKey"]
        ),
        "profileEndpointOverridePresent": cfg.has_option("default", "endpoint_url"),
        "serviceSectionCount": sum(s.startswith("services ") for s in cfg.sections()),
        "environmentOverrideNamesPresent": [
            k
            for k in [
                "AWS_REGION",
                "AWS_DEFAULT_REGION",
                "AWS_PROFILE",
                "AWS_CONFIG_FILE",
                "AWS_SHARED_CREDENTIALS_FILE",
                "AWS_LOGIN_CACHE_DIRECTORY",
                "AWS_ENDPOINT_URL",
                "AWS_ENDPOINT_URL_SIGNIN",
                "AWS_CA_BUNDLE",
                "HTTP_PROXY",
                "HTTPS_PROXY",
                "ALL_PROXY",
                "NO_PROXY",
                "http_proxy",
                "https_proxy",
                "all_proxy",
                "no_proxy",
            ]
            if k in os.environ
        ],
    }
    value["cacheUnchangedDuringMetadataRead"] = cache.read_bytes() == raw
    return value


def custom_client(Session, Config, unsigned):
    # Extract the real helper's nested client factory without invoking authenticate
    # or ExistingCache. The extracted function has no credential-loader access.
    path = ROOT / "scripts/preview-meter/authentication.py"
    if sha(path) != "5abf92df10c60209bd764bb76f8d149f2d6b4cc7918aa62640504503201043a9":
        raise OfflineViolation()
    tree = ast.parse(path.read_text())
    auth = next(
        x
        for x in tree.body
        if isinstance(x, ast.FunctionDef) and x.name == "authenticate"
    )
    session_assignment = next(x for x in auth.body if isinstance(x, ast.Try)).body
    factory = next(
        x
        for x in session_assignment
        if isinstance(x, ast.FunctionDef) and x.name == "client"
    )
    assignment = next(
        x
        for x in session_assignment
        if isinstance(x, ast.Assign)
        and any(isinstance(t, ast.Name) and t.id == "session" for t in x.targets)
    )
    variables = ast.literal_eval(assignment.value.keywords[0].value)
    constants = {
        x.targets[0].id: ast.literal_eval(x.value)
        for x in tree.body
        if isinstance(x, ast.Assign)
        and isinstance(x.targets[0], ast.Name)
        and x.targets[0].id in {"SIGNIN", "STS"}
    }
    env = {
        "Session": Session,
        "Config": Config,
        "UNSIGNED": unsigned,
        "session": Session(session_vars=variables),
        "budget": SimpleNamespace(check=lambda: None, stop=blocked),
        "window": {"deadline": time.monotonic() + 3600},
        "time": time,
        **constants,
    }
    module = ast.Module(body=[factory], type_ignores=[])
    exec(compile(ast.fix_missing_locations(module), str(path), "exec"), env)  # noqa: S102 - hash-pinned local factory, with networking blocked
    return env["client"]("signin"), sha(path)


def child(mode):
    # Guard native sockets as well as the SDK's send entry point. The outer
    # verification namespace supplies another independent no-network boundary.
    with contextlib.ExitStack() as stack:
        for name in ["connect", "connect_ex", "send", "sendall", "sendto", "sendmsg"]:
            stack.enter_context(patch.object(socket.socket, name, blocked))
        for name in [
            "getaddrinfo",
            "gethostbyname",
            "gethostbyname_ex",
            "create_connection",
        ]:
            stack.enter_context(patch.object(socket, name, blocked))
        cli_version = None
        if mode == "cli-default":
            import awscli  # Must precede botocore: CLI installs its vendored alias.

            cli_version = awscli.__version__
        import botocore
        import urllib3
        from botocore.config import Config
        from botocore.httpsession import URLLib3Session
        from botocore.session import Session

        stack.enter_context(patch.object(URLLib3Session, "send", blocked))
        stack.enter_context(patch.object(Session, "get_credentials", blocked))
        if mode == "custom":
            client, helper_sha = custom_client(Session, Config, botocore.UNSIGNED)
        else:
            if mode == "sdk-default":
                import boto3

                session = boto3.Session(region_name="us-east-2")
                creator = session.client
            else:
                session = Session()
                creator = session.create_client
            client = creator(
                "signin",
                aws_access_key_id="offline-only",
                aws_secret_access_key="offline-only",
                config=Config(signature_version=botocore.UNSIGNED),
            )
            helper_sha = None
        base = Path(botocore.__file__).parent
        result = {
            "mode": mode,
            "botocoreVersion": botocore.__version__,
            "cliVersion": cli_version,
            "clientBaseHostname": hostname(client.meta.endpoint_url),
            "region": safe_region(client.meta.region_name),
            "helperSourceSha256": helper_sha,
            "modelSha256": {
                name: sha(base / name)
                for name in [
                    "credentials.py",
                    "utils.py",
                    "httpsession.py",
                    "data/signin/2023-01-01/service-2.json",
                    "data/signin/2023-01-01/endpoint-rule-set-1.json",
                ]
            },
            "connectTimeoutSeconds": client.meta.config.connect_timeout,
            "readTimeoutSeconds": client.meta.config.read_timeout,
            "retryMode": client.meta.config.retries.get("mode"),
            "totalMaxAttemptsConfigured": client.meta.config.retries.get(
                "total_max_attempts"
            ),
            "maxAttemptsConfigured": client.meta.config.retries.get("max_attempts"),
            "useFipsEndpoint": client.meta.config.use_fips_endpoint is True,
            "useDualStackEndpoint": client.meta.config.use_dualstack_endpoint is True,
            "urllib3Version": urllib3.__version__,
            "proxyConfigInheritance": client.meta.config.proxies is None,
            "effectiveProxyConfigured": bool(
                client._endpoint.http_session._proxy_config._proxies
            ),
            "tlsVerifyRequested": client._endpoint.http_session._verify is True,
        }

        def capture(params, **_):
            result["operationHostname"] = hostname(params["url"])
            result["expectedTlsServerHostname"] = result["operationHostname"]
            connection = SimpleNamespace()
            http = client._endpoint.http_session
            http._setup_ssl_cert(connection, params["url"], http._verify)
            result["tlsCertRequired"] = connection.cert_reqs == "CERT_REQUIRED"
            result["caAnchorsSha256"] = sha(connection.ca_certs)
            result["clientCertificateConfigured"] = bool(http._cert_file)
            result["urllib3HostnameAssertionDisabled"] = (
                http._manager.connection_pool_kw.get("assert_hostname") is False
            )
            ctx = http._manager.connection_pool_kw["ssl_context"]
            result["tlsContext"] = {
                "minimumVersion": ctx.minimum_version.name,
                "maximumVersion": ctx.maximum_version.name,
                "options": ctx.options,
                "checkHostnameProperty": ctx.check_hostname,
                "cipherConfigurationSha256": hashlib.sha256(
                    json.dumps([x["name"] for x in ctx.get_ciphers()]).encode()
                ).hexdigest(),
            }
            result["beforeCallCaptured"] = True
            raise Captured()

        client.meta.events.register_first(
            "before-call.signin.CreateOAuth2Token", capture
        )
        try:
            client.create_o_auth2_token(
                tokenInput={
                    "clientId": "offline-fixture",
                    "refreshToken": "offline-fixture",
                    "grantType": "refresh_token",
                }
            )
        except Captured:
            pass
        finally:
            client._endpoint.http_session.close()
        if not result.get("beforeCallCaptured"):
            raise OfflineViolation()
        result.update(
            networkRequests=0, credentialProviderInvoked=False, cacheWritten=False
        )
        return result


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--child", choices=sorted(MODES))
    a = p.parse_args()
    if a.child:
        print(json.dumps(child(a.child)))
        return
    values = []
    for mode in ["sdk-default", "custom", "cli-default"]:
        result = subprocess.run(
            [sys.executable, "-B", __file__, "--child", mode],
            capture_output=True,
            text=True,
            check=False,
            timeout=30,
        )
        if result.returncode:
            # Child stderr may contain arbitrary SDK messages; never relay it.
            raise OfflineViolation()
        values.append(json.loads(result.stdout))
    print(
        json.dumps(
            {
                "localOnly": True,
                "networkRequests": 0,
                "credentialValuesPublished": False,
                "metadata": metadata(),
                "paths": values,
            },
            indent=2,
        )
    )


if __name__ == "__main__":
    try:
        main()
    except BaseException:  # noqa: BLE001 - suppress raw SDK/config/credential material
        print(
            "Offline comparison stopped without publishing raw diagnostics.",
            file=sys.stderr,
        )
        raise SystemExit(1) from None
