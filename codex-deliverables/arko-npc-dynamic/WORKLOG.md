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
