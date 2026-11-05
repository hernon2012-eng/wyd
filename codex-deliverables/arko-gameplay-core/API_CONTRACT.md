# Contrato de API

Contrato lógico v1 independente de transporte. Ainda não define endpoint HTTP, WebSocket ou integração com `arko-gateway`.

Os schemas correspondentes estão em `schemas/v1/`: `craft-request.schema.json`, `craft-result.schema.json`, `recipe.schema.json` e `inventory.schema.json`. Os exemplos no diretório `examples/` são fixtures DEMONSTRAÇÃO e não definem dados ou regras de produção.

## Solicitação

Uma intenção de fabricação contém `request_id`, `idempotency_key`, `character_id`, `recipe_id`, `recipe_version` e `quantity` (número de lotes, positivo e limitado). Materiais e saída escalam linearmente por lotes com checagem de overflow. A identidade autenticada deve vir do contexto autoritativo do servidor e ser autorizada para `character_id`; não confie nessa identidade enviada pelo cliente. Requisitos são avaliados contra estado autoritativo, não contra declarações do cliente.

## Resultado

O resultado contém `operation_id`, `request_id`, estado terminal (`completed` ou `rejected`), quantidade, consumo e produtos efetivamente confirmados, ou erro estável (`invalid_request`, `recipe_not_found`, `recipe_inactive`, `requirements_not_met`, `insufficient_materials`, `inventory_full`, `idempotency_conflict`). O payload é emitido após commit. Fabricação é síncrona; `in_progress` não é observável. Falhas de armazenamento são erros de infraestrutura, não resultados de domínio memorizados.

## Semântica de repetição

A unicidade é definida pela tupla `(character_id, operation_type, idempotency_key)`. Na mesma tupla e fingerprint semântico igual, a repetição devolve o resultado terminal; na mesma tupla com conteúdo diferente, responde conflito. A mesma string de chave em outra tupla é uma operação independente. `request_id` é correlação, não integra o fingerprint nem substitui idempotência. Rejeições determinísticas são memorizadas; falha de armazenamento não. Timeout/desconexão tem resultado incerto: reenviar a mesma chave resolve o estado somente enquanto o registro idempotente continuar disponível.

O backend em memória pode repetir uma operação após reinício porque perde registros e inventários. Ele não fornece garantia exactly-once nem recuperação durável; essa semântica só é válida com armazenamento durável que comite inventário e registro idempotente juntos.

Receitas são resolvidas pela chave e versão imutável exatas. Não encontrar a versão é rejeição; receita inativa não fabrica. A definição não implica conteúdo oficial.

## Inventário

Comandos de adicionar, remover e transferir também exigem identidade da operação e validação no servidor. O servidor aplica limites de stack/slot e só responde sucesso após confirmação da transação. `Actor` é um objeto confiável fornecido pelo adaptador de autenticação do servidor; nunca deve ser desserializado diretamente de uma mensagem cliente. Não há endpoint de rede ou IDs oficiais nesta versão.