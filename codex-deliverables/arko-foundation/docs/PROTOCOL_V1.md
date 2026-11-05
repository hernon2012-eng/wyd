# Protocolo HTTP v1

Base local: `http://127.0.0.1:8080`. JSON UTF-8. O cliente envia `X-Request-ID` opcional (1–64 caracteres alfanuméricos, `_` ou `-`); o servidor responde com o mesmo ou gera um. Erros têm `error.code`, `error.message` e `meta.request_id`.

## Handshake

`POST /v1/handshake` com `{"contract_version":"1.0.0"}`. Versão incompatível retorna HTTP 409. A resposta declara `heartbeat_seconds: 30`, mas o MVP HTTP não implementa heartbeat de socket.

## MOCK

`POST /v1/auth/demo`, header `Idempotency-Key` (1–128 caracteres alfanuméricos, `_` ou `-`), corpo `{"display_name":"demo"}`. Retorna HTTP 201 e sessão MOCK. Repetir a mesma chave e bytes devolve a resposta original; reutilizar com corpo diferente retorna 409. O endpoint não recebe senha e não autentica identidade WYD.

`GET /v1/session`, `DELETE /v1/session` e `POST /v1/operation/ping` exigem `Authorization: Bearer <token>`. DELETE revoga a sessão local. O ping nunca altera estado do jogo e informa `authoritative:false`.

Payload acima de 16 KiB retorna 413. Payload JSON v1 desconhecido/malformado retorna 400. A sessão expira após 15 minutos. Operações HTTP encerradas pelo cliente propagam o contexto; não há job de gameplay cancelável no MOCK.