package audit

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	DefaultPageSize = 50
	MaxPageSize     = 200
	MaxOffset       = 1_000_000
)

var (
	ErrNilDB                = errors.New("audit: database is nil")
	ErrInvalidEvent         = errors.New("audit: invalid event")
	ErrInvalidFilter        = errors.New("audit: invalid filter")
	ErrUnsupportedMigration = errors.New("audit: unsupported migration version")
)

// Event is an immutable record of an administrative operation.
// Action and Outcome are intentionally unrestricted strings.
type Event struct {
	ID         int64
	Actor      string
	ActorRole  string
	Action     string
	Capability string
	TargetType string
	TargetID   string
	Reason     string
	RequestID  string
	Outcome    string
	OccurredAt time.Time
}

// Filter selects events and controls bounded offset pagination.
type Filter struct {
	Actor      string
	Action     string
	TargetType string
	TargetID   string
	RequestID  string
	Outcome    string
	Since      time.Time
	Until      time.Time
	Limit      int
	Offset     int
}

// Store exposes append and read operations only.
type Store interface {
	Append(context.Context, Event) (Event, error)
	List(context.Context, Filter) ([]Event, error)
}

// Postgres stores audit events using an injected database connection pool.
type Postgres struct {
	db *sql.DB
}

// NewPostgres creates a store. It does not run or inspect migrations.
func NewPostgres(db *sql.DB) (*Postgres, error) {
	if db == nil {
		return nil, ErrNilDB
	}
	return &Postgres{db: db}, nil
}

const insertEventSQL = `
INSERT INTO arko_admin_audit_events
    (actor, actor_role, action, capability, target_type, target_id, reason, request_id, outcome, occurred_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING id, occurred_at`

// Append validates and inserts one event. The returned event contains the
// database-assigned ID and normalized timestamp.
func (p *Postgres) Append(ctx context.Context, event Event) (Event, error) {
	if err := validateEvent(event); err != nil {
		return Event{}, err
	}
	event.OccurredAt = event.OccurredAt.UTC()
	err := p.db.QueryRowContext(ctx, insertEventSQL,
		event.Actor,
		event.ActorRole,
		event.Action,
		event.Capability,
		event.TargetType,
		event.TargetID,
		event.Reason,
		event.RequestID,
		event.Outcome,
		event.OccurredAt,
	).Scan(&event.ID, &event.OccurredAt)
	if err != nil {
		return Event{}, fmt.Errorf("audit: append event: %w", err)
	}
	return event, nil
}

const listEventsSQL = `
SELECT id, actor, actor_role, action, capability, target_type, target_id, reason, request_id, outcome, occurred_at
FROM arko_admin_audit_events`

// List returns events ordered newest-first using bounded, stable pagination.
func (p *Postgres) List(ctx context.Context, filter Filter) ([]Event, error) {
	limit, err := validateFilter(filter)
	if err != nil {
		return nil, err
	}

	where := make([]string, 0, 8)
	args := make([]any, 0, 10)
	addStringFilter := func(column, value string) {
		if value == "" {
			return
		}
		args = append(args, value)
		where = append(where, fmt.Sprintf("%s = $%d", column, len(args)))
	}
	addStringFilter("actor", filter.Actor)
	addStringFilter("action", filter.Action)
	addStringFilter("target_type", filter.TargetType)
	addStringFilter("target_id", filter.TargetID)
	addStringFilter("request_id", filter.RequestID)
	addStringFilter("outcome", filter.Outcome)
	if !filter.Since.IsZero() {
		args = append(args, filter.Since.UTC())
		where = append(where, fmt.Sprintf("occurred_at >= $%d", len(args)))
	}
	if !filter.Until.IsZero() {
		args = append(args, filter.Until.UTC())
		where = append(where, fmt.Sprintf("occurred_at <= $%d", len(args)))
	}

	query := listEventsSQL
	if len(where) > 0 {
		query += "\nWHERE " + strings.Join(where, " AND ")
	}
	query += "\nORDER BY occurred_at DESC, id DESC"
	args = append(args, limit)
	query += fmt.Sprintf("\nLIMIT $%d", len(args))
	args = append(args, filter.Offset)
	query += fmt.Sprintf(" OFFSET $%d", len(args))

	rows, err := p.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("audit: list events: %w", err)
	}
	defer rows.Close()

	events := make([]Event, 0)
	for rows.Next() {
		var event Event
		if err := rows.Scan(
			&event.ID,
			&event.Actor,
			&event.ActorRole,
			&event.Action,
			&event.Capability,
			&event.TargetType,
			&event.TargetID,
			&event.Reason,
			&event.RequestID,
			&event.Outcome,
			&event.OccurredAt,
		); err != nil {
			return nil, fmt.Errorf("audit: scan event: %w", err)
		}
		event.OccurredAt = event.OccurredAt.UTC()
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("audit: iterate events: %w", err)
	}
	return events, nil
}

