# Test matrix

| Area | Coverage |
|---|---|
| Contract and loader | Strict JSON decoding, schema-shaped integer notation, invalid fields, duplicate refs, atomic full replacement |
| CRUD and duplication | Create, duplicate, position/model edits, multiple service labels, revision assignment |
| Lifecycle and publication | Activate/deactivate, idempotent activation, publish/unpublish, immutable published snapshots |
| History and rollback | Ordered revision history, defensive copies, restoration as a new revision, rollback publication |
| Removal protection | Missing checker, checker error, referenced definition, published definition, successful unreferenced removal |
| Atomicity | Invalid JSON create/update and rejected operations leave definitions/history unchanged |
| Concurrency | Concurrent catalog mutation and concurrent snapshot reads/replacements under the race detector |
| Contract fixtures | Synthetic example catalog validated against the Draft 2020-12 schema |

This matrix covers only in-memory metadata behavior. It does not assert world
runtime behavior, external reference tracking, persistence, or integration.
