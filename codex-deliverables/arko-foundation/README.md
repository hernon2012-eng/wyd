# WYD ARKO Foundation

Pacote independente de referência para o `arko-gateway`. Todo comportamento de autenticação e sessão deste pacote é **MOCK**; ele não conecta ao W2PP, não autentica contas WYD e não retorna estado real de personagem, inventário ou mundo.

## Executar localmente

```sh
cd gateway
go run ./cmd/arko-gateway
go test -race ./...
```

O serviço escuta por padrão somente em `127.0.0.1:8080`. `ARKO_GATEWAY_ADDR` pode substituir o endereço; não exponha a instância MOCK à Internet.

Abra `godot-client-demo/project.godot` no Godot 4.4.1 e execute o projeto. O painel deve indicar explicitamente `MOCK`.

## Conteúdo

- `gateway/`: serviço HTTP JSON v1, backend simulado e testes Go.
- `schemas/v1/`: schemas JSON versionados para mensagens e domínios futuros.
- `examples/`: somente exemplos sintéticos, não aprovados para produção.
- `godot-client-demo/`: cliente diagnóstico isolado, sem recursos do WYD.
- `docs/`: auditoria pública, segurança, contratos e handoff.
- `.github/workflows/arko-foundation.yml`: pipeline do pacote; precisa ficar no diretório especial da raiz para ser reconhecido pelo GitHub Actions.

## Estado da entrega

Implementado: API MOCK, sessões voláteis, expiração, idempotência, limite de requisições e payload, health/version/handshake e testes Go.

Documentado: adaptação futura, domínios de personagens, NPC, fabricação, GM e integração com o servidor real.

Não implementado: integração W2PP/CPSock, persistência, autorização GM real, gameplay, receitas aprovadas ou recursos WYD.