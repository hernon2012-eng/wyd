# Worklog

## 2026-11-05 — Correção SA4000

- Separadas em verificações sequenciais as três chamadas `Allow("peer")` no teste do limitador.
- Mantida a lógica: dois tokens do burst são aceitos; a terceira chamada é bloqueada; o teste de refill permanece inalterado.
- Nenhum código de produção foi modificado.
- Validação aprovada: `gofmt`, `go test -race ./...`, `go vet ./...` e `staticcheck ./...` no módulo `gateway`.
- `staticcheck ./...` não reportou SA4000 nem outras ocorrências.
