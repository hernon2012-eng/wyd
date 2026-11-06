# JSON catalog preflight

`validate_json_catalog.py` is a generic preflight for JSON documents and a
caller-supplied JSON Schema. It rejects malformed JSON, non-JSON constants,
and duplicate object keys. It can optionally check a caller-selected member
for duplicate values in an array reached by JSON Pointer. It emits a structured
JSON report on standard output and exits with status `0` for valid input or `1`
when validation fails.

```sh
python tools/validate_json_catalog.py \
  --input path/to/catalog.json \
  --schema path/to/catalog.schema.json \
  --records-pointer /records \
  --unique-key source_identifier
```

The only supported schema references are references contained in the supplied
schema document (`#...`); external schema fetching is deliberately disabled.
Validation does not rewrite, normalize, or remap source identifiers and does
not import or mutate any WYD database.

There are no original asset files or established schemas for item, monster,
map, or world configuration in this environment. No formats or IDs are
invented here. The test schema and fixtures under `tools/tests/fixtures/` are
synthetic and explicitly marked **DEMONSTRAÇÃO**; they are not game schemas,
asset parsers, or evidence of any binary format. The tool can be reused for
recipes, NPCs, services, characters, or other catalogs after their actual JSON
formats and schemas are supplied. It does not parse binary formats.

The validator uses the repository's existing `jsonschema` dependency.
Run tests with:

```sh
python -m unittest discover -s tools/tests -v
```
