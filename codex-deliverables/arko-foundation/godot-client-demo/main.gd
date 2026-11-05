extends Control

const BASE_URL := "http://127.0.0.1:8080"
const CONTRACT_VERSION := "1.0.0"

@onready var state_label: Label = %State
@onready var request_id_label: Label = %RequestID
@onready var session_label: Label = %Session
@onready var reconnect_button: Button = %Reconnect
@onready var disconnect_button: Button = %Disconnect
@onready var http: HTTPRequest = %HTTPRequest
@onready var retry_timer: Timer = %RetryTimer

var access_token := ""
var auth_idempotency_key := ""
var retry_delay := 1.0
var pending_stage := ""
var request_in_flight := false

func _ready() -> void:
	http.request_completed.connect(_on_request_completed)
	retry_timer.timeout.connect(_on_retry_timeout)
	reconnect_button.pressed.connect(_begin_connect)
	disconnect_button.pressed.connect(_disconnect_gateway)
	_begin_connect()

func _begin_connect() -> void:
	if request_in_flight:
		return
	if auth_idempotency_key.is_empty():
		auth_idempotency_key = Crypto.new().generate_random_bytes(16).hex_encode()
	access_token = ""
	session_label.text = "Sessão: não autenticada"
	state_label.text = "Estado: verificando gateway"
	pending_stage = "health"
	_send(HTTPClient.METHOD_GET, "/healthz", "")

func _disconnect_gateway() -> void:
	retry_timer.stop()
	if request_in_flight:
		http.cancel_request()
		request_in_flight = false
	if access_token.is_empty():
		auth_idempotency_key = ""
		state_label.text = "Estado: desconectado"
		pending_stage = ""
		return
	pending_stage = "logout"
	state_label.text = "Estado: encerrando sessão MOCK"
	_send(HTTPClient.METHOD_DELETE, "/v1/session", "", PackedStringArray(["Authorization: Bearer " + access_token]))

func _send(method: HTTPClient.Method, path: String, body: String, extra_headers: PackedStringArray = []) -> void:
	var headers := PackedStringArray(["Content-Type: application/json"])
	for header in extra_headers:
		headers.append(header)
	var err := http.request(BASE_URL + path, headers, method, body)
	if err != OK:
		_schedule_retry("Falha ao iniciar requisição (%s)" % error_string(err))
	else:
		request_in_flight = true

func _on_request_completed(result: int, code: int, headers: PackedStringArray, body: PackedByteArray) -> void:
	request_in_flight = false
	if pending_stage == "logout":
		access_token = ""
		auth_idempotency_key = ""
		pending_stage = ""
		state_label.text = "Estado: desconectado" if result == HTTPRequest.RESULT_SUCCESS and code == 200 else "Estado: sessão encerrada localmente"
		return
	if result != HTTPRequest.RESULT_SUCCESS:
		_schedule_retry("Gateway indisponível (%s)" % result)
		return
	var request_id := "-"
	for header in headers:
		if header.to_lower().begins_with("x-request-id:"):
			request_id = header.get_slice(":", 1).strip_edges()
	request_id_label.text = "Request ID: " + request_id
	if code < 200 or code >= 300:
		if pending_stage == "session" and code == 401:
			auth_idempotency_key = ""
		_schedule_retry("Gateway respondeu HTTP %d" % code)
		return
	var parsed: Variant = JSON.parse_string(body.get_string_from_utf8())
	if typeof(parsed) != TYPE_DICTIONARY:
		_schedule_retry("Resposta JSON inválida")
		return
	match pending_stage:
		"health":
			if parsed.get("backend_mode", "") != "MOCK":
				_schedule_retry("Modo do backend inesperado")
				return
			state_label.text = "Estado: negociando contrato"
			pending_stage = "handshake"
			_send(HTTPClient.METHOD_POST, "/v1/handshake", JSON.stringify({"contract_version": CONTRACT_VERSION}))
		"handshake":
			pending_stage = "auth"
			state_label.text = "Estado: autenticação demonstrativa"
			_send(HTTPClient.METHOD_POST, "/v1/auth/demo", JSON.stringify({"display_name": "godot-demo"}), PackedStringArray(["Idempotency-Key: " + auth_idempotency_key]))
		"auth":
			var session_data: Dictionary = parsed.get("session", {})
			access_token = str(session_data.get("access_token", ""))
			if access_token.is_empty():
				_schedule_retry("Sessão MOCK ausente na resposta")
				return
			session_label.text = "Sessão: MOCK ativa até %s" % str(session_data.get("expires_at", ""))
			pending_stage = "session"
			_send(HTTPClient.METHOD_GET, "/v1/session", "", PackedStringArray(["Authorization: Bearer " + access_token]))
		"session":
			state_label.text = "Estado: conectado / MOCK"
			retry_delay = 1.0
			pending_stage = ""
			retry_timer.stop()
			retry_timer.start(30.0)

func _on_retry_timeout() -> void:
	if access_token.is_empty():
		_begin_connect()
		return
	pending_stage = "session"
	_send(HTTPClient.METHOD_GET, "/v1/session", "", PackedStringArray(["Authorization: Bearer " + access_token]))

func _schedule_retry(message: String) -> void:
	state_label.text = "Estado: " + message
	access_token = ""
	pending_stage = ""
	retry_timer.start(retry_delay)
	retry_delay = minf(retry_delay * 2.0, 30.0)