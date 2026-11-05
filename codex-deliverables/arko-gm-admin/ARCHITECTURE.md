# Architecture

## Trust and data boundaries

This package is an independent administrative service. It has no connection to the original WYD server, client, or databases. Its directory adapter currently serves synthetic **DEMONSTRAÇÃO** account and character records only. All references are opaque identifiers; no gameplay mutation exists in this module.

The HTTP service and static UI run from `cmd/arko-gm-admin`; the UI assets are embedded in the binary. API handlers use injected identity, authorization, directory, audit, and command-preview interfaces.

## Identity and sessions

`internal/identity` stores bcrypt password hashes, opaque bearer-token hashes, expiry, revocation, and identity status in dedicated `arko_admin_*` PostgreSQL tables. The server exposes only tokens on login and resolves an active identity on every authenticated request. Session storage is durable across process restarts. Session lifetime is configured by the host.

## Authorization

Known roles are `player`, `moderator`, and `admin`; official permissions and role assignments are not documented. `internal/access` recognizes explicit capabilities only, denies by default, and prevents role assignment at or above the actor's rank. The runnable service starts with no grants. Optional configuration can explicitly grant only the read-only capabilities `directory.read` and `audit.read`; this is an operational choice, not an asserted official policy.

## Audit and commands

`internal/audit` stores append-only PostgreSQL events with actor, role, capability, target, reason, request identifier, outcome, and timestamp. The migration adds database triggers rejecting update, delete, and truncate operations. Protected reads and preview requests are audited before their work proceeds; audit storage failure blocks the work.

`internal/commands` provides a generic typed registry/dispatcher. It records an intent before executor invocation and records a final outcome afterward. The HTTP boundary is deliberately preview-only; the runnable service registers no commands. No official command, gameplay effect, or permission mapping is inferred.

## Persistence and migrations

The schema is isolated under `arko_admin_*` names, with versioned SQL migrations in the identity and audit packages. `pgx` v5 is used through its `database/sql` adapter so stores share Go's standard SQL interfaces while retaining the PostgreSQL driver. Startup does not inspect or run migrations. Operators must apply migration files explicitly to a dedicated database named `arko_admin`; runtime connection setup rejects any other database name.

## Request protections

The API enforces strict JSON decoding, bounded request bodies, bounded search and audit pagination, loopback-only plaintext HTTP binding, response security headers, and a bounded per-IP rate limiter. The panel retains its bearer token only in memory and inserts returned text using `textContent`.

## Decisions pending

- The official capability matrix, including who may view audit history, remains unconfirmed.
- Official GM command names, arguments, effects, and confirmation requirements remain unconfirmed.
- Account/character database schema and read integration remain intentionally absent.
- Remote deployment requires an independently reviewed TLS/authentication boundary; the current executable is local-only.
