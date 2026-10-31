"""Local path and exact child-command regressions; no credentials or network."""

import json
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch

from common import ROOT, Held
from driver import (
    CHILD_CONTEXT,
    campaign_path,
    child,
    mounts,
    phase_command,
    required_path,
)
from isolation import launch


class PathTests(unittest.TestCase):
    def test_documented_relative_campaign_resolves_from_repository(self):
        relative = Path(".build/project-environments-execution/release-499-p1")
        self.assertEqual(campaign_path(relative), ROOT / relative)

    def test_outside_and_parent_traversal_campaign_paths_hold(self):
        for path in [
            Path("/tmp/unowned"),
            Path(".build/project-environments-execution/../other/release"),
        ]:
            with self.subTest(path=path), self.assertRaises(Held):
                campaign_path(path)

    def test_context_mount_is_one_regular_file_at_fixed_read_only_destination(self):
        with tempfile.TemporaryDirectory() as directory:
            context = Path(directory) / "collect-readers-context.json"
            context.write_text("{}")
            actual = mounts(context)
            self.assertEqual(actual[-1], (context, CHILD_CONTEXT))
            self.assertEqual(
                [src for src, _ in actual if src.parent == context.parent], [context]
            )
            self.assertNotIn(context.parent, [src for src, _ in actual])
            self.assertTrue(
                all(src.is_absolute() and dst.is_absolute() for src, dst in actual)
            )

    def test_missing_directory_and_symlink_contexts_are_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            target = root / "target.json"
            target.write_text("{}")
            link = root / "link.json"
            link.symlink_to(target)
            for path in [root, link]:
                with self.subTest(path=path), self.assertRaises(Held):
                    mounts(path)
            with self.assertRaises(FileNotFoundError):
                mounts(root / "missing.json")

    def test_symlink_parent_and_relative_files_are_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            target = root / "target"
            target.mkdir()
            (target / "context.json").write_text("{}")
            link = root / "alias"
            link.symlink_to(target, target_is_directory=True)
            for path in [link / "context.json", Path("context.json")]:
                with self.subTest(path=path), self.assertRaises(Held):
                    required_path(path, file=True)

    def test_all_sandbox_phases_share_context_mapping_and_absolute_entrypoint(self):
        cases = [
            ("collect", "collect-readers", "collect"),
            ("collect", "collect-diff", "diff"),
            *[
                (phase, phase, phase)
                for phase in ["close", "deploy", "verify", "activate", "final"]
            ],
        ]
        with tempfile.TemporaryDirectory() as directory:
            campaign = SimpleNamespace(directory=Path(directory), get=lambda _: False)

            def fake_launch(source, output, command, _creds, **options):
                self.assertTrue(source.is_absolute())
                self.assertTrue(output.is_absolute())
                self.assertEqual(command[-1], str(CHILD_CONTEXT))
                self.assertTrue(Path(command[1]).is_absolute())
                src, dest = options["extra_ro"][-1]
                self.assertTrue(src.is_file())
                self.assertEqual(json.loads(src.read_text()), {"canary": "synthetic"})
                self.assertEqual(dest, CHILD_CONTEXT)
                output.mkdir()
                (output / "proxy-summary.json").write_text('{"identityVerified":true}')
                return {"exitCode": 0}

            with patch("driver.launch", side_effect=fake_launch) as invoke:
                for phase, subphase, action in cases:
                    with self.subTest(phase=phase, subphase=subphase):
                        child(campaign, phase, subphase, {"canary": "synthetic"}, {})
                        self.assertEqual(invoke.call_args.args[2][2], action)

    def test_offline_stage_plan_uses_validated_absolute_host_context(self):
        with tempfile.TemporaryDirectory() as directory:
            context = Path(directory) / "close-plan-input.json"
            context.write_text("{}")
            self.assertEqual(phase_command("plan", context)[-1], str(context))
            with self.assertRaises(Held):
                phase_command("plan", Path("close-plan-input.json"))

    def test_unknown_commands_and_context_destinations_hold(self):
        with self.assertRaises(Held):
            phase_command("arbitrary-command", CHILD_CONTEXT)
        with self.assertRaises(Held):
            phase_command("collect", Path("relative-context.json"))

    def test_mount_validation_precedes_any_child_or_proxy_start(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            with patch("isolation.subprocess.Popen") as spawn:
                with self.assertRaises(Held):
                    launch(
                        root,
                        root / "output",
                        [],
                        {},
                        campaign=None,
                        phase="collect",
                        policy_context={},
                        extra_ro=[(Path("relative.json"), CHILD_CONTEXT)],
                    )
                spawn.assert_not_called()
            self.assertFalse((root / "output").exists())


if __name__ == "__main__":
    unittest.main()
