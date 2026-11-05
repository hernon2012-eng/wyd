# WYD ARKO Gameplay Core

Fundação independente em Go para inventário e fabricação, sem integração com o servidor, gateway ou cliente WYD. Tudo que aparece nos exemplos é **DEMONSTRAÇÃO** e não representa itens, IDs, receitas, drops ou regras oficiais.

## Estado

Motor Go de referência implementado: inventário por personagem, slots/pilhas, add/remove/transfer, planejamento de crafting, operações idempotentes e transação atômica no repositório em memória. O repositório serializa todas as transações e serve apenas para execução/testes; não é persistência de produção. Nenhum banco é acessado ou migrado.

## Estrutura

- `go.mod`, `cmd/`, `internal/`: módulo Go e motor independente.
- `schemas/v1/`: contratos JSON de receita, inventário, solicitação e resultado.
- `examples/`: dados sintéticos marcados como demonstração.
- `tests/`: validação Draft 2020-12 para schemas/exemplos.
- `README.md`, `WORKLOG.md`, `CHECKPOINTS.md`, `ARCHITECTURE.md`, `API_CONTRACT.md`, `SECURITY.md`, `TEST_MATRIX.md`, `LIMITATIONS.md`, `HANDOFF_CLAUDE_CODE.md`: operação, contratos e integração futura.

## Verificação

Na pasta do módulo, execute `gofmt -w $(find . -name '*.go')`, `go test ./...`, `go test -race ./...`, `go vet ./...`, `go build ./...` e `python tests/validate_contracts.py` (instale `tests/requirements.txt` para a última verificação).

Consulte `LIMITATIONS.md` antes de considerar qualquer backend pronto para uso real.