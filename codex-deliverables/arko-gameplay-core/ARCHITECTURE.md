# Arquitetura

## Limites

O pacote é um módulo Go independente. Ele não depende do gateway MOCK, não fala CPSock e não conhece o protocolo 12000. A regra de estoque/fabricação é autoritativa no servidor. O cliente futuro só envia intenção e recebe resultado; nunca decide quantidade, posse ou permissão.

## Domínio

- `ItemDefinition`: chave opaca e estável, limites de pilha e classificação genérica. A chave não é um ID WYD confirmado. O contrato básico não modela atributos por instância; se forem necessários, a chave de compatibilidade de pilha terá de incluí-los.
- `Inventory`: pertence a um personagem, tem slots limitados e pilhas; quantidades devem ser positivas e limitadas sem overflow.
- `Recipe`: chave, versão, nome, materiais, produto, quantidade, requisitos configuráveis e estado ativo.
- `Craft`: operação síncrona de fabricação identificada por personagem e chave idempotente; `quantity` representa número de lotes.
- `OperationRecord`: tipo, personagem, request ID, fingerprint, estado terminal (`completed`/`rejected`), resultado e timestamps para impedir repetição.

## Fronteira transacional

Uma única unidade de trabalho deve carregar inventários afetados e o registro idempotente. Validação, consumo e entrega são planejados em estado temporário e persistidos atomicamente; qualquer falha de persistência aborta sem alterar estado. Rejeições determinísticas de domínio (por exemplo, material insuficiente) também podem ser registradas como resultado terminal sem mutar o inventário. Falha de infraestrutura não é resultado de domínio e não deve ser memorizada como rejeição. O replay devolve o resultado terminal gravado; a tupla idempotente é `(character_id, operation_type, idempotency_key)` e fingerprint incompatível nessa tupla é conflito. A persistência durável futura deve gravar estado e resultado idempotente no mesmo commit de banco.

O contrato Go é `Repository.Transact(ctx, fn)` com operações por `Transaction`. Uma implementação durável precisa tornar o callback serializável para escopo da chave e inventários envolvidos; impor unicidade no banco; descartar writes se callback/commit falhar; e manter o resultado terminal na mesma transação que o inventário. O contrato não promete que uma transação SQL possa ser repetida com efeitos externos; callbacks devem permanecer puros fora da unidade de trabalho.

O fluxo é síncrono; estado `in_progress` não é visível fora da transação. O `MemoryStore` de referência usa uma transação copy-on-write sob um mutex global, serializando também operações em personagens diferentes. Essa escolha reduz o risco no protótipo, mas não escala; uma implementação futura pode usar locks/linhas de banco, aquisição determinística para vários personagens e unicidade durável. Não há eviction: ao atingir capacidade, falha fechada. Isso demonstra o contrato local, mas não garante recuperação após reinício, idempotência após reinício ou coordenação entre réplicas.

Consumo determinístico percorre itens e slots em ordem estável; a alocação do produto ocorre sobre a cópia já atualizada pelo consumo, priorizando pilhas compatíveis e depois slots vazios de menor índice. Se não couber tudo, o plano inteiro é rejeitado. Quantidades por lote multiplicadas por lotes e ingredientes repetidos são agregadas com detecção de overflow.

## Extensão

Requisitos de nível, receita aprendida, estação, duração, fila, pedidos, retirada, personagens auxiliares, limites por conta e especialização serão interfaces/políticas futuras. O fluxo básico não cria fila, timer ou produção assíncrona.

Persistência PostgreSQL futura poderá implementar os repositórios/transaction boundary com transações SQL e constraints, sem migrações neste pacote. Esquema, isolamento, chaves e estratégia de lock precisam ser validados com o servidor real antes da integração.