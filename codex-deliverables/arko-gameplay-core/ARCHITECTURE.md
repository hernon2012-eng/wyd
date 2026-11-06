# Arquitetura

## Limites

O pacote é um módulo Go independente. Ele não depende do gateway MOCK, não fala CPSock e não conhece o protocolo 12000. A regra de estoque/fabricação é autoritativa no servidor. O cliente futuro só envia intenção e recebe resultado; nunca decide quantidade, posse ou permissão.

## Domínio

- `ItemDefinition`: chave opaca e estável, limites de pilha e classificação genérica. A chave não é um ID WYD confirmado. O contrato básico não modela atributos por instância; se forem necessários, a chave de compatibilidade de pilha terá de incluí-los.
- `Inventory`: pertence a um personagem, tem slots limitados e pilhas; quantidades devem ser positivas e limitadas sem overflow.
- `Recipe`: chave e versão imutáveis, entradas por ID ou seletor, produto, payloads de requisito tipados e estado ativo. As definições v2 são arquivos de configuração em shards de categoria; não armazenam estado de produção.
- `Craft`: operação síncrona de fabricação identificada por personagem e chave idempotente; `quantity` representa número de lotes.
- `OperationRecord`: tipo, personagem, request ID, fingerprint, estado terminal (`completed`/`rejected`), resultado e timestamps para impedir repetição.

## Fronteira transacional

Uma única unidade de trabalho deve carregar inventários afetados e o registro idempotente. Validação, consumo e entrega são planejados em estado temporário e persistidos atomicamente; qualquer falha de persistência aborta sem alterar estado. Rejeições determinísticas de domínio (por exemplo, material insuficiente) também podem ser registradas como resultado terminal sem mutar o inventário. Falha de infraestrutura não é resultado de domínio e não deve ser memorizada como rejeição. O replay devolve o resultado terminal gravado; a tupla idempotente é `(character_id, operation_type, idempotency_key)` e fingerprint incompatível nessa tupla é conflito. A persistência durável futura deve gravar estado e resultado idempotente no mesmo commit de banco.

O contrato Go é `Repository.Transact(ctx, fn)` com operações por `Transaction`. O adapter PostgreSQL usa `READ COMMITTED`, bloqueia a chave idempotente via advisory transaction lock, bloqueia inventários com `SELECT ... FOR UPDATE` em ordem determinística e grava inventário e resultado terminal na mesma transação. Chaves exclusivas no banco são a defesa final contra duplicação. Deadlocks e erros de serialização são expostos como `ErrRetryableStorage`; não há retry automático do callback. Cancelamentos são propagados às operações SQL e qualquer falha aborta a transação. A checagem de capacidade de operações usa um advisory lock global, serializando essa etapa de commit para manter o limite exato. O contrato não promete que uma transação SQL possa ser repetida com efeitos externos; callbacks devem permanecer puros fora da unidade de trabalho.

O fluxo é síncrono; estado `in_progress` não é visível fora da transação. O `MemoryStore` de referência usa uma transação copy-on-write sob um mutex global, serializando também operações em personagens diferentes. Essa escolha reduz o risco no protótipo, mas não escala; uma implementação futura pode usar locks/linhas de banco, aquisição determinística para vários personagens e unicidade durável. Não há eviction: ao atingir capacidade, falha fechada. Isso demonstra o contrato local, mas não garante recuperação após reinício, idempotência após reinício ou coordenação entre réplicas.

Consumo determinístico percorre itens e slots em ordem estável; a alocação do produto ocorre sobre a cópia já atualizada pelo consumo, priorizando pilhas compatíveis e depois slots vazios de menor índice. Se não couber tudo, o plano inteiro é rejeitado. Quantidades por lote multiplicadas por lotes e ingredientes repetidos são agregadas com detecção de overflow.

## Extensão

O catálogo v2 publica um snapshot completo por troca atômica de ponteiro. Cada arquivo JSON inválido é isolado como diagnóstico; receitas válidas continuam disponíveis e versões anteriormente publicadas permanecem no histórico. Reescrever o conteúdo de um mesmo `(recipe_id, version)` é rejeitado. Duplicatas dentro do mesmo reload, inclusive conteúdo idêntico, geram diagnóstico por arquivo e apenas a primeira candidata em ordem estável entra no histórico. Uma falha ao ler a raiz ou um shard preserva o snapshot atual.

Seletores por ID são concretos. Seletores por categoria só são executados com `MaterialSelectorResolver` fornecido pelo servidor, que deve devolver alocações exatas; a engine valida soma, IDs, posse/quantidade e inventário pelo planejador existente. Nenhuma regra de correspondência ou prioridade de categoria é presumida. Erros de infraestrutura do resolver abortam a operação sem memorização; alocações inválidas falham fechadas. Payloads de requisito são opacos e só passam com `TypedRequirementChecker`; não há semântica de jogo embutida. O produto `product` suporta transformações e novos IDs quando a definição correspondente existe no catálogo de itens já fornecido ao repositório de gameplay.

Definições v2 declaram `output_mode`: `new_item` entrega o produto configurado; `transform` também consome o `transform_source` declarado. Fonte, materiais e entrega passam pelo mesmo `PlanCraft`; não há cópia de atributos por instância nem regras implícitas. Ver `RECIPES_V2.md` para formato e semântica detalhados.

`EngineOptions.RecipeProvider` permite que o catálogo de arquivos forneça versões ao motor existente; inventário, planejamento, transação e idempotência continuam no `Engine` e no `Repository`/adapter PostgreSQL. O catálogo não persiste inventários, receitas não criam itens no banco e nenhum estado de produção é guardado nos arquivos.

Requisitos de nível, receita aprendida, estação, duração, fila, pedidos, retirada, personagens auxiliares, limites por conta e especialização serão interfaces/políticas futuras. O fluxo básico não cria fila, timer ou produção assíncrona.

O adaptador existente em `internal/postgres` executa explicitamente `Store.Migrate`, grava inventários/operações em JSONB e usa migrações versionadas embutidas. Isso cria apenas tabelas `arko_gameplay_*`; não migra nem acessa o banco WYD. O construtor não aplica migrações automaticamente. Testes `sqlmock` complementam, mas não substituem a suíte de integração PostgreSQL real.

`Store` recebe um `*sql.DB` fornecido pelo chamador e não abre/constrói conexões. O driver pgx v5 é registrado via `database/sql` (`pgx/v5/stdlib`) para manter o contrato `*sql.DB` já usado pelo repositório, usando um driver PostgreSQL mantido sem alterar o chamador. Migrações só devem ser chamadas explicitamente após o operador selecionar e validar o banco ARKO de destino. O workflow fornece `ARKO_POSTGRES_TEST_DSN` para um serviço descartável `arko_test`; esse arranjo valida o adapter, não certifica uma configuração de produção.