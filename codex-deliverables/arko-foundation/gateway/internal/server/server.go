package server

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"arko-foundation/gateway/internal/backend"
	"arko-foundation/gateway/internal/config"
	"arko-foundation/gateway/internal/idempotency"
	"arko-foundation/gateway/internal/ratelimit"
	"arko-foundation/gateway/internal/session"
)

const contractVersion = "1.0.0"
const maxBodyBytes = 16 << 10

type Service struct {
	authMu   sync.Mutex
	backend  backend.Backend
	sessions *session.Manager
	idem     *idempotency.Store
	limiter  *ratelimit.Limiter
	log      *slog.Logger
}

func New(_ config.Config, logger *slog.Logger) *Service {
	return &Service{
		backend:  backend.Mock{},
		sessions: session.New(15 * time.Minute),
		idem:     idempotency.New(15 * time.Minute),
		limiter:  ratelimit.New(20, 40),
		log:      logger,
	}
}

func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /v1/version", s.version)
	mux.HandleFunc("POST /v1/handshake", s.handshake)
	mux.HandleFunc("POST /v1/auth/demo", s.demoAuth)
	mux.HandleFunc("GET /v1/session", s.currentSession)
	mux.HandleFunc("DELETE /v1/session", s.deleteSession)
	mux.HandleFunc("POST /v1/operation/ping", s.ping)
	return s.requestContext(s.observe(s.rateLimit(mux)))
}

type responseMeta struct {
	RequestID string `json:"request_id"`
}

type apiError struct {
	Meta  responseMeta `json:"meta"`
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (s *Service) health(w http.ResponseWriter, _ *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "backend_mode": "MOCK"})
}

func (s *Service) version(w http.ResponseWriter, _ *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]string{"service": "arko-gateway", "contract_version": contractVersion, "backend_mode": "MOCK"})
}

func (s *Service) handshake(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ContractVersion string `json:"contract_version"`
	}
	if !s.decode(w, r, &req) {
		return
	}
	if req.ContractVersion != contractVersion {
		s.writeError(w, r, http.StatusConflict, "version_mismatch", "unsupported contract version")
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"meta": responseMeta{RequestID: requestID(r)}, "contract_version": contractVersion, "heartbeat_seconds": 30, "backend_mode": "MOCK"})
}

func (s *Service) demoAuth(w http.ResponseWriter, r *http.Request) {
	body, ok := s.readBody(w, r)
	if !ok {
		return
	}
	key := r.Header.Get("Idempotency-Key")
	fingerprint := idempotency.Fingerprint(body)
	if !validIdempotencyKey(key) {
		s.writeError(w, r, http.StatusBadRequest, "idempotency_key_required", idempotency.ErrRequired.Error())
		return
	}
	s.authMu.Lock()
	defer s.authMu.Unlock()
	if cached, found, err := s.idem.Get(key, fingerprint); err != nil {
		s.writeError(w, r, http.StatusConflict, "idempotency_conflict", err.Error())
		return
	} else if found {
		var cachedResponse map[string]any
		if err := json.Unmarshal(cached.Body, &cachedResponse); err != nil {
			s.writeError(w, r, http.StatusInternalServerError, "idempotency_record_invalid", "cached response is unavailable")
			return
		}
		meta, _ := cachedResponse["meta"].(map[string]any)
		if meta == nil {
			meta = make(map[string]any)
			cachedResponse["meta"] = meta
		}
		meta["request_id"] = requestID(r)
		cached.Body, _ = json.Marshal(cachedResponse)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Idempotency-Replayed", "true")
		w.WriteHeader(cached.Status)
		_, _ = w.Write(cached.Body)
		return
	}
	var req struct {
		DisplayName string `json:"display_name"`
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil || decoder.Decode(new(any)) != io.EOF || strings.TrimSpace(req.DisplayName) == "" || len(req.DisplayName) > 64 {
		s.writeError(w, r, http.StatusBadRequest, "invalid_payload", "display_name must contain 1 to 64 bytes")
		return
	}
	principal, err := s.backend.Authenticate(r.Context(), backend.AuthRequest{DisplayName: strings.TrimSpace(req.DisplayName)})
	if err != nil {
		s.writeError(w, r, http.StatusUnauthorized, "authentication_failed", "mock authentication failed")
		return
	}
	created, err := s.sessions.Create(principal.ID, principal.BackendMode)
	if err != nil {
		s.writeError(w, r, http.StatusInternalServerError, "session_create_failed", "could not create session")
		return
	}
	response, err := json.Marshal(map[string]any{"meta": responseMeta{RequestID: requestID(r)}, "session": created, "notice": "MOCK ONLY: no WYD account or character was authenticated"})
	if err != nil {
		s.writeError(w, r, http.StatusInternalServerError, "internal_error", "could not encode response")
		return
	}
	if err := s.idem.Put(key, fingerprint, http.StatusCreated, response); err != nil {
		s.writeError(w, r, http.StatusConflict, "idempotency_conflict", err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write(response)
}

func (s *Service) currentSession(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.authorize(w, r)
	if !ok {
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"meta": responseMeta{RequestID: requestID(r)}, "session": map[string]any{"session_id": sess.ID, "backend_mode": sess.Backend, "expires_at": sess.ExpiresAt}})
}

func (s *Service) deleteSession(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := s.sessions.Revoke(sess.Token); err != nil {
		s.writeError(w, r, http.StatusUnauthorized, "session_expired", "session is invalid or expired")
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"meta": responseMeta{RequestID: requestID(r)}, "revoked": true, "backend_mode": "MOCK"})
}

func (s *Service) ping(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authorize(w, r); !ok {
		return
	}
	var req struct {
		Payload json.RawMessage `json:"payload"`
	}
	if !s.decode(w, r, &req) {
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"meta": responseMeta{RequestID: requestID(r)}, "operation": "ping", "accepted": true, "authoritative": false, "backend_mode": "MOCK"})
}

