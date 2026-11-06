# Dependency graph and integration status

This is a **candidate logical relationship map**, not a runtime dependency graph. The modules are currently isolated; no arrow below means code is wired, imported, deployed, or interoperable.

```text
P01 Foundation gateway (MOCK)
  └── proposal contracts/examples only ──> future adapters/domains

P02 Gameplay core (MemoryStore) ── candidate persistence seam ──> P03 PostgreSQL
  └── crafting primitive ── candidate domain input ──> future crafting service

P04 World metadata (in-memory revisions/publication)
  └── metadata only ──> no spawn/world runtime

P05 GM admin (API/panel demo, synthetic directory, preview commands)
  └── no live command adapter or WYD account authority

P06 Multichar (read-only snapshot validator/CLI)
  └── no session or slot mutation

Earlier NPC catalog (metadata only) ── describes service labels ──> Services registry (un-numbered, read-only)
Services registry
  └── no service execution

P07–P12
  └── no implementation package found at the audited refs
```

The association between earlier NPC catalog and services descriptors is conceptual only. The P04 continuation reuses the earlier NPC package path at its branch tip; that repository-layout issue is not an implemented dependency.

## Integration boundary

There is no verified end-to-end path such as `Godot client → gateway → WYD server → WYD database`. In particular:

1. P01's backend is `MOCK`; its HTTP JSON v1 contract is not the legacy WYD protocol.
2. P02's `MemoryStore` and P03's ARKO-only PostgreSQL adapter are separate implementation options, not a shared live store.
3. P04 and the NPC/services packages manage or read metadata; they do not execute world behavior.
4. P05's directory and command preview are synthetic/dry-run. P06 reads snapshots; it does not enforce account rules.
5. No schema, sample file, local passing test, or green CI job proves cross-package compatibility.

## Proposed integration sequence (requirements, not completed work)

1. Identify and obtain authorization for the actual server, client, data store, protocol version, and assets; preserve immutable baselines.
2. Agree on ownership boundaries and stable versioned contracts before adapting any module.
3. Build explicit adapters behind feature flags, keeping MOCK/demo modes impossible to confuse with real operation.
4. Test isolated adapters against a disposable, authorized environment, then test cross-package behavior and failure/recovery.
5. Require operator approval, rollback plans, observability, and security review before any deployment.

See `04-INTEGRATION_BOUNDARIES.md`, `05-PROTOCOL_12000_COMPATIBILITY.md`, and `09-INTEGRATION_TEST_PLAN.md`.
