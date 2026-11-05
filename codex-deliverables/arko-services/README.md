# ARKO Services Registry

Independent Go catalog for versioned service-contract descriptors. It accepts only the service labels from the NPC v1 schema: `shop`, `teleport`, `crafting`, `production`, `quest`, `arena`, and `administration`. The latter is a service label, not an access grant. This package stores/query metadata; it does not execute services.

Every included descriptor is explicitly labeled `DEMONSTRAÇÃO` and `demo_only: true`. Refs are synthetic opaque identifiers. The catalog does not contain NPC/world-map data, official recipes, prices, gameplay rules, access grants, or operational configuration. It does not import the draft crafting example as an approved recipe.

## Run

From this directory:

```sh
go run ./cmd/arko-services -catalog examples/service-catalog-demo.json
go run ./cmd/arko-services -catalog examples/service-catalog-demo.json -type crafting
go run ./cmd/arko-services -catalog examples/service-catalog-demo.json -ref demo.shop
```

The CLI prints descriptors as JSON in stable service-ref order. It only reads the catalog file.

## Registry behavior

- Catalog and descriptor contract versions must be `1.0.0`.
- Reload decodes and validates the full catalog before atomically publishing one immutable snapshot. Any read, JSON, or validation error leaves the previous snapshot available.
- Unknown service types, malformed refs, duplicate descriptor refs, duplicate/self references, and non-demonstration descriptors are rejected. Refs are lowercase ASCII slugs (up to 128 characters) with single `.`, `_`, `/`, or `-` separators.
- `List`, `ListByType`, and `Lookup` return defensive copies; list results are sorted.
- JSON fields are strict; unknown fields and trailing JSON values are rejected.
- `references` are opaque identifiers only. They are not resolved, authorized, or executed.

The registry provides structural catalog validation, not the server-side policy, permission, ownership, range, or behavior validation required before any future service implementation.

## Layout

- `registry.go`: immutable-snapshot registry and query API.
- `cmd/arko-services/`: read-only catalog inspection CLI.
- `schemas/v1/`: JSON Schema Draft 2020-12 contract.
- `examples/`: synthetic demonstration catalog.
- `registry_test.go`: validation, atomic reload, defensive read, and concurrency tests.
- `tests/`: JSON Schema and example validation.
- `WORKLOG.md`, `CHECKPOINTS.md`, `LIMITATIONS.md`, `HANDOFF.md`: implementation record and scope.

## Verify

```sh
test -z "$(gofmt -l .)"
go test ./...
go test -race ./...
go vet ./...
go build ./...
python -m pip install -r tests/requirements.txt
python tests/validate_contracts.py
git diff --check
```

The package has no Go third-party dependencies. Do not connect this demonstration catalog to a WYD service or treat it as an authorization or gameplay policy.