func validateEvent(event Event) error {
	if event.ID != 0 {
		return fmt.Errorf("%w: ID is assigned by the database", ErrInvalidEvent)
	}
	fields := []struct {
		name, value string
		max         int
		required    bool
	}{
		{"actor", event.Actor, 256, true},
		{"actor_role", event.ActorRole, 64, true},
		{"action", event.Action, 256, true},
		{"capability", event.Capability, 256, true},
		{"target_type", event.TargetType, 256, true},
		{"target_id", event.TargetID, 512, true},
		{"reason", event.Reason, 4096, false},
		{"request_id", event.RequestID, 256, false},
		{"outcome", event.Outcome, 256, true},
	}
	for _, field := range fields {
		if err := validateText(field.name, field.value, field.max, field.required, ErrInvalidEvent); err != nil {
			return err
		}
	}
	if event.OccurredAt.IsZero() {
		return fmt.Errorf("%w: occurred_at is required", ErrInvalidEvent)
	}
	return nil
}

func validateFilter(filter Filter) (int, error) {
	fields := []struct {
		name, value string
		max         int
	}{
		{"actor", filter.Actor, 256},
		{"action", filter.Action, 256},
		{"target_type", filter.TargetType, 256},
		{"target_id", filter.TargetID, 512},
		{"request_id", filter.RequestID, 256},
		{"outcome", filter.Outcome, 256},
	}
	for _, field := range fields {
		if field.value == "" {
			continue
		}
		if err := validateText(field.name, field.value, field.max, false, ErrInvalidFilter); err != nil {
			return 0, err
		}
	}
	if !filter.Since.IsZero() && !filter.Until.IsZero() && filter.Until.Before(filter.Since) {
		return 0, fmt.Errorf("%w: until precedes since", ErrInvalidFilter)
	}
	if filter.Limit < 0 || filter.Limit > MaxPageSize {
		return 0, fmt.Errorf("%w: limit must be between 0 and %d", ErrInvalidFilter, MaxPageSize)
	}
	if filter.Offset < 0 || filter.Offset > MaxOffset {
		return 0, fmt.Errorf("%w: offset must be between 0 and %d", ErrInvalidFilter, MaxOffset)
	}
	limit := filter.Limit
	if limit == 0 {
		limit = DefaultPageSize
	}
	return limit, nil
}

func validateText(name, value string, maxRunes int, required bool, sentinel error) error {
	if !utf8.ValidString(value) || strings.ContainsRune(value, '\x00') {
		return fmt.Errorf("%w: %s must be valid text", sentinel, name)
	}
	if required && strings.TrimSpace(value) == "" {
		return fmt.Errorf("%w: %s is required", sentinel, name)
	}
	if utf8.RuneCountInString(value) > maxRunes {
		return fmt.Errorf("%w: %s exceeds %d characters", sentinel, name, maxRunes)
	}
	return nil
}

//go:embed migrations/001_create_audit_events.sql
var migrationFiles embed.FS

// Migration returns the SQL for an explicitly selected migration version.
// It only returns migration text; it never executes SQL.
func Migration(version int) (string, error) {
	if version != 1 {
		return "", fmt.Errorf("%w: %d", ErrUnsupportedMigration, version)
	}
	sql, err := migrationFiles.ReadFile("migrations/001_create_audit_events.sql")
	if err != nil {
		return "", fmt.Errorf("audit: read migration %d: %w", version, err)
	}
	return string(sql), nil
}
