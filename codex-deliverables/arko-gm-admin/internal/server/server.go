package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"arko-gm-admin/internal/ratelimit"
)

const (
	maxRequestBytes = 16 << 10
	maxSearchLimit  = 100
)

var (
	ErrUnauthenticated = errors.New("unauthenticated")
	ErrForbidden       = errors.New("forbidden")
	ErrNotFound        = errors.New("not found")
	ErrInvalid         = errors.New("invalid request")
	ErrUnavailable     = errors.New("service unavailable")
)

type Principal struct {
	ID          string `json:"principal_ref"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	Status      string `json:"status"`
}

type Session struct {
	Token     string    `json:"access_token"`
	TokenType string    `json:"token_type"`
	ExpiresAt time.Time `json:"expires_at"`
	Principal Principal `json:"principal"`
}

type Identity interface {
	Login(context.Context, string, string) (Session, error)
	Resolve(context.Context, string) (Principal, error)
	Revoke(context.Context, string) error
}

type Authorizer interface {
	Allows(role, capability string) bool
}

type DirectoryQuery struct {
	Type     string
	Query    string
	Page     int
	PageSize int
}

type DirectoryResult struct {
	Accounts   []AccountSummary   `json:"accounts"`
	Characters []CharacterSummary `json:"characters"`
	NextCursor *string            `json:"next_cursor"`
}

type AccountSummary struct {
	Reference string `json:"account_ref"`
	Name      string `json:"display_name"`
}

type CharacterSummary struct {
	Reference  string `json:"character_ref"`
	AccountRef string `json:"account_ref"`
	Name       string `json:"display_name"`
}

type Directory interface {
	Search(context.Context, DirectoryQuery) (DirectoryResult, error)
}

type AuditEvent struct {
	EventID    string    `json:"event_ref"`
	ActorID    string    `json:"actor_ref"`
	ActorRole  string    `json:"actor_role"`
	Action     string    `json:"action"`
	TargetType string    `json:"target_type"`
	TargetRef  string    `json:"target_ref"`
	Capability string    `json:"permission"`
	Reason     string    `json:"reason"`
	RequestID  string    `json:"request_id"`
	Outcome    string    `json:"outcome"`
	OccurredAt time.Time `json:"occurred_at"`
}

type AuditQuery struct {
	ActorID string
	Limit   int
	Before  time.Time
}

type Audit interface {
	Append(context.Context, AuditEvent) (AuditEvent, error)
	List(context.Context, AuditQuery) ([]AuditEvent, error)
}

type CommandRequest struct {
	Actor     Principal
	Name      string
	Reason    string
	TargetRef string
	Payload   json.RawMessage
	RequestID string
	DryRun    bool
}

type CommandResult struct {
	IntentRef string `json:"intent_ref"`
	Status    string `json:"status"`
	Message   string `json:"message"`
}

type Commands interface {
	RequiredCapability(string) (string, bool)
	// Preview must not mutate gameplay state.
	Preview(context.Context, CommandRequest) (CommandResult, error)
}

type Dependencies struct {
	Identity   Identity
	Authorizer Authorizer
	Directory  Directory
	Audit      Audit
	Commands   Commands
	Limiter    *ratelimit.Limiter
	Now        func() time.Time
}

type Server struct {
	deps Dependencies
	mux  *http.ServeMux
}

func New(deps Dependencies) (*Server, error) {
	if deps.Identity == nil || deps.Authorizer == nil || deps.Directory == nil || deps.Audit == nil || deps.Commands == nil || deps.Limiter == nil {
		return nil, ErrInvalid
	}
	if deps.Now == nil {
		deps.Now = time.Now
	}
	s := &Server{deps: deps, mux: http.NewServeMux()}
	s.mux.HandleFunc("GET /healthz", s.health)
	s.mux.HandleFunc("POST /api/v1/session", s.login)
	s.mux.HandleFunc("GET /api/v1/session", s.currentSession)
	s.mux.HandleFunc("DELETE /api/v1/session", s.logout)
	s.mux.HandleFunc("POST /api/v1/directory/search", s.searchDirectory)
	s.mux.HandleFunc("GET /api/v1/audit", s.listAudit)
	s.mux.HandleFunc("POST /api/v1/commands/{name}", s.executeCommand)
	return s, nil
}

func (s *Server) Handler() http.Handler {
	return s.deps.Limiter.Middleware(securityHeaders(s.mux))
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "mode": "LOCAL DEMONSTRAÇÃO"})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	if request.Username == "" || len(request.Username) > 128 || len(request.Password) == 0 || len(request.Password) > 1024 {
		writeError(w, http.StatusBadRequest, "invalid_request", "credentials are invalid")
		return
	}
	session, err := s.deps.Identity.Login(r.Context(), request.Username, request.Password)
	if err != nil {
		if errors.Is(err, ErrUnauthenticated) {
			writeError(w, http.StatusUnauthorized, "authentication_failed", "authentication failed")
		} else {
			writeError(w, http.StatusServiceUnavailable, "identity_unavailable", "authentication service is unavailable")
		}
		return
	}
	event := s.event(session.Principal, "session.login", "session", session.Principal.ID, "session.login", "Authenticated session", requestID(r))
	_, auditErr := s.deps.Audit.Append(r.Context(), event)
	if auditErr == nil {
		auditErr = s.appendOutcome(r.Context(), event, "approved")
	}
	if auditErr != nil {
		if revokeErr := s.deps.Identity.Revoke(r.Context(), session.Token); revokeErr != nil {
			writeError(w, http.StatusServiceUnavailable, "audit_unavailable", "login could not be recorded; session revocation failed")
			return
		}
		writeError(w, http.StatusServiceUnavailable, "audit_unavailable", "login could not be recorded")
		return
	}
	writeJSON(w, http.StatusCreated, session)
}

func (s *Server) currentSession(w http.ResponseWriter, r *http.Request) {
	principal, _, ok := s.authenticate(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, principal)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	principal, token, ok := s.authenticate(w, r)
	if !ok {
		return
	}
	event := s.event(principal, "session.logout", "session", principal.ID, "session.logout", "Session revocation", requestID(r))
	if _, err := s.deps.Audit.Append(r.Context(), event); err != nil {
		writeError(w, http.StatusServiceUnavailable, "audit_unavailable", "logout could not be recorded")
		return
	}
	if err := s.deps.Identity.Revoke(r.Context(), token); err != nil {
		if auditErr := s.appendOutcome(r.Context(), event, "failed"); auditErr != nil {
			writeError(w, http.StatusServiceUnavailable, "audit_unavailable", "logout outcome could not be recorded")
			return
		}
		writeError(w, http.StatusServiceUnavailable, "session_revoke_failed", "session could not be revoked")
		return
	}
	if err := s.appendOutcome(r.Context(), event, "approved"); err != nil {
		writeError(w, http.StatusServiceUnavailable, "audit_unavailable", "logout completed but its outcome could not be recorded")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"revoked": true})
}

func (s *Server) searchDirectory(w http.ResponseWriter, r *http.Request) {
	principal, _, ok := s.require(w, r, "directory.read")
	if !ok {
		return
	}
	var request struct {
		Type     string `json:"type"`
		Query    string `json:"query"`
		Page     int    `json:"page,omitempty"`
		PageSize int    `json:"page_size,omitempty"`
		Reason   string `json:"reason"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	request.Type = strings.TrimSpace(request.Type)
	request.Query = strings.TrimSpace(request.Query)
	request.Reason = strings.TrimSpace(request.Reason)
	if (request.Type != "account" && request.Type != "character") || request.Query == "" || !utf8.ValidString(request.Query) || len(request.Query) > 100 ||
		request.Reason == "" || !utf8.ValidString(request.Reason) || strings.ContainsRune(request.Reason, '\x00') || len(request.Reason) > 512 {
		writeError(w, http.StatusBadRequest, "invalid_request", "type, bounded query, and reason are required")
		return
	}
	if request.Page == 0 {
		request.Page = 1
	}
	if request.PageSize == 0 {
		request.PageSize = 25
	}
	if request.Page < 1 || request.Page > 1_000_000 || request.PageSize < 1 || request.PageSize > maxSearchLimit {
		writeError(w, http.StatusBadRequest, "invalid_request", "page and page_size are outside allowed bounds")
		return
	}
	event := s.event(principal, "directory.search", request.Type, request.Query, "directory.read", request.Reason, requestID(r))
	if _, err := s.deps.Audit.Append(r.Context(), event); err != nil {
		writeError(w, http.StatusServiceUnavailable, "audit_unavailable", "search could not be audited")
		return
	}
	result, err := s.deps.Directory.Search(r.Context(), DirectoryQuery{Type: request.Type, Query: request.Query, Page: request.Page, PageSize: request.PageSize})
	if err != nil {
		if auditErr := s.appendOutcome(r.Context(), event, "failed"); auditErr != nil {
			writeError(w, http.StatusServiceUnavailable, "audit_unavailable", "search failure could not be audited")
			return
		}
		writeError(w, http.StatusServiceUnavailable, "directory_unavailable", "directory search is unavailable")
		return
	}
	if err := s.appendOutcome(r.Context(), event, "approved"); err != nil {
		writeError(w, http.StatusServiceUnavailable, "audit_unavailable", "search completed but its outcome could not be recorded")
		return
	}
	if result.Accounts == nil {
		result.Accounts = []AccountSummary{}
	}
	if result.Characters == nil {
		result.Characters = []CharacterSummary{}
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) listAudit(w http.ResponseWriter, r *http.Request) {
	principal, _, ok := s.require(w, r, "audit.read")
	if !ok {
		return
	}
	query := AuditQuery{Limit: 100}
	if value := r.URL.Query().Get("limit"); value != "" {
		limit, err := strconv.Atoi(value)
		if err != nil || limit < 1 || limit > 100 {
			writeError(w, http.StatusBadRequest, "invalid_request", "limit must be between 1 and 100")
			return
		}
		query.Limit = limit
	}
	query.ActorID = r.URL.Query().Get("actor_ref")
	if !utf8.ValidString(query.ActorID) || strings.ContainsRune(query.ActorID, '\x00') || len(query.ActorID) > 256 {
		writeError(w, http.StatusBadRequest, "invalid_request", "actor_ref is invalid")
		return
	}
	target := query.ActorID
	if target == "" {
		target = "audit-log"
	}
	event := s.event(principal, "audit.list", "audit", target, "audit.read", "Audit history query", requestID(r))
	if _, err := s.deps.Audit.Append(r.Context(), event); err != nil {
		writeError(w, http.StatusServiceUnavailable, "audit_unavailable", "audit query could not be recorded")
		return
	}
	events, err := s.deps.Audit.List(r.Context(), query)
	if err != nil {
		if auditErr := s.appendOutcome(r.Context(), event, "failed"); auditErr != nil {
			writeError(w, http.StatusServiceUnavailable, "audit_unavailable", "audit query failure could not be recorded")
			return
		}
		writeError(w, http.StatusServiceUnavailable, "audit_unavailable", "audit history is unavailable")
		return
	}
	if err := s.appendOutcome(r.Context(), event, "approved"); err != nil {
		writeError(w, http.StatusServiceUnavailable, "audit_unavailable", "audit query completed but its outcome could not be recorded")
		return
	}
	if events == nil {
		events = []AuditEvent{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events, "next_cursor": nil})
}

func (s *Server) executeCommand(w http.ResponseWriter, r *http.Request) {
	principal, _, ok := s.authenticate(w, r)
	if !ok {
		return
	}
	name := r.PathValue("name")
	capability, found := s.deps.Commands.RequiredCapability(name)
	if !found {
		writeError(w, http.StatusNotFound, "command_not_found", "command is not registered")
		return
	}
	var request struct {
		Reason    string          `json:"reason"`
		TargetRef string          `json:"target_ref"`
		Arguments json.RawMessage `json:"arguments,omitempty"`
		DryRun    bool            `json:"dry_run"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	request.Reason = strings.TrimSpace(request.Reason)
	if request.Reason == "" || !utf8.ValidString(request.Reason) || strings.ContainsRune(request.Reason, '\x00') ||
		len(request.Reason) > 512 || !utf8.ValidString(request.TargetRef) || strings.ContainsRune(request.TargetRef, '\x00') ||
		len(request.TargetRef) > 128 || len(request.Arguments) > maxRequestBytes/2 || !request.DryRun {
		writeError(w, http.StatusBadRequest, "invalid_request", "a bounded command request and dry_run=true are required")
		return
	}
	if len(request.Arguments) > 0 {
		var arguments map[string]json.RawMessage
		if err := json.Unmarshal(request.Arguments, &arguments); err != nil || arguments == nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "arguments must be a JSON object")
			return
		}
	}
	event := s.event(principal, "command."+name, "command", request.TargetRef, capability, request.Reason, requestID(r))
	if !s.deps.Authorizer.Allows(principal.Role, capability) {
		event.Outcome = "denied"
		if _, err := s.deps.Audit.Append(r.Context(), event); err != nil {
			writeError(w, http.StatusServiceUnavailable, "audit_unavailable", "denied command could not be audited")
			return
		}
		writeError(w, http.StatusForbidden, "forbidden", "required capability is not granted")
		return
	}
	if _, err := s.deps.Audit.Append(r.Context(), event); err != nil {
		writeError(w, http.StatusServiceUnavailable, "audit_unavailable", "command could not be audited")
		return
	}
	result, err := s.deps.Commands.Preview(r.Context(), CommandRequest{
		Actor: principal, Name: name, Reason: request.Reason, TargetRef: request.TargetRef,
		Payload: request.Arguments, RequestID: event.RequestID, DryRun: true,
	})
	if err != nil {
		if auditErr := s.appendOutcome(r.Context(), event, "failed"); auditErr != nil {
			writeError(w, http.StatusServiceUnavailable, "audit_unavailable", "command failure could not be audited")
			return
		}
		writeError(w, http.StatusServiceUnavailable, "command_failed", "command execution failed")
		return
	}
	outcome := "approved"
	if result.Status == "failed" {
		outcome = "failed"
	} else if result.Status == "rejected" {
		outcome = "denied"
	} else if result.Status != "completed" && result.Status != "accepted" {
		writeError(w, http.StatusServiceUnavailable, "command_invalid_result", "command returned an unsupported status")
		return
	}
	if err := s.appendOutcome(r.Context(), event, outcome); err != nil {
		writeError(w, http.StatusServiceUnavailable, "audit_unavailable", "command completed but its outcome could not be recorded")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) authenticate(w http.ResponseWriter, r *http.Request) (Principal, string, bool) {
	header := r.Header.Get("Authorization")
	scheme, token, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" || strings.ContainsAny(token, " \t\r\n") {
		writeError(w, http.StatusUnauthorized, "unauthorized", "valid bearer session required")
		return Principal{}, "", false
	}
	principal, err := s.deps.Identity.Resolve(r.Context(), token)
	if errors.Is(err, ErrUnauthenticated) || err == nil && (principal.ID == "" || principal.Role == "" || principal.Status != "active") {
		writeError(w, http.StatusUnauthorized, "unauthorized", "valid bearer session required")
		return Principal{}, "", false
	}
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "identity_unavailable", "authentication service is unavailable")
		return Principal{}, "", false
	}
	return principal, token, true
}

func (s *Server) require(w http.ResponseWriter, r *http.Request, capability string) (Principal, string, bool) {
	principal, token, ok := s.authenticate(w, r)
	if !ok {
		return Principal{}, "", false
	}
	if !s.deps.Authorizer.Allows(principal.Role, capability) {
		event := s.event(principal, r.Method+" "+r.URL.Path, "api", r.URL.Path, capability, "Authorization denied", requestID(r))
		event.Outcome = "denied"
		if _, err := s.deps.Audit.Append(r.Context(), event); err != nil {
			writeError(w, http.StatusServiceUnavailable, "audit_unavailable", "denied request could not be audited")
			return Principal{}, "", false
		}
		writeError(w, http.StatusForbidden, "forbidden", "required capability is not granted")
		return Principal{}, "", false
	}
	return principal, token, true
}

func (s *Server) event(actor Principal, action, targetType, target, capability, reason, requestID string) AuditEvent {
	return AuditEvent{
		ActorID: actor.ID, ActorRole: actor.Role, Action: action, TargetType: targetType,
		TargetRef: target, Capability: capability, Reason: reason, RequestID: requestID,
		Outcome: "requested", OccurredAt: s.deps.Now().UTC(),
	}
}

func (s *Server) appendOutcome(ctx context.Context, original AuditEvent, outcome string) error {
	switch outcome {
	case "requested", "approved", "denied", "failed":
	default:
		return fmt.Errorf("%w: unsupported audit outcome", ErrInvalid)
	}
	event := original
	event.EventID = ""
	event.Outcome = outcome
	event.OccurredAt = s.deps.Now().UTC()
	_, err := s.deps.Audit.Append(ctx, event)
	return err
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	mediaType, _, mediaTypeErr := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if mediaTypeErr != nil || !strings.EqualFold(mediaType, "application/json") {
		writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "content type must be application/json")
		return false
	}
	if r.Body == nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "JSON body is required")
		return false
	}
	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBytes+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body could not be read")
		return false
	}
	if len(body) > maxRequestBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "payload_too_large", "request body exceeds 16 KiB")
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must match the JSON contract")
		return false
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must contain one JSON value")
		return false
	}
	return true
}

func requestID(r *http.Request) string {
	if id := r.Header.Get("X-Request-ID"); id != "" && len(id) <= 128 && validRequestID(id) {
		return id
	}
	return strconv.FormatInt(time.Now().UTC().UnixNano(), 10)
}

func validRequestID(value string) bool {
	if !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; frame-ancestors 'none'; base-uri 'none'")
		next.ServeHTTP(w, r)
	})
}
