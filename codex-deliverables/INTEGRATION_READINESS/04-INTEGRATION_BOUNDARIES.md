# Integration boundaries

## Observed package boundaries

| Component | Current authority/data boundary | Safe interpretation |
|---|---|---|
| P01 gateway | Local HTTP/JSON v1; MOCK principal/session; volatile in-memory state. Demo auth does not accept a password. | Protocol and control-flow prototype only. Never label it WYD authentication or game state. |
| P02 gameplay core | Independent Go module; MemoryStore; synthetic examples. | Reference inventory/crafting operations only. |
| P03 PostgreSQL | Explicit ARKO-owned schema and migrations; no WYD table contract. | Testable persistence adapter for ARKO data only. Do not point it at a WYD database. |
| P04 world metadata | In-memory CRUD, revisions, publication and rollback. | Metadata lifecycle only; no NPC spawn, world runtime, persistence, or live effects. |
| P05 GM admin | Isolated API/panel; synthetic directory; preview-only commands; default deny posture. | Demo admin foundations only; no official capability matrix or gameplay mutation. |
| P06 multichar | Read-only snapshot validation/index/lookup/listing. | Contract-shape utility only; no account/session enforcement or slot activation. |
| Earlier NPC catalog | Standalone metadata catalog and synthetic examples. | Metadata reads only; it is not an NPC runtime. |
| Services registry | Read-only versioned descriptors, synthetic `demo_only` entries. | Service catalog only; no service handler execution or permission grant. |

## Non-negotiable boundaries

- Keep all demo data visibly synthetic. No real account data, credentials, protected assets, or production endpoints are included.
- Keep P01 MOCK behavior explicit; do not promote it to a real backend through a configuration toggle without a designed and reviewed adapter.
- Keep P03 migrations confined to an isolated ARKO database. Never reuse a WYD DSN or run these migrations against a game database.
- Treat client input as untrusted. Any future gameplay mutation, role/capability decision, session limit, and item/recipe validation must be authoritative server-side.
- Do not infer rules from proposal schemas, enum values, demo content, public-source observations, or package names.
- Preserve the existing WYD client/server and protocol until authorized source inspection and compatibility tests justify a change.
- Resolve the P04 branch/path collision (`arko-world-core` branch using `arko-npc-dynamic` directory) before composing or packaging the packages.

## Evidence required before a boundary may move

For each new adapter or shared service, record the source and destination owners, exact version/commit, data authority, schema and migration ownership, failure semantics, authentication/authorization model, replay/idempotency behavior, timeout/retry behavior, observability, rollback, and tests. Obtain written authorization for any real server, database, account data, protocol capture, or asset.

No boundary has moved in the audited work: there is no real WYD integration.
