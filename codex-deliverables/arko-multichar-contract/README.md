# Multi-character account contract package

Independent Go module implementing the v1 multi-character contract as a
read-only snapshot loader, validator, index, lookup, and sorted listing utility.
It is a proposal-shaped local contract package; it does not integrate with or
change a game server.

## Contract scope

The copied schema is `schemas/v1/multi-character-account.schema.json`. Account
records use `contract_version`, `account_ref`, `characters`, and
`active_sessions`; character entries use `character_ref`, `progression`, and
`revision`. Accepted progression categories are exactly `Arch`, `Mortal`,
`Celestial`, `CelestialCS`, and `SCelestial`.

The schema's limits and category enum are validation shape only. In particular,
`active_sessions` is not a runtime session limit, and array size is not a slot
activation policy. The package does not activate slots, create sessions,
enforce concurrent session or mutation rules, access inventory/storage, define
PvP/matchmaking behavior, or claim server integration or authority.

## Go API and CLI

The module path is `github.com/arko/multichar-contract`. `DecodeSnapshots`,
`Load`, and `Store.ReplaceJSON` accept a JSON array whose elements each satisfy
the account schema. A store replacement validates the complete input before an
atomic swap. Account references must be unique in a store; character references
must be unique within their account. The schema does not establish a
cross-account character identity rule, so none is assumed.

`Lookup`, `LookupCharacter`, and `List` are read-only and concurrency-safe.
Returned lists are defensive copies; accounts and per-account characters are
sorted by reference.

The `cmd/multichar` utility reads from stdin by default or from `-file`:

```sh
go run ./cmd/multichar validate -file examples/DEMONSTRACAO.json
go run ./cmd/multichar list -file examples/DEMONSTRACAO.json
go run ./cmd/multichar get -file examples/DEMONSTRACAO.json DEMONSTRAÇÃO-conta-001
go run ./cmd/multichar character -file examples/DEMONSTRACAO.json \
  DEMONSTRAÇÃO-conta-001 DEMONSTRAÇÃO-personagem-arch
```

All example records are synthetic `DEMONSTRAÇÃO` data.

## Validation

From this directory:

```sh
gofmt -w multichar.go multichar_test.go cmd/multichar/main.go
go test ./...
go test -race ./...
go vet ./...
go build ./...
python -m pip install -r tests/requirements.txt
python tests/validate_examples.py
```

The CI workflow runs the Go checks and validates the schema and synthetic
examples using a Draft 2020-12 JSON Schema validator.
