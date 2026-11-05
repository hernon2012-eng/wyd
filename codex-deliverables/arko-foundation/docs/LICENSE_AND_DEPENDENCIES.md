# Licença e dependências

A API pública consultada de [W2PP-OpenWYD](https://github.com/Jean1dev/w2pp-OpenWYD) identifica `GPL-2.0`. Nenhum trecho de código ou ativo foi copiado. A referência técnica citada não concede direitos sobre assets WYD ou outros projetos.

O gateway usa somente biblioteca padrão do Go (`go.mod` sem dependências externas). O validador de schemas do CI usa Python `jsonschema`, instalado no job e não necessário para executar o gateway. O projeto Godot usa APIs nativas, sem addons. Nenhum LICENSE próprio foi inventado; a inclusão de licença do pacote aguarda decisão do titular.

Não incluímos código de `alanpetry/OpenWyd`, TMProject2GlobalClient, cliente WYD ou recursos gráficos. A compatibilidade de licença para quaisquer dependências futuras precisa ser revisada antes de adoção.