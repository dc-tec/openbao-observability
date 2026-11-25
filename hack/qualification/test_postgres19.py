"""Non-service checks for qualification failure handling and TCP authentication."""

import json
import subprocess
import unittest
from unittest.mock import patch

import postgres19 as qualification


class QualificationTests(unittest.TestCase):
    def test_failed_container_command_does_not_expose_credentials(self):
        result = subprocess.CompletedProcess([], 1, b"", b"sensitive-password")
        with (
            patch.object(qualification.subprocess, "run", return_value=result),
            self.assertRaisesRegex(RuntimeError, r"^docker exec failed with status 1$"),
        ):
            qualification.docker("exec", "--env", "PGPASSWORD=sensitive-password")

    def test_image_evidence_excludes_environment(self):
        info = [
            {
                "Id": "sha256:synthetic",
                "RepoDigests": ["image@sha256:synthetic"],
                "Os": "linux",
                "Architecture": "amd64",
                "Config": {"Env": ["secret"]},
            }
        ]
        result = subprocess.CompletedProcess([], 0, json.dumps(info).encode(), b"")
        with patch.object(qualification, "docker", return_value=result):
            self.assertNotIn("Config", qualification.image_identity("image"))

    def test_issued_credentials_use_tcp_and_stop_on_sql_errors(self):
        fixture = qualification.Fixture()
        self.addCleanup(fixture.directory.cleanup)
        with patch.object(qualification, "docker") as command:
            fixture.sql(
                "owned-container",
                "SELECT 1",
                user="issued-user",
                password="issued-password",
            )
            args = command.call_args.args
            self.assertIn("PGPASSWORD=issued-password", args)
            self.assertIn("issued-user", args)
            self.assertEqual(args[args.index("-h") + 1], "127.0.0.1")
            self.assertIn("ON_ERROR_STOP=1", args)

    def test_missing_restored_rows_fails(self):
        fixture = qualification.Fixture()
        self.addCleanup(fixture.directory.cleanup)
        secret = {"data": {"username": "issued", "password": "synthetic"}}
        with (
            patch.object(
                fixture,
                "sql",
                return_value=subprocess.CompletedProcess([], 0, b"", b""),
            ),
            self.assertRaises(AssertionError),
        ):
            fixture.read_rows("owned-container", secret)

    def test_revocation_requires_authentication_failure(self):
        fixture = qualification.Fixture()
        self.addCleanup(fixture.directory.cleanup)
        secret = {
            "lease_id": "synthetic-lease",
            "data": {"username": "issued", "password": "synthetic"},
        }
        with (
            patch.object(fixture, "api"),
            patch.object(
                fixture,
                "sql",
                return_value=subprocess.CompletedProcess([], 0, b"1", b""),
            ),
            self.assertRaisesRegex(
                AssertionError, "revoked database credentials still authenticate"
            ),
        ):
            fixture.revoke("owned-container", secret)


if __name__ == "__main__":
    unittest.main()
