# Repository and worktree inventory

**Audit date:** 2026-11-06
**Scope:** Repository evidence at the verified refs below. This inventory is not evidence that branches have been merged or packages integrated.

## Verified remote refs

| Line | Remote branch | Verified commit |
|---|---|---|
| Main | `origin/main` | `d5d7b49d0e10acaa181115406ac07caf50b0b5af` |
| P01 foundation | `origin/codex/arko-foundation` | `3907b862a5f76af85e40a78facbfe58686032f5e` |
| P02 gameplay core | `origin/codex/arko-gameplay-core` | `298e1ed50259bb6f6e8419a20160c77711354d72` |
| P03 gameplay PostgreSQL | `origin/codex/arko-gameplay-postgres` | `1870b89d7fa4a2546c32aac87be83b5f80e3da3b` |
| P05 GM admin | `origin/codex/arko-gm-admin-20261105` | `f4f9d574a42e749cb7e13ec56549ba21429bc0e7` |
| P06 multichar contract | `origin/codex/arko-multichar-20261105` | `51cebfd2e144e2ec1e1f4a80a1818379d97236ad` |
| NPC earlier branch | `origin/codex/arko-npc-dynamic-20261105` | `64898fa9e83e3b0e28dcd084e15a9c6936cead6d` |
| Services registry | `origin/codex/arko-services-20261105` | `ee2a968dea83b14b330130ca0cce39d49f3f058f` |
| P04 continuation | `origin/codex/arko-world-core-20261106` | `dd4d948efc9769966d6e2203e3d6a4a4dc14331b` |

These are the supplied, reconfirmed remote refs; do not substitute a branch tip or an older checkpoint when reporting status.

## Worktrees observed

| Worktree | Branch | HEAD |
|---|---|---|
| `/workspaces/wyd` | `codex/arko-foundation` | `3907b862a5f76af85e40a78facbfe58686032f5e` |
| `/workspaces/wyd-arko-gm-admin` | `codex/arko-gm-admin-20261105` | `f4f9d574a42e749cb7e13ec56549ba21429bc0e7` |
| `/workspaces/wyd-arko-integration-readiness` | `codex/arko-integration-readiness-20261106` | `3907b862a5f76af85e40a78facbfe58686032f5e` |
| `/workspaces/wyd-arko-multichar` | `codex/arko-multichar-20261105` | `51cebfd2e144e2ec1e1f4a80a1818379d97236ad` |
| `/workspaces/wyd-arko-npc-dynamic` | `codex/arko-npc-dynamic-20261105` | `64898fa9e83e3b0e28dcd084e15a9c6936cead6d` |
| `/workspaces/wyd-arko-services` | `codex/arko-services-20261105` | `ee2a968dea83b14b330130ca0cce39d49f3f058f` |
| `/workspaces/wyd-arko-world-core-20261106` | `codex/arko-world-core-20261106` | `dd4d948efc9769966d6e2203e3d6a4a4dc14331b` |
| `/workspaces/wyd-gameplay-core` | `codex/arko-gameplay-postgres` | `1870b89d7fa4a2546c32aac87be83b5f80e3da3b` |

The P02 `codex/arko-gameplay-core` ref exists, but did not have a dedicated linked worktree in the observed inventory. The P04 continuation branch is named `arko-world-core`; its commits reuse the `codex-deliverables/arko-npc-dynamic/` package path. This path/branch naming mismatch must be resolved before treating that tree as a cleanly separated deliverable.

Other local branch refs observed without linked worktrees include `codex/arko-admin-core`, `codex/arko-multichar-core`, and `codex/arko-world-core` at the shared `dd592f4` checkpoint. They are not substitutes for the verified remote refs above.

## Current readiness-documentation checkout

The requested branch currently has HEAD `3907b862a5f76af85e40a78facbfe58686032f5e`; its upstream tracking metadata points to `origin/codex/arko-foundation`. The tracked tree at that commit contains P01 foundation files and workflow, not a merged collection of all listed packages. This deliverable adds only the ten readiness documents; it does not merge or modify package code.

## Evidence discipline

Use the terms **implemented**, **tested**, **published**, **integrated**, **demonstrated**, and **pending** separately. A remote ref, passing workflow, proposal schema, or synthetic example does not by itself establish integration with WYD.
