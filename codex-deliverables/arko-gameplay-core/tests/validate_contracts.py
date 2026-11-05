import json
from pathlib import Path

from jsonschema import Draft202012Validator


ROOT = Path(__file__).resolve().parents[1]
SCHEMAS = ROOT / "schemas" / "v1"
EXAMPLES = {
    "recipe-demo.json": ("recipe.schema.json", "recipe"),
    "craft-request-demo.json": ("craft-request.schema.json", "request"),
    "craft-result-demo.json": ("craft-result.schema.json", "result"),
    "inventory-demo.json": ("inventory.schema.json", "inventory"),
}


def load_json(path: Path):
    with path.open(encoding="utf-8") as source:
        return json.load(source)


def main() -> None:
    schemas = {path.name: load_json(path) for path in SCHEMAS.glob("*.schema.json")}
    for schema in schemas.values():
        Draft202012Validator.check_schema(schema)
    for example_name, (schema_name, payload_key) in EXAMPLES.items():
        example = load_json(ROOT / "examples" / example_name)
        if "DEMONSTRAÇÃO" not in example.get("notice", ""):
            raise ValueError(f"{example_name} must be marked DEMONSTRAÇÃO")
        validator = Draft202012Validator(schemas[schema_name], format_checker=Draft202012Validator.FORMAT_CHECKER)
        validator.validate(example[payload_key])
    print(f"validated {len(schemas)} schemas and {len(EXAMPLES)} demonstration examples")


if __name__ == "__main__":
    main()