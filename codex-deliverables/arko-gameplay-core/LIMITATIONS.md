# Limitações e premissas

- Nenhum arquivo do servidor, banco, cliente, catálogo ou assets WYD local está disponível neste ambiente.
- Não se conhecem IDs oficiais, limites de stack, slots, categorias de item, receitas ou regras econômicas; exemplos são fictícios e não aprovados.
- Este pacote não integra ao servidor W2PP, ao gateway MOCK ou ao protocolo 12000.
- O armazenamento em memória é apenas demonstrativo/testável: perde sessões/operações em reinício, não fornece garantia exactly-once após restart e não coordena processos.
- Não há migração PostgreSQL nem alegação de garantia de durabilidade até implementar e testar backend persistente.
- Autenticação, autorização, propriedade real do personagem, logs de auditoria e decisões de gameplay dependem do servidor local.
- Produção com filas, estações, tempo, retirada, personagens auxiliares e cotas por conta está fora do motor básico.
- Não foi selecionada licença para o pacote; não se altera a licença do repositório ou de dependências anteriores.