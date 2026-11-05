# Checkpoints

| Checkpoint | Commit | Estado | Validação |
|---|---|---|---|
| C0 — Branch e contrato inicial | `ca207be` | Documentos-base; publicada em `origin/codex/arko-gameplay-core` | `git diff --check` e presença dos arquivos passaram; testes Go não aplicáveis |
| C1 — Modelo e operações puras | pronto para commit | Slots, pilhas, transferência e plano puro de crafting | `gofmt`; `go test ./...` passaram |