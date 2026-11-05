# Contrato do painel GM

`schemas/v1/gm-audit.schema.json` descreve pedidos auditáveis para pesquisa/seleção de jogador e ações administrativas como intenção tipada, incluindo ator, justificativa, permissão e correlação. O pacote não implementa painel GM nem concede benefícios.

Toda ação futura exige autenticação forte, autorização server-side por capability, confirmação, limite operacional, idempotência, trilha append-only e registro de resultado. Nunca aceitar do cliente quantidade/ID de item como concessão autorizada. Ações reais e permissões dependem de decisão da desenvolvedora e do backend local.