# Matriz de testes

| # | Caso | Esperado | Cobertura |
|---|---|---|---|
| 1 | Fabricação bem-sucedida | Consome todos os materiais e entrega o produto | `TestCraftSuccessfulConsumesMaterialsAndDeliversProduct` passou |
| 2 | Material insuficiente | Rejeição e inventário idêntico ao baseline | `TestCraftInsufficientMaterialsDoesNotMutate` passou |
| 3 | Inventário cheio | Rejeição sem consumo | `TestCraftInventoryFullDoesNotConsume` passou |
| 4 | Quantidade zero/inválida | Rejeição sem mutação | `TestCraftInvalidQuantityDoesNotMutate` passou |
| 5 | Receita desativada | Erro estável sem mutação | `TestCraftInactiveRecipeRejected` passou |
| 6 | Receita inexistente/versão não encontrada | Erro estável sem mutação | `TestCraftMissingRecipeRejected` passou |
| 7 | Repetição da mesma requisição | Mesmo resultado, uma fabricação | `TestCraftReplayReturnsStoredResultWithoutSecondDelivery` passou |
| 8 | Duas requisições simultâneas com mesma chave | Um commit e respostas consistentes | `TestCraftConcurrentSameRequestCommitsOnce` passou |
| 9 | Desconexão depois do commit | Retry da mesma chave devolve resultado committed | `TestCraftRetryAfterLostResponseReturnsCommittedResult` passou |
| 10 | Recuperação após falha transitória | Retry não duplica; storage failure não vira rejeição memorizada | `TestCraftRetriesAfterTransientStorageFailure` passou |
| 11 | Duas sessões no mesmo personagem | Serialização sem saldo negativo ou duplicação | `TestCraftTwoSessionsSerializeSameCharacter` passou |
| 12 | Consumo parcial seguido de erro | Rollback integral dos materiais | `TestCraftPartialMaterialConsumptionRollsBack` passou |
| 13 | Entrega duplicada | Replay não credita produto novamente | coberto por casos 7 e 9 |
| 14 | Quantidades negativas | Rejeição sem mutação | `TestCraftNegativeQuantityRejected` passou |
| 15 | Overflow numérico | Rejeição segura sem mutação | `TestCraftOverflowRejected` e `TestPlanCraftRejectsOverflowAndPreservesInput` passaram |
| 16 | Falha de gravação em cada ponto | Erro de infraestrutura e estado integralmente inalterado | `TestCraftWriteFailuresRollbackInventoryAndOperation` passou para `inventory_write`, `operation_write`, `commit` |
| 17 | Rollback após falha antes do commit | Inventário e idempotência retornam ao baseline | coberto no caso 16 em `commit` |

Casos adicionais: receita ativa/inativa, fingerprint conflitante, capacidade esgotada, sessões em personagens diferentes e operações multicharacter futuras. Fuzz/property tests devem verificar conservação, pilhas positivas dentro do limite, slots únicos e estado invariável em rejeições.

Fault injection deve falhar antes/depois de cada write (consumo, crédito, registro idempotente e commit). O gate inclui `go test -race ./...`; propriedades não substituem testes explícitos concorrentes e de rollback.

Toda linha deve ser marcada como coberta com referência ao teste correspondente somente após sua execução. O gate inclui `go test -race ./...`.