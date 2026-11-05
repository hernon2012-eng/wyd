# Visão do projeto

Este pacote cria uma base isolada para explorar o fluxo Godot → gateway → backend sem assumir que o servidor local informado pela desenvolvedora está disponível. O código executável usa apenas a biblioteca padrão do Go e um backend `MOCK`.

O gateway oferece HTTP/JSON local, contrato `v1`, handshake, request ID, autenticação demonstrativa, sessão em memória com expiração, idempotência, limite de taxa e limites de payload/tempo. O MOCK só cria um principal fictício com ID `mock:demo`; não afirma validar conta ou personagem.

O cliente Godot é um projeto diagnóstico sem ativos originais. Schemas v1 descrevem contratos futuros, não habilitam funcionalidades de jogo. Não há integração, deployment, produção, alteração do servidor WYD ou mudança no protocolo legado.

## Estados

- A, implementado e executado: servidor HTTP, backend MOCK, controles básicos, contratos JSON e testes Go.
- B, implementado mas não executado: projeto gráfico Godot neste ambiente.
- C, documentado: adaptadores reais, domínios e arquitetura de evolução.
- D, dependente de arquivos locais: compatibilidade com branch `expansao`, cliente, assets, catálogo e bancos locais.
- E, decisão da desenvolvedora: regras finais PvP, política de multi-sessão e autorização administrativa.