func (s *Service) authorize(w http.ResponseWriter, r *http.Request) (session.Session, bool) {
	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		s.writeError(w, r, http.StatusUnauthorized, "unauthorized", "bearer session token required")
		return session.Session{}, false
	}
	sess, err := s.sessions.Get(parts[1])
	if err != nil {
		s.writeError(w, r, http.StatusUnauthorized, "session_expired", "session is invalid or expired")
		return session.Session{}, false
	}
	return sess, true
}

func (s *Service) decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	body, ok := s.readBody(w, r)
	if !ok {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		s.writeError(w, r, http.StatusBadRequest, "invalid_payload", "request body must match the v1 JSON contract")
		return false
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		s.writeError(w, r, http.StatusBadRequest, "invalid_payload", "request body must contain one JSON value")
		return false
	}
	return true
}

func (s *Service) readBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	if r.Body == nil {
		s.writeError(w, r, http.StatusBadRequest, "invalid_payload", "request body is required")
		return nil, false
	}
	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes+1))
	if err != nil || len(body) > maxBodyBytes {
		s.writeError(w, r, http.StatusRequestEntityTooLarge, "payload_too_large", "request body exceeds 16 KiB")
		return nil, false
	}
	return body, true
}

func (s *Service) writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (s *Service) writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	var response apiError
	response.Meta.RequestID = requestID(r)
	response.Error.Code = code
	response.Error.Message = message
	s.writeJSON(w, status, response)
}

func requestID(r *http.Request) string { return r.Header.Get("X-Request-ID") }

func (s *Service) requestContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if !validRequestID(id) {
			id = newRequestID()
		}
		w.Header().Set("X-Request-ID", id)
		r.Header.Set("X-Request-ID", id)
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (w *statusRecorder) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
		w.ResponseWriter.WriteHeader(status)
	}
}

func (w *statusRecorder) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}

func (s *Service) observe(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		recorder := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(recorder, r)
		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}
		s.log.Info("http request", "request_id", requestID(r), "method", r.Method, "path", r.URL.Path, "status", status, "duration_ms", time.Since(started).Milliseconds())
	})
}

func validRequestID(id string) bool {
	if len(id) < 1 || len(id) > 64 {
		return false
	}
	for _, r := range id {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

func newRequestID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err == nil {
		return hex.EncodeToString(bytes[:])
	}
	return time.Now().UTC().Format("20060102T150405.000000000")
}

func validIdempotencyKey(key string) bool {
	if len(key) < 1 || len(key) > 128 {
		return false
	}
	for _, r := range key {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

func (s *Service) rateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peer, _, _ := net.SplitHostPort(r.RemoteAddr)
		if peer == "" {
			peer = r.RemoteAddr
		}
		if !s.limiter.Allow(peer) {
			s.writeError(w, r, http.StatusTooManyRequests, "rate_limited", "request rate limit exceeded")
			return
		}
		next.ServeHTTP(w, r)
	})
}
