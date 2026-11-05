package integration

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"arko-gm-admin/internal/access"
	"arko-gm-admin/internal/audit"
	"arko-gm-admin/internal/directory"
	"arko-gm-admin/internal/identity"
	"arko-gm-admin/internal/ratelimit"
	"arko-gm-admin/internal/server"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

func TestPostgresAdminAPIEndToEnd(t *testing.T) {
	db, config := testDatabase(t)
	identityMigrations, err := identity.Migrations()
	if err != nil {
		t.Fatal(err)
	}
	applyMigration(t, config, identityMigrations[0].SQL)
	auditMigration, err := audit.Migration(1)
	if err != nil {
		t.Fatal(err)
	}
	applyMigration(t, config, auditMigration)

	identityStore, err := identity.New(db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := identityStore.CreateCredentials(context.Background(), "demo-admin", "DEMONSTRAÇÃO-password", identity.RoleAdmin); err != nil {
		t.Fatal(err)
	}
	auditStore, err := audit.NewPostgres(db)
	if err != nil {
		t.Fatal(err)
	}
	authorizer, err := access.NewAuthorizer(map[access.Role][]access.Capability{
		access.RoleAdmin: {"directory.read", "audit.read"},
	})
	if err != nil {
		t.Fatal(err)
	}
	limiter, err := ratelimit.New(ratelimit.Config{
		DefaultLimit: ratelimit.Limit{Requests: 100, Window: time.Minute},
		MaxKeys:      8,
		IdleTTL:      time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	api, err := server.New(server.Dependencies{
		Identity:   server.IdentityAdapter{Store: identityStore, SessionTTL: time.Hour},
		Authorizer: server.AccessAdapter{Authorizer: authorizer},
		Directory:  server.DirectoryAdapter{Directory: directory.NewDemonstrationDirectory()},
		Audit:      server.AuditAdapter{Store: auditStore},
		Commands:   server.NoCommands{},
		Limiter:    limiter,
	})
	if err != nil {
		t.Fatal(err)
	}

	loginResponse := apiRequest(t, api.Handler(), http.MethodPost, "/api/v1/session", "", `{"username":"demo-admin","password":"DEMONSTRAÇÃO-password"}`)
	if loginResponse.Code != http.StatusCreated {
		t.Fatalf("login status = %d: %s", loginResponse.Code, loginResponse.Body)
	}
	var login struct {
		Token     string `json:"access_token"`
		TokenType string `json:"token_type"`
	}
	if err := json.Unmarshal(loginResponse.Body.Bytes(), &login); err != nil {
		t.Fatal(err)
	}
	if login.Token == "" || login.TokenType != "Bearer" {
		t.Fatalf("login response = %#v", login)
	}

	searchResponse := apiRequest(t, api.Handler(), http.MethodPost, "/api/v1/directory/search", login.Token,
		`{"type":"account","query":"DEMONSTRAÇÃO","reason":"DEMONSTRAÇÃO integration query"}`)
	if searchResponse.Code != http.StatusOK {
		t.Fatalf("directory status = %d: %s", searchResponse.Code, searchResponse.Body)
	}
	var search struct {
		Accounts []struct {
			Reference string `json:"account_ref"`
		} `json:"accounts"`
		Characters []json.RawMessage `json:"characters"`
	}
	if err := json.Unmarshal(searchResponse.Body.Bytes(), &search); err != nil {
		t.Fatal(err)
	}
	if len(search.Accounts) != 3 || len(search.Characters) != 0 {
		t.Fatalf("search result counts = %d/%d, want 3/0", len(search.Accounts), len(search.Characters))
	}

	auditResponse := apiRequest(t, api.Handler(), http.MethodGet, "/api/v1/audit", login.Token, "")
	if auditResponse.Code != http.StatusOK {
		t.Fatalf("audit status = %d: %s", auditResponse.Code, auditResponse.Body)
	}
	var auditPage struct {
		Events []json.RawMessage `json:"events"`
	}
	if err := json.Unmarshal(auditResponse.Body.Bytes(), &auditPage); err != nil {
		t.Fatal(err)
	}
	if len(auditPage.Events) < 4 {
		t.Fatalf("audit events = %d, want login and search intent/outcome records", len(auditPage.Events))
	}

	logoutResponse := apiRequest(t, api.Handler(), http.MethodDelete, "/api/v1/session", login.Token, "")
	if logoutResponse.Code != http.StatusOK {
		t.Fatalf("logout status = %d: %s", logoutResponse.Code, logoutResponse.Body)
	}
	revokedResponse := apiRequest(t, api.Handler(), http.MethodGet, "/api/v1/session", login.Token, "")
	if revokedResponse.Code != http.StatusUnauthorized {
		t.Fatalf("revoked session status = %d, want %d", revokedResponse.Code, http.StatusUnauthorized)
	}
}

func TestPostgresIdentityPersistenceAndSessionRecovery(t *testing.T) {
	db, config := testDatabase(t)
	migration, err := identity.Migrations()
	if err != nil {
		t.Fatal(err)
	}
	applyMigration(t, config, migration[0].SQL)

	store, err := identity.New(db)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := store.CreateCredentials(context.Background(), "demo-admin", "DEMONSTRAÇÃO-password", identity.RoleAdmin)
	if err != nil {
		t.Fatalf("CreateCredentials() error = %v", err)
	}
	if _, err := store.VerifyCredentials(context.Background(), "demo-admin", "incorrect"); !errors.Is(err, identity.ErrAuthentication) {
		t.Fatalf("VerifyCredentials(wrong password) error = %v", err)
	}
	created, err := store.CreateSession(context.Background(), admin.ID, time.Hour)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	const concurrentSessions = 12
	tokens := make(chan string, concurrentSessions)
	errs := make(chan error, concurrentSessions)
	var wait sync.WaitGroup
	for range concurrentSessions {
		wait.Add(1)
		go func() {
			defer wait.Done()
			session, err := store.CreateSession(context.Background(), admin.ID, time.Hour)
			if err != nil {
				errs <- err
				return
			}
			tokens <- session.Token
		}()
	}
	wait.Wait()
	close(tokens)
	close(errs)
	for err := range errs {
		t.Errorf("concurrent CreateSession() error = %v", err)
	}
	concurrentTokens := make([]string, 0, concurrentSessions)
	for token := range tokens {
		concurrentTokens = append(concurrentTokens, token)
	}
	if len(concurrentTokens) != concurrentSessions {
		t.Fatalf("concurrent sessions = %d, want %d", len(concurrentTokens), concurrentSessions)
	}

	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db = stdlib.OpenDB(*config)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	recoveredStore, err := identity.New(db)
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := recoveredStore.ResolveSession(context.Background(), created.Token)
	if err != nil || recovered.Admin.ID != admin.ID {
		t.Fatalf("ResolveSession after reopening pool = (%#v, %v)", recovered, err)
	}
	for _, token := range concurrentTokens {
		if _, err := recoveredStore.ResolveSession(context.Background(), token); err != nil {
			t.Fatalf("ResolveSession(concurrent session) error = %v", err)
		}
	}
	if err := recoveredStore.RevokeSession(context.Background(), created.Token); err != nil {
		t.Fatalf("RevokeSession() error = %v", err)
	}
	if _, err := recoveredStore.ResolveSession(context.Background(), created.Token); !errors.Is(err, identity.ErrAuthentication) {
		t.Fatalf("ResolveSession(revoked token) error = %v", err)
	}
}

func TestPostgresAuditConcurrentAppendAndAppendOnlyGuard(t *testing.T) {
	db, config := testDatabase(t)
	migration, err := audit.Migration(1)
	if err != nil {
		t.Fatal(err)
	}
	applyMigration(t, config, migration)
	store, err := audit.NewPostgres(db)
	if err != nil {
		t.Fatal(err)
	}

	const eventCount = 24
	var wait sync.WaitGroup
	errs := make(chan error, eventCount)
	ids := make(chan int64, eventCount)
	for i := 0; i < eventCount; i++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			event, err := store.Append(context.Background(), audit.Event{
				Actor: "demo-admin", ActorRole: "admin", Action: "DEMONSTRAÇÃO.audit",
				Capability: "audit.read", TargetType: "audit", TargetID: fmt.Sprintf("demo-target-%d", index),
				Reason: "DEMONSTRAÇÃO concurrent append", RequestID: fmt.Sprintf("demo-request-%d", index),
				Outcome: "approved", OccurredAt: time.Now().UTC(),
			})
			if err != nil {
				errs <- err
				return
			}
			ids <- event.ID
		}(i)
	}
	wait.Wait()
	close(errs)
	close(ids)
	for err := range errs {
		t.Errorf("Append() error = %v", err)
	}
	uniqueIDs := make(map[int64]struct{}, eventCount)
	for id := range ids {
		uniqueIDs[id] = struct{}{}
	}
	if len(uniqueIDs) != eventCount {
		t.Fatalf("unique audit IDs = %d, want %d", len(uniqueIDs), eventCount)
	}

	events, err := store.List(context.Background(), audit.Filter{Limit: eventCount})
	if err != nil || len(events) != eventCount {
		t.Fatalf("List() returned %d events, error = %v; want %d", len(events), err, eventCount)
	}
	if _, err := db.ExecContext(context.Background(), `UPDATE arko_admin_audit_events SET reason = 'tampered'`); err == nil {
		t.Fatal("audit table accepted UPDATE")
	}
	if _, err := db.ExecContext(context.Background(), `DELETE FROM arko_admin_audit_events`); err == nil {
		t.Fatal("audit table accepted DELETE")
	}
	if _, err := db.ExecContext(context.Background(), `TRUNCATE arko_admin_audit_events`); err == nil {
		t.Fatal("audit table accepted TRUNCATE")
	}
}

func testDatabase(t *testing.T) (*sql.DB, *pgx.ConnConfig) {
	t.Helper()
	dsn := os.Getenv("ARKO_ADMIN_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ARKO_ADMIN_TEST_DATABASE_URL is not set")
	}
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse test database URL: %v", err)
	}
	if config.Database != "arko_admin_test" {
		t.Fatalf("integration tests refuse database %q; want isolated arko_admin_test", config.Database)
	}
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		t.Fatal(err)
	}
	schema := "arko_admin_test_" + hex.EncodeToString(raw)

	adminDB := stdlib.OpenDB(*config)
	adminDB.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := adminDB.PingContext(ctx); err != nil {
		adminDB.Close()
		t.Fatalf("connect to isolated PostgreSQL test database: %v", err)
	}
	if _, err := adminDB.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		adminDB.Close()
		t.Fatalf("create isolated schema: %v", err)
	}
	config.RuntimeParams["search_path"] = schema
	db := stdlib.OpenDB(*config)
	db.SetMaxOpenConns(1)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		_, _ = adminDB.ExecContext(ctx, "DROP SCHEMA "+schema+" CASCADE")
		adminDB.Close()
		t.Fatalf("connect to isolated schema: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		_, _ = adminDB.ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		_ = adminDB.Close()
	})
	return db, config
}

func applyMigration(t *testing.T, config *pgx.ConnConfig, migration string) {
	t.Helper()
	migrationConfig := config.Copy()
	migrationConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := pgx.ConnectConfig(ctx, migrationConfig)
	if err != nil {
		t.Fatalf("connect to apply explicit test migration: %v", err)
	}
	defer conn.Close(context.Background())
	if _, err := conn.Exec(ctx, migration); err != nil {
		t.Fatalf("apply migration to disposable test schema: %v", err)
	}
}

func apiRequest(t *testing.T, handler http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.RemoteAddr = "127.0.0.1:12345"
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
