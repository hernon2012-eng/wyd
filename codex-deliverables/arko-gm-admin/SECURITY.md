# Security

## Authorization and privilege boundaries

- Role vocabulary is restricted to `player`, `moderator`, and `admin`; the official role-to-capability matrix is unknown.
- No role receives a capability implicitly. Runnable-service grants default to empty.
- `access.AssignRole` requires the explicit `role.assign` capability and rejects target roles equal to or above the actor's rank.
- The HTTP command route accepts only `dry_run: true` and invokes a preview interface. The runnable service registers no commands.
- Unauthenticated, unauthorized, malformed, and unavailable identity states are distinguished internally; storage failures are not reported as invalid credentials.

## Credentials and sessions

- Passwords are bcrypt-hashed and capped at 72 bytes.
- Session tokens are 256-bit random values; only their SHA-256 digests are stored.
- Sessions are expiring and revocable. Suspended or disabled identities cannot resolve sessions.
- The bootstrap password is read from standard input, not a command-line argument.
- The server binds to loopback by default and refuses plaintext HTTP binds to non-loopback IPs.

## Auditing and request handling

- Protected operations write an audit intent before work and a separate outcome afterward; failure to persist the intent prevents work.
- Audit SQL uses parameters for values; database triggers reject update, delete, and truncate.
- Request bodies are strictly decoded and capped at 16 KiB. Search and audit page bounds are enforced.
- Rate limiting uses server-derived remote IP addresses and bounded storage; client identity headers are not trusted.
- Static UI uses a restrictive content-security policy, security headers, `textContent`, and memory-only bearer-token storage.

## Database boundaries

- The executable requires an explicit URL and the exact database name `arko_admin`; database names `wyd` and `wyd_dev` are refused.
- It never applies migrations automatically.
- Integration tests require the separate database name `arko_admin_test`, create a random schema, and remove it.
- No real WYD server, client, game database, or player data is used.

## Remaining review limits

No external security audit has been performed for this new package. Capability policy, production deployment, credential lifecycle and recovery, TLS termination, and real account/character data access require separate review and authoritative specifications. Do not treat package tests as approval for production exposure.
