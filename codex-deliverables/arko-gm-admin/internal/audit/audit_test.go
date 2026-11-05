package audit

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestValidateEvent(t *testing.T) {
	valid := Event{
		Actor:      "operator-1",
		ActorRole:  "moderator",
		Action:     "custom.action",
		Capability: "directory.read",
		TargetType: "account",
		TargetID:   "account-2",
		Reason:     "support request",
		RequestID:  "request-3",
		Outcome:    "approved",
		OccurredAt: time.Date(2026, 11, 5, 12, 0, 0, 0, time.FixedZone("offset", 2*60*60)),
	}

	tests := []struct {
		name  string
		edit  func(*Event)
		valid bool
	}{
		{name: "valid", valid: true},
		{name: "empty actor", edit: func(e *Event) { e.Actor = "" }},
		{name: "blank action", edit: func(e *Event) { e.Action = "  " }},
		{name: "missing target", edit: func(e *Event) { e.TargetID = "" }},
		{name: "missing outcome", edit: func(e *Event) { e.Outcome = "" }},
		{name: "missing timestamp", edit: func(e *Event) { e.OccurredAt = time.Time{} }},
		{name: "caller assigned id", edit: func(e *Event) { e.ID = 1 }},
		{name: "oversized reason", edit: func(e *Event) { e.Reason = strings.Repeat("x", 4097) }},
		{name: "invalid text", edit: func(e *Event) { e.Actor = "bad\x00actor" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := valid
			if tt.edit != nil {
				tt.edit(&event)
			}
			err := validateEvent(event)
			if tt.valid && err != nil {
				t.Fatalf("validateEvent() error = %v", err)
			}
			if !tt.valid && !errors.Is(err, ErrInvalidEvent) {
				t.Fatalf("validateEvent() error = %v, want ErrInvalidEvent", err)
			}
		})
	}
}

func TestValidateFilterBounds(t *testing.T) {
	tests := []struct {
		name      string
		filter    Filter
		wantLimit int
		wantErr   bool
	}{
		{name: "default limit", wantLimit: DefaultPageSize},
		{name: "maximum page", filter: Filter{Limit: MaxPageSize, Offset: MaxOffset}, wantLimit: MaxPageSize},
		{name: "negative limit", filter: Filter{Limit: -1}, wantErr: true},
		{name: "oversized limit", filter: Filter{Limit: MaxPageSize + 1}, wantErr: true},
		{name: "negative offset", filter: Filter{Offset: -1}, wantErr: true},
		{name: "oversized offset", filter: Filter{Offset: MaxOffset + 1}, wantErr: true},
		{name: "reversed time range", filter: Filter{
			Since: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
			Until: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		}, wantErr: true},
		{name: "invalid filter value", filter: Filter{Action: strings.Repeat("x", 257)}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLimit, err := validateFilter(tt.filter)
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidFilter) {
					t.Fatalf("validateFilter() error = %v, want ErrInvalidFilter", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("validateFilter() error = %v", err)
			}
			if gotLimit != tt.wantLimit {
				t.Fatalf("validateFilter() limit = %d, want %d", gotLimit, tt.wantLimit)
			}
		})
	}
}

func TestNewPostgresRejectsNilDB(t *testing.T) {
	if _, err := NewPostgres(nil); !errors.Is(err, ErrNilDB) {
		t.Fatalf("NewPostgres(nil) error = %v, want ErrNilDB", err)
	}
}

func TestAppendSQLMappingAndContext(t *testing.T) {
	occurredAt := time.Date(2026, 11, 5, 12, 0, 0, 0, time.FixedZone("offset", 2*60*60))
	db, mock := openMockDB(t, mockResult{
		columns: []string{"id", "occurred_at"},
		values:  [][]driver.Value{{int64(42), occurredAt.UTC()}},
	})
	store, err := NewPostgres(db)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.WithValue(context.Background(), mockContextKey("trace"), "trace-7")
	event := Event{
		Actor:      "operator-1",
		ActorRole:  "moderator",
		Action:     "custom.action",
		Capability: "directory.read",
		TargetType: "account",
		TargetID:   "account-2",
		Reason:     "support request",
		RequestID:  "request-3",
		Outcome:    "approved",
		OccurredAt: occurredAt,
	}
	got, err := store.Append(ctx, event)
	if err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	if got.ID != 42 || !got.OccurredAt.Equal(occurredAt.UTC()) {
		t.Fatalf("Append() returned ID/timestamp = %d/%s", got.ID, got.OccurredAt)
	}
	if len(mock.calls) != 1 {
		t.Fatalf("query calls = %d, want 1", len(mock.calls))
	}
	call := mock.calls[0]
	if !strings.Contains(call.query, "INSERT INTO arko_admin_audit_events") || !strings.Contains(call.query, "VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)") {
		t.Fatalf("unexpected insert SQL: %s", call.query)
	}
	wantArgs := []any{"operator-1", "moderator", "custom.action", "directory.read", "account", "account-2", "support request", "request-3", "approved", occurredAt.UTC()}
	if len(call.args) != len(wantArgs) {
		t.Fatalf("insert argument count = %d, want %d", len(call.args), len(wantArgs))
	}
	for i := range wantArgs {
		if gotArg := call.args[i].Value; gotArg != wantArgs[i] {
			t.Errorf("insert argument %d = %#v, want %#v", i+1, gotArg, wantArgs[i])
		}
	}
	if call.contextValue != "trace-7" {
		t.Fatalf("driver context value = %#v, want trace-7", call.contextValue)
	}
}

