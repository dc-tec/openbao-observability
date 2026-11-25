"""Non-service regression checks for qualification assertions."""

import unittest

import exporter_builder as qualification


class QualificationTests(unittest.TestCase):
    def test_candidate_recipe_preserves_every_other_instruction(self):
        source = qualification.DOCKERFILE.read_text()
        candidate = qualification.candidate_recipe(source, "sha256:" + "a" * 64)
        self.assertEqual(source.split("\n", 1)[1], candidate.split("\n", 1)[1])
        self.assertTrue(candidate.startswith("FROM golang:1.27.4-alpine@sha256:"))

    def test_changed_builder_requires_review(self):
        with self.assertRaises(AssertionError):
            qualification.candidate_recipe(
                "FROM unexpected AS build\n", "sha256:" + "a" * 64
            )

    def test_missing_metric_fails(self):
        with self.assertRaises(AssertionError):
            qualification.verify_metrics("", {"enabled": 1})

    def test_wrong_metric_value_fails(self):
        text = "openbao_audit_archive_enabled" + qualification.LABELS + " 0\n"
        with self.assertRaises(AssertionError):
            qualification.verify_metrics(text, {"enabled": 1})

    def test_wrong_metric_labels_fail(self):
        with self.assertRaises(AssertionError):
            qualification.verify_metrics(
                'openbao_audit_archive_enabled{backend="other"} 1\n', {"enabled": 1}
            )

    def test_valid_metrics_pass(self):
        text = (
            "# TYPE openbao_audit_archive_enabled gauge\nopenbao_audit_archive_enabled"
            + qualification.LABELS
            + " 1\n"
        )
        qualification.verify_metrics(text, {"enabled": 1})

    def test_duplicate_metrics_fail(self):
        line = "openbao_audit_archive_enabled" + qualification.LABELS + " 1\n"
        with self.assertRaises(AssertionError):
            qualification.verify_metrics(line + line, {"enabled": 1})

    def test_scratch_rejects_root_user_and_extra_layers(self):
        image = {
            "Config": {"User": "0", "Entrypoint": ["/" + qualification.BINARY]},
            "RootFS": {"Layers": ["one"]},
        }
        with self.assertRaises(AssertionError):
            qualification.verify_scratch(image, b"\x7fELF")
        image["Config"]["User"] = "65532:65532"
        image["RootFS"]["Layers"].append("unexpected")
        with self.assertRaises(AssertionError):
            qualification.verify_scratch(image, b"\x7fELF")


if __name__ == "__main__":
    unittest.main()
