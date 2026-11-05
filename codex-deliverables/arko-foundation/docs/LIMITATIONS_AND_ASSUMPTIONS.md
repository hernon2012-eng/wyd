# Limitações e premissas

- Workspace conectado: `https://github.com/hernon2012-eng/wyd`, branch inicial `main`, commit inicial `d5d7b49`; contém somente README com título. Nenhum AGENTS.md ou projeto local foi encontrado.
- Código descrito pelo usuário para branch `expansao`, protocolo 12000, Godot 4.4.1, banco, catálogos e Marcos 4A/4B não foi acessado nem confirmado.
- Pesquisa pública refere-se ao W2PP-OpenWYD em `main` na data da coleta; não equivale à cópia local ou ao estado privado. W2PP público documenta o wire de CPSock do cliente WYD original 7662, não confirma compatibilidade com cliente Godot próprio/protocolo 12000 informado.
- O gateway não integra com backend real nem implementa codec CPSock; não é proxy de produção.
- Sessão e idempotência são voláteis, limitadas ao processo. Rate limit é em memória por peer e não substitui proteção distribuída.
- `go test -race ./...`, `go vet ./...` e a validação dos cinco schemas com exemplos foram executados e passaram. Godot não está instalado neste ambiente; a demo não foi executada no editor nem em modo headless. O workflow foi configurado, mas não foi executado pelo GitHub Actions nesta entrega.
- Regras de slots, duas sessões, progressão, PvP, NPC e crafting são propostas de contrato, não comportamento aprovado.
- Actions precisa estar em `.github/workflows/` na raiz para executar; esse arquivo é a única peça de CI fora da pasta de pacote por exigência da plataforma.