func TestListSQLMappingAndParameterizedFilters(t *testing.T) {
	occurredAt := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)
	db, mock := openMockDB(t, mockResult{
		columns: []string{"id", "actor", "actor_role", "action", "capability", "target_type", "target_id", "reason", "request_id", "outcome", "occurred_at"},
		values: [][]driver.Value{{
			int64(9), "operator-1", "moderator", "custom.action", "directory.read", "account", "account-2",
			"reason", "req-4", "approved", occurredAt,
		}},
	})
	store, err := NewPostgres(db)
	if err != nil {
		t.Fatal(err)
	}
	filter := Filter{
		Actor:      "actor' OR TRUE --",
		Action:     "custom.action",
		TargetType: "account",
		TargetID:   "account-2",
		RequestID:  "request-3",
		Outcome:    "approved",
		Since:      occurredAt.Add(-time.Hour),
		Until:      occurredAt.Add(time.Hour),
		Limit:      25,
		Offset:     50,
	}
	got, err := store.List(context.Background(), filter)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 1 || got[0].ID != 9 || got[0].Action != "custom.action" || !got[0].OccurredAt.Equal(occurredAt) {
		t.Fatalf("List() returned unexpected event: %#v", got)
	}
	if len(mock.calls) != 1 {
		t.Fatalf("query calls = %d, want 1", len(mock.calls))
	}
	call := mock.calls[0]
	for _, fragment := range []string{
		"actor = $1", "action = $2", "target_type = $3", "target_id = $4",
		"request_id = $5", "outcome = $6", "occurred_at >= $7", "occurred_at <= $8",
		"ORDER BY occurred_at DESC, id DESC", "LIMIT $9 OFFSET $10",
	} {
		if !strings.Contains(call.query, fragment) {
			t.Errorf("list SQL missing %q: %s", fragment, call.query)
		}
	}
	if strings.Contains(call.query, "actor' OR TRUE") {
		t.Fatalf("filter value interpolated into SQL: %s", call.query)
	}
	if len(call.args) != 10 {
		t.Fatalf("list argument count = %d, want 10", len(call.args))
	}
	if call.args[0].Value != filter.Actor || call.args[8].Value != int64(25) || call.args[9].Value != int64(50) {
		t.Fatalf("unexpected filter/paging arguments: %#v", call.args)
	}
}

func TestMigrationIsExplicitAndRejectsMutation(t *testing.T) {
	migration, err := Migration(1)
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		"CREATE TABLE arko_admin_audit_events",
		"BEFORE UPDATE OR DELETE ON arko_admin_audit_events",
		"BEFORE TRUNCATE ON arko_admin_audit_events",
	} {
		if !strings.Contains(migration, statement) {
			t.Errorf("migration missing %q", statement)
		}
	}
	if _, err := Migration(2); !errors.Is(err, ErrUnsupportedMigration) {
		t.Fatalf("Migration(2) error = %v, want ErrUnsupportedMigration", err)
	}
}

type mockResult struct {
	columns []string
	values  [][]driver.Value
	err     error
}

type mockContextKey string

type mockCall struct {
	query        string
	args         []driver.NamedValue
	contextValue any
}

type mockState struct {
	mu     sync.Mutex
	result mockResult
	calls  []mockCall
}

var (
	mockDriverOnce sync.Once
	mockDriverMu   sync.Mutex
	activeMock     *mockState
)

func openMockDB(t *testing.T, result mockResult) (*sql.DB, *mockState) {
	t.Helper()
	mockDriverOnce.Do(func() { sql.Register("audit-test-driver", mockDriver{}) })
	mock := &mockState{result: result}
	mockDriverMu.Lock()
	activeMock = mock
	mockDriverMu.Unlock()
	db, err := sql.Open("audit-test-driver", "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		mockDriverMu.Lock()
		if activeMock == mock {
			activeMock = nil
		}
		mockDriverMu.Unlock()
	})
	return db, mock
}

type mockDriver struct{}

func (mockDriver) Open(string) (driver.Conn, error) {
	mockDriverMu.Lock()
	defer mockDriverMu.Unlock()
	if activeMock == nil {
		return nil, errors.New("no active audit SQL mock")
	}
	return &mockConn{state: activeMock}, nil
}

type mockConn struct {
	state *mockState
}

func (*mockConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("Prepare is not supported")
}

func (*mockConn) Close() error { return nil }

func (*mockConn) Begin() (driver.Tx, error) {
	return nil, errors.New("transactions are not supported")
}

func (c *mockConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var contextValue any
	contextValue = ctx.Value(mockContextKey("trace"))

	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	c.state.calls = append(c.state.calls, mockCall{
		query: query, args: append([]driver.NamedValue(nil), args...), contextValue: contextValue,
	})
	if c.state.result.err != nil {
		return nil, c.state.result.err
	}
	return &mockRows{columns: c.state.result.columns, values: c.state.result.values}, nil
}

type mockRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *mockRows) Columns() []string { return r.columns }

func (*mockRows) Close() error { return nil }

func (r *mockRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	if len(dest) != len(r.values[r.index]) {
		return fmt.Errorf("destination width %d differs from row width %d", len(dest), len(r.values[r.index]))
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}

var _ driver.Driver = mockDriver{}
var _ driver.Conn = (*mockConn)(nil)
var _ driver.QueryerContext = (*mockConn)(nil)
var _ driver.Rows = (*mockRows)(nil)
