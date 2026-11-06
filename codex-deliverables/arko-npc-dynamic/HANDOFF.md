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
# Handoff — ARKO NPC Dynamic

- Branch: `codex/arko-npc-dynamic-20261105`
- Base: Pacote 3, `1870b89d7fa4a2546c32aac87be83b5f80e3da3b`
- HEAD publicado: `2b6e804df7b6ed587a894bf2f0c732be5033c2fa`
- Commits: `b5bdabca465e91739128f89b887b6bb49542c803` (módulo) e
  `2b6e804df7b6ed587a894bf2f0c732be5033c2fa` (checkpoint).
- CI: runs [42132717429](https://github.com/hernon2012-eng/wyd/actions/runs/42132717429)
  e [42132832412](https://github.com/hernon2012-eng/wyd/actions/runs/42132832412)
  passaram nos jobs Go e contratos/schema.
- Local: `gofmt`, `go test ./...`, `go test -race ./...`, `go vet ./...`,
  `go build ./...`, `python tests/validate_contracts.py` e `git diff --check`
  passaram.
- Limite: catálogo de metadados apenas. Não executa NPC, spawn, IA ou serviços,
  não verifica mapas/modelos reais e não acessa WYD.
- Próxima frente escolhida e executada: registro de contratos de serviço,
  seguido do contrato multi-personagem.
