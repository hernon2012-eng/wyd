# Integração do cliente Godot

`godot-client-demo/` é um projeto isolado Godot 4.4.1. Usa `HTTPRequest` para health, handshake, autenticação MOCK e consulta de sessão; mostra request ID, estado e modo do backend. Repetição manual e automática têm backoff limitado. O token permanece em memória e não é exibido.

O demo não importa arte, mapas, personagens ou código do cliente WYD. A configuração aponta para loopback. Não substituir o fluxo de rede do cliente original com este exemplo sem revisão do Claude Code.

Validação esperada: abrir o projeto no Godot 4.4.1, iniciar o gateway Go, executar, verificar badge `MOCK`, sessão e request IDs; parar o gateway e observar estado offline/reconexão. A execução gráfica não foi validada neste ambiente até que Godot esteja disponível.