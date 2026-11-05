# Arquitetura do gateway

## Escolha de transporte

O protótipo usa HTTP/JSON sobre loopback, não WebSocket. Para uma demonstração de request/response, HTTP funciona com `HTTPRequest` nativo do Godot, facilita health/version, timeout e correlação e evita manter um canal sem necessidade. Eventos de jogo contínuos podem justificar WebSocket depois de medidos; essa decisão não está fechada para integração.

## Fluxo

`Godot demo → HTTP JSON /v1 → handlers → interfaces backend/session/idempotency → MOCK`

O backend concreto atual só implementa `MOCK`. O listener padrão `127.0.0.1:8080`; sem banco, sem dependência do W2PP e sem autoridade sobre estado do jogo. O código não faz proxy de CPSock.

## Endpoints

- `GET /healthz`: saúde do processo e rótulo MOCK.
- `GET /v1/version`: serviço e versão do contrato.
- `POST /v1/handshake`: valida `contract_version`.
- `POST /v1/auth/demo`: cria sessão fictícia; exige `Idempotency-Key`.
- `GET /v1/session`: valida token temporário.
- `POST /v1/operation/ping`: demonstra request/response sem mutação autoritativa.

Há limites HTTP de tempo/cabeçalho, payload de 16 KiB, rate limit por endereço de origem, expiração em memória de 15 minutos e chaves idempotentes limitadas. Sessões e idempotência desaparecem ao reiniciar. HTTPS, CSRF/cookies e autenticação real não fazem parte deste serviço local.

## Evolução

Definir interface de backend de produção somente após o handoff comparar autenticação, comandos e ciclo de vida da sessão com o servidor verdadeiro. Uma implementação CPSock independente requer especificação e testes de interoperabilidade autorizados. Ela não deve ser introduzida como fallback transparente nem alterar o protocolo legado.