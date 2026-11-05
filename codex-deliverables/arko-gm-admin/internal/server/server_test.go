package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"arko-gm-admin/internal/ratelimit"
)

type fakeIdentity struct {
	principal  Principal
	session    Session
	resolveErr error
	revokeErr  error
	revoked    int
	logins     int
}

func (i *fakeIdentity) Login(context.Context, string, string) (Session, error) {
	i.logins++
	return i.session, nil
}

func (i *fakeIdentity) Resolve(context.Context, string) (Principal, error) {
	return i.principal, i.resolveErr
}

func (i *fakeIdentity) Revoke(context.Context, string) error {
	i.revoked++
	return i.revokeErr
}

type fakeAuthorizer map[string]bool

func (a fakeAuthorizer) Allows(_ string, capability string) bool { return a[capability] }

type fakeDirectory struct {
	called int
	result DirectoryResult
	err    error
}

func (d *fakeDirectory) Search(context.Context, DirectoryQuery) (DirectoryResult, error) {
	d.called++
	return d.result, d.err
}

type fakeAudit struct {
	events []AuditEvent
	list   []AuditEvent
	err    error
}

func (a *fakeAudit) Append(_ context.Context, event AuditEvent) (AuditEvent, error) {
	if a.err != nil {
		return AuditEvent{}, a.err
	}
	event.EventID = fmt.Sprintf("event-%d", len(a.events)+1)
	a.events = append(a.events, event)
	return event, nil
}

func (a *fakeAudit) List(context.Context, AuditQuery) ([]AuditEvent, error) {
	return a.list, a.err
}

type fakeCommands struct {
	capability string
	found      bool
	called     int
	request    CommandRequest
	result     CommandResult
	err        error
}

func (c *fakeCommands) RequiredCapability(string) (string, bool) { return c.capability, c.found }
func (c *fakeCommands) Preview(_ context.Context, request CommandRequest) (CommandResult, error) {
	c.called++
	c.request = request
	return c.result, c.err
}

