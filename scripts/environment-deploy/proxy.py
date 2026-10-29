"""Host-only execution gate. No flow dumps, headers, bodies or credential receipts."""

import asyncio
import json
import os
import resource
import sys
import time
import zipfile
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
sys.path.insert(0, str(HERE.parent / "preview-meter"))
from artifacts import load_objects
from common import ROOT, Campaign, Held, canonical, digest, private_write
from mitmproxy import http
from wire import Policy


class Gate:
    def __init__(self):
        resource.setrlimit(resource.RLIMIT_CORE, (0, 0))
        config = json.loads(Path(os.environ["KURIER_ENV_EXECUTION_CONFIG"]).read_text())
        self.config = config
        self.directory = Path(os.environ["KURIER_ENV_EXECUTION_CONFIG"]).parent
        self.c = Campaign(Path(config["campaign"]))
        self.c.check()
        if self.c.get("activePhase") != config["phase"]:
            raise Held("phase-context-held")
        release = json.loads(
            (
                ROOT / "docs/development/project-environments-adopted-release.json"
            ).read_text()
        )
        scope = json.loads(
            (
                ROOT / "docs/development/project-environments-execution-scope.json"
            ).read_text()
        )
        if digest(
            (
                ROOT / "docs/development/project-environments-adopted-release.json"
            ).read_bytes()
        ) != self.c.get("manifestSha256") or digest(
            (
                ROOT / "docs/development/project-environments-execution-scope.json"
            ).read_bytes()
        ) != self.c.get("scopeSha256"):
            raise Held("campaign-pins-held")
        ctx = config["context"]
        self.release = release
        self.policy = Policy(
            scope,
            release,
            load_objects(scope, release),
            phase=config["phase"],
            stage_transaction=ctx.get("transaction"),
            closed_marker=ctx.get("closedMarker"),
        )
        self.fixture = config.get("fixtureOrigin")
        if self.fixture is not None and (
            type(self.fixture) is not int or not ctx.get("fixtureOnly")
        ):
            raise Held("fixture-held")
        private_write(self.directory / "proxy-ready", b"ready")

    async def running(self):
        async def watch():
            while True:
                await asyncio.sleep(0.05)
                try:
                    self.c.check()
                except Held:
                    break
                except Exception:  # noqa: BLE001 - redact private request/response material
                    os._exit(74)

        self.watcher = asyncio.create_task(watch())

    def held(self, flow, reason):
        try:
            try:
                self.c.stop(reason)
            except Held:
                pass
        except Exception:  # noqa: BLE001 - redact private request/response material
            os._exit(74)
        flow.response = http.Response.make(
            599, b'{"executionHeld":true}', {"Content-Type": "application/json"}
        )

    def http_connect(self, flow):
        try:
            self.c.check()
            if flow.request.port != 443:
                raise Held("tls-port-held")
            self.policy.host(flow.request.host)
        except Held:
            self.held(flow, "endpoint-or-budget-held")
        except Exception:  # noqa: BLE001 - redact private request/response material
            os._exit(74)

    async def barrier(self):
        if self.policy.artifacts_ready:
            return
        until = min(
            self.c.get("deadline"),
            self.c.get("closedDeadline") or self.c.get("deadline"),
        )
        while time.monotonic() < until:
            self.c.check()
            ready = True
            for p in self.release["packages"]:
                f = self.directory / "work/sst/artifacts" / p["name"] / "code.zip"
                if not f.is_file():
                    ready = False
                    continue
                try:
                    with zipfile.ZipFile(f) as z:
                        if set(z.namelist()) != {"./", "bootstrap", "resource.enc"}:
                            raise Held("generated-members-held")
                except zipfile.BadZipFile:
                    ready = False
                    continue
                b = f.read_bytes()
                if len(b) != p["zipBytes"] or digest(b) != p["zipSha256"]:
                    raise Held("regenerated-archive-held")
            marker = self.directory / "work/sst/frontend-hosting-artifact.json"
            if not marker.is_file():
                ready = False
            elif json.loads(marker.read_bytes()) != self.release["frontend"]:
                raise Held("regenerated-frontend-held")
            if ready:
                self.policy.artifacts_ready = True
                return
            await asyncio.sleep(0.05)
        raise Held("artifact-barrier-deadline")

    async def request(self, flow):
        try:
            self.c.check()
            q = flow.request
            if q.scheme != "https" or q.port != 443:
                raise Held("tls-required")
            meta = self.policy.request(
                q.method, q.host, q.path, q.headers, q.raw_content or b""
            )
            if (
                meta["write"]
                and self.config["phase"] == "deploy"
                and not meta.get("prewarm")
                and meta["operation"]
                not in {
                    "s3/SstStageMetadataPut",
                    "s3/SstStageUnlock",
                    "cloudfront-keyvaluestore/UpdateKeys",
                }
            ):
                await self.barrier()
            n = self.c.admit(meta["operation"], meta["write"])
            flow.metadata.update(executionAttempt=n, executionMeta=meta)
            if self.fixture:
                q.scheme, q.host, q.port = "http", "127.0.0.1", self.fixture
        except Held:
            self.held(flow, "operation-resource-byte-or-budget-held")
        except Exception:  # noqa: BLE001 - redact private request/response material
            self.held(flow, "wire-or-meter-held")

    def response(self, flow):
        n = flow.metadata.get("executionAttempt")
        if n:
            try:
                self.c.outcome(n, flow.response.status_code)
                self.policy.response(
                    flow.metadata["executionMeta"],
                    flow.response.status_code,
                    flow.response.headers,
                    flow.response.raw_content or b"",
                )
            except Held:
                self.held(flow, "response-held-no-retry")
            except Exception:  # noqa: BLE001 - redact private request/response material
                self.held(flow, "response-validation-held")

    def error(self, flow):
        n = flow.metadata.get("executionAttempt")
        try:
            if n:
                self.c.outcome(n, None)
            else:
                self.c.stop("transport-held")
        except Held:
            pass
        except Exception:  # noqa: BLE001 - redact private request/response material
            os._exit(74)

    def done(self):
        private_write(
            self.directory / "proxy-summary.json",
            canonical(
                {
                    "identityVerified": self.policy.identity_verified,
                    "closedVerified": self.policy.closed_verified,
                    "lastMarkerSha256": self.policy.last_marker_sha256,
                    "artifactsMatched": self.policy.artifacts_ready,
                    "confirmedUploads": len(self.policy.uploads.finished),
                }
            ),
        )
        self.c.close()


try:
    addons = [Gate()]
except Exception:
    # Startup only: private stderr, before any provider request can be admitted.
    import traceback

    traceback.print_exc()
    raise
