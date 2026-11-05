import json
from pathlib import Path

from jsonschema import Draft202012Validator

ROOT = Path(__file__).resolve().parents[1]
SCHEMA_PATH = ROOT / "schemas/v1/multi-character-account.schema.json"
schema = json.loads(SCHEMA_PATH.read_text(encoding="utf-8"))
Draft202012Validator.check_schema(schema)
validator = Draft202012Validator(schema)

example_count = 0
for example_path in sorted((ROOT / "examples").glob("*.json")):
    examples = json.loads(example_path.read_text(encoding="utf-8"))
    if not isinstance(examples, list):
        examples = [examples]
    for index, example in enumerate(examples):
        errors = sorted(validator.iter_errors(example), key=lambda error: list(error.path))
        if errors:
            details = "; ".join(error.message for error in errors)
            raise SystemExit(f"{example_path.name}[{index}] is invalid: {details}")
        example_count += 1

if not example_count:
    raise SystemExit("no schema examples were validated")
print(f"validated schema and {example_count} synthetic example(s)")
