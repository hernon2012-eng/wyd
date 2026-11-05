import json
from pathlib import Path

from jsonschema import Draft202012Validator


ROOT = Path(__file__).resolve().parents[1]
SCHEMA_PATH = ROOT / "schemas" / "v1" / "npc-definition.schema.json"
EXAMPLE_PATH = ROOT / "examples" / "DEMONSTRAÇÃO" / "catalog.json"


def main() -> None:
    schema = json.loads(SCHEMA_PATH.read_text(encoding="utf-8"))
    Draft202012Validator.check_schema(schema)
    validator = Draft202012Validator(schema)
    definitions = json.loads(EXAMPLE_PATH.read_text(encoding="utf-8"))
    if not isinstance(definitions, list):
        raise ValueError("the demonstration catalog must be a JSON array")
    for index, definition in enumerate(definitions):
        errors = sorted(validator.iter_errors(definition), key=lambda error: list(error.path))
        if errors:
            raise ValueError(f"{EXAMPLE_PATH.name}[{index}]: {errors[0].message}")
    print(f"validated 1 schema and {len(definitions)} DEMONSTRAÇÃO definitions")


if __name__ == "__main__":
    main()
