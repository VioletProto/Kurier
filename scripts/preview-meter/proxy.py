"""Host-side gate; optional private host-only receipt, no request bodies/headers."""

import asyncio
import json
import os
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent))
from diagnostics import endpoint_metadata
from handoff import SharedBudget, consume
from mitmproxy import http
from operation_diagnostics import capture_operation
from policy import Budget, Stop, classify, service
from private_diagnostics import capture
from s3control_tags import target_allowed as s3control_target_allowed


class Gate:
    def __init__(self):
        config = json.loads(Path(os.environ["KURIER_PREVIEW_METER_CONFIG"]).read_text())
        self.test = config["mode"] == "fixture"
        self.origin = config.get("fixtureOrigin")
        self.fixture_failure = config.get("fixtureFailure", False)
        self.private_connect = config.get("privateDeniedConnectCapture", False)
        self.private_operation = config.get("privateS3ControlOperationCapture", False)
        if (
            type(self.private_connect) is not bool
            or type(self.private_operation) is not bool
        ):
            raise ValueError("private-capture-option-held")
        self.private_directory = Path(config["ledger"]).parent
        if self.test and not isinstance(self.origin, int):
            raise ValueError("fixture-origin-required")
        if self.test:
            if "handoff" in config:
                raise ValueError("fixture-handoff-held")
            self.budget = Budget(config["ledger"], **config["budget"])
        else:
            if config["mode"] != "preview" or "budget" in config or self.origin:
                raise ValueError("preview-budget-reset-held")
            window, count = consume(
                config["handoff"], Path(__file__).resolve().parents[2]
            )
            self.budget = SharedBudget(config["ledger"], window, prior=count)
        if "accounting" in config:
            self.budget.record(event="accounting-baseline", **config["accounting"])

    async def running(self):
        async def watch():
            while not self.budget.stopped:
                await asyncio.sleep(0.05)
                try:
                    self.budget.check()
                except Stop:
                    break
                except OSError:
                    os._exit(74)

        self.watch = asyncio.create_task(watch())

    def held(self, flow, reason, phase):
        try:
            if not self.budget.stopped:
                q = flow.request
                self.budget.record(
                    event="denied-endpoint",
                    count=self.budget.count,
                    reason=reason,
                    **endpoint_metadata(
                        q.host, q.port, q.scheme, q.http_version, q.method, phase
                    ),
                )
                if (
                    self.private_connect
                    and phase == "connect"
                    and reason == "endpoint-held"
                ):
                    capture(self.private_directory, q.host, q.port, self.budget.count)
                if (
                    self.private_operation
                    and phase == "request"
                    and reason == "s3control-operation-held"
                ):
                    capture_operation(
                        self.private_directory, q.method, q.path, self.budget.count
                    )
            self.budget.stop(reason)
        except Stop:
            pass
        except Exception:  # noqa: BLE001 - diagnostics must never permit forwarding
            flow.kill()
            os._exit(74)
        flow.response = http.Response.make(
            599, b'{"previewMeterStopped":true}', {"Content-Type": "application/json"}
        )

    def http_connect(self, flow):
        try:
            self.budget.check()
            if flow.request.port != 443:
                raise Stop("port-held")
            service(flow.request.host)
        except Stop as e:
            self.held(flow, str(e), "connect")
        except OSError:
            flow.kill()
            os._exit(74)

    def request(self, flow):
        try:
            self.budget.check()
            q = flow.request
            if q.scheme != "https" or q.port != 443:
                raise Stop("tls-required")
            if service(q.host) == "s3control" and not s3control_target_allowed(
                q.method, q.host, q.path
            ):
                raise Stop("s3control-operation-held")
            svc, op = classify(
                q.method, q.host, q.path, q.headers, q.raw_content or b""
            )
            if self.fixture_failure and self.test:
                os.close(self.budget.fd)
            n = self.budget.admit(svc, op, q.http_version)
            flow.metadata["preview_attempt"] = n
            if self.test:
                q.scheme, q.host, q.port = "http", "127.0.0.1", self.origin
        except Stop as e:
            self.held(flow, str(e), "request")
        except Exception:  # noqa: BLE001 - hook failures must terminate rather than forward
            flow.kill()
            os._exit(74)

    def response(self, flow):
        n = flow.metadata.get("preview_attempt")
        if n:
            try:
                self.budget.outcome(n, flow.response.status_code)
            except Stop as e:
                if str(e) == "redirect-held":
                    flow.response = http.Response.make(
                        599, b'{"previewMeterStopped":true}'
                    )
            except OSError:
                flow.kill()
                os._exit(74)

    def error(self, flow):
        n = flow.metadata.get("preview_attempt")
        if n:
            try:
                self.budget.outcome(n, error=True)
            except Stop:
                pass
            except OSError:
                flow.kill()
                os._exit(74)

    def done(self):
        if hasattr(self, "watch"):
            self.watch.cancel()
        self.budget.close()


addons = [Gate()]
