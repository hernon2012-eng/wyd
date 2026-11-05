package server

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"arko-gm-admin/internal/access"
	"arko-gm-admin/internal/audit"
	"arko-gm-admin/internal/directory"
	"arko-gm-admin/internal/identity"
)

type IdentityAdapter struct {
	Store      *identity.Store
	SessionTTL time.Duration
}

func (a IdentityAdapter) Login(ctx context.Context, username, password string) (Session, error) {
	if a.Store == nil || a.SessionTTL <= 0 {
		return Session{}, ErrUnavailable
	}
	admin, err := a.Store.VerifyCredentials(ctx, username, password)
	if errors.Is(err, identity.ErrAuthentication) {
		return Session{}, ErrUnauthenticated
	}
	if err != nil {
		return Session{}, fmt.Errorf("verify credentials: %w", err)
	}
	created, err := a.Store.CreateSession(ctx, admin.ID, a.SessionTTL)
	if err != nil {
		return Session{}, fmt.Errorf("create session: %w", err)
	}
	principal := Principal{ID: strconv.FormatInt(admin.ID, 10), DisplayName: admin.Username, Role: string(admin.Role), Status: "active"}
	return Session{Token: created.Token, TokenType: "Bearer", ExpiresAt: created.ExpiresAt, Principal: principal}, nil
}

func (a IdentityAdapter) Resolve(ctx context.Context, token string) (Principal, error) {
	if a.Store == nil {
		return Principal{}, ErrUnavailable
	}
	session, err := a.Store.ResolveSession(ctx, token)
	if errors.Is(err, identity.ErrAuthentication) {
		return Principal{}, ErrUnauthenticated
	}
	if err != nil {
		return Principal{}, fmt.Errorf("resolve session: %w", err)
	}
	return Principal{
		ID: strconv.FormatInt(session.Admin.ID, 10), DisplayName: session.Admin.Username,
		Role: string(session.Admin.Role), Status: "active",
	}, nil
}

func (a IdentityAdapter) Revoke(ctx context.Context, token string) error {
	if a.Store == nil {
		return ErrUnavailable
	}
	if err := a.Store.RevokeSession(ctx, token); errors.Is(err, identity.ErrAuthentication) {
		return ErrUnauthenticated
	} else if err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}

type AccessAdapter struct {
	Authorizer *access.Authorizer
}

func (a AccessAdapter) Allows(role, capability string) bool {
	return a.Authorizer != nil && a.Authorizer.Allows(
		access.Principal{ID: "authenticated", Role: access.Role(role)},
		access.Capability(capability),
	)
}

type DirectoryAdapter struct {
	Directory directory.Directory
}

func (a DirectoryAdapter) Search(ctx context.Context, query DirectoryQuery) (DirectoryResult, error) {
	if a.Directory == nil {
		return DirectoryResult{}, ErrUnavailable
	}
	if err := ctx.Err(); err != nil {
		return DirectoryResult{}, err
	}
	request := directory.SearchRequest{Query: query.Query, Page: query.Page, PageSize: query.PageSize}
	result := DirectoryResult{Accounts: []AccountSummary{}, Characters: []CharacterSummary{}}
	switch query.Type {
	case "account":
		page, err := a.Directory.SearchAccounts(request)
		if err != nil {
			return DirectoryResult{}, fmt.Errorf("search accounts: %w", err)
		}
		for _, item := range page.Items {
			result.Accounts = append(result.Accounts, AccountSummary{Reference: item.Reference, Name: item.Name})
		}
		result.NextCursor = nextPage(page.Page, page.PageSize, page.Total)
	case "character":
		page, err := a.Directory.SearchCharacters(request)
		if err != nil {
			return DirectoryResult{}, fmt.Errorf("search characters: %w", err)
		}
		for _, item := range page.Items {
			result.Characters = append(result.Characters, CharacterSummary{
				Reference: item.Reference, AccountRef: item.AccountReference, Name: item.Name,
			})
		}
		result.NextCursor = nextPage(page.Page, page.PageSize, page.Total)
	default:
		return DirectoryResult{}, fmt.Errorf("%w: unsupported directory type", ErrInvalid)
	}
	if err := ctx.Err(); err != nil {
		return DirectoryResult{}, err
	}
	return result, nil
}

func nextPage(page, pageSize, total int) *string {
	if page*pageSize >= total {
		return nil
	}
	next := strconv.Itoa(page + 1)
	return &next
}

type AuditAdapter struct {
	Store audit.Store
}

func (a AuditAdapter) Append(ctx context.Context, event AuditEvent) (AuditEvent, error) {
	if a.Store == nil {
		return AuditEvent{}, ErrUnavailable
	}
	stored, err := a.Store.Append(ctx, audit.Event{
		Actor: event.ActorID, ActorRole: event.ActorRole, Action: event.Action,
		Capability: event.Capability, TargetType: event.TargetType, TargetID: event.TargetRef,
		Reason: event.Reason, RequestID: event.RequestID, Outcome: event.Outcome, OccurredAt: event.OccurredAt,
	})
	if err != nil {
		return AuditEvent{}, err
	}
	event.EventID = strconv.FormatInt(stored.ID, 10)
	event.OccurredAt = stored.OccurredAt
	return event, nil
}

func (a AuditAdapter) List(ctx context.Context, query AuditQuery) ([]AuditEvent, error) {
	if a.Store == nil {
		return nil, ErrUnavailable
	}
	filter := audit.Filter{Actor: query.ActorID, Limit: query.Limit}
	if !query.Before.IsZero() {
		filter.Until = query.Before.UTC()
	}
	events, err := a.Store.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	result := make([]AuditEvent, 0, len(events))
	for _, event := range events {
		result = append(result, AuditEvent{
			EventID: strconv.FormatInt(event.ID, 10), ActorID: event.Actor, ActorRole: event.ActorRole,
			Action: event.Action, TargetType: event.TargetType, TargetRef: event.TargetID,
			Capability: event.Capability, Reason: event.Reason, RequestID: event.RequestID,
			Outcome: event.Outcome, OccurredAt: event.OccurredAt,
		})
	}
	return result, nil
}

type NoCommands struct{}

func (NoCommands) RequiredCapability(string) (string, bool) { return "", false }

func (NoCommands) Preview(context.Context, CommandRequest) (CommandResult, error) {
	return CommandResult{}, ErrNotFound
}
