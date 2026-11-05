# Checkpoints

| Checkpoint | Commit | Estado | Validação |
|---|---|---|---|
| C0 — Branch e contrato inicial | `ca207be` | Documentos-base; publicada em `origin/codex/arko-gameplay-core` | `git diff --check` e presença dos arquivos passaram; testes Go não aplicáveis |
| C1 — Modelo e operações puras | `8e56c6d` | Publicado em `origin/codex/arko-gameplay-core` | `gofmt`; `go test ./...` passaram |
| C2 — Serviço transacional | `798294c` | Publicado em `origin/codex/arko-gameplay-core` | `gofmt`, `go test ./...`, `go test -race ./...`, `go vet ./...`, `go build ./...` passaram; 17 cenários cobertos |
| C3 — Contratos, exemplos e CI | `599c933` | Publicado em `origin/codex/arko-gameplay-core` | `gofmt`, `go test ./...`, `go test -race ./...`, `go vet ./...`, `go build ./...`, validador JSON, fuzz smoke (42.587 execuções) e `git diff --check` passaram |