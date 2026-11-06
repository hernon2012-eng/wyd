# Handoff

## Package

`codex-deliverables/arko-services` is an independent Go 1.22 module containing a versioned descriptor registry, deterministic read-only query API, and local JSON inspection CLI. See `README.md` for commands and API guarantees.

## Scope and safety

The known descriptor types are `shop`, `teleport`, `crafting`, `production`, `quest`, `arena`, and `administration`, matching the NPC v1 schema. `administration` is a label only, never an access grant. All catalog descriptors require the literal `DEMONSTRAÇÃO` marker and `demo_only: true`. No service execution, game effects, NPC/world map data, pricing, official recipes, or WYD system connection is present.

The registry validates version, type, refs and duplicate/self refs before atomic publication. Failed reloads leave the current snapshot intact. Returned descriptors and reference slices are copies.

## Files and validation

- Contract: `schemas/v1/service-catalog.schema.json`
- Synthetic example: `examples/service-catalog-demo.json`
- Tests: `registry_test.go`
- CI: `.github/workflows/arko-services.yml`
- Scope limits: `LIMITATIONS.md`

Run `test -z "$(gofmt -l .)"`, `go test ./...`, `go test -race ./...`, `go vet ./...`, `go build ./...`, `python tests/validate_contracts.py` (after installing `tests/requirements.txt`), and `git diff --check` from this module/worktree. Results from the implementation session are appended below.

## Session results

- `gofmt`, `go test ./...`, `go test -race ./...`, `go vet ./...`, and `go build ./...`: passed.
- JSON Schema and demonstration-example validation: passed.
- CLI administration filter returned the expected synthetic descriptor; CLI crafting filter, single-ref lookup, and rejection of an unknown type were also checked.
- `git diff --check`: passed.

## Publication

- Branch: `codex/arko-services-20261105`, based on Pacote 3 HEAD `1870b89d7fa4a2546c32aac87be83b5f80e3da3b`.
- Implementation commit: `c3155721dc1cf36997474f42c18be08eb0574a99`.
- CI: [run 42133133330](https://github.com/hernon2012-eng/wyd/actions/runs/42133133330) passed the Go and contracts jobs.
- Published report checkpoint: `44037fb8af85e0287c42b61831054cf56ac5f98b`; CI
  [42133218945](https://github.com/hernon2012-eng/wyd/actions/runs/42133218945)
  passed on that checkpoint.
