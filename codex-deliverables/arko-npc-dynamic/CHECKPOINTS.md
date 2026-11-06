# Checkpoints

| Checkpoint | Estado | Gate |
|---|---|---|
| C0 — Módulo e contrato | Schema v1 e módulo independente estabelecidos | Contrato preserva os campos, restrições e propriedades fechadas do schema |
| C1 — Catálogo | Recarga integral validada antes de publicar; consultas ordenadas e defensivas | Testes de rejeição, IDs duplicados, atomicidade e concorrência |
| C2 — Entrega publicada | Commit `b5bdabca465e91739128f89b887b6bb49542c803` em `codex/arko-npc-dynamic-20261105` | CI [42132717429](https://github.com/hernon2012-eng/wyd/actions/runs/42132717429) passou nos jobs Go e contratos; validações locais também passaram |

O pacote implementa metadados e catálogo substituível, não comportamento de NPC em jogo.

| Checkpoint | Estado | Gate |
|---|---|---|
| P04 — Operações em memória | Publicado em `codex/arko-world-core-20261106`, commit `3e03ecff373db0f3ba060d5c7141af6e142dc361`; HEAD local/remoto coincidem | CI [42138452469](https://github.com/hernon2012-eng/wyd/actions/runs/42138452469) passou no SHA exato; testes Go incluem atomicidade, JSON inválido, histórico/publicação e concorrência |

P04 não adiciona persistência nem integração real. Consulte `LIMITATIONS.md`
para as garantias e fronteiras do verificador externo de referências.
Não houve merge nem deploy.
