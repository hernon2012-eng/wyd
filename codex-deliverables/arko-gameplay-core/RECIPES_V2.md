# Contrato de receitas v2

## API e schemas

Definições são arquivos JSON de configuração, validados por `schemas/v2/recipe-definition.schema.json`. Os contratos de solicitação e resultado permanecem identificados por `recipe_id` e `recipe_version`; `schemas/v2/craft-request.schema.json` e `craft-result.schema.json` descrevem esse envelope. O chamador continua usando `Engine.Craft` e o contrato `RecipeProvider`; o catálogo não implementa uma segunda operação de fabricação.

Uma definição v2 requer `output_mode`:

- `new_item`: consome os materiais declarados e entrega `product`.
- `transform`: além dos materiais, consome `transform_source` (um `item_id` e `quantity` por lote) e entrega o `product` configurado. A fonte e os materiais são agregados pelo planejador core existente, que verifica quantidade/overflow, simula consumo e entrega e só persiste inventário e resultado idempotente na transação atual.

A transformação é genérica: não clona nem transfere atributos por instância, identidade de equipamento ou qualquer estado não representado pelo inventário de itens/quantidades. A definição do item-alvo precisa existir no catálogo de itens já fornecido ao repositório. Nenhum item ou regra é criado implicitamente.

## Arquivos e shards

O caminho é `recipes/<categoria>/<arquivo>.json`. O campo `category` no documento precisa corresponder ao nome do shard. O ID de receita é a chave lógica, não o nome do arquivo. Um arquivo inválido gera diagnóstico isolado e não impede carregar os demais arquivos válidos. Symlinks não são aceitos como arquivos de receita. O diretório único demonstrativo contém somente dados sintéticos e não oficiais.

## Versões, duplicatas e reload

O catálogo mantém histórico imutável por `(recipe_id, version)`. Reutilizar uma versão com conteúdo diferente não substitui o valor publicado. Uma segunda declaração da mesma dupla ID/versão em arquivos distintos no mesmo reload também é diagnosticada, mesmo que o JSON seja idêntico; a primeira definição válida na ordem lexicográfica de shard/arquivo é a única candidata desse reload. Conteúdo de versões válidas anteriormente carregadas continua no histórico.

`Reload` constrói uma geração privada e publica o snapshot completo por troca atômica. Leituras concorrentes observam o snapshot anterior ou o novo, nunca uma geração parcial; valores retornados são cópias defensivas. Falha ao enumerar a raiz ou um shard preserva o snapshot publicado. Erros de arquivos individuais são obtidos no retorno do reload e em `Catalog.Diagnostics()`.

O histórico é somente em memória do processo: não há replicação, persistência de catálogo ou promoção distribuída. As alterações de arquivos e a sincronização entre instâncias são responsabilidade operacional.

## Idempotência e execução

O motor existente calcula fingerprint semântico usando `(recipe_id, recipe_version, quantity)`; `request_id` continua sendo correlação e não integra o fingerprint. Repetir a mesma chave idempotente com o mesmo conteúdo devolve o resultado gravado sem repetir consumo/entrega. A mesma tupla idempotente com conteúdo diferente conflita. A garantia depende do repositório escolhido; com PostgreSQL, estado do inventário e resultado terminal usam a transação do adapter já existente. Uma versão de receita é imutável justamente para a mesma referência continuar significando a mesma definição.

O `Catalog` implementa `RecipeProvider`; configurar `EngineOptions.RecipeProvider` seleciona as definições v2, enquanto inventário, planejador, transação e idempotência continuam no core. Não se cria estado de produção em arquivo nem banco.

## Políticas de servidor e limites

Materiais por ID são seletores concretos. Seletores por categoria exigem `MaterialSelectorResolver` fornecido pelo servidor, com alocação exata; o módulo não define regra de correspondência, prioridade ou escolha. Requisitos genéricos antigos dependem do `RequirementChecker`; `typed_requirements` carrega tipo e payload JSON opaco e exige `TypedRequirementChecker`. Sem policy/checker a operação falha fechada. Erro operacional de extensão aborta a transação e não é memorizado como rejeição.

Não há dados oficiais, IDs oficiais, atributos por instância, regras específicas de jogo, criação automática de item, filas, duração, ordens, estação ou produção assíncrona. O esquema valida formato, não prova que IDs existam ou que uma policy seja adequada. A integração deve ser bloqueada até as regras e definições autorizadas do servidor serem fornecidas.
