# Worklog

## 2026-11-05 — Service contract catalog

- Added an independent Go module with a read-only service descriptor registry and CLI.
- Restricted descriptor types to the seven exact enum labels in the NPC v1 schema; `administration` is a label, not a permission.
- Added versioned descriptor/catalog validation, opaque ref syntax and duplicate checks, sorted defensive queries, and atomic snapshot reloads that preserve the active catalog on error.
- Labeled the synthetic example `DEMONSTRAÇÃO`; required that label and `demo_only: true` for loaded descriptors.
- Added Draft 2020-12 JSON Schema, Go unit/concurrency tests, schema validation, CI, and package scope/handoff documentation.
- No gameplay execution, authorization, world/NPC data, prices, official recipes, WYD system access, or previous package changes are included.

Validation results are recorded in the handoff after running the package checks.
