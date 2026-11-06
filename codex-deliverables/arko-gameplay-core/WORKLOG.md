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

## 2026-11-05 — Publicação e CI remota do Pacote 3

- Autorização GitHub concluída pelo fluxo de dispositivo; o escopo `workflow` foi concedido para publicar os arquivos de Actions.
- Publicada sem force a branch `codex/arko-gameplay-postgres`, incluindo `7f744402ae4bcc2111c242af8aec79bf2374c286`, `3f046a517da20063cd913acf9d5f5b11383e5b88` e `f38d3d85dc34509fc0b57a49fc902b39affb8d76`.
- SHA remoto confirmado por `git ls-remote`: `f38d3d85dc34509fc0b57a49fc902b39affb8d76`, igual ao HEAD local.
- GitHub Actions `42123905756` para o push de `f38d3d85dc34509fc0b57a49fc902b39affb8d76` concluiu com `success`; jobs `go`, `contracts` e `postgres-integration` passaram. O job de integração executou `go test ./... -count=1` e `go test -race ./... -count=1` contra o serviço PostgreSQL 16.
- A execução local anterior também passou com PostgreSQL 16 descartável; o container de teste foi removido.
- A revisão de segurança independente previamente registrada não encontrou vulnerabilidades no escopo revisado. Isso não equivale a integração, deploy ou aprovação para usar o WYD real.
- O `STATUS_GERAL_ARKO.md` permanece sem alteração no worktree `codex/arko-foundation`; sua seção do Pacote 3 está desatualizada e precisa de atualização coordenada após revisão. Não foram localizados `ROADMAP_PRODUCAO_ARKO.md` nem `CONSOLIDACAO_ARQUITETURAL_ARKO.md`.
- Nenhum banco WYD foi acessado/migrado; não houve merge nem deploy. Ponto de parada: aguardar revisão do usuário, sem iniciar outros pacotes.

## 2026-11-06 — P07: catálogo de receitas v2

- Trabalho iniciado isoladamente no worktree `codex/arko-recipes-v2-20261106`, base `1870b89`; alterações ficam somente neste worktree e sem commit/push.
- Adicionado contrato v2 de definição/request/result e arquivo sintético `recipes/demonstration/demo-alloy-v2.json`. O loader lê um JSON por arquivo sob shard de categoria, isola arquivos inválidos e mantém versões históricas imutáveis por `(recipe_id, version)`.
- Reload publica uma geração por troca atômica; falha na raiz mantém snapshot. Diagnósticos por arquivo são expostos; leituras retornam cópias defensivas.
- Extensões v2 conectadas ao `Engine.Craft` atual. Resolução por ID é direta; resolução por categoria exige policy server-side explícita e soma exata; payloads de requisitos permanecem opacos até um checker tipado autorizar. Erro operacional de policy aborta sem memorizar rejeição.
- Transformação/produto usa `product` e motor de crafting atual; definição do item produzido deve já existir no catálogo de itens do repositório, sem criar estado de item a partir do arquivo de receita. Nenhuma regra oficial foi inferida.
- Não foram adicionadas filas, timers, ordens, migração de gameplay ou acesso a servidor/banco WYD.
- Verificação: `gofmt`, `go test -race -count=1 ./...`, `go vet ./...`, `go build ./...`, `python tests/validate_contracts.py` e `git diff --check` passaram; contratos validaram 7 schemas e 7 contratos/exemplos sintéticos. Nenhum commit/push foi feito.

## 2026-11-06 — P07 follow-up de revisão

- Adicionado `RECIPES_V2.md` com API/schema, sharding, reload/histórico, duplicatas, idempotência, policies e limites.
- Tornado `output_mode` obrigatório no contrato v2 (`new_item` ou `transform`). Transformação declara `transform_source` por ID+quantidade; engine agrega essa fonte aos materiais e delega consumo/entrega ao `PlanCraft` existente, sem atributo por instância. Testes cobrem os dois modos e rollback após entrega parcial planejada.
- O loader agora diagnostica por arquivo duplicatas da mesma dupla ID+versão no reload, mesmo com conteúdo idêntico, mantendo apenas primeira candidata válida em ordem estável. Fixture e teste regressivo adicionados.
- Mantida compatibilidade de receitas legadas v1: validação core permite modo vazio nos dados existentes; o loader de definições v2 exige explicitamente um dos modos.
- Verificação follow-up: `go test -count=1 ./...`, `go test -race -count=1 ./...`, `go vet ./...`, `go build ./...`, `python tests/validate_contracts.py` (7 schemas e 7 contratos/exemplos), `gofmt` e `git diff --check` passaram. Sem commit/push; sem acesso a banco.
- Commit de implementação local: `982f39272d0d92d7ea9cc21703e26f9954305533`. O SHA remoto ainda precisa ser confirmado após publicação normal da branch isolada.