# Worklog

## 2026-11-05 — Service contract catalog

- Added an independent Go module with a read-only service descriptor registry and CLI.
- Restricted descriptor types to the seven exact enum labels in the NPC v1 schema; `administration` is a label, not a permission.
- Added versioned descriptor/catalog validation, opaque ref syntax and duplicate checks, sorted defensive queries, and atomic snapshot reloads that preserve the active catalog on error.
- Labeled the synthetic example `DEMONSTRAÇÃO`; required that label and `demo_only: true` for loaded descriptors.
- Added Draft 2020-12 JSON Schema, Go unit/concurrency tests, schema validation, CI, and package scope/handoff documentation.
- Corrected the admin service enum to exactly `administration`, matching the source NPC schema rather than inventing a renamed wire value.
- No gameplay execution, authorization, world/NPC data, prices, official recipes, WYD system access, or previous package changes are included.

Validation passed locally (`gofmt`, Go test/race/vet/build, JSON Schema/example validation, CLI filter, and diff check). Commit `c3155721dc1cf36997474f42c18be08eb0574a99` was published on `codex/arko-services-20261105`; GitHub Actions `42133133330` passed both jobs.
