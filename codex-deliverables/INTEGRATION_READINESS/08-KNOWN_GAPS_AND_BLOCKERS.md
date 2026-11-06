# Known gaps and blockers

## Hard blockers to real integration

| Blocker | Current evidence | Exit evidence required |
|---|---|---|
| Protocol 12000 | No authoritative definitions, approved captures, or compatibility tests available. **UNVERIFIED.** | Authorized version-pinned specification, implementation review, sanitized fixtures, and interoperability tests. |
| WYD server/client source and version | No validated target build, branch, listener, or dispatch path in this audit. | Owner-authorized source/artifact inventory with exact versions and a reviewed adapter design. |
| Real assets and domain data | Worktrees contain only synthetic JSON/demo examples; no real authorized asset bundle/catalog supplied. | Licensed/authorized, versioned data/assets and owner-approved semantic acceptance fixtures. |
| Real account/character/session policy | P05/P06 are demo/read-only boundaries; official policies are not established. | Owner-approved policy and server-side authority implementation/tests. |
| Cross-package runtime | Modules are isolated; no merged or end-to-end path exists. | Explicit adapter/wiring commits and cross-package integration tests. |
| P04 package identity | Continuation branch `arko-world-core` reuses `codex-deliverables/arko-npc-dynamic/`. | Clean package boundary, history/provenance, and naming reviewed before merge. |
| Operational readiness | No production deployment, monitoring, recovery, security approval, or rollback evidence. | Threat model, deployment controls, observability, recovery drill, and named operator approval. |

## Deliberate scope limits (not bugs to “fill in” by assumption)

- P01 auth/session is MOCK and volatile.
- P02 uses MemoryStore; P03 uses ARKO-owned PostgreSQL tables, not WYD data.
- P04 is metadata CRUD/revisions/publication/rollback in memory, not spawn/world runtime.
- P05 uses a synthetic directory and preview-only command interface; no official capability matrix exists.
- P06 loads/validates snapshots; its schema bounds do not become runtime policy.
- Services is a read-only registry; NPC/service metadata does not execute game behavior.
- P07–P12 implementation packages are absent from the audited refs; P01 contracts and P02/P03 crafting primitives do not satisfy that work.

## Documentation and repository hygiene

- Resolve the P03 README’s stale CI-pending statement against successful run `42124212922`.
- Reconcile older NPC handoff statements with the P04 continuation at `dd4d948`; keep the two branches’ scopes distinct.
- Refresh the P05 WORKLOG head/run references to reflect the verified P05 ref and run `42133943633`, while retaining historical checkpoints as history.
- Do not treat old local branch refs at `dd592f4` as substitutes for the verified remote commits.
- PowerShell tooling was not run because PowerShell is unavailable on this Linux host. No PowerShell changes are part of this deliverable.

## Stop conditions

Stop and request owner input if protocol behavior, authoritative data, policies, license/asset provenance, production endpoints, or database ownership cannot be verified. Do not “unblock” by inventing protocol details, using synthetic data as real, connecting to production, or bypassing authorization.
