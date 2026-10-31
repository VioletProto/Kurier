"""Production launcher -> pinned packager source, retained inputs, offline namespaces."""

import json
import os
import stat
import subprocess
import sys
import tempfile
import zipfile
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
from common import PRIOR, ROOT, Campaign, canonical, private_write
from driver import SOURCE_DIR, check, mounts
from isolation import launch


def worker(config_path):
    config = json.loads(Path(config_path).read_bytes())
    root = Path(config["root"])
    release, _scope, pins = check()
    c = Campaign(
        root / "campaign",
        create={
            "prior": PRIOR,
            "maximum": 1799,
            "seconds": 3300,
            "manifestSha256": pins["manifest"],
            "scopeSha256": pins["scope"],
            "reviewCommit": "c" * 40,
            "authorizationSha256": "d" * 64,
        },
    )
    try:
        c.begin("collect")
        output = root / "child"
        expected_umask = next(
            x.split()[1]
            for x in Path("/proc/self/status").read_text().splitlines()
            if x.startswith("Umask:")
        )
        result = launch(
            SOURCE_DIR,
            output,
            ["node", str(HERE / "fixture_packaging.mjs"), "/fixture/config.json"],
            {},
            campaign=c,
            phase="collect",
            policy_context={"fixtureOnly": True},
            extra_ro=[
                *mounts(),
                (Path(config["inputConfig"]), Path("/fixture/config.json")),
                (Path(config["inputs"]), Path("/fixture/inputs")),
            ],
        )
        assert result["exitCode"] == 0 and not c.get("stopped")
        actual = json.loads((output / "private-output.txt").read_bytes())
        assert (
            c.get("count") == PRIOR
            and c.db.execute("SELECT COUNT(*) FROM attempts").fetchone()[0] == 0
        )
        assert expected_umask == next(
            x.split()[1]
            for x in Path("/proc/self/status").read_text().splitlines()
            if x.startswith("Umask:")
        )
        for row in actual:
            package = next(v for v in release["packages"] if v["name"] == row["name"])
            assert (
                row["sha256"] == package["zipSha256"]
                and row["bootstrapExecutable"]
                and row["umask"] == 0o077
            )
            archive = output / "work/packaging/artifacts" / package["name"] / "code.zip"
            adopted = ROOT / release["artifactDirectory"] / package["name"] / "code.zip"
            with zipfile.ZipFile(archive) as a, zipfile.ZipFile(adopted) as b:
                assert a.namelist() == b.namelist()
                assert all(a.read(n) == b.read(n) for n in b.namelist())
                assert stat.S_IMODE(a.getinfo("bootstrap").external_attr >> 16) == 0o700
        print(
            json.dumps(
                {
                    "parentUmask": expected_umask,
                    "exactAdoptedArchives": 4,
                    "unchangedMembers": True,
                    "bootstrapExecutable": True,
                    "childUmask": "0077",
                    "parentUmaskUnchanged": True,
                    "awsCalls": 0,
                    "cumulativeUsageUnchanged": PRIOR,
                }
            )
        )
    finally:
        c.close()


def main():
    if len(sys.argv) > 1:
        worker(sys.argv[1])
        return
    release, _, pins = check()
    with tempfile.TemporaryDirectory(prefix="kp-") as directory:
        root = Path(directory)
        inputs = root / "inputs"
        inputs.mkdir(mode=0o700)
        for package in release["packages"]:
            target = inputs / package["name"]
            target.mkdir(mode=0o700)
            with zipfile.ZipFile(
                ROOT / release["artifactDirectory"] / package["name"] / "code.zip"
            ) as z:
                for name in ["bootstrap", "resource.enc"]:
                    private_write(target / name, z.read(name))
        input_config = root / "packager.json"
        private_write(
            input_config,
            canonical(
                {
                    "platform": str(SOURCE_DIR / ".sst/platform"),
                    "sourceSha256": pins["packager"][
                        ".sst/platform/src/components/aws/function.ts"
                    ],
                    "inputs": "/fixture/inputs",
                    "output": "/work/packaging",
                    "packages": release["packages"],
                }
            ),
        )
        results = []
        for mask in [0o000, 0o022, 0o027, 0o077]:
            run = root / ("m" + str(mask))
            run.mkdir(mode=0o700)
            config = root / ("worker" + str(mask) + ".json")
            private_write(
                config,
                canonical(
                    {
                        "root": str(run),
                        "inputConfig": str(input_config),
                        "inputs": str(inputs),
                    }
                ),
            )
            env = {
                k: v
                for k, v in os.environ.items()
                if not k.startswith(("AWS_", "KURIER_"))
            }
            env.update(
                AWS_CONFIG_FILE="/dev/null",
                AWS_SHARED_CREDENTIALS_FILE="/dev/null",
                AWS_EC2_METADATA_DISABLED="true",
                PYTHONDONTWRITEBYTECODE="1",
                GOPROXY="off",
                GOTOOLCHAIN="local",
            )
            result = subprocess.run(
                [sys.executable, "-B", str(Path(__file__).resolve()), str(config)],
                umask=mask,
                env=env,
                capture_output=True,
                check=False,
            )
            if result.returncode:
                # This is synthetic/offline only. Retain the private startup error.
                private = Path(os.environ["KURIER_TEST_PRIVATE_DIR"]) / (
                    "packaging-error-" + str(mask) + ".txt"
                )
                private_write(private, result.stderr)
                raise AssertionError("offline-packaging-child-failed")
            results.append(json.loads(result.stdout))
        private_write(
            Path(os.environ["KURIER_TEST_PRIVATE_DIR"]) / "packaging-result.json",
            canonical(results),
        )
        print(
            json.dumps(
                {
                    "case": "actual-launcher-pinned-packager-parent-umask-matrix",
                    "cases": results,
                }
            )
        )


if __name__ == "__main__":
    main()
