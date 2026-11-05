# Segurança

- Servidor é autoridade exclusiva sobre inventário, materiais, requisitos, receita e produto.
- Quantidade zero/negativa, overflow, slots inválidos, itens desconhecidos, receita inativa e payload divergente são rejeitados.
- Operações mutáveis requerem chave única por `(character_id, operation_type, idempotency_key)` e fingerprint semântico. Sessões distintas do mesmo personagem compartilham o mesmo escopo de replay.
- Consumo, entrega e registro da operação precisam ser atômicos. Um resultado de sucesso nunca deve preceder o commit.
- Não confie no `character_id` fornecido no payload; a camada autenticada deve vincular o personagem à sessão e autorização.
- Limite tamanho de payload, comprimento/charset das chaves, quantidade solicitada, número de materiais e recursos consumidos. Ao atingir capacidade, o armazenamento em memória deve falhar fechado; evicção silenciosa reabilitaria uma chave.
- Falhas internas não devem revelar tokens, estado de outros personagens ou dados de banco. Operações em personagens diferentes não compartilham inventário/resultado; operações multicharacter futuras exigem locks em ordem determinística.
- O mock em memória não oferece durabilidade, coordenação multi-instância, autenticação ou proteção contra replay entre reinícios. Não o exponha como backend de produção.

## Persistência durável necessária

Persistir resultado idempotente e mutações de inventário no mesmo commit. Uma tabela de operações deve ter unicidade por personagem/tipo/chave e guardar fingerprint, estado terminal, resultado e tempos. A fabricação básica é síncrona e não expõe `in_progress`; não transformar timeout do cliente em nova chave. Retenção deve ser explícita e compatível com a janela de retry. Revisar isolamento, lock, retenção e privacidade antes de qualquer migração real.

Na referência em memória, reiniciar o processo perde operações e permite replay não detectado; isso é uma limitação declarada, não uma garantia de segurança. A implementação deve usar limite de capacidade e rejeitar novas mutações quando cheio, sem descartar operações ainda dentro da retenção.