# WYD ARKO Gameplay Core

Fundação independente em Go para inventário e fabricação, sem integração com o servidor, gateway ou cliente WYD. Tudo que aparece nos exemplos é **DEMONSTRAÇÃO** e não representa itens, IDs, receitas, drops ou regras oficiais.

## Estado

Motor Go de referência implementado: inventário por personagem, slots/pilhas, add/remove/transfer, planejamento de crafting e operações idempotentes. O adaptador PostgreSQL em `internal/postgres` usa migrações versionadas explícitas, transações atômicas e idempotência durável. Testes de integração foram executados contra um PostgreSQL 16 descartável local; o workflow GitHub [42123905756](https://github.com/hernon2012-eng/wyd/actions/runs/42123905756) também passou na branch PostgreSQL publicada. Essa execução não certifica mudanças posteriores do P07, nem representa migração do banco WYD ou integração com servidor.

## Estrutura

- `go.mod`, `internal/`: módulo Go, motor independente e adaptador PostgreSQL.
- `internal/postgres/migrations/`: esquema versionado exclusivo do ARKO (`arko_gameplay_*`).
- `schemas/v1/`: contratos JSON legados de receita, inventário, solicitação e resultado.
- `schemas/v2/`, `recipes/<categoria>/`: definições versionadas por arquivo, separadas por categoria; o único arquivo é sintético e marcado DEMONSTRAÇÃO.
- `internal/recipes/`: catálogo imutável por ID+versão, com recarga por snapshot e diagnósticos por arquivo.
- `examples/`: dados sintéticos marcados como demonstração.
- `tests/`: validação Draft 2020-12 para schemas/exemplos.
- `README.md`, `WORKLOG.md`, `CHECKPOINTS.md`, `ARCHITECTURE.md`, `API_CONTRACT.md`, `RECIPES_V2.md`, `SECURITY.md`, `TEST_MATRIX.md`, `LIMITATIONS.md`, `HANDOFF_CLAUDE_CODE.md`: operação, contratos e integração futura.

## Verificação

Na pasta do módulo, execute `test -z "$(gofmt -l .)"`, `go test ./...`, `go test -race ./...`, `go vet ./...`, `go build ./...` e `python tests/validate_contracts.py` (instale `tests/requirements.txt` para a última verificação). Para rodar os testes PostgreSQL reais, configure `ARKO_POSTGRES_TEST_DSN` apenas para o serviço descartável descrito em `.github/workflows/arko-gameplay-postgres.yml`.

`NewStore` não executa migrações. `Store.Migrate` é explícito e deve ser usado apenas após validar que o destino é o banco ARKO correto. Nunca configure um DSN WYD. Consulte `LIMITATIONS.md` antes de considerar qualquer backend pronto para uso real.