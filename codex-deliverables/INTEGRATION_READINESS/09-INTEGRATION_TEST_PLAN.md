# Integration test plan

This is a proposed gated plan, not a report of completed integration tests. All real-system testing requires written authorization and a disposable isolated environment. Never point tests at production.

## Stage 0 — Baseline and input approval

- Freeze and record exact server/client/package commits, binaries, configuration, protocol version, and hashes.
- Obtain approved protocol documentation, domain rules, schema owners, and data/asset licenses.
- Preserve baseline behavior; record initial startup, login, and representative read-only flows where authorized.
- Confirm no secrets or protected production captures enter source control or test logs.

**Gate:** Every input has an owner, provenance, scope, and authorization.

## Stage 1 — Repeat package-level checks

- Run each package's exact-ref workflow commands (format, unit/race tests, vet/build, schema validation, package-specific checks).
- For P03, use only the isolated PostgreSQL service and verify migrations affect only ARKO-owned schemas.
- For P04, verify in-memory CRUD, revision monotonicity, publication/unpublication, guarded removal, rollback-as-new-revision, and process-restart behavior is documented as non-persistent.
- For P05/P06/services/NPC, retain their synthetic/read-only/preview-only limits in test assertions.

**Gate:** Package checks pass at pinned SHAs; failures and tool versions are recorded. Do not interpret this gate as integration.

## Stage 2 — Protocol adapter conformance

- First complete the protocol evidence gate in `05-PROTOCOL_12000_COMPATIBILITY.md`.
- Test parser/serializer round trips, field bounds, malformed/truncated frames, unsupported versions, unknown messages, disconnects, and error mapping using sanitized fixtures.
- Verify encryption/auth/session behavior against approved definitions; keep credentials out of fixtures and logs.
- Run negative tests proving no unrecognized or unauthorized operation mutates state.

**Gate:** Reviewed specification and fixture-backed results for every supported version; no guessed wire behavior.

## Stage 3 — Adapter and data boundary tests

- Use a fake/deterministic backend to test mapping, authorization, schema/version rejection, timeout/cancellation, retries, idempotency, ordering, and failure recovery.
- Verify gameplay mutations are authorized and validated by server-side rules, not client claims.
- Verify all P03 database access is confined to disposable ARKO tables and rollback is rehearsed.
- Verify P04 metadata publication cannot spawn or alter a live world without a separately approved runtime.
- Verify P05 default-deny, audit integrity, and that preview handlers never invoke a mutation path.
- Verify P06 snapshots are read-only and do not enforce policy through schema limits.

**Gate:** Owners approve data authority, security controls, failure semantics, and migration/rollback.

## Stage 4 — End-to-end staging

- Run an isolated staging topology with pinned components and test-only identities/assets.
- Cover connect/disconnect/reconnect, stale sessions, duplicate requests, timeouts, version mismatch, partial service failure, rollback, and audit/trace correlation.
- Compare results against approved baseline fixtures; check legacy client/server behavior remains unchanged where required.
- Run bounded load/resource tests with agreed limits, then a recovery/restore exercise.

**Gate:** No unresolved critical failures; security, data, operations, and product owners approve results.

## Stage 5 — Release readiness

- Review deployment permissions, secrets delivery, network exposure, database backups, observability, alerts, incident procedures, feature flags, and rollback.
- Require explicit approval and a documented change window before any deployment.
- Keep MOCK/demo endpoints disabled or unmistakably isolated in release configuration.

## Minimum result record

For every test, retain package/ref, environment and version, command, fixture identifier/hash, timestamp, outcome, logs with secrets redacted, deviations, approver, and issue/blocker. A green package CI job must not be reported as a successful end-to-end test.
