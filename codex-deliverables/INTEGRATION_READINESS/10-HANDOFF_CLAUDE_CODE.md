# Handoff to Claude Code

## Mission

Continue integration discovery from the verified refs in `01-REPOSITORY_INVENTORY.md`. The deliverables are readiness evidence, not an implementation mandate. All current packages are isolated; there is no completed real WYD integration.

## First actions

1. Read all ten `codex-deliverables/INTEGRATION_READINESS/` documents and the exact package handoff, limitations, and workflow at each pinned ref.
2. Confirm target server/client versions and obtain explicit authorization before inspecting local source, databases, protocol traffic, or assets.
3. Resolve the P04 `arko-world-core` branch versus `arko-npc-dynamic` directory naming/history issue.
4. Correct the three known documentation contradictions listed in `07-CLAUDE_CODE_FINAL_AUDIT.md` in their owning package branches, without rewriting historical facts.
5. Produce a verified protocol/data/policy inventory. Keep protocol 12000 marked **UNVERIFIED** until evidence satisfies the acceptance gate.
6. Review P07 at `codex/arko-recipes-v2-20261106` (`7c201921a40b9c318f62c049ab93692b11b5e986`) and its `RECIPES_V2.md`; treat it as an isolated, published reference implementation, not as integrated gameplay. Verify CI for the exact branch HEAD before relying on its status.
7. Continue P08–P12 only on isolated branches and in dependency order; use the documented P07 extension points without inventing recipes, policies, or WYD interfaces.

## Implementation rules

- Do not merge or wire packages solely because their schemas or concepts look similar.
- Preserve the original WYD client/server and legacy protocol. Do not invent packet formats, opcodes, handshakes, data, or gameplay rules.
- Never connect P03 or any migration to a WYD database. Test PostgreSQL changes only against a disposable ARKO-owned database.
- Keep MOCK, synthetic examples, read-only snapshot tools, metadata catalogs, and preview commands clearly separated from real authority.
- Do not enable game mutation until an owner-approved server-side authorization and validation design exists.
- Do not copy assets or real data without provenance, authorization, and licensing review.
- Add adapters behind explicit versioned interfaces and feature flags; add isolated tests before end-to-end staging.
- No production deployment or irreversible operation without owner approval and rehearsed rollback.

## Acceptance criteria for declaring integration ready

All must be true:

1. Authorized source/build, protocol version, data owners, policy, and assets are identified and pinned.
2. Protocol compatibility is supported by a reviewed specification and sanitized fixture-based interoperability tests.
3. Cross-package adapters are implemented and tested against disposable authorized systems; package CI alone is insufficient.
4. Security, persistence, idempotency, retries, migrations, audit, failure recovery, and rollback have explicit passing evidence.
5. No synthetic data or demo-only path can be mistaken for live authority.
6. Security, operations, and project owners have approved the staging results and deployment plan.

Until then, report status as **integration readiness blocked / pending evidence**, not integrated or production-ready.
