# Checkpoints

| Checkpoint | Commit | Estado | Validação |
|---|---|---|---|
| C0 — Branch e contrato inicial | `ca207be` | Documentos-base; publicada em `origin/codex/arko-gameplay-core` | `git diff --check` e presença dos arquivos passaram; testes Go não aplicáveis |
| C1 — Modelo e operações puras | `8e56c6d` | Publicado em `origin/codex/arko-gameplay-core` | `gofmt`; `go test ./...` passaram |
| C2 — Serviço transacional | pronto para commit | Repositório em memória, crafting e idempotência com failpoints | `gofmt`, `go test ./...`, `go test -race ./...`, `go vet ./...`, `go build ./...` passaram; 17 cenários cobertos |