import json
from pathlib import Path

from jsonschema import Draft202012Validator


ROOT = Path(__file__).resolve().parents[1]
schema = json.loads((ROOT / "schemas/v1/service-catalog.schema.json").read_text())
example = json.loads((ROOT / "examples/service-catalog-demo.json").read_text())
Draft202012Validator.check_schema(schema)
Draft202012Validator(schema).validate(example)
print("service catalog schema and DEMONSTRAÇÃO example are valid")
