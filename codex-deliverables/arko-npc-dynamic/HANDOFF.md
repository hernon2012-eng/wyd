# Handoff

## P04 publicado

- Branch: `codex/arko-world-core-20261106`
- Commit: `3e03ecff373db0f3ba060d5c7141af6e142dc361`
- HEAD local e remoto coincidem no SHA acima.
- CI [42138452469](https://github.com/hernon2012-eng/wyd/actions/runs/42138452469)
  passou no SHA exato e na branch indicados.

P04 oferece CRUD/duplicação, ativação/desativação, publicação/despublicação,
remoção protegida por verificador obrigatório e rollback que cria uma nova
revisão. Histórico e publicação são voláteis. Não houve merge, deploy ou
integração WYD; o módulo continua limitado a metadados em memória.

## Verificações locais

Validações executadas para o incremento:

- [x] `gofmt`
- [x] `go test ./...`
- [x] `go test -race -count=1 ./...`
- [x] `go vet ./...`
- [x] `go build ./...`
- [x] `python tests/validate_contracts.py`
- [x] `git diff --check`

Consulte `LIMITATIONS.md` para limites do histórico volátil e do verificador
externo de referências.
