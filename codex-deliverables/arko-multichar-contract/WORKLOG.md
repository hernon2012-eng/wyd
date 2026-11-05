# Worklog

## 2026-11-05

- Implemented the independent Go module from the multi-character proposal
  contract and its v1 JSON Schema.
- Added strict JSON loading, schema-shape validation, duplicate reference
  checks, atomic replacement, sorted defensive read APIs, and a read-only CLI.
- Added synthetic `DEMONSTRAÇÃO` fixture, JSON Schema validation, CI, and tests
  for malformed input, duplicates, replacement atomicity, copies, sorting, and
  concurrent reads/writes.
- Kept scope proposal-only: no slot/session activation or enforcement, gameplay
  behavior, inventory/storage, PvP/matchmaking, or server integration.
- Verified with gofmt, `go test ./...`, `go test -race ./...`, `go vet ./...`,
  `go build ./...`, schema/example validation, and CLI validation/list smoke
  checks.
- Updated the synthetic fixture markers to use the exact `DEMONSTRAÇÃO` spelling
  and added a Go formatting check to the dedicated workflow.
