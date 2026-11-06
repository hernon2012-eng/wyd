#!/usr/bin/env python3
"""Preflight JSON documents against caller-supplied JSON Schema files."""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path
from typing import Any

from jsonschema.exceptions import SchemaError

from referencing.exceptions import Unresolvable
from jsonschema.validators import validator_for


class DuplicateObjectKeyError(ValueError):
    """Raised when a JSON object contains a repeated member name."""


def _object_without_duplicate_keys(pairs: list[tuple[str, Any]]) -> dict[str, Any]:
    result: dict[str, Any] = {}
    for key, value in pairs:
        if key in result:
            raise DuplicateObjectKeyError(f"duplicate object key: {key!r}")
        result[key] = value
    return result


def _reject_non_json_constant(value: str) -> None:
    raise ValueError(f"invalid JSON constant: {value}")


def parse_json_text(text: str) -> Any:
    """Parse strict JSON, rejecting duplicate object keys and non-JSON constants."""
    return json.loads(
        text,
        object_pairs_hook=_object_without_duplicate_keys,
        parse_constant=_reject_non_json_constant,
    )


def _error(
    code: str,
    message: str,
    *,
    instance_path: str = "",
    schema_path: str | None = None,
    keyword: str | None = None,
) -> dict[str, Any]:
    result: dict[str, Any] = {
        "code": code,
        "message": message,
        "instance_path": instance_path,
    }
    if schema_path is not None:
        result["schema_path"] = schema_path
    if keyword is not None:
        result["keyword"] = keyword
    return result


def _json_pointer(parts: Any) -> str:
    return "".join(
        "/" + str(part).replace("~", "~0").replace("/", "~1") for part in parts
    )


def _decode_pointer(pointer: str) -> list[str]:
    if pointer == "":
        return []
    if not pointer.startswith("/"):
        raise ValueError("JSON Pointer must be empty or begin with '/'")

    segments: list[str] = []
    for segment in pointer[1:].split("/"):
        if re.search(r"~(?![01])", segment):
            raise ValueError(f"invalid escape in JSON Pointer segment: {segment!r}")
        segments.append(segment.replace("~1", "/").replace("~0", "~"))
    return segments


def _at_pointer(document: Any, pointer: str) -> Any:
    value = document
    for segment in _decode_pointer(pointer):
        if isinstance(value, dict) and segment in value:
            value = value[segment]
        elif isinstance(value, list) and segment.isdigit():
            index = int(segment)
            if str(index) != segment or index >= len(value):
                raise ValueError(f"JSON Pointer does not exist: {pointer!r}")
            value = value[index]
        else:
            raise ValueError(f"JSON Pointer does not exist: {pointer!r}")
    return value


def _has_external_reference(schema: Any) -> bool:
    if isinstance(schema, dict):
        for key, value in schema.items():
            if key in {"$ref", "$dynamicRef", "$recursiveRef"}:
                if not isinstance(value, str) or not value.startswith("#"):
                    return True
            if _has_external_reference(value):
                return True
    elif isinstance(schema, list):
        return any(_has_external_reference(item) for item in schema)
    return False


