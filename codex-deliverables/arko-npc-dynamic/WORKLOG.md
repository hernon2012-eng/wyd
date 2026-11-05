# Worklog

## 2026-11-05 — Pacote 5: catálogo de NPC dinâmico

- Implementado módulo Go independente baseado no contrato NPC v1 e no schema
  `schemas/v1/npc-definition.schema.json`.
- Adicionadas validação de definições, proteção contra `definition_ref`
  duplicado, troca atômica de snapshot e consultas concorrentes que retornam
  cópias defensivas.
- Criados testes de formato, dados inválidos, duplicatas, atomicidade e leituras
  simultâneas a recargas.
- Adicionados schema, exemplo sintético DEMONSTRAÇÃO, documentação e workflow
  de CI isolado.
- Sem alterações nos pacotes anteriores, sem conexão ou migração de sistemas/
  bancos WYD, sem commit e sem push.
# Worklog

## 2026-11-05 — pacote publicado

- Implementado catálogo Go de definições NPC v1 com recarga atômica, validação estrita do schema, referências duplicadas rejeitadas, leituras ordenadas/cópias defensivas e suporte concorrente.
- Adicionados schema, exemplo sintético DEMONSTRAÇÃO, testes, documentação e workflow próprio.
- Validação local: formato, testes, race, vet, build, validador JSON Schema e `git diff --check` passaram.
- Commit `b5bdabca465e91739128f89b887b6bb49542c803` publicado na branch `codex/arko-npc-dynamic-20261105`.
- CI `42132717429` passou nos jobs `go` e `contracts`.
- Nenhuma regra de mundo, modelo, coordenada, spawn, IA, permissão ou comportamento foi inferida.
