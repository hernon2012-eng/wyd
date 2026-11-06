from copy import deepcopy
import json
import unittest
from pathlib import Path
from unittest.mock import patch

from tools import validate_json_catalog
from tools.validate_json_catalog import (
    DuplicateObjectKeyError,
    parse_json_text,
    validate_document,
    validate_files,
)


FIXTURES = Path(__file__).parent / "fixtures"
SCHEMA = FIXTURES / "DEMONSTRAÇÃO-schema.json"


class ValidateJsonCatalogTests(unittest.TestCase):
    def test_synthetic_schema_accepts_document_without_changing_identifiers(self):
        source = FIXTURES / "DEMONSTRAÇÃO-valid.json"
        original = json.loads(source.read_text(encoding="utf-8"))
        schema = json.loads(SCHEMA.read_text(encoding="utf-8"))
        before_validation = deepcopy(original)
        errors = validate_document(
            original,
            schema,
            records_pointer="/records",
            unique_key="source_identifier",
        )

        self.assertEqual(errors, [])
        self.assertEqual(original, before_validation)
        self.assertEqual(
            [row["source_identifier"] for row in original["records"]],
            ["DEMO-SYNTHETIC-001", "DEMO-SYNTHETIC-002"],
        )

    def test_rejects_duplicate_values_for_requested_record_key(self):
        report = validate_files(
            FIXTURES / "DEMONSTRAÇÃO-duplicate-values.json",
            SCHEMA,
            records_pointer="/records",
            unique_key="source_identifier",
        )

        self.assertFalse(report["valid"])
        self.assertEqual(report["errors"][0]["code"], "duplicate_record_value")
        self.assertEqual(report["errors"][0]["instance_path"], "/records/1/source_identifier")

    def test_unique_check_is_opt_in(self):
        report = validate_files(
            FIXTURES / "DEMONSTRAÇÃO-duplicate-values.json",
            SCHEMA,
            records_pointer="/records",
        )
        self.assertTrue(report["valid"])

    def test_reports_schema_validation_paths(self):
        errors = validate_files(
            FIXTURES / "DEMONSTRAÇÃO-invalid-schema-data.json",
            SCHEMA,
            records_pointer="/records",
        )["errors"]

        self.assertTrue(any(error["code"] == "schema_validation" for error in errors))
        self.assertTrue(
            any(error["instance_path"] == "/records/0/label" for error in errors)
        )

    def test_rejects_duplicate_object_keys(self):
        with self.assertRaises(DuplicateObjectKeyError):
            parse_json_text('{"outer":{"id":"first","id":"second"}}')

    def test_rejects_non_json_constants(self):
        with self.assertRaisesRegex(ValueError, "invalid JSON constant"):
            parse_json_text('{"value":NaN}')

    def test_reports_malformed_json_as_structured_error(self):
        report = validate_files(FIXTURES / "DEMONSTRAÇÃO-malformed.json", SCHEMA)

        self.assertFalse(report["valid"])
        self.assertEqual(report["errors"][0]["code"], "input_json_error")
        self.assertIn("line:", report["errors"][0]["instance_path"])

    def test_rejects_external_schema_references(self):
        errors = validate_document({}, {"$ref": "https://example.invalid/schema.json"})
        self.assertEqual(errors[0]["code"], "external_schema_reference")

    def test_reports_unresolved_local_schema_references(self):
        errors = validate_document({}, {"$ref": "#/$defs/missing"})

        self.assertEqual(errors[0]["code"], "unresolved_schema_reference")
        self.assertIn("/$defs/missing", errors[0]["message"])

    def test_does_not_swallow_unrelated_validator_failures(self):
        class BrokenValidator:
            def __init__(self, schema):
                pass

            @staticmethod
            def check_schema(schema):
                pass

            @staticmethod
            def iter_errors(document):
                raise RuntimeError("unexpected validator failure")

        with patch.object(validate_json_catalog, "validator_for", return_value=BrokenValidator):
            with self.assertRaisesRegex(RuntimeError, "unexpected validator failure"):
                validate_document({}, {})

    def test_reports_records_missing_the_requested_unique_key(self):
        errors = validate_document(
            {"records": [{"id": "present"}, {}]},
            {"type": "object"},
            records_pointer="/records",
            unique_key="id",
        )

        self.assertEqual(
            [error["code"] for error in errors],
            ["missing_record_key"],
        )
        self.assertEqual(errors[0]["instance_path"], "/records/1/id")

    def test_invalid_records_pointer_is_reported(self):
        errors = validate_document(
            {"records": []},
            {"type": "object"},
            records_pointer="/absent",
            unique_key="source_identifier",
        )
        self.assertTrue(any(error["code"] == "records_pointer" for error in errors))


if __name__ == "__main__":
    unittest.main()
