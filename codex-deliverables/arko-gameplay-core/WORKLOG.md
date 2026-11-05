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