def validate_document(
    document: Any,
    schema: Any,
    *,
    records_pointer: str = "",
    unique_key: str | None = None,
) -> list[dict[str, Any]]:
    """Return schema and optional record-key validation errors."""
    errors: list[dict[str, Any]] = []

    if not isinstance(schema, dict):
        return [_error("invalid_schema", "JSON Schema root must be an object")]

    try:
        validator_class = validator_for(schema)
        validator_class.check_schema(schema)
    except SchemaError as exc:
        return [_error("invalid_schema", str(exc))]

    if _has_external_reference(schema):
        return [
            _error(
                "external_schema_reference",
                "only references within the supplied schema document are supported",
            )
        ]

    validator = validator_class(schema)
    try:
        validation_errors = sorted(
            validator.iter_errors(document),
            key=lambda item: (
                _json_pointer(item.absolute_path),
                item.validator or "",
                item.message,
            ),
        )
    except Unresolvable as exc:
        return [_error("unresolved_schema_reference", str(exc))]

    for item in validation_errors:
        errors.append(
            _error(
                "schema_validation",
                item.message,
                instance_path=_json_pointer(item.absolute_path),
                schema_path=_json_pointer(item.absolute_schema_path),
                keyword=item.validator,
            )
        )

    if unique_key is not None:
        try:
            records = _at_pointer(document, records_pointer)
        except ValueError as exc:
            errors.append(_error("records_pointer", str(exc)))
        else:
            if not isinstance(records, list):
                errors.append(
                    _error(
                        "records_not_array",
                        f"records pointer {records_pointer!r} does not identify an array",
                        instance_path=records_pointer,
                    )
                )
            else:
                seen: dict[str, int] = {}
                for index, record in enumerate(records):
                    if not isinstance(record, dict) or unique_key not in record:
                        errors.append(
                            _error(
                                "missing_record_key",
                                f"record does not contain key {unique_key!r}",
                                instance_path=_json_pointer(
                                    _decode_pointer(records_pointer)
                                    + [str(index), unique_key]
                                ),
                                keyword="unique_key",
                            )
                        )
                        continue
                    try:
                        canonical = json.dumps(
                            record[unique_key],
                            ensure_ascii=True,
                            sort_keys=True,
                            separators=(",", ":"),
                            allow_nan=False,
                        )
                    except (TypeError, ValueError):
                        continue
                    if canonical in seen:
                        errors.append(
                            _error(
                                "duplicate_record_value",
                                f"duplicate value for key {unique_key!r}; "
                                f"first seen in record {seen[canonical]}",
                                instance_path=_json_pointer(
                                    _decode_pointer(records_pointer)
                                    + [str(index), unique_key]
                                ),
                                keyword="unique_key",
                            )
                        )
                    else:
                        seen[canonical] = index

    errors.sort(
        key=lambda item: (
            item["instance_path"],
            item["code"],
            item["message"],
        )
    )
    return errors


def _read_json(path: Path, role: str) -> tuple[Any | None, dict[str, Any] | None]:
    try:
        text = path.read_text(encoding="utf-8")
    except (OSError, UnicodeError) as exc:
        return None, _error(f"{role}_read_error", str(exc))

    try:
        return parse_json_text(text), None
    except json.JSONDecodeError as exc:
        return None, _error(
            f"{role}_json_error",
            exc.msg,
            instance_path=f"line:{exc.lineno}:column:{exc.colno}",
        )
    except (DuplicateObjectKeyError, ValueError) as exc:
        return None, _error(f"{role}_json_error", str(exc))


def validate_files(
    input_path: Path,
    schema_path: Path,
    *,
    records_pointer: str = "",
    unique_key: str | None = None,
) -> dict[str, Any]:
    report: dict[str, Any] = {
        "tool": "json-catalog-preflight",
        "valid": False,
        "input": str(input_path),
        "schema": str(schema_path),
        "records_pointer": records_pointer,
        "unique_key": unique_key,
        "errors": [],
    }

    document, input_error = _read_json(input_path, "input")
    schema, schema_error = _read_json(schema_path, "schema")
    errors = [error for error in (input_error, schema_error) if error is not None]

    if not errors:
        errors = validate_document(
            document,
            schema,
            records_pointer=records_pointer,
            unique_key=unique_key,
        )

    report["errors"] = errors
    report["valid"] = not errors
    report["summary"] = {"error_count": len(errors)}
    return report


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(
        description="Validate a JSON document using a caller-provided JSON Schema."
    )
    parser.add_argument("--input", required=True, type=Path, help="JSON document")
    parser.add_argument("--schema", required=True, type=Path, help="JSON Schema file")
    parser.add_argument(
        "--records-pointer",
        default="",
        help="JSON Pointer to the records array (default: document root)",
    )
    parser.add_argument(
        "--unique-key",
        help="optional object member whose values must be unique in that array",
    )
    args = parser.parse_args(argv)

    report = validate_files(
        args.input,
        args.schema,
        records_pointer=args.records_pointer,
        unique_key=args.unique_key,
    )
    print(json.dumps(report, ensure_ascii=True, indent=2))
    return 0 if report["valid"] else 1


if __name__ == "__main__":
    sys.exit(main())
