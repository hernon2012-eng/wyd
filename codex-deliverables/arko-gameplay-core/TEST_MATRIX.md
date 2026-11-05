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

Casos adicionais cobertos: fingerprint conflitante (`TestCraftSameKeyDifferentPayloadConflicts`), capacidade esgotada (`TestMemoryStoreFailsClosedAtOperationCapacity`), autorização e modo demo (`TestCraftRequiresAuthorizedCharacterAndDemoMode`), política server-side (`TestCraftRequirementsUseServerSideChecker`), operações add/remove/transfer idempotentes (`TestInventoryAddRemoveAndTransferAreIdempotent`) e falha de transferência (`TestTransferFailureLeavesBothInventoriesUnchanged`). Fuzz smoke `FuzzAddRemoveConservesQuantity` verifica conservação em adições.

Fault injection falha após a gravação local de inventário, após o registro idempotente e no commit; todas as mutações locais são descartadas. O gate inclui `go test -race ./...`; o fuzz smoke não substitui testes explícitos concorrentes e de rollback.

## PostgreSQL

| Caso | Esperado | Cobertura/estado |
|---|---|---|
| Persistência/reinício e fabricação | Dados e resultado idempotente sobrevivem ao reinício do adapter; craft atômico | `TestPostgresPersistentInventoryAndRestartRecovery`, `TestPostgresCraftingTransactionAtomicity` passaram contra PostgreSQL 16 local |
| Transferência e replay concorrente | Transferência única; requisições duplicadas compartilham um resultado | `TestPostgresTransfers`, `TestPostgresConcurrentDuplicateRequests`, `TestPostgresSameIdempotencyKeyDifferentContent` passaram |
| Rollbacks | Falha de gravação/entrega/registro não persiste inventário parcial nem operação | `TestPostgresRollbackOnInventoryWriteFailure`, `TestPostgresRollbackOnDeliveryFailure`, `TestPostgresRollbackOnOperationRecordFailure` passaram |
| Concorrência/deadlock | Fabricações e transferências concorrentes mantêm estoque consistente sem deadlock | `TestPostgresSimultaneousCrafts`, `TestPostgresOppositeDirectionTransfers`, `TestPostgresIndependentCharacterConcurrency` passaram |
| Recuperação de conexão | Falha de conexão aborta transação e serviço processa transação subsequente | `TestPostgresRecoveryAfterConnectionFailure` passou |
| Isolamento do banco de teste | Conexão apenas ao serviço descartável `arko_test` | `ARKO_POSTGRES_TEST_DSN` explícito; harness recusa endpoint fora do host/porta/banco/usuário isolados |

Os testes em `internal/postgres/store_test.go` usam `sqlmock` e complementam, mas não contam como integração real. Os testes `TestPostgres*` foram executados localmente contra PostgreSQL 16 descartável em `127.0.0.1:55432/arko_test`, com `go test ./... -count=1` e `go test -race ./... -count=1`. A execução GitHub Actions [42123905756](https://github.com/hernon2012-eng/wyd/actions/runs/42123905756) passou nos jobs `go`, `contracts` e `postgres-integration`; o job PostgreSQL executou ambos os comandos contra o serviço PostgreSQL 16.