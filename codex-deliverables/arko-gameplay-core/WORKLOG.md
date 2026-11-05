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