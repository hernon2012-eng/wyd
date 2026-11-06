# Limitações e pressupostos

- O módulo contém somente metadados de definição conforme contrato/schema v1.
  `contract_version` é validado pelo catálogo. Histórico de revisões e ponteiro
  de publicação são mantidos apenas em memória e se perdem no encerramento do
  processo; `ReloadJSON`, `ReloadFile` e `Replace` substituem o snapshot e
  reiniciam esse histórico volátil. Não há armazenamento persistente nem
  resolução distribuída de conflitos entre revisões.
- `definition_ref`, `model_ref` e `map_ref` são strings opacas. Sua existência,
  validade ou significado não é verificado.
- Coordenadas são inteiros sem limites de mapa impostos por este pacote. A
  representação Go usa `int64`; valores JSON fora dessa faixa não podem ser
  representados e serão rejeitados pelo decodificador.
- Não há validação de autorização, execução de serviços, spawn, IA,
  interação, alcance, estado de jogo ou qualquer comportamento de gameplay.
- `Remove` requer um `ReferenceChecker` fornecido pelo chamador e falha fechado
  quando ele está ausente ou retorna erro. Este módulo não mantém o grafo de
  referências nem fornece um verificador concreto; portanto, sem uma fonte
  autoritativa fornecida pelo integrador, a operação não pode remover
  definições. Referências externas também não podem ser sincronizadas por este
  catálogo, nem há atomicidade transacional entre o verificador externo e este
  catálogo; o chamador deve coordenar mudanças concorrentes de referências.
- Publicação significa apenas o ponteiro em memória para uma revisão; não há
  entrega a runtime, ativação real no mundo ou garantia de disponibilidade.
  Rollback regrava metadados de uma revisão anterior como uma nova revisão e
  move esse ponteiro de publicação em memória.
- Não há servidor HTTP, CLI, watcher de arquivos, persistência, conexão com
  W2PP/CPSock, integração com servidor WYD ou acesso a banco.
- Os exemplos contêm placeholders sintéticos e não são dados reais nem
  aprovados para produção.
- O schema não define limite máximo para `revision` nem para `x`/`y`; os tipos
  inteiros Go impõem limite de representação conforme descrito acima.
