import json
from pathlib import Path

from jsonschema import Draft202012Validator


ROOT = Path(__file__).resolve().parents[1]
SCHEMAS = ROOT / "schemas" / "v1"
EXAMPLES = {
    "npc-demo.json": "npc-definition.schema.json",
    "recipe-demo.json": "crafting.schema.json",
    "multi-character-demo.json": "multi-character-account.schema.json",
    "gm-audit-demo.json": "gm-audit.schema.json",
}


def load_json(path: Path):
    with path.open(encoding="utf-8") as source:
        return json.load(source)


def main() -> None:
    schemas = {path.name: load_json(path) for path in SCHEMAS.glob("*.schema.json")}
    for schema in schemas.values():
        Draft202012Validator.check_schema(schema)
    for example_name, schema_name in EXAMPLES.items():
        validator = Draft202012Validator(schemas[schema_name], format_checker=Draft202012Validator.FORMAT_CHECKER)
        validator.validate(load_json(ROOT / "examples" / example_name))
    handshake = load_json(SCHEMAS / "gateway-message.schema.json")
    Draft202012Validator(handshake).validate({"contract_version": "1.0.0"})
    print(f"validated {len(schemas)} schemas and {len(EXAMPLES) + 1} contract examples")


if __name__ == "__main__":
    main()