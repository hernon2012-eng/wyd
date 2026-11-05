# Limitações e premissas

- Nenhum arquivo do servidor, banco, cliente, catálogo ou assets WYD local está disponível neste ambiente.
- Não se conhecem IDs oficiais, limites de stack, slots, categorias de item, receitas ou regras econômicas; exemplos são fictícios e não aprovados.
- O contrato de autorização recebe `Actor` confiável do chamador; este módulo não implementa autenticação/sessão nem transporte.
- Este pacote não integra ao servidor W2PP, ao gateway MOCK ou ao protocolo 12000.
- O armazenamento em memória é apenas demonstrativo/testável: perde sessões/operações em reinício, não fornece garantia exactly-once após restart e não coordena processos.
- O adaptador/migrações foram testados em PostgreSQL 16 descartável local, incluindo recuperação e concorrência. A execução GitHub Actions [42123905756](https://github.com/hernon2012-eng/wyd/actions/runs/42123905756) passou, incluindo a suíte contra PostgreSQL 16. Isso não migra o banco WYD nem autoriza uso em produção.
- O adapter usa pgx v5 via `database/sql` para preservar o contrato existente `*sql.DB`. O chamador ainda é responsável por abrir a conexão com uma configuração segura e por chamar `Store.Migrate` somente depois de validar o destino; migrações não são automáticas.
- O limite de capacidade idempotente é exato e usa um advisory lock global na etapa de escrita do registro; isso serializa a finalização de operações entre personagens e pode limitar throughput. Não há retenção/expurgo automático dos registros.
- Autenticação, autorização, propriedade real do personagem, logs de auditoria e decisões de gameplay dependem do servidor local.
- Produção com filas, estações, tempo, retirada, personagens auxiliares e cotas por conta está fora do motor básico.
- Não foi selecionada licença para o pacote; não se altera a licença do repositório ou de dependências anteriores.
- O workflow `.github/workflows/arko-gameplay-postgres.yml` define validações Go, schemas e integração usando exclusivamente o serviço PostgreSQL descartável e `ARKO_POSTGRES_TEST_DSN`. Não configure um DSN de banco WYD.