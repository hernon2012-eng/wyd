# Contrato de NPC

`schemas/v1/npc-definition.schema.json` descreve identificador externo opaco, versão, modelo visual sem ID real, nome, mapa/coordenadas, serviços e ativação. Exemplo em `examples/npc-demo.json` usa valores sintéticos e não identifica NPC existente.

Serviços previstos: loja, teleporte, fabricação, produção, quest, arena e administração autorizada. O schema apenas valida forma; permissão, mapa/coordenadas válidos, modelo, alcance, versão e comportamento precisam de confirmação no servidor e no catálogo local. Alterações administrativas precisam de autorização e trilha de auditoria no backend.