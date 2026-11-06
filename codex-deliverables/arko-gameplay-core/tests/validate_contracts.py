import json
from copy import deepcopy
from pathlib import Path

from jsonschema import Draft202012Validator


ROOT = Path(__file__).resolve().parents[1]
SCHEMAS = ROOT / "schemas" / "v1"
SCHEMAS_V2 = ROOT / "schemas" / "v2"
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
    schemas_v2 = {path.name: load_json(path) for path in SCHEMAS_V2.glob("*.schema.json")}
    for schema in schemas.values():
        Draft202012Validator.check_schema(schema)
    for schema in schemas_v2.values():
        Draft202012Validator.check_schema(schema)
    for example_name, (schema_name, payload_key) in EXAMPLES.items():
        example = load_json(ROOT / "examples" / example_name)
        if "DEMONSTRAÇÃO" not in example.get("notice", ""):
            raise ValueError(f"{example_name} must be marked DEMONSTRAÇÃO")
        validator = Draft202012Validator(schemas[schema_name], format_checker=Draft202012Validator.FORMAT_CHECKER)
        validator.validate(example[payload_key])
    recipe_v2 = load_json(ROOT / "examples" / "recipes-v2" / "demonstration" / "demo-alloy-v2.json")
    if "DEMONSTRAÇÃO" not in recipe_v2.get("notice", ""):
        raise ValueError("demo-alloy-v2.json must be marked DEMONSTRAÇÃO")
    recipe_v2_definition = {key: value for key, value in recipe_v2.items() if key != "notice"}
    recipe_v2_validator = Draft202012Validator(schemas_v2["recipe-definition.schema.json"])
    recipe_v2_validator.validate(recipe_v2_definition)
    production_recipe = load_json(ROOT / "recipes" / "demonstration" / "demo-alloy-v2.json")
    recipe_v2_validator.validate(production_recipe)
    missing_version = deepcopy(production_recipe)
    del missing_version["recipe"]["version"]
    if recipe_v2_validator.is_valid(missing_version):
        raise ValueError("v2 recipe schema accepted a missing version")
    invalid_selector = deepcopy(production_recipe)
    invalid_selector["recipe"]["material_selectors"][0].pop("category")
    if recipe_v2_validator.is_valid(invalid_selector):
        raise ValueError("v2 recipe schema accepted a selector without ID or category")
    missing_output_mode = deepcopy(production_recipe)
    del missing_output_mode["recipe"]["output_mode"]
    if recipe_v2_validator.is_valid(missing_output_mode):
        raise ValueError("v2 recipe schema accepted a missing output mode")
    missing_transform_source = deepcopy(production_recipe)
    del missing_transform_source["recipe"]["transform_source"]
    if recipe_v2_validator.is_valid(missing_transform_source):
        raise ValueError("v2 recipe schema accepted transform without source amount")
    new_item_with_source = deepcopy(production_recipe)
    new_item_with_source["recipe"]["output_mode"] = "new_item"
    if recipe_v2_validator.is_valid(new_item_with_source):
        raise ValueError("v2 recipe schema accepted transform source for new_item mode")
    new_item = deepcopy(new_item_with_source)
    del new_item["recipe"]["transform_source"]
    recipe_v2_validator.validate(new_item)
    Draft202012Validator(schemas_v2["craft-request.schema.json"]).validate({
        "request_id": "req_demo", "idempotency_key": "craft_demo", "character_id": "char_demo",
        "recipe_id": "demo:alloy", "recipe_version": 2, "quantity": 1,
    })
    Draft202012Validator(schemas_v2["craft-result.schema.json"]).validate({
        "operation_id": "operation_demo", "request_id": "req_demo", "status": "rejected",
        "code": "material_resolution_failed",
    })
    print(f"validated {len(schemas) + len(schemas_v2)} schemas and {len(EXAMPLES) + 3} demonstration examples/contracts")


if __name__ == "__main__":
    main()