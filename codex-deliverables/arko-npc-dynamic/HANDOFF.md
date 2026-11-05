# Handoff

## Estado

Pacote independente de catálogo dinâmico concluído no worktree isolado. O
catálogo aceita JSON como array de definições individuais do contrato NPC v1.
Uma recarga é aplicada integralmente ou não altera o snapshot anterior.
Consultas (`Lookup`, `List`, `Len`) são somente leitura; as estruturas
retornadas são cópias.

## Verificações

Registrar abaixo os resultados dos comandos efetivamente executados antes de
encerrar a entrega:

- [x] `gofmt`
- [x] `go test ./...`
- [x] `go test -race ./...`
- [x] `go vet ./...`
- [x] `go build ./...`
- [x] `python tests/validate_contracts.py`
- [x] `git diff --check`

## Não feito/intencionalmente fora do escopo

Sem CLI/HTTP, persistência, validação do mundo real, comportamento de NPC,
autorização, integração WYD, commit ou push. Detalhes em `LIMITATIONS.md`.
