# Worklog

## 2026-11-05 — Início do Pacote 2

- Estado inicial verificado: branch `codex/arko-foundation`, commit `3c90ec9`, árvore limpa.
- Criada a branch isolada `codex/arko-gameplay-core`, derivada do pacote anterior.
- Hipótese de arquitetura: estoque, consumo, produto e registro idempotente precisam compartilhar uma transação atômica. Check discriminante: testes devem demonstrar ausência de mudanças após falha em qualquer etapa e replay sem segunda fabricação.
- Revisão independente (arquitetura, fabricação, segurança e QA): recomendou fabricação síncrona sem estado `in_progress` observável; rejeições de domínio determinísticas devem ser registradas junto com o replay; falhas de infraestrutura não são rejeições; a chave é única por personagem + tipo de operação + chave e fingerprint divergente nessa tupla conflita.
- Corrigida a declaração prematura de implementação no README e detalhados 17 cenários de teste individualmente.
- Validação do checkpoint documental: `git diff --check` e presença dos nove documentos obrigatórios + `go.mod` passaram. Testes Go não aplicáveis ainda: nenhum pacote de código foi criado neste estágio.
- Não há acesso a servidor/banco/cliente local nem a catálogo oficial; IDs e receitas reais não serão inferidos.
- Próximo checkpoint: commitar/publicar os documentos-base e implementar o planejador de inventário e as transações.

## 2026-11-05 — Modelos e operações puras

- C0 publicado: `ca207be` (`docs: bootstrap ARKO gameplay core`) em `origin/codex/arko-gameplay-core`.
- Implementados modelos genéricos, validação de slots/stacks, adição, remoção, transferência atômica em memória e planejamento puro de fabricação com agregação determinística e overflow checks.
- Testes cobrindo rollback local de add/transfer, ordem de consumo, slots liberados, agregação de ingredientes e fuzz de conservação em add.
- Validação executada: `gofmt -w internal/core && go test ./...` passou.
- Limite desta etapa: ainda não há serviço de crafting, repositório transacional ou idempotência.

## 2026-11-05 — Serviço transacional e idempotência

- C1 publicado: `8e56c6d` (`feat: add inventory planning primitives`) em `origin/codex/arko-gameplay-core`.
- Implementados contrato `Repository/Transaction`, `MemoryStore` copy-on-write serializado, escopo idempotente `(personagem, operação, chave)`, capacidade fail-closed e failpoints em gravação de inventário/operação/commit.
- Implementados crafting/add/remove/transfer com autorização via `Actor`, fingerprint sem request ID, replay de resultado terminal e rejeições determinísticas persistidas. Sucesso só retorna após commit.
- Testes cobrindo os 17 casos da matriz, incluindo concorrência, retry após resposta perdida, recuperação de storage failure e rollback por failpoint.
- Validação executada: `gofmt -w internal/core`, `go test ./...`, `go test -race ./...`, `go vet ./...`, `go build ./...` e `gofmt -l` passaram.
- Limite desta etapa: `MemoryStore` é globalmente serializado e volátil; a interface não comprova implementação durável nem multi-processo.

## 2026-11-05 — Contratos, exemplos e CI

- C2 publicado: `798294c` (`feat: add atomic crafting and idempotency`) em `origin/codex/arko-gameplay-core`.
- Adicionados quatro schemas v1, quatro fixtures DEMONSTRAÇÃO, validação Draft 2020-12 e workflow próprio do pacote.
- Acrescentados testes do serviço de add/remove/transfer, slots livres e decisões de requisito no servidor; limites de requisitos também validados.
- Revisão contrato/schema identificou e corrigiu `operation_id` ausente em conflito idempotente; teste regressivo adicionado.
- `python tests/validate_contracts.py` passou. A última rodada do fuzz smoke `FuzzAddRemoveConservesQuantity` executou 42.587 casos em 2 s.
- Após a correção de conflito, passaram `gofmt -l`, `go test ./...`, `go test -race ./...`, `go vet ./...`, `go build ./...`, validador JSON, fuzz smoke e `git diff --check`.
- C3 publicado: `599c933` (`feat: add gameplay contracts and CI`) em `origin/codex/arko-gameplay-core`.
- Conferência final da branch e estado Git registrada após o commit de metadados.

## 2026-11-05 — Gate PostgreSQL real

- Implementado adapter pgx v5 via `database/sql`, mantendo o contrato `*sql.DB`; criadas migrações ARKO versionadas e explícitas para tabelas `arko_gameplay_*`. `NewStore` não migra automaticamente.
- Refatorado `Transaction` para leituras que retornam erros; `Engine` e `MemoryStore` preservam erros de storage/cancelamento. O PostgreSQL propaga contextos, usa `READ COMMITTED`, advisory lock de chave idempotente e `SELECT FOR UPDATE` para inventários em ordem estável. Erros SQLSTATE 40001/40P01 são marcados `ErrRetryableStorage`; não há retry automático.
- Idempotência é imutável: insert em conflito falha e não atualiza resultado anterior. Inventário e registro terminal são gravados no mesmo commit.
- Implementada integração real com schema descartável único por teste, host/porta/banco/usuário fixos; nenhum banco WYD é usado. Adicionados casos de persistência/restart, crafting, transfer, duplicação, conflito de payload, rollback, concorrência e falha de conexão.
- C4 de código commitado localmente em `7f74440` (`feat: harden PostgreSQL gameplay persistence`). `git push origin codex/arko-gameplay-postgres` falhou: `fatal: unable to get password from user`; não há credencial GitHub configurada.
- CI/documentação/handoff commitados localmente em `3f046a5` (`ci: validate ARKO gameplay PostgreSQL`); branch remota continua sem publicação pela mesma limitação de autenticação.
- Execuções locais contra PostgreSQL 16 descartável `arko_test`: `go test ./...` passou; `go test -race ./...` passou; `go vet ./...` passou; `go build ./...` passou; `test -z "$(gofmt -l .)"` passou; `python tests/validate_contracts.py` validou quatro schemas/exemplos; `git diff --check` passou.
- O workflow dedicado `.github/workflows/arko-gameplay-postgres.yml` contém serviço PostgreSQL e o gate CI, mas ainda não foi executado no GitHub. Preservado também o workflow de CI publicado do Pacote 2.
- Segurança: duas revisões independentes do adapter, incluindo migrações, concorrência, isolamento e alterações finais, não encontraram vulnerabilidades.
- Nenhum teste foi executado contra `wyd`/`wyd_dev`; sem merge ou deploy. Push remoto depende de autenticação GitHub disponível.