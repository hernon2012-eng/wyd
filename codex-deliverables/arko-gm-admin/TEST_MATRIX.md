# Test Matrix

The local checks below were executed in the current Pacote 4 worktree. PostgreSQL tests used a disposable PostgreSQL 16 container and the isolated `arko_admin_test` database; no WYD database was accessed.

| Check | Coverage | Local result | Remote CI |
| --- | --- | --- | --- |
| `gofmt` | All Go sources | Pass; `gofmt -l cmd internal` empty | Pass, run `42128608396` |
| `go test ./...` | Unit and package tests | Pass | Pass, run `42128608396` |
| `go test -race ./...` | Concurrency and race checks | Pass | Pass, run `42128608396` |
| PostgreSQL integration | API login/search/audit/logout; session persistence after pool reconnect; revocation; concurrent session creation; concurrent audit append; update/delete/truncate rejection | Pass on PostgreSQL 16 | Pass, run `42128608396` |
| `go vet ./...` | Static analysis | Pass | Pass, run `42128608396` |
| `go build ./...` | All Go packages | Pass | Pass, run `42128608396` |
| JavaScript syntax | Embedded `app.js` | Pass with `node --check` | Not included in workflow |
| JSON syntax | All example and schema JSON files | Pass | Pass, run `42128608396` |
| JSON Schema meta-schemas | Every `*.schema.json` | Pass | Pass, run `42128608396` |
| Example/schema contracts | All nine **DEMONSTRAÇÃO** examples against their schemas | Pass | Pass, run `42128608396` |
| Authorization | Deny-by-default, unauthorized search/commands, invalid identity handling, preview-only request | Pass in unit tests | Pass, run `42128608396` |
| Audit-before-work | Intent storage failure prevents generic command execution; HTTP protected operations audit intent first | Pass in unit/integration tests | Pass, run `42128608396` |
| Startup migration boundary | Migrations are exposed as SQL and never run by startup; executable refuses non-`arko_admin` DB names | Verified in code and unit tests | Pass, run `42128608396` |

Remote CI run [42128608396](https://github.com/hernon2012-eng/wyd/actions/runs/42128608396) passed on implementation commit `ba1f521946eb566df61303d4c49ffa6acf1be540`. UI syntax is checked locally but not by the Go/Schema workflow.
