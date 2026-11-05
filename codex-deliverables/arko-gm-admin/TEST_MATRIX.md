# Test Matrix

The local checks below were executed in the current Pacote 4 worktree. PostgreSQL tests used a disposable PostgreSQL 16 container and the isolated `arko_admin_test` database; no WYD database was accessed.

| Check | Coverage | Local result | Remote CI |
| --- | --- | --- | --- |
| `gofmt` | All Go sources | Pass; `gofmt -l cmd internal` empty | Pending |
| `go test ./...` | Unit and package tests | Pass | Pending |
| `go test -race ./...` | Concurrency and race checks | Pass | Pending |
| PostgreSQL integration | API login/search/audit/logout; session persistence after pool reconnect; revocation; concurrent session creation; concurrent audit append; update/delete/truncate rejection | Pass on PostgreSQL 16 | Pending |
| `go vet ./...` | Static analysis | Pass | Pending |
| `go build ./...` | All Go packages | Pass | Pending |
| JavaScript syntax | Embedded `app.js` | Pass with `node --check` | Pending |
| JSON syntax | All example and schema JSON files | Pass | Pending |
| JSON Schema meta-schemas | Every `*.schema.json` | Pass | Pending |
| Example/schema contracts | All nine **DEMONSTRAÇÃO** examples against their schemas | Pass | Pending |
| Authorization | Deny-by-default, unauthorized search/commands, invalid identity handling, preview-only request | Pass in unit tests | Pending |
| Audit-before-work | Intent storage failure prevents generic command execution; HTTP protected operations audit intent first | Pass in unit/integration tests | Pending |
| Startup migration boundary | Migrations are exposed as SQL and never run by startup; executable refuses non-`arko_admin` DB names | Verified in code and unit tests | Pending |

CI is intentionally still marked pending until the branch is pushed and the dedicated workflow result is observed.
