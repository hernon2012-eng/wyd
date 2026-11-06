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
- Sem alterações nos pacotes anteriores, nem conexão ou migração de sistemas/
  bancos WYD.
# Worklog

## 2026-11-05 — pacote publicado

- Implementado catálogo Go de definições NPC v1 com recarga atômica, validação estrita do schema, referências duplicadas rejeitadas, leituras ordenadas/cópias defensivas e suporte concorrente.
- Adicionados schema, exemplo sintético DEMONSTRAÇÃO, testes, documentação e workflow próprio.
- Validação local: formato, testes, race, vet, build, validador JSON Schema e `git diff --check` passaram.
- Commit `b5bdabca465e91739128f89b887b6bb49542c803` publicado na branch `codex/arko-npc-dynamic-20261105`.
- CI `42132717429` passou nos jobs `go` e `contracts`.
- Nenhuma regra de mundo, modelo, coordenada, spawn, IA, permissão ou comportamento foi inferida.

## 2026-11-06 — incremento P04: operações de catálogo

- Adicionadas operações em memória para criação, duplicação, edição, ativação,
  desativação, publicação, despublicação, remoção protegida por referência e
  rollback como nova revisão.
- O histórico de versões e a publicação são voláteis; reload/substituição
  integral reinicia esses estados, sem dependência de banco.
- Remoção exige verificador de referências não-nil; referência positiva ou
  falha de verificação bloqueia a operação. Definições publicadas também
  precisam ser despublicadas antes da remoção.
- Criados testes de metadados e serviços múltiplos, revisões, rollback,
  publicação, proteção de referência, JSON inválido/atomicidade e concorrência.
- Sem spawn, IA, runtime de mundo, efeitos de serviço, integração externa ou
  alterações de schema.

## 2026-11-06 — publicação do checkpoint P04

- Código P04 publicado no commit `3e03ecff373db0f3ba060d5c7141af6e142dc361`
  na branch `codex/arko-world-core-20261106`.
- HEAD local e remoto coincidem nesse SHA.
- CI [42138452469](https://github.com/hernon2012-eng/wyd/actions/runs/42138452469)
  concluiu com sucesso no mesmo SHA e branch.
- Nenhum merge, deploy ou integração com WYD foi realizado.
