# Limitações e pressupostos

- O módulo contém somente metadados de definição conforme contrato/schema v1.
  `contract_version` e `revision` são declarados pelo catálogo; não há política
  de compatibilidade, histórico, armazenamento persistente ou resolução de
  conflitos entre revisões.
- `definition_ref`, `model_ref` e `map_ref` são strings opacas. Sua existência,
  validade ou significado não é verificado.
- Coordenadas são inteiros sem limites de mapa impostos por este pacote. A
  representação Go usa `int64`; valores JSON fora dessa faixa não podem ser
  representados e serão rejeitados pelo decodificador.
- Não há validação de autorização, execução de serviços, spawn, IA,
  interação, alcance, estado de jogo ou qualquer comportamento de gameplay.
- Não há servidor HTTP, CLI, watcher de arquivos, persistência, conexão com
  W2PP/CPSock, integração com servidor WYD ou acesso a banco.
- Os exemplos contêm placeholders sintéticos e não são dados reais nem
  aprovados para produção.
- O schema não define limite máximo para `revision` nem para `x`/`y`; os tipos
  inteiros Go impõem limite de representação conforme descrito acima.
