# Arko GM Admin

**Status: safe administrative foundation implemented and published; no production permissions or game commands are enabled.**

This Go module provides an isolated administrative API, a locally embedded demonstration panel, persistent identities/sessions, and append-only administrative audit records. It does not connect to or change the original WYD server, client, or databases.

## Safety boundaries

- The known role values are `player`, `moderator`, and `admin`; official role-to-capability grants are not documented. The default configuration grants no capabilities.
- Account and character search uses synthetic **DEMONSTRAÇÃO** records only.
- No official or gameplay-mutating commands are registered. The shipped command adapter always returns not found; the extension point accepts dry-run previews only.
- No production integration or data is included.
- PostgreSQL migrations are operator-applied. Application startup never runs migrations.

## Database setup

Use an isolated PostgreSQL database named exactly `arko_admin`; do not use `wyd`, `wyd_dev`, or any WYD database. The executable refuses a database URL whose database name is not `arko_admin` and also requires `ARKO_ADMIN_DATABASE_NAME=arko_admin`.

Apply the versioned schemas explicitly after creating the dedicated database:

```text
psql "$ARKO_ADMIN_DATABASE_URL" -v ON_ERROR_STOP=1 \
  -f internal/identity/migrations/001_admin_identity.sql \
  -f internal/audit/migrations/001_create_audit_events.sql
```

Provide `ARKO_ADMIN_DATABASE_URL` and `ARKO_ADMIN_DATABASE_NAME=arko_admin` to the process. Create an initial identity by running `go run ./cmd/arko-gm-admin bootstrap --username NAME --role admin` and supplying the password through standard input. The password is not accepted as a command-line argument.

Start the local service with `go run ./cmd/arko-gm-admin serve`. It binds to `127.0.0.1:8080` by default and refuses a non-loopback HTTP address. Routes include:

- `GET /healthz`
- `POST`, `GET`, and `DELETE /api/v1/session`
- `POST /api/v1/directory/search` (read-only synthetic demonstration data)
- `GET /api/v1/audit`
- `POST /api/v1/commands/{name}` (dry-run only; no commands are registered)

The static UI is embedded in the binary and served at `/`. Role capability grants can be explicitly supplied with `ARKO_ADMIN_PLAYER_CAPABILITIES`, `ARKO_ADMIN_MODERATOR_CAPABILITIES`, and `ARKO_ADMIN_ADMIN_CAPABILITIES`; currently only `directory.read` and `audit.read` are accepted. Leave these unset unless an authorized policy decision has been made.

## Development and validation

```text
go test ./...
go test -race ./...
go vet ./...
go build ./...
```

The real PostgreSQL integration tests skip unless `ARKO_ADMIN_TEST_DATABASE_URL` points to a disposable database named exactly `arko_admin_test`. They create a random isolated schema, apply the package migrations only within that schema, and remove it afterward. The dedicated GitHub Actions workflow runs them against PostgreSQL 16.

JSON request/response schemas and **DEMONSTRAÇÃO** examples are in `schemas/v1/` and `examples/`. The workflow checks schema meta-validity and validates each example against its corresponding schema.

See [ARCHITECTURE.md](ARCHITECTURE.md), [SECURITY.md](SECURITY.md), [TEST_MATRIX.md](TEST_MATRIX.md), [LIMITATIONS.md](LIMITATIONS.md), and [HANDOFF_CLAUDE_CODE.md](HANDOFF_CLAUDE_CODE.md).
