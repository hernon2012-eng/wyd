# WYD ARKO Gameplay Core

Fundação independente em Go para inventário e fabricação, sem integração com o servidor, gateway ou cliente WYD. Tudo que aparece nos exemplos é **DEMONSTRAÇÃO** e não representa itens, IDs, receitas, drops ou regras oficiais.

## Estado

Arquitetura e contratos iniciais estão documentados; o motor Go ainda não foi implementado. O armazenamento de referência planejado será apenas para execução e testes, não persistência de produção. Nenhum banco é acessado ou migrado.

## Estrutura

- `go.mod`, `cmd/`, `internal/`: módulo Go e motor independente.
- `examples/`: dados sintéticos marcados como demonstração.
- `README.md`, `WORKLOG.md`, `CHECKPOINTS.md`, `ARCHITECTURE.md`, `API_CONTRACT.md`, `SECURITY.md`, `TEST_MATRIX.md`, `LIMITATIONS.md`, `HANDOFF_CLAUDE_CODE.md`: operação, contratos e integração futura.

## Verificação

Na pasta do módulo, execute `gofmt -w .`, `go test ./...`, `go test -race ./...`, `go vet ./...` e `go build ./...`.

Consulte `LIMITATIONS.md` antes de considerar qualquer backend pronto para uso real.