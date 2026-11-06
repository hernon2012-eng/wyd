# ARKO NPC Dynamic

Módulo Go independente para carregar, consultar e gerenciar um catálogo em
memória de metadados versionados de definição de NPC. As definições obedecem
ao contrato NPC v1 e ao schema JSON copiado em
`schemas/v1/npc-definition.schema.json`. Os dados de
`examples/DEMONSTRAÇÃO/` são sintéticos.

## Uso

```go
package main

import (
    "fmt"

    npc "arko-npc-dynamic"
)

func main() {
    var catalog npc.Catalog
    if err := catalog.ReloadFile("examples/DEMONSTRAÇÃO/catalog.json"); err != nil {
        panic(err)
    }

    if err := catalog.Create(npc.Definition{
        ContractVersion: npc.ContractVersion,
        DefinitionRef:   "demo:npc-placeholder",
        Revision:        1,
        ModelRef:        "demo:model",
        Name:            "NPC demonstração",
        Location:        npc.Location{MapRef: "demo:map", X: 0, Y: 0},
        Services:        []npc.Service{npc.ServiceQuest, npc.ServiceShop},
    }); err != nil {
        panic(err)
    }
    _ = catalog.Activate("demo:npc-placeholder")
    _ = catalog.Publish("demo:npc-placeholder")

    definition, ok := catalog.Published("demo:npc-placeholder")
    if ok {
        fmt.Println(definition.Name, definition.Revision)
    }
}
```

`Create`, `Duplicate`, `Update`, `Activate`, `Deactivate`, `Publish`,
`Unpublish`, `Rollback` e `Remove` oferecem operações de catálogo. Cada
alteração de metadados cria uma revisão crescente; rollback cria uma nova
revisão dos metadados históricos e a publica. Histórico e publicação existem
somente em memória. `ReloadJSON`, `ReloadFile` e `Replace` continuam sendo
substituições integrais, validadas antes da publicação atômica, e reiniciam o
histórico/publicação voláteis. `Lookup`, `List`, `Published` e `History`
retornam cópias defensivas; `List` é ordenado por `definition_ref`. O tipo
`Catalog` é seguro para uso concorrente e não deve ser copiado depois do
primeiro uso.

`Remove` exige um `ReferenceChecker` não-nil; erros ou indisponibilidade do
verificador bloqueiam a remoção. Definições publicadas também devem ser
explicitamente despublicadas antes da remoção. O verificador deve responder
com base na fonte autoritativa das referências rastreadas e não deve chamar
recursivamente operações do mesmo catálogo. A coordenação de concorrência
entre essa fonte e o catálogo continua sendo responsabilidade do chamador.

## Validação

Na pasta deste módulo:

```sh
gofmt -w .
go test ./...
go test -race ./...
go vet ./...
go build ./...
python tests/validate_contracts.py
```

Instale `tests/requirements.txt` para executar o validador Draft 2020-12.

## Escopo

O módulo só representa e valida a forma contratual e mantém estado volátil.
Não confirma IDs/mapas/modelos, coordenadas, permissão, spawn, IA, interação,
execução de serviço ou semântica de jogo. Não conecta nem altera qualquer
sistema ou banco WYD. Consulte `LIMITATIONS.md` antes de qualquer proposta de
integração.
