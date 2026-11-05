# Contrato de fabricação

`schemas/v1/crafting.schema.json` define receitas versionadas, ingredientes/quantidades, nível, estação, duração, solicitação idempotente, resultado, erro e retirada. `examples/recipe-demo.json` está rotulado como demonstração; seus materiais não são itens reais nem receita aprovada.

Execução futura deve reservar ingredientes atomicamente, registrar chave idempotente, validar posse/progressão/estação no servidor, sobreviver a restart e garantir que conclusão/retirada não duplique produção. Cancelamento e reembolso precisam de política explícita. Não importar nem alterar catálogo local ou receitas sem comparação e aprovação.