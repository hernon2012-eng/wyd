# Checkpoints

| Checkpoint | Status | Evidence / remaining work |
| --- | --- | --- |
| Isolated Pacote 4 branch and module boundary | Complete locally | Branch `codex/arko-gm-admin-20261105`, based on Pacote 3 HEAD `1870b89d7fa4a2546c32aac87be83b5f80e3da3b`; core checkpoint commit `712d4cab0a55253d584c7dd4a197a49231979957`; no earlier branch changed |
| Identity, sessions, authorization, audit, directory, API, and panel foundation | Complete locally | Go packages and embedded UI; deny-by-default grants; synthetic read-only directory; no registered commands |
| PostgreSQL persistence and recovery validation | Complete locally | PostgreSQL 16 integration tests passed for API, session persistence/reconnect/revoke, concurrent session/audit writes, and audit mutation rejection |
| Go and JSON contract validation | Complete locally | gofmt, `go test ./...`, `go test -race ./...`, `go vet ./...`, `go build ./...`, JavaScript syntax, all schema meta-checks/examples passed |
| Remote checkpoint publication | Pending | Commit/push the new branch, verify remote SHA, do not force-push |
| Dedicated GitHub Actions validation | Pending | Await the workflow run after publication |
| Official privilege and gameplay policy | Blocked on authoritative specifications | Role-to-capability matrix and official commands are not documented; keep all grants disabled by default |
| Subsequent package order | Blocked on authoritative roadmap | Reviewed repository status/contracts do not establish the order for NPC, services, and multi-character packages |

Pacote 4 is **not declared fully complete or production-ready** until remote CI succeeds and the blocking policy/specification questions are resolved or explicitly accepted by the project owner.
