package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"arko-foundation/gateway/internal/config"
)

func newTestServer() *Service {
	return New(config.Config{Address: "127.0.0.1:0"}, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func request(t *testing.T, handler http.Handler, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	return response
}

func TestHealthAndVersion(t *testing.T) {
	handler := newTestServer().Handler()
	if got := request(t, handler, http.MethodGet, "/healthz", "", nil).Code; got != http.StatusOK {
		t.Fatalf("health status = %d, want %d", got, http.StatusOK)
	}
	version := request(t, handler, http.MethodGet, "/v1/version", "", nil)
	if version.Code != http.StatusOK || !strings.Contains(version.Body.String(), `"contract_version":"1.0.0"`) {
		t.Fatalf("version response = %d %s", version.Code, version.Body.String())
	}
}

func TestHandshakeRejectsUnknownVersionAndCorrelatesRequest(t *testing.T) {
	handler := newTestServer().Handler()
	response := request(t, handler, http.MethodPost, "/v1/handshake", `{"contract_version":"9.0.0"}`, map[string]string{"X-Request-ID": "req_123"})
	if response.Code != http.StatusConflict || response.Header().Get("X-Request-ID") != "req_123" || !strings.Contains(response.Body.String(), `"request_id":"req_123"`) {
		t.Fatalf("unexpected handshake response: %d %s", response.Code, response.Body.String())
	}
}

func TestDemoAuthIdempotencyAndSession(t *testing.T) {
	handler := newTestServer().Handler()
	headers := map[string]string{"Idempotency-Key": "login_1", "Content-Type": "application/json"}
	first := request(t, handler, http.MethodPost, "/v1/auth/demo", `{"display_name":"demo"}`, headers)
	second := request(t, handler, http.MethodPost, "/v1/auth/demo", `{"display_name":"demo"}`, headers)
	if first.Code != http.StatusCreated || second.Code != http.StatusCreated || second.Header().Get("Idempotency-Replayed") != "true" {
		t.Fatalf("auth statuses = %d, %d; replay = %q", first.Code, second.Code, second.Header().Get("Idempotency-Replayed"))
	}
	var one, two struct {
		Session struct {
			Token string `json:"access_token"`
		} `json:"session"`
	}
	if err := json.Unmarshal(first.Body.Bytes(), &one); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(second.Body.Bytes(), &two); err != nil {
		t.Fatal(err)
	}
	if one.Session.Token == "" || one.Session.Token != two.Session.Token {
		t.Fatalf("idempotent replay tokens differ or are empty")
	}
	if !strings.Contains(second.Body.String(), `"request_id":"`) {
		t.Fatal("replay response is missing request correlation")
	}
	response := request(t, handler, http.MethodGet, "/v1/session", "", map[string]string{"Authorization": "Bearer " + one.Session.Token})
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"backend_mode":"MOCK"`) {
		t.Fatalf("session response = %d %s", response.Code, response.Body.String())
	}
}

func TestDemoAuthRejectsConflictingAndInvalidRequests(t *testing.T) {
	handler := newTestServer().Handler()
	headers := map[string]string{"Idempotency-Key": "same"}
	if got := request(t, handler, http.MethodPost, "/v1/auth/demo", `{"display_name":"one"}`, headers).Code; got != http.StatusCreated {
		t.Fatalf("first auth status = %d", got)
	}
	if got := request(t, handler, http.MethodPost, "/v1/auth/demo", `{"display_name":"two"}`, headers).Code; got != http.StatusConflict {
		t.Fatalf("conflicting auth status = %d, want %d", got, http.StatusConflict)
	}
	for index, body := range []string{`{"display_name":"demo","extra":true}`, `{"display_name":"demo"} {}`} {
		if got := request(t, handler, http.MethodPost, "/v1/auth/demo", body, map[string]string{"Idempotency-Key": "invalid_" + string(rune('a'+index))}).Code; got != http.StatusBadRequest {
			t.Fatalf("invalid body status = %d for %s", got, body)
		}
	}
	if got := request(t, handler, http.MethodPost, "/v1/auth/demo", `{"display_name":"demo"}`, nil).Code; got != http.StatusBadRequest {
		t.Fatalf("missing idempotency key status = %d", got)
	}
}

func TestDemoAuthConcurrentReplayCreatesOneSession(t *testing.T) {
	handler := newTestServer().Handler()
	const concurrent = 20
	responses := make([]string, concurrent)
	var wg sync.WaitGroup
	for index := range responses {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			response := request(t, handler, http.MethodPost, "/v1/auth/demo", `{"display_name":"demo"}`, map[string]string{"Idempotency-Key": "parallel"})
			if response.Code != http.StatusCreated {
				t.Errorf("response status = %d", response.Code)
			}
			responses[index] = response.Body.String()
		}(index)
	}
	wg.Wait()
	var first struct {
		Session struct {
			Token string `json:"access_token"`
		} `json:"session"`
	}
	if err := json.Unmarshal([]byte(responses[0]), &first); err != nil {
		t.Fatal(err)
	}
	for _, response := range responses[1:] {
		var replay struct {
			Session struct {
				Token string `json:"access_token"`
			} `json:"session"`
		}
		if err := json.Unmarshal([]byte(response), &replay); err != nil {
			t.Fatal(err)
		}
		if replay.Session.Token != first.Session.Token {
			t.Fatal("concurrent idempotent requests returned different sessions")
		}
	}
}

func TestPingRequiresSessionAndIsNonAuthoritative(t *testing.T) {
	handler := newTestServer().Handler()
	if got := request(t, handler, http.MethodPost, "/v1/operation/ping", `{}`, nil).Code; got != http.StatusUnauthorized {
		t.Fatalf("unauthenticated ping status = %d", got)
	}
	auth := request(t, handler, http.MethodPost, "/v1/auth/demo", `{"display_name":"demo"}`, map[string]string{"Idempotency-Key": "ping"})
	var response struct {
		Session struct {
			Token string `json:"access_token"`
		} `json:"session"`
	}
	if err := json.Unmarshal(auth.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	ping := request(t, handler, http.MethodPost, "/v1/operation/ping", `{"payload":{"probe":true}}`, map[string]string{"Authorization": "Bearer " + response.Session.Token})
	if ping.Code != http.StatusOK || !strings.Contains(ping.Body.String(), `"authoritative":false`) {
		t.Fatalf("ping response = %d %s", ping.Code, ping.Body.String())
	}
}

func TestSessionCanBeRevoked(t *testing.T) {
	handler := newTestServer().Handler()
	auth := request(t, handler, http.MethodPost, "/v1/auth/demo", `{"display_name":"demo"}`, map[string]string{"Idempotency-Key": "logout"})
	var response struct {
		Session struct {
			Token string `json:"access_token"`
		} `json:"session"`
	}
	if err := json.Unmarshal(auth.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	headers := map[string]string{"Authorization": "Bearer " + response.Session.Token}
	if got := request(t, handler, http.MethodDelete, "/v1/session", "", headers).Code; got != http.StatusOK {
		t.Fatalf("logout status = %d", got)
	}
	if got := request(t, handler, http.MethodGet, "/v1/session", "", headers).Code; got != http.StatusUnauthorized {
		t.Fatalf("revoked session status = %d", got)
	}
}

func TestBodyLimitAndGeneratedRequestID(t *testing.T) {
	handler := newTestServer().Handler()
	large := strings.Repeat("a", maxBodyBytes+1)
	response := request(t, handler, http.MethodPost, "/v1/handshake", large, nil)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("large payload status = %d", response.Code)
	}
	if response.Header().Get("X-Request-ID") == "" {
		t.Fatal("missing generated request id")
	}
}

func TestRequestIDAndIdempotencyKeyValidation(t *testing.T) {
	for _, id := range []string{"", strings.Repeat("x", 65), "bad id"} {
		if validRequestID(id) {
			t.Fatalf("accepted invalid request id %q", id)
		}
	}
	for _, key := range []string{"", strings.Repeat("x", 129), "bad/key"} {
		if validIdempotencyKey(key) {
			t.Fatalf("accepted invalid idempotency key %q", key)
		}
	}
}
