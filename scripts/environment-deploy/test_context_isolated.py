"""Actual relative campaign, entrypoint and mounts; synthetic CLI, no AWS/network."""

import json
from pathlib import Path
from unittest.mock import patch

from common import PRIOR, ROOT, Campaign, Held
from driver import CHILD_CONTEXT, SOURCE_DIR, campaign_path, child
from isolation import launch


def main():
    # The runner overlays BASE with a fresh private fixture directory. The real
    # stopped campaigns/claims never become writable or get opened by this test.
    relative = Path(".build/project-environments-execution/release-from-390")
    directory = campaign_path(relative)
    assert Path.cwd() == ROOT
    assert (directory.parent / "fixture-only-overlay").read_text() == "synthetic"
    pins = json.loads(Path(__file__).with_name("pins.json").read_bytes())
    c = Campaign(
        relative,
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
        assert c.directory == directory and c.directory.is_absolute()
        c.begin("collect")
        (directory / "private-canary").write_text("must stay outside child")
        (directory.parent / "window-from-390.json").write_text("synthetic-claim")
        (directory.parent / "authorization-from-390.json").write_text(
            "synthetic-approval"
        )
        scope = json.loads(
            (
                ROOT / "docs/development/project-environments-execution-scope.json"
            ).read_bytes()
        )
        identity = {
            "Account": scope["account"],
            "Arn": scope["identityArn"],
            "UserId": "synthetic",
        }
        fake = Path("/tmp/context-fixture-aws")
        # Only this test replaces aws. It is not a production fixture mode or a
        # response/credential injection path in the released driver.
        fake.write_text(
            "#!/usr/bin/python3\n"
            "import errno,json,os,socket,sys\nfrom pathlib import Path\n"
            "def forbidden(*a,**k): raise AssertionError('fixture forbids networking')\n"
            "socket.socket=forbidden;socket.getaddrinfo=forbidden\n"
            f"assert Path.cwd()==Path({str(SOURCE_DIR)!r})\n"
            "assert not any(os.environ.get(k) for k in ['AWS_ACCESS_KEY_ID','AWS_SECRET_ACCESS_KEY','AWS_SESSION_TOKEN'])\n"
            f"ctx=Path({str(CHILD_CONTEXT)!r});data=json.loads(ctx.read_text())\n"
            "assert data=={'fixtureOnly':True,'canary':'context-read-proof'}\n"
            "try:\n ctx.open('a').close()\n raise AssertionError('context must be read only')\n"
            "except OSError as e:\n assert e.errno in [errno.EROFS,errno.EACCES]\n"
            f"hidden={[str(directory / name) for name in ['campaign.sqlite', 'private-canary', 'collect-readers-context.json']] + [str(directory.parent / name) for name in ['window-from-390.json', 'authorization-from-390.json']]!r}\n"
            "assert all(not Path(x).exists() for x in hidden)\n"
            "assert not (Path.home()/'.aws').exists()\n"
            "op=sys.argv[1:3]\n"
            "assert op in [['sts','get-caller-identity'],['dynamodb','get-item']]\n"
            "with Path('/work/synthetic-replies.jsonl').open('a') as f: f.write(json.dumps({'operation':op,'isolationChecked':True})+'\\n')\n"
            f"print(json.dumps({identity!r}) if op[0]=='sts' else json.dumps({{'Item':{{}}}}))\n"
        )
        fake.chmod(0o700)
        calls = []

        def fixture_launch(*args, **kwargs):
            assert args[0] == SOURCE_DIR
            assert args[2][-1] == str(CHILD_CONTEXT)
            kwargs["extra_ro"] = [*kwargs["extra_ro"], (fake, Path("/usr/bin/aws"))]
            result = launch(*args, **kwargs)
            calls.append(result)
            return result

        with patch("driver.launch", side_effect=fixture_launch):
            try:
                child(
                    c,
                    "collect",
                    "collect-readers",
                    {"fixtureOnly": True, "canary": "context-read-proof"},
                    {},
                )
            except Held:
                pass
            else:
                raise AssertionError(
                    "Synthetic empty marker must stop the actual collector"
                )
        assert len(calls) == 1 and calls[0]["exitCode"] == 1
        output = directory / "collect-readers"
        replies = [
            json.loads(line)
            for line in (output / "work/synthetic-replies.jsonl")
            .read_text()
            .splitlines()
        ]
        assert [r["operation"] for r in replies] == [
            ["sts", "get-caller-identity"],
            ["dynamodb", "get-item"],
        ]
        assert all(r["isolationChecked"] for r in replies)
        assert c.db.execute("SELECT COUNT(*) FROM attempts").fetchone()[0] == 0
        assert c.get("count") == PRIOR
        summary = json.loads((output / "proxy-summary.json").read_bytes())
        assert not summary["identityVerified"] and summary["confirmedUploads"] == 0
        raw = (output / "private-output.txt").read_text()
        assert "ENOENT" not in raw and "Execution phase held/uncertain" in raw
        print(
            json.dumps(
                {
                    "case": "documented-relative-campaign-real-entrypoint-context",
                    "fixtureOnly": True,
                    "awsCalls": 0,
                    "networkForwards": 0,
                    "liveCredentials": False,
                    "syntheticReplies": len(replies),
                    "sameReleaseWorkingDirectory": True,
                    "actualChildEntryPoint": True,
                    "readOnlyContextRead": True,
                    "campaignAndClaimIsolationChecked": True,
                    "syntheticInvalidMarkerStoppedCollector": True,
                    "priorUsageUnchanged": PRIOR,
                }
            )
        )
    finally:
        c.close()


if __name__ == "__main__":
    main()
