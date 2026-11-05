# Modelo de segurança

- O serviço escuta em loopback por padrão. Configurar `ARKO_GATEWAY_ADDR` para endereço público não é recomendado; não há TLS neste protótipo.
- `MOCK` é rótulo obrigatório em logs, respostas e UI. O principal fictício nunca representa conta ou personagem real.
- Tokens são aleatórios, voláteis e expiram. Não são persistidos nem registrados intencionalmente.
- Há limites de payload/cabeçalhos/tempo, rate limit por peer e validação estrita de JSON nos endpoints contratuais.
- Idempotência evita repetição no endpoint demonstrativo enquanto o processo está vivo. O armazenamento é em memória, não distribuído; produção exigiria chave por principal/operação, atomicidade persistente e retenção explicitamente definida.
- O servidor WYD deve validar ouro, itens, experiência, posição, equipamento, fabricação, GM e qualquer mutação. Nenhum campo de cliente pode autorizar esses valores.
- Não existe autorização GM, proteção de replay distribuída, revogação individual, auditoria persistente, TLS ou backend real neste pacote.
- Segredos não são incluídos. Não enviar credenciais reais ao endpoint demonstrativo.

Antes de exposição a rede: threat model, autenticação/identidade verificadas, autorização por operação, TLS, gestão de segredo, persistência idempotente, auditoria imutável, limites por usuário e testes de abuso aprovados.