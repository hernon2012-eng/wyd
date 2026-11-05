# ARKO NPC Dynamic

Módulo Go independente para carregar e consultar um catálogo substituível de
metadados versionados de definição de NPC. As definições obedecem ao contrato
NPC v1 e ao schema JSON copiado em `schemas/v1/npc-definition.schema.json`.
Os dados de `examples/DEMONSTRAÇÃO/` são sintéticos.

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

    definition, ok := catalog.Lookup("demo:npc-placeholder")
    if ok {
        fmt.Println(definition.Name, definition.Revision)
    }
}
```

O módulo não inicia servidor nem oferece endpoint de escrita. `ReloadJSON`,
`ReloadFile` e `Replace` validam todo o lote antes de publicar uma única nova
snapshot; falhas mantêm o catálogo anterior. `Lookup` e `List` retornam cópias
defensivas, e `List` é ordenado por `definition_ref`. O tipo `Catalog` é seguro
para uso concorrente e não deve ser copiado depois do primeiro uso.

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

O módulo só representa e valida a forma contratual. Não confirma IDs/mapas/
modelos, coordenadas, permissão, spawn, IA, interação, execução de serviço ou
semântica de jogo. Não conecta nem altera qualquer sistema ou banco WYD.
Consulte `LIMITATIONS.md` antes de qualquer proposta de integração.
