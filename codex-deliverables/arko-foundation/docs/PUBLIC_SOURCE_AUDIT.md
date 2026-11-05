# Auditoria de fontes públicas

Consulta realizada em 2026-11-05 na branch pública `main` de [W2PP-OpenWYD](https://github.com/Jean1dev/w2pp-OpenWYD). A API reportou licença SPDX `GPL-2.0`, `main` como branch padrão e atualização em 2026-10-29. Nenhum código do W2PP foi copiado para este pacote.

## Arquitetura e protocolo observados

- [CPSock.h](https://github.com/Jean1dev/w2pp-OpenWYD/blob/main/Source/Code/CPSock.h): `HEADER` tem campos `Size`, `KeyWord`, `CheckSum`, `Type`, `ID` e `ClientTick`; comentários/documentação descrevem 12 bytes.
- [CPSock.cpp](https://github.com/Jean1dev/w2pp-OpenWYD/blob/main/Source/Code/CPSock.cpp) e [deep dive CPSock](https://github.com/Jean1dev/w2pp-OpenWYD/blob/main/docs/agents/component-deep-analyzer/component-analysis-CPSock-2026-06-19_16-06-38.md): handshake `INITCODE` `0x1F11F311`, enquadramento por tamanho, máximo legado de 8192 bytes, transformação por tabela e checksum aditivo. A análise registra que checksum divergente é reportado mas o pacote ainda pode ser entregue ao dispatcher. Não portamos o codec.
- [tmserver main](https://github.com/Jean1dev/w2pp-OpenWYD/blob/main/tmserver/cmd/tmserver/main.go), [world server](https://github.com/Jean1dev/w2pp-OpenWYD/blob/main/tmserver/internal/world/server.go), [edge](https://github.com/Jean1dev/w2pp-OpenWYD/blob/main/tmserver/internal/world/edge.go) e [dispatch](https://github.com/Jean1dev/w2pp-OpenWYD/blob/main/tmserver/internal/handler/dispatch.go): servidor Go inicia listener TCP, separa conexão/protocolo da game loop e despacha mensagens ao mundo stateful. O cliente também faz probe HTTP de status separado antes da conexão CPSock.
- Os subdiretórios `tmserver`, `dbserver`, `binserver`, `webserver` e pacotes compartilhados organizam processos/serviços; persistência usa PostgreSQL e chamadas internas gRPC, com configuração mTLS disponível. O domínio do jogo e a sessão são controlados pelo servidor, não por este gateway.
- Há testes de handlers e protocolo no repositório, mas a cobertura e o status de todos os jobs não foram calculados nesta auditoria.

## Achados com evidência

| Tema | Evidência pública | Implicação |
|---|---|---|
| Backpressure | [Issue 419](https://github.com/Jean1dev/w2pp-OpenWYD/issues/419): entrada no Coliseu podia exceder 64 mensagens `CreateMob`; [PR 428](https://github.com/Jean1dev/w2pp-OpenWYD/pull/428) limitou fila de saída por bytes, em 128 KiB conforme código/documentação | Gateway deve limitar bytes e desconexões devem limpar estado; não basta limitar contagem de requests |
| Autoridade de movimento | [Issue 393](https://github.com/Jean1dev/w2pp-OpenWYD/issues/393): teleporte aceitava coordenadas fornecidas pelo cliente | Posição enviada pelo cliente nunca deve virar estado autoritativo sem validação do servidor |
| Validação de serviço NPC | [Issue 391](https://github.com/Jean1dev/w2pp-OpenWYD/issues/391), corrigida em [PR 416](https://github.com/Jean1dev/w2pp-OpenWYD/pull/416): compra/venda aceitava NPCs com tipo merchant inadequado | Operações futuras precisam validar serviço, proximidade e permissão do lado servidor |
| Concorrência/persistência de estado | [Issue 417](https://github.com/Jean1dev/w2pp-OpenWYD/issues/417) e [PR 418](https://github.com/Jean1dev/w2pp-OpenWYD/pull/418) registram persistência tardia do nome da guilda | Novas mutações precisam de commit persistente antes de confirmar sucesso |
| Progressão e persistência | [Issue 410](https://github.com/Jean1dev/w2pp-OpenWYD/issues/410) e [PR 426](https://github.com/Jean1dev/w2pp-OpenWYD/pull/426) abordam classe Sub-Celestial não salva | Contratos devem separar progressão e estado persistido, mas precisam ser comparados com o servidor local |
| Paridade PvP | [Issues 420–425](https://github.com/Jean1dev/w2pp-OpenWYD/issues/420) e [PR 425](https://github.com/Jean1dev/w2pp-OpenWYD/pull/425) agrupam regras de arena, cidade, PK e duelo | As regras são sensíveis ao contexto; este pacote não decide compatibilidade entre categorias |

## Issues, PRs e Actions

A lista de issues fechadas consultada incluía as issues 391, 393, 410, 417, 419 e 420–427; as PRs 416, 418, 425, 426 e 428 foram fechadas/mergeadas conforme metadados consultados. Issues abertas observadas incluíam 430 e 431 (a 431 aparece fechada na consulta subsequente); estado de issue pode mudar após esta coleta.

As 20 execuções Actions mais recentes retornadas pela API estavam `completed/success`, inclusive [CI main, execução 40873815432](https://github.com/Jean1dev/w2pp-OpenWYD/actions/runs/40873815432), no commit `da7b27b01c11b81ca5f2e3412cfbbd697e63120e`. Isso é uma amostra recente, não uma auditoria histórica completa. Não foi identificada uma correção rejeitada/revertida na amostra consultada; isso não demonstra que nunca ocorreu.

Links úteis: [issues abertas](https://github.com/Jean1dev/w2pp-OpenWYD/issues), [issues fechadas](https://github.com/Jean1dev/w2pp-OpenWYD/issues?q=is%3Aissue+state%3Aclosed), [PRs fechadas](https://github.com/Jean1dev/w2pp-OpenWYD/pulls?q=is%3Apr+state%3Aclosed) e [Actions](https://github.com/Jean1dev/w2pp-OpenWYD/actions).

Esta auditoria descreve a fonte pública no momento da consulta. Não verifica alterações privadas, branch local `expansao`, banco ou cliente original.