func testServer(t *testing.T, allowed map[string]bool) (*Server, *fakeIdentity, *fakeDirectory, *fakeAudit, *fakeCommands) {
	t.Helper()
	identity := &fakeIdentity{
		principal: Principal{ID: "operator-1", Role: "moderator", Status: "active"},
		session: Session{
			Token:     "opaque-token",
			TokenType: "Bearer",
			Principal: Principal{ID: "operator-1", Role: "moderator", Status: "active"},
			ExpiresAt: time.Date(2026, 11, 5, 23, 0, 0, 0, time.UTC),
		},
	}
	directory := &fakeDirectory{result: DirectoryResult{}}
	audit := &fakeAudit{}
	commands := &fakeCommands{capability: "demo.command", found: true, result: CommandResult{Status: "completed"}}
	limiter, err := ratelimit.New(ratelimit.Config{
		DefaultLimit: ratelimit.Limit{Requests: 100, Window: time.Minute},
		MaxKeys:      32,
		IdleTTL:      time.Hour,
	})
	if err != nil {
		t.Fatalf("ratelimit.New() error = %v", err)
	}
	s, err := New(Dependencies{
		Identity: identity, Authorizer: fakeAuthorizer(allowed),
		Directory: directory, Audit: audit, Commands: commands, Limiter: limiter,
		Now: func() time.Time { return time.Date(2026, 11, 5, 22, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return s, identity, directory, audit, commands
}

func TestDirectorySearchDeniesByDefaultAndAudits(t *testing.T) {
	s, _, directory, audit, _ := testServer(t, nil)
	response := request(s.Handler(), http.MethodPost, "/api/v1/directory/search", "opaque-token",
		`{"type":"account","query":"demo","page":1,"page_size":10,"reason":"DEMONSTRAÇÃO audit test"}`)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
	if directory.called != 0 {
		t.Fatalf("directory calls = %d, want 0", directory.called)
	}
	if len(audit.events) != 1 || audit.events[0].Outcome != "denied" {
		t.Fatalf("audit events = %+v, want one denied event", audit.events)
	}
}

func TestDirectorySearchRequiresReasonAndCapability(t *testing.T) {
	s, _, directory, audit, _ := testServer(t, map[string]bool{"directory.read": true})
	response := request(s.Handler(), http.MethodPost, "/api/v1/directory/search", "opaque-token",
		`{"type":"account","query":"demo","page":1,"page_size":10}`)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	if directory.called != 0 || len(audit.events) != 0 {
		t.Fatalf("directory calls/events = %d/%d, want 0/0", directory.called, len(audit.events))
	}

	response = request(s.Handler(), http.MethodPost, "/api/v1/directory/search", "opaque-token",
		`{"type":"account","query":"demo","page":1,"page_size":10,"reason":"DEMONSTRAÇÃO query"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusOK, response.Body)
	}
	if directory.called != 1 || len(audit.events) != 2 {
		t.Fatalf("directory calls/events = %d/%d, want 1/2", directory.called, len(audit.events))
	}
	if audit.events[0].ActorID != "operator-1" || audit.events[0].Outcome != "requested" || audit.events[1].Outcome != "approved" {
		t.Fatalf("audit events do not record actor and completion: %+v", audit.events)
	}
}

func TestCommandRequiresCapabilityAndAuditsDenial(t *testing.T) {
	s, _, _, audit, commands := testServer(t, nil)
	response := request(s.Handler(), http.MethodPost, "/api/v1/commands/demo-change", "opaque-token",
		`{"reason":"DEMONSTRAÇÃO denied command","target_ref":"demo:target","dry_run":true}`)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
	if commands.called != 0 {
		t.Fatalf("command calls = %d, want 0", commands.called)
	}
	if len(audit.events) != 1 || audit.events[0].ActorID != "operator-1" || audit.events[0].Outcome != "denied" {
		t.Fatalf("audit events = %+v, want actor-attributed denial", audit.events)
	}
}

func TestCommandRecordsActorReasonAndOutcome(t *testing.T) {
	s, _, _, audit, commands := testServer(t, map[string]bool{"demo.command": true})
	response := request(s.Handler(), http.MethodPost, "/api/v1/commands/demo-change", "opaque-token",
		`{"reason":"DEMONSTRAÇÃO approved command","target_ref":"demo:target","dry_run":true}`)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if commands.called != 1 || commands.request.Actor.ID != "operator-1" || commands.request.Reason == "" || !commands.request.DryRun {
		t.Fatalf("command request = %+v, calls=%d", commands.request, commands.called)
	}
	if len(audit.events) != 2 || audit.events[0].Outcome != "requested" || audit.events[1].Outcome != "approved" {
		t.Fatalf("audit events = %+v, want request and completion", audit.events)
	}
}

func TestCommandRejectsNonDryRunBeforePreview(t *testing.T) {
	s, _, _, audit, commands := testServer(t, map[string]bool{"demo.command": true})
	response := request(s.Handler(), http.MethodPost, "/api/v1/commands/demo-change", "opaque-token",
		`{"reason":"DEMONSTRAÇÃO command","target_ref":"demo:target","dry_run":false}`)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	if commands.called != 0 || len(audit.events) != 0 {
		t.Fatalf("preview calls/audit events = %d/%d, want 0/0", commands.called, len(audit.events))
	}
}

func TestCommandRejectsNonObjectArgumentsBeforePreview(t *testing.T) {
	s, _, _, audit, commands := testServer(t, map[string]bool{"demo.command": true})
	response := request(s.Handler(), http.MethodPost, "/api/v1/commands/demo-change", "opaque-token",
		`{"reason":"DEMONSTRAÇÃO command","arguments":[1],"dry_run":true}`)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	if commands.called != 0 || len(audit.events) != 0 {
		t.Fatalf("preview calls/audit events = %d/%d, want 0/0", commands.called, len(audit.events))
	}
}

func TestLoginRevokesSessionIfAuditCannotRecord(t *testing.T) {
	s, identity, _, audit, _ := testServer(t, nil)
	audit.err = errors.New("audit storage unavailable")
	response := request(s.Handler(), http.MethodPost, "/api/v1/session", "",
		`{"username":"operator","password":"not-a-real-password"}`)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
	if identity.logins != 1 || identity.revoked != 1 {
		t.Fatalf("login/revoke calls = %d/%d, want 1/1", identity.logins, identity.revoked)
	}
}

func TestAuthenticationFailureIsGenericAndNoActionRuns(t *testing.T) {
	s, _, directory, _, commands := testServer(t, map[string]bool{"directory.read": true, "demo.command": true})
	s.deps.Identity.(*fakeIdentity).resolveErr = ErrUnauthenticated
	response := request(s.Handler(), http.MethodPost, "/api/v1/directory/search", "opaque-token",
		`{"type":"account","query":"demo","page":1,"page_size":10,"reason":"DEMONSTRAÇÃO query"}`)
	if response.Code != http.StatusUnauthorized || strings.Contains(response.Body.String(), "storage detail") {
		t.Fatalf("response leaked auth failure detail: %d %q", response.Code, response.Body)
	}
	if directory.called != 0 || commands.called != 0 {
		t.Fatalf("unauthenticated call reached backend: directory=%d commands=%d", directory.called, commands.called)
	}
}

func TestIdentityStorageFailureIsNotReportedAsInvalidCredentials(t *testing.T) {
	s, _, directory, _, commands := testServer(t, map[string]bool{"directory.read": true, "demo.command": true})
	s.deps.Identity.(*fakeIdentity).resolveErr = errors.New("storage detail")
	response := request(s.Handler(), http.MethodPost, "/api/v1/directory/search", "opaque-token",
		`{"type":"account","query":"demo","page":1,"page_size":10,"reason":"DEMONSTRAÇÃO query"}`)
	if response.Code != http.StatusServiceUnavailable || strings.Contains(response.Body.String(), "storage detail") {
		t.Fatalf("response must report generic availability failure: %d %q", response.Code, response.Body)
	}
	if directory.called != 0 || commands.called != 0 {
		t.Fatalf("identity storage failure reached backend: directory=%d commands=%d", directory.called, commands.called)
	}
}

func TestStrictJSONAndRequestBodyLimit(t *testing.T) {
	s, _, _, _, _ := testServer(t, nil)
	response := request(s.Handler(), http.MethodPost, "/api/v1/session", "",
		`{"username":"operator","password":"DEMONSTRAÇÃO","extra":true}`)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("unknown field status = %d, want %d", response.Code, http.StatusBadRequest)
	}

	body := `{"username":"operator","password":"` + strings.Repeat("x", maxRequestBytes) + `"}`
	response = request(s.Handler(), http.MethodPost, "/api/v1/session", "", body)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("large body status = %d, want %d", response.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestLoginResponseDoesNotIncludePassword(t *testing.T) {
	s, _, _, _, _ := testServer(t, nil)
	response := request(s.Handler(), http.MethodPost, "/api/v1/session", "",
		`{"username":"operator","password":"DEMONSTRAÇÃO"}`)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusCreated)
	}
	var decoded map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if strings.Contains(response.Body.String(), "DEMONSTRAÇÃO") || decoded["access_token"] != "opaque-token" {
		t.Fatalf("unexpected login response: %s", response.Body)
	}
}

func TestSecurityHeadersAreSet(t *testing.T) {
	s, _, _, _, _ := testServer(t, nil)
	response := httptest.NewRecorder()
	s.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if got := response.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if got := response.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q", got)
	}
}

func request(handler http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	handler.ServeHTTP(response, request)
	return response
}
