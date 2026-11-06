# Limitações e premissas

- Nenhum arquivo do servidor, banco, cliente, catálogo ou assets WYD local está disponível neste ambiente.
- Não se conhecem IDs oficiais, limites de stack, slots, categorias de item, receitas ou regras econômicas; exemplos são fictícios e não aprovados.
- O contrato de autorização recebe `Actor` confiável do chamador; este módulo não implementa autenticação/sessão nem transporte.
- Este pacote não integra ao servidor W2PP, ao gateway MOCK ou ao protocolo 12000.
- O armazenamento em memória é apenas demonstrativo/testável: perde sessões/operações em reinício, não fornece garantia exactly-once após restart e não coordena processos.
- O adaptador/migrações foram testados em PostgreSQL 16 descartável local, incluindo recuperação e concorrência. A execução GitHub Actions [42123905756](https://github.com/hernon2012-eng/wyd/actions/runs/42123905756) passou, incluindo a suíte contra PostgreSQL 16. Isso não migra o banco WYD nem autoriza uso em produção.
- O adapter usa pgx v5 via `database/sql` para preservar o contrato existente `*sql.DB`. O chamador ainda é responsável por abrir a conexão com uma configuração segura e por chamar `Store.Migrate` somente depois de validar o destino; migrações não são automáticas.
- O limite de capacidade idempotente é exato e usa um advisory lock global na etapa de escrita do registro; isso serializa a finalização de operações entre personagens e pode limitar throughput. Não há retenção/expurgo automático dos registros.
- Autenticação, autorização, propriedade real do personagem, logs de auditoria e decisões de gameplay dependem do servidor local.
- Produção com filas, estações, tempo, retirada, personagens auxiliares e cotas por conta está fora do motor básico.
- Não foi selecionada licença para o pacote; não se altera a licença do repositório ou de dependências anteriores.
- O workflow `.github/workflows/arko-gameplay-postgres.yml` define validações Go, schemas e integração usando exclusivamente o serviço PostgreSQL descartável e `ARKO_POSTGRES_TEST_DSN`. Não configure um DSN de banco WYD.
- P07 não introduz IDs oficiais, classes/categorias de itens, regra de escolha de material, requisitos específicos do jogo ou produção assíncrona. O arquivo sintético `recipes/demonstration/demo-alloy-v2.json` só é exemplo.
- `output_mode=new_item` e `output_mode=transform` são distinções de contrato, não regras WYD. Transformação consome somente um ID de fonte e quantidade declarados e entrega o produto pelo planner atual; não há cópia ou modelagem de atributos por instância.
- Seletor de categoria depende de política `MaterialSelectorResolver`; requisito opaco depende de `TypedRequirementChecker`. Sem implementação autoritativa, a engine rejeita sem consumir materiais. Erro retornado por essas extensões é infraestrutura e não é memorizado.
- Novos produtos/transforms precisam de `ItemDefinition` no catálogo de itens existente antes da operação. Receita v2 não cria nem atualiza definição de item; o adapter PostgreSQL e o motor core continuam sendo a autoridade de inventário.
- O carregador mantém versões históricas em memória e rejeita alteração de conteúdo para uma mesma dupla ID+versão. Não há persistência/replicação do catálogo entre processos; distribuição e promoção dos arquivos são responsabilidade operacional pendente.
- Duplicatas válidas da mesma dupla ID+versão no reload são diagnosticadas por arquivo mesmo se idênticas; exatamente a primeira definição válida na ordem estável é candidata. O loader não faz schema validation com biblioteca externa em runtime; aplica validação estrutural equivalente no core e o gate de CI valida Draft 2020-12.