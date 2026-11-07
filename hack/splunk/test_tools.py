"""Offline regression checks for credentials, API boundaries, and ACL rollback."""

import io
import json
import os
import stat
import tempfile
import threading
import unittest
import urllib.error
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import Mock, patch

import access_checks
import archive_status
import local
from support import write_private


class StateTests(unittest.TestCase):
    def test_launchers_read_shared_compose_pins_and_explicit_image_overrides(self):
        with patch.object(local, "environment", return_value={}), patch.dict(os.environ, {}, clear=True):
            for name in ("SPLUNK_IMAGE", "SPLUNK_OTEL_IMAGE", "SPLUNK_BLACKBOX_IMAGE"):
                self.assertIn("@sha256:", local.image(name))
                self.assertTrue(local.image(name).startswith("docker.io/"))
            self.assertTrue(local.image("OPENBAO_IMAGE").startswith("quay.io/openbao/openbao:"))
            with patch.dict(os.environ, {"SPLUNK_IMAGE": "registry.example.test/splunk:fixture"}):
                self.assertEqual(local.image("SPLUNK_IMAGE"), "registry.example.test/splunk:fixture")
            with self.assertRaises(RuntimeError):
                local.image("UNDECLARED_IMAGE")

    def test_setup_requires_acceptance_and_preserves_credentials(self):
        with tempfile.TemporaryDirectory() as directory, patch.object(local, "DIRECTORY", Path(directory)):
            with self.assertRaises(RuntimeError):
                local.setup(False)
            self.assertFalse((Path(directory) / ".env").exists())
            local.setup(True)
            path = Path(directory) / ".env"
            first = path.read_bytes()
            self.assertEqual(stat.S_IMODE(path.stat().st_mode), 0o600)
            local.setup(True)
            self.assertEqual(path.read_bytes(), first)

    def test_failed_state_replacement_preserves_previous_private_state(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "state.json"
            write_private(path, "original")
            with patch.object(Path, "replace", side_effect=OSError("disk write failed")):
                with self.assertRaises(OSError):
                    write_private(path, "replacement")
            self.assertEqual(path.read_text(), "original")
            self.assertEqual(stat.S_IMODE(path.stat().st_mode), 0o600)
            self.assertEqual(list(Path(directory).iterdir()), [path])

    def test_archive_health_requires_a_complete_log_record(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            self.assertFalse(archive_status.observe(root)["delivery_success"])
            archive = root / "audit.jsonl"
            archive.write_text('{"resourceLogs":[')
            self.assertFalse(archive_status.observe(root)["delivery_success"])
            record = {"resourceLogs": [{"scopeLogs": [{"logRecords": [{"body": {"stringValue": "fixture"}}]}]}]}
            archive.write_text(json.dumps(record) + "\n")
            result = archive_status.observe(root)
            self.assertTrue(result["delivery_success"])
            self.assertEqual(result["last_success_timestamp_seconds"], archive.stat().st_mtime)


class APITests(unittest.TestCase):
    def test_multivalue_form_encoding_and_empty_delete_response(self):
        response = Mock()
        response.__enter__ = Mock(return_value=response)
        response.__exit__ = Mock(return_value=False)
        response.read.return_value = b"{}"
        opener = Mock()
        opener.open.return_value = response
        with patch.object(local, "OPENER", opener):
            local.request("/services/example", {"roles": ["one", "two"]}, "Splunk fixture")
            request = opener.open.call_args.args[0]
            self.assertEqual(request.data, b"roles=one&roles=two")
            self.assertEqual(request.headers["Authorization"], "Splunk fixture")
            response.read.return_value = b""
            local.delete("/services/example", "Splunk fixture")
            self.assertEqual(opener.open.call_args.args[0].get_method(), "DELETE")

    def test_error_does_not_echo_sensitive_server_body(self):
        body = io.BytesIO(b'{"messages":[{"text":"password=must-not-appear"}]}')
        error = urllib.error.HTTPError(local.API, 500, "failed", None, body)
        with patch.object(local.OPENER, "open", side_effect=error):
            with self.assertRaises(local.APIError) as raised:
                local.request("/services/authentication/users", {"password": "must-not-appear"})
        self.assertEqual(raised.exception.status, 500)
        self.assertNotIn("must-not-appear", str(raised.exception))

    def test_redirect_is_rejected_before_the_target_receives_credentials(self):
        visited = []

        class Handler(BaseHTTPRequestHandler):
            def do_GET(self):
                visited.append(self.path)
                self.send_response(302)
                self.send_header("Location", "/credential-target")
                self.end_headers()

            def log_message(self, *_args):
                pass

        server = HTTPServer(("127.0.0.1", 0), Handler)
        worker = threading.Thread(target=server.serve_forever, daemon=True)
        worker.start()
        try:
            with patch.object(local, "API", f"http://127.0.0.1:{server.server_port}"):
                with self.assertRaisesRegex(RuntimeError, "Redirects are disabled"):
                    local.request("/services/example", auth="Splunk fixture")
            self.assertEqual(len(visited), 1)
        finally:
            server.shutdown()
            server.server_close()
            worker.join()

    def test_wait_has_a_bounded_failure_path(self):
        with (
            patch.object(local, "environment", return_value={"SPLUNK_PASSWORD": "fixture"}),
            patch.object(local, "request", side_effect=urllib.error.URLError("not ready")),
            patch.object(local.time, "sleep"),
            patch.object(local.time, "monotonic", side_effect=[0, 0, 0, 1, 2]),
        ):
            with self.assertRaisesRegex(RuntimeError, "deadline"):
                local.wait_ready(1)


class AccessTests(unittest.TestCase):
    def test_partial_role_creation_rolls_back_recorded_resources(self):
        with tempfile.TemporaryDirectory() as directory:
            fake = SimpleNamespace(DIRECTORY=Path(directory), APIError=local.APIError, session=lambda: "fixture")
            fake.request = Mock(side_effect=[{}, local.APIError(500, "/services/authentication/users")])
            fake.delete = Mock()
            with self.assertRaises(local.APIError):
                access_checks.run(fake)
            removed = [call.args[0] for call in fake.delete.call_args_list]
            self.assertEqual(len(removed), 2)
            self.assertTrue(all("openbao_qualification_reader_" in name for name in removed))
            self.assertFalse((fake.DIRECTORY / ".runtime/access-private.json").exists())

    def test_cleanup_failure_keeps_recovery_state_for_retry(self):
        with tempfile.TemporaryDirectory() as directory:
            fake = SimpleNamespace(DIRECTORY=Path(directory), APIError=local.APIError, session=lambda: "fixture")
            private = fake.DIRECTORY / ".runtime/access-private.json"
            access_checks.save(
                private,
                {
                    "accounts": [],
                    "acls": [
                        {
                            "path": "/fixture",
                            "original": {
                                "owner": "nobody",
                                "sharing": "app",
                                "perms": {"read": ["admin"], "write": ["admin"]},
                            },
                        }
                    ],
                },
            )
            fake.request = Mock(side_effect=local.APIError(500, "/fixture/acl"))
            with self.assertRaises(local.APIError):
                access_checks.cleanup(fake)
            self.assertTrue(private.exists())
            fake.request = Mock()
            access_checks.cleanup(fake)
            self.assertFalse(private.exists())

    def test_browser_state_is_retained_only_after_success(self):
        fake = SimpleNamespace()
        with patch.object(access_checks, "qualify"), patch.object(access_checks, "cleanup") as cleanup:
            access_checks.run(fake)
            cleanup.assert_called_once_with(fake)
            cleanup.reset_mock()
            access_checks.run(fake, keep_for_browser=True)
            cleanup.assert_not_called()
        with (
            patch.object(access_checks, "qualify", side_effect=KeyboardInterrupt),
            patch.object(access_checks, "cleanup") as cleanup,
        ):
            with self.assertRaises(KeyboardInterrupt):
                access_checks.run(fake, keep_for_browser=True)
            cleanup.assert_called_once_with(fake)


if __name__ == "__main__":
    unittest.main()
