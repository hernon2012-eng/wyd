# Handoff

## Estado

P04 implementado no worktree isolado, branch
`codex/arko-world-core-20261106`. O catálogo oferece CRUD/duplicação,
ativação/desativação, publicação/despublicação, remoção protegida por
verificador obrigatório e rollback que grava uma nova revisão. Histórico e
publicação são voláteis; reload/substituição integral reinicia esses estados.
Consultas e histórico retornam cópias defensivas. Não há persistência,
integração real ou comportamento de NPC.

## Verificações

Resultados executados em 2026-11-06:

- [x] `gofmt`
- [x] `go test ./...`
- [x] `go test -race ./...`
- [x] `go vet ./...`
- [x] `go build ./...`
- [x] `python tests/validate_contracts.py`
- [x] `git diff --check`

Também passou `go test ./...` antes da validação race.

## Não feito/intencionalmente fora do escopo

Sem CLI/HTTP, persistência, verificador concreto de referências, validação do
mundo real, comportamento de NPC, autorização, integração WYD, commit ou push.
Detalhes em `LIMITATIONS.md`.
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
