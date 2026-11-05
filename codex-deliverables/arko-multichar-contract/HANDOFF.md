# Handoff

The independent module is located at
`codex-deliverables/arko-multichar-contract`. It exposes a Go `Store` for
validating and atomically replacing account snapshots, safe account/character
lookups, and deterministic listing. A small CLI supports `validate`, `list`,
`get`, and `character` against a JSON array.

The v1 schema is copied into the package and checked with synthetic
`DEMONSTRAÇÃO` data. Read `README.md` for commands and scope boundaries,
`LIMITATIONS.md` for explicit non-goals, and `WORKLOG.md`/`CHECKPOINTS.md` for
implementation notes.

Do not interpret snapshot counts or enum values as production slot/login
behavior or as evidence of server integration.

## Publication

- Branch: `codex/arko-multichar-20261105`, based on Pacote 3 HEAD
  `1870b89d7fa4a2546c32aac87be83b5f80e3da3b`.
- Implementation commit: `dc239df7edc95ae907542e2bcff9ecc27f181412`.
- Initial CI: [run 42133323446](https://github.com/hernon2012-eng/wyd/actions/runs/42133323446)
  passed format, Go tests/race/vet/build, and schema/example validation.
- Validation run locally: Go format, unit/race tests, vet, build, Draft 2020-12
  schema/example checks, CLI validation and lookup.
