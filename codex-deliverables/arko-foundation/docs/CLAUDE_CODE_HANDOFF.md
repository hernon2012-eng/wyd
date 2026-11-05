# Handoff para Claude Code

1. Confira a árvore deste pacote e leia `LIMITATIONS_AND_ASSUMPTIONS.md`; mantenha a separação MOCK/real.
2. No projeto local, identifique servidor, branch, commit, listener, `CPSock`, dispatch, sessão, login e protocolo efetivamente usados. Compare com cada versão e campo de `schemas/v1/`; não presuma que protocolo 12000 seja igual ao CPSock do W2PP público.
3. Registre incompatibilidades com arquivo, tipo/operação e evidência. Revise licença/dependências antes de copiar ou integrar código; rejeite assets protegidos sem autorização.
4. Revise o gateway quanto a ameaça, auth, expiração, idempotência persistente, limites, cancelamento e observabilidade. O MOCK não pode ser promovido a backend real por configuração acidental.
5. Se compatível, implemente backend real em módulo isolado, com autorização server-side. Preserve intacto o protocolo antigo e exija testes de interoperabilidade antes de habilitar qualquer adaptador.
6. Adapte o cliente Godot em uma branch local separada, sem misturar o projeto demo ao cliente original até revisar o contrato e o fluxo de assets.
7. Execute `cd codex-deliverables/arko-foundation/gateway && go test -race ./... && go vet ./...`; valide schemas e exemplos; execute testes/headless do Godot 4.4.1 se instalado. Execute também testes locais específicos do servidor/cliente autorizado.
8. Rejeite qualquer implementação que contradiga o protocolo real, atribua autoridade ao cliente, invente dados, exponha tokens, altere regras/catálogos ou faça deploy/merge sem aprovação.

Nenhuma integração WYD está concluída por este pacote.