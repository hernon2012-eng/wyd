# Handoff para Claude Code

1. Confirme branch, commits e árvore; mantenha este pacote separado de `arko-foundation`.
2. Leia `LIMITATIONS.md`, `ARCHITECTURE.md`, `API_CONTRACT.md` e `SECURITY.md` antes de integrar.
3. No projeto local autorizado, identifique o modelo real de item, stack, slots, equipamento, inventário por personagem, persistência e transação. Registre arquivo/linha/evidência, sem inferir a partir dos exemplos DEMONSTRAÇÃO.
4. Compare os limites e a identidade do personagem com os dados reais; proponha mapeamento separado. Não reutilize ID fictício como ID WYD.
5. Inspecione cada garantia de atomicidade: consumo, entrega e idempotência devem comitar juntos; injete falhas antes/depois de cada write. Rejeite adaptador que permita perda/duplicação.
6. Implemente persistência PostgreSQL apenas após decisão de schema/isolation/lock e revisão das migrações autorizadas. Não conecte ao gateway MOCK nem altere protocolo 12000.
7. Vincule personagem à sessão no servidor; não aceite autoridade ou autorização do cliente Godot.
8. Execute `gofmt`, `go test ./...`, `go test -race ./...`, `go vet ./...`, `go build ./...` e testes de integração contra ambiente local isolado e autorizado.
9. Preserve ativos, dados e credenciais; não faça deploy, merge ou alteração de bancos sem autorização explícita.
10. Documente incompatibilidades, decisões pendentes e testes faltantes; rejeite a integração se qualquer evidência contradizer este contrato.