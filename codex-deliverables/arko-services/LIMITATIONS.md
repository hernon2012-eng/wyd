# Limitations and assumptions

- This is a standalone contract registry/catalog and inspection CLI, not a network service or gameplay implementation.
- The seven type labels match the NPC v1 schema in `arko-foundation/schemas/v1/npc-definition.schema.json`. `administration` is only a service label; it does not grant administrative permissions.
- Refs are restricted synthetic identifiers with a documented lexical shape. They are opaque and are not checked against NPCs, maps, services, recipes, or any live catalog.
- `DEMONSTRAÇÃO` and `demo_only: true` are mandatory for all loaded descriptors in this package, intentionally preventing example data from being mistaken for an approved live catalog.
- The JSON Schema checks document shape. Runtime validation adds catalog-wide uniqueness, self-reference, strict-field, and snapshot integrity checks.
- `CRAFTING_CONTRACT.md` and its schema describe draft/example material only. This package defines no recipe, crafting policy, price, teleport destination, quest/arena rule, production behavior, or service authorization.
- Any future server integration requires independent server-side authorization, policy and input validation, operational review, and explicit approval. Nothing here contacts or changes live WYD systems.
