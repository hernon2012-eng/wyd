# Limitations

## Authoritative decisions still missing

- The role names `player`, `moderator`, and `admin` are known, but official capability grants and audit-history visibility are not specified. All runnable-service grants therefore default to none.
- No official GM command names, arguments, effects, or confirmation policy are available. The HTTP command endpoint is preview-only and the runnable service registers no commands.
- Repository status and reviewed contracts do not establish the official order of subsequent NPC, service, or multi-character packages. Do not select or start one based on assumption.

## Integration deliberately not implemented

- Account and character search is backed only by synthetic **DEMONSTRAÇÃO** records. There is no authoritative account/character schema or read-only WYD integration contract.
- This package does not connect to, modify, or deploy the original WYD server, client, or databases.
- It does not implement gameplay mutation, live GM actions, or a production remote-access/TLS configuration.

## Operational and release status

- PostgreSQL must be a dedicated database named `arko_admin`; migrations must be applied explicitly by an operator. The service refuses other database names and performs no automatic migration.
- Current local validation passed against disposable PostgreSQL 16, including race tests and schema/example validation.
- The new branch has not yet been published and remote CI has not yet been run; this work remains an unpublished checkpoint until remote SHA and CI are confirmed.
- No independent external security audit has been performed. Production use requires review of authorization policy, secrets/credential recovery, TLS boundary, and any future data integration.
