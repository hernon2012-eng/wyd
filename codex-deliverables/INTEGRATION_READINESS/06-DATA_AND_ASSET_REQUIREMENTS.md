# Data and asset requirements

## Available evidence

- The audited worktrees contain synthetic JSON examples and demonstration UI/catalog content.
- No real WYD assets, production catalog, approved recipes, account/character dataset, protocol-12000 definitions, or authorized server/client fixture set was available.
- The implementation set contains no credentials/secrets or proprietary assets.
- P02/P03 use isolated gameplay examples; P03's persistence schema is ARKO-owned.
- P05 directory entries and preview operations are synthetic/dry-run; P06 consumes a local snapshot contract.

Absence from these worktrees does not prove that the underlying project has no such data elsewhere. It means this readiness effort cannot claim to have inspected or validated it.

## Inputs required from the project owner

| Input | Required provenance and controls |
|---|---|
| Server and client artifacts | Written authorization; product/version/build/commit and hashes; isolated copies; no production modification. |
| Protocol definitions/fixtures | Authorized authoritative docs or sanitized capture, exact version, handling restrictions, and known-good test cases. Never include secrets or live tokens. |
| Database shape | Owner-approved schema/API documentation and disposable test database; identify authoritative source, ownership, retention, migrations, and rollback. No WYD DSN in ARKO test config. |
| Gameplay catalogs and rules | Owner-approved item IDs, stacks, recipes, currencies, constraints, and versioning; synthetic fixture set first, then approved non-production fixtures. |
| Character/account policy | Approved slot, progression, session, and concurrency rules; P06's schema limits are not policy. |
| World/NPC/service data | Authorized definitions, coordinates, spawn/runtime semantics, service execution rules, access policy, and lifecycle. Metadata examples alone are insufficient. |
| GM policy | Official roles/capabilities, rank assignment, audit/approval, break-glass and revocation procedures. P05 intentionally does not infer a capability matrix. |
| Visual/audio assets | License/ownership and redistribution permission, source manifest, format/version, and client import/build verification. Do not copy assets until authorized. |

## Data handling requirements

1. Keep synthetic fixtures labeled `DEMONSTRAÇÃO`/`demo_only`; prohibit production identifiers and secrets in committed examples and logs.
2. Use least-privilege, disposable environments and test-only credentials supplied through secret management.
3. Redact personal data and tokens in traces; define retention and access controls before collecting any fixture.
4. Validate schema and semantic rules separately. Schema validity does not establish official data or gameplay correctness.
5. Version and hash every approved dataset and asset bundle; record license, source, transformation, and acceptance owner.
6. Never run ARKO migrations or test scripts against a WYD database.

## Blocking inputs

Protocol 12000 and asset compatibility remain unverified; official data/rules and real authority policies are not available in this audit. Therefore no real-system integration dataset or asset acceptance can be completed yet.
