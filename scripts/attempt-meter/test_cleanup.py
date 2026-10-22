import json
from pathlib import Path
import tempfile
import unittest
from budget import Budget, MeterStop
from cleanup import parent_proof, cleanup_allowed, validate_scope


class CleanupTests(unittest.TestCase):
    def test_only_known_blocked_request_with_no_uncertain_writes(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "parent.jsonl"
            rows = [
                {"event": "meter-created"},
                {
                    "event": "attempt",
                    "sequence": 2,
                    "at": 100,
                    "write": True,
                    "counts": {"api": 62, "aws": 766, "poll": 7, "static": 9},
                },
                {
                    "event": "outcome",
                    "attempt": 2,
                    "status": 202,
                    "transportError": False,
                },
                {"event": "stop", "reason": "write-held"},
            ]

            def write():
                path.write_text("\n".join(map(json.dumps, rows)) + "\n")

            write()
            proof = parent_proof(path)
            self.assertEqual(proof["counts"]["aws"], 766)
            b = Budget(
                Path(directory) / "cleanup", clock=lambda: 200, cleanup_parent=proof
            )
            self.assertEqual(b.counts, proof["counts"])
            self.assertEqual(b.limits["api"], 122)
            self.assertEqual(b.limits["aws"], 966)
            self.assertEqual(b.seconds, 1300)
            for _ in range(60):
                b.admit("api", "GET", False)
            with self.assertRaises(MeterStop):
                b.admit("api", "GET", False)
            b.close()
            rows[2]["transportError"] = True
            write()
            with self.assertRaises(ValueError):
                parent_proof(path)
            rows[2]["transportError"] = False
            rows[3]["reason"] = "uncertain-write"
            write()
            with self.assertRaises(ValueError):
                parent_proof(path)
            rows.pop(2)
            rows[-1]["reason"] = "write-held"
            write()
            with self.assertRaises(ValueError):
                parent_proof(path)

    def test_scope_denies_every_campaign_and_stage_write(self):
        scope = validate_scope(
            {
                "projects": [
                    "11111111-1111-4111-8111-111111111111",
                    "22222222-2222-4222-8222-222222222222",
                ],
                "users": [
                    "kurier-assertions-11111111-1111-4111-8111-111111111111-0@example.invalid",
                    "kurier-assertions-11111111-1111-4111-8111-111111111111-1@example.invalid",
                ],
            }
        )
        for method in ["GET", "DELETE"]:
            self.assertTrue(
                cleanup_allowed(
                    "api",
                    method,
                    "",
                    "/api/v1/projects/" + scope["projects"][0],
                    "",
                    b"",
                    scope,
                )
            )
        for method, path in [
            ("POST", "/api/v1/projects"),
            ("POST", "/api/v1/projects/" + scope["projects"][0] + "/workflows/x/runs"),
            ("DELETE", "/api/v1/projects/33333333-3333-4333-8333-333333333333"),
        ]:
            self.assertFalse(cleanup_allowed("api", method, "", path, "", b"", scope))
        for operation in [
            "TransactWriteItems",
            "AdminCreateUser",
            "AdminSetUserPassword",
            "PUT",
            "POST",
            "DELETE",
            "OPTIONS",
        ]:
            self.assertFalse(
                cleanup_allowed(
                    "aws",
                    "POST",
                    "dynamodb.us-east-2.amazonaws.com",
                    "/",
                    operation,
                    b"{}",
                    scope,
                )
            )
        for operation in ["AdminDeleteUser", "AdminUserGlobalSignOut"]:
            good = json.dumps(
                {"UserPoolId": "us-east-2_qTDZQT1FE", "Username": scope["users"][0]}
            ).encode()
            self.assertTrue(
                cleanup_allowed(
                    "aws",
                    "POST",
                    "cognito-idp.us-east-2.amazonaws.com",
                    "/",
                    operation,
                    good,
                    scope,
                )
            )
            self.assertFalse(
                cleanup_allowed(
                    "aws",
                    "POST",
                    "cognito-idp.us-east-2.amazonaws.com",
                    "/",
                    operation,
                    b'{"UserPoolId":"foreign","Username":"foreign"}',
                    scope,
                )
            )

    def test_cleanup_time_and_absolute_caps_cannot_reset(self):
        with tempfile.TemporaryDirectory() as directory:
            now = [200]
            proof = {
                "firstAttemptAt": 100,
                "counts": {"api": 490, "aws": 1490, "poll": 7, "static": 9},
                "sha256": "0" * 64,
            }
            b = Budget(
                Path(directory) / "caps", clock=lambda: now[0], cleanup_parent=proof
            )
            self.assertEqual(b.limits["api"], 500)
            self.assertEqual(b.limits["aws"], 1500)
            now[0] = 1400
            with self.assertRaises(MeterStop):
                b.admit("aws", "GetItem", False)
            self.assertEqual(b.counts, proof["counts"])
            b.close()
            now[0] = 7100
            c = Budget(
                Path(directory) / "deadline", clock=lambda: now[0], cleanup_parent=proof
            )
            self.assertEqual(c.seconds, 7200)
            now[0] = 7300
            with self.assertRaises(MeterStop):
                c.admit("api", "GET", False)
            c.close()
