# Contrato de multi-personagem

Proposta de tipos está em `schemas/v1/multi-character-account.schema.json`. É somente contrato; não ativa slots, login múltiplo ou regras de conta.

Uma conta pode evoluir de quatro para oito slots, com no máximo duas sessões simultâneas na hipótese inicial. Cada personagem tem progressão/atributos/inventário próprios; cofres compartilhados exigem operações transacionais e controle de concorrência. Uma sessão deve identificar conta, personagem, geração do token e lease; impedir que duas sessões mutem o mesmo personagem sem política explícita.

Categorias listadas para validação local: Arch, Mortal, Celestial, CelestialCS e SCelestial. Não tratar Rei ou Lorde como estágios. Não definir PvP entre categorias por suposição: matriz de competição, matchmaking e exceções dependem de decisão da desenvolvedora e validação das regras reais. O servidor continua autoridade para seleção, progresso, estado e conflitos.