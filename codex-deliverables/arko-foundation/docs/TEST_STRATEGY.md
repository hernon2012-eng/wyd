# Estratégia de testes

Go: testes unitários de sessão, expiração, idempotência, concorrência, rate limit e endpoints; `go test -race ./...`, `go vet ./...` e `gofmt -l` no CI. Os testes usam HTTP em memória e nenhum servidor externo.

Contratos: validar schemas Draft 2020-12 e exemplos sintéticos com `jsonschema` no CI. Godot: verificar serialização, transições de conexão/erro/reconexão e request IDs ao executar no Godot 4.4.1; não afirmar execução gráfica sem esse binário.

Antes de integração: testes de interoperabilidade isolados contra servidor local autorizado, protocolo e versão conhecidos; fixtures sem credenciais/ativos protegidos; testes de reconexão, duplicação, timeouts e cancelamento; benchmark e teste de carga com limites revisados. Nenhum teste deve apontar para produção.