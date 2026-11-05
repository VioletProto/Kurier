"""Test-only inline responses for denied bucket reads; never forwards them.

The test harness alone selects this addon. The live launcher always loads
proxy.py and has no option/config field that selects this observer.
"""

import importlib.util
import json
import os
import sys
from pathlib import Path
from types import MethodType

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))

from common import Held, canonical, private_write
from fixture_bucket_reads import fixture_response, validate_model
from mitmproxy import http

configuration = json.loads(Path(os.environ["KURIER_ENV_EXECUTION_CONFIG"]).read_bytes())
interfaces = {
    line.split(":", 1)[0].strip()
    for line in Path("/proc/net/dev").read_text().splitlines()
    if ":" in line
}
if (
    not configuration["context"].get("fixtureOnly")
    or type(configuration.get("fixtureOrigin")) is not int
    or not 0 < configuration["fixtureOrigin"] < 65536
    or configuration["phase"] != "collect"
    or interfaces != {"lo"}
):
    raise RuntimeError("offline-fixture-namespace-required")
validate_model()

spec = importlib.util.spec_from_file_location(
    "production_execution_gate", Path(__file__).with_name("proxy.py")
)
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)
gate = module.addons[0]
normal_request = gate.request
observed = []


async def _observe_request(self, flow):
    self.c.check()
    result = fixture_response(flow.request, self.scope)
    if result is not None:
        try:
            self.policy.request(
                flow.request.method,
                flow.request.host,
                flow.request.path,
                flow.request.headers,
                flow.request.raw_content or b"",
            )
        except Held as error:
            if str(error) not in {"s3-read-scope-held", "s3-operation-id-held"}:
                return await normal_request(flow)
            operation, selector, status, body = result
            entry = {
                "operation": operation,
                "selector": selector,
                "condition": str(error),
                "admitted": False,
                "forwarded": False,
                "syntheticResponse": True,
            }
            observed.append(entry)
            private_write(
                self.directory / ("observed-read-" + str(len(observed)) + ".json"),
                canonical(entry),
            )
            flow.response = http.Response.make(
                status, body, {"Content-Type": "application/xml"}
            )
            return
    await normal_request(flow)


gate.request = MethodType(_observe_request, gate)
addons = [gate]
