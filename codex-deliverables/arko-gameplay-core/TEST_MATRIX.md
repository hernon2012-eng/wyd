# Matriz de testes

| # | Caso | Esperado | Cobertura |
|---|---|---|---|
| 1 | Fabricação bem-sucedida | Consome todos os materiais e entrega o produto | pendente |
| 2 | Material insuficiente | Rejeição e inventário idêntico ao baseline | pendente |
| 3 | Inventário cheio | Rejeição sem consumo | pendente |
| 4 | Quantidade zero/inválida | Rejeição sem mutação | pendente |
| 5 | Receita desativada | Erro estável sem mutação | pendente |
| 6 | Receita inexistente/versão não encontrada | Erro estável sem mutação | pendente |
| 7 | Repetição da mesma requisição | Mesmo resultado, uma fabricação | pendente |
| 8 | Duas requisições simultâneas com mesma chave | Um commit e respostas consistentes | pendente |
| 9 | Desconexão depois do commit | Retry da mesma chave devolve resultado committed | pendente |
| 10 | Recuperação após falha transitória | Retry não duplica; storage failure não vira rejeição memorizada | pendente |
| 11 | Duas sessões no mesmo personagem | Serialização sem saldo negativo ou duplicação | pendente |
| 12 | Consumo parcial seguido de erro | Rollback integral dos materiais | pendente |
| 13 | Entrega duplicada | Replay não credita produto novamente | pendente |
| 14 | Quantidades negativas | Rejeição sem mutação | pendente |
| 15 | Overflow numérico | Rejeição segura sem mutação | pendente |
| 16 | Falha de gravação em cada ponto | Erro de infraestrutura e estado integralmente inalterado | pendente |
| 17 | Rollback após falha antes do commit | Inventário e idempotência retornam ao baseline | pendente |

Casos adicionais: receita ativa/inativa, fingerprint conflitante, capacidade esgotada, sessões em personagens diferentes e operações multicharacter futuras. Fuzz/property tests devem verificar conservação, pilhas positivas dentro do limite, slots únicos e estado invariável em rejeições.

Fault injection deve falhar antes/depois de cada write (consumo, crédito, registro idempotente e commit). O gate inclui `go test -race ./...`; propriedades não substituem testes explícitos concorrentes e de rollback.

Toda linha deve ser marcada como coberta com referência ao teste correspondente somente após sua execução. O gate inclui `go test -race ./...`.