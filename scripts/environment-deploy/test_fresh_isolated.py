"""Actual driver startup and launcher: mid-phase cache drift kills a local child."""

import contextlib
import json
import socket
import sqlite3
import threading
import time
from unittest.mock import patch

import driver
from common import PRIOR, Held
from test_fresh_start import FreshStartTests


def main():
    fixture = FreshStartTests()
    fixture.fresh_case()
    fixture.pins = json.loads((driver.HERE / "pins.json").read_bytes())
    approved = json.loads(fixture.auth.read_bytes())
    approved["manifestSha256"] = fixture.pins["manifest"]
    approved["scopeSha256"] = fixture.pins["scope"]
    approved["completedParentSha256"] = fixture.pins["completedParent608"]
    fixture.auth.write_text(json.dumps(approved))
    pulse = fixture.path / "collect-readers/work/pulse"
    done = threading.Event()
    actual_child = driver.child
    real_socket, real_dns = socket.socket, socket.getaddrinfo

    def tamper():
        for _ in range(800):
            if pulse.exists():
                fixture.token["refreshToken"] += "-external-during-child"
                fixture.write_cache()
                done.set()
                return
            time.sleep(0.01)

    thread = threading.Thread(target=tamper, daemon=True)
    thread.start()
    code = """import socket,time
from pathlib import Path
def forbidden(*a,**k): raise AssertionError('no-network')
socket.socket=forbidden;socket.getaddrinfo=forbidden
assert Path('/meter/phase-context.json').is_file()
assert not (Path.home()/'.aws').exists()
for i in range(1000):
 Path('/work/pulse').write_text(str(i))
 time.sleep(.01)
"""
    try:
        with (
            fixture.driver_context(
                patch.object(driver, "child", side_effect=actual_child),
                patch.object(
                    driver, "phase_command", return_value=["python3", "-c", code]
                ),
                patch.object(socket, "socket", real_socket),
                patch.object(socket, "getaddrinfo", real_dns),
            ),
            fixture.assertRaises(Held),
        ):
            driver.main()
        thread.join(timeout=10)
        assert done.is_set() and not thread.is_alive()
        before = pulse.read_text()
        time.sleep(0.15)
        assert pulse.read_text() == before
        with contextlib.closing(
            sqlite3.connect(fixture.path / "campaign.sqlite")
        ) as db:
            config = {k: json.loads(v) for k, v in db.execute("SELECT k,v FROM config")}
            stops = [
                json.loads(v)["reason"]
                for (v,) in db.execute(
                    "SELECT data FROM events WHERE json_extract(data,'$.event')='stop'"
                )
            ]
        assert config["stopped"] and config["count"] == PRIOR + 1
        assert "active-cache-checkpoint-changed" in stops
        assert fixture.sends == ["sts/GetCallerIdentity"]
        print(
            json.dumps(
                {
                    "case": "actual-driver-fresh-capture-midphase-drift",
                    "awsCalls": 0,
                    "syntheticStsAttempts": 1,
                    "externalDriftStoppedChild": True,
                    "childPulseStopped": True,
                    "captureReplayed": False,
                    "realCacheUsed": False,
                }
            )
        )
    finally:
        fixture.doCleanups()


if __name__ == "__main__":
    main()
