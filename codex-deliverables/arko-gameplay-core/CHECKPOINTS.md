# Checkpoints

| Checkpoint | Commit | Estado | Validação |
|---|---|---|---|
| C0 — Branch e contrato inicial | `ca207be` | Documentos-base; publicada em `origin/codex/arko-gameplay-core` | `git diff --check` e presença dos arquivos passaram; testes Go não aplicáveis |
| C1 — Modelo e operações puras | `8e56c6d` | Publicado em `origin/codex/arko-gameplay-core` | `gofmt`; `go test ./...` passaram |
| C2 — Serviço transacional | `798294c` | Publicado em `origin/codex/arko-gameplay-core` | `gofmt`, `go test ./...`, `go test -race ./...`, `go vet ./...`, `go build ./...` passaram; 17 cenários cobertos |
| C3 — Contratos, exemplos e CI | `599c933` | Publicado em `origin/codex/arko-gameplay-core` | `gofmt`, `go test ./...`, `go test -race ./...`, `go vet ./...`, `go build ./...`, validador JSON, fuzz smoke (42.587 execuções) e `git diff --check` passaram |
| C4 — Adapter PostgreSQL real | `7f74440`, docs/CI `3f046a5` (locais) | Adapter corrigido, migração versionada e suíte PostgreSQL real implementada; PostgreSQL 16 descartável local validado | `go test ./... -count=1`, `go test -race ./... -count=1` (incluindo integração), `go vet ./...`, `go build ./...`, `gofmt` e `python tests/validate_contracts.py` passaram localmente; CI remoto ainda pendente |