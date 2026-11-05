package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
)

var (
	ErrNotRegistered    = errors.New("command is not registered")
	ErrNotAuthorized    = errors.New("command is not authorized")
	ErrAuditUnavailable = errors.New("command audit is unavailable")
	ErrAuditFailed      = errors.New("command audit failed")
)

type Capability string

type Actor struct {
	ID   string
	Role string
}

type Request[A any] struct {
	Actor  *Actor
	Reason string
	Args   A
}

type Outcome string

const (
	OutcomeRequested Outcome = "requested"
	OutcomeDenied    Outcome = "denied"
	OutcomeSucceeded Outcome = "approved"
	OutcomeFailed    Outcome = "failed"
)

type AuditEvent struct {
	Command    string
	Capability Capability
	ActorID    string
	ActorRole  string
	Reason     string
	Outcome    Outcome
	Error      string
}

type Authorizer interface {
	Authorize(ctx context.Context, actor Actor, capability Capability, reason string) (bool, error)
}

type Auditor interface {
	Record(ctx context.Context, event AuditEvent) error
}

type Executor[A, R any] func(ctx context.Context, actor Actor, reason string, args A) (R, error)

type Registry struct {
	mu       sync.RWMutex
	commands map[string]*registration
}

func NewRegistry() *Registry {
	return &Registry{commands: make(map[string]*registration)}
}

type registration struct {
	identity byte
}

type Command[A, R any] struct {
	name         string
	capability   Capability
	maxArgs      int
	executor     Executor[A, R]
	registration *registration
}

func Register[A, R any](registry *Registry, name string, capability Capability, maxArgumentBytes int, executor Executor[A, R]) (Command[A, R], error) {
	capability = Capability(strings.TrimSpace(string(capability)))
	command := Command[A, R]{
		name:       strings.TrimSpace(name),
		capability: capability,
		maxArgs:    maxArgumentBytes,
		executor:   executor,
	}
	if registry == nil {
		return Command[A, R]{}, errors.New("command registry is nil")
	}
	if command.name == "" {
		return Command[A, R]{}, errors.New("command name is required")
	}
	if strings.TrimSpace(string(capability)) == "" {
		return Command[A, R]{}, errors.New("command capability is required")
	}
	if maxArgumentBytes <= 0 {
		return Command[A, R]{}, errors.New("maximum argument size must be positive")
	}
	if executor == nil {
		return Command[A, R]{}, errors.New("command executor is required")
	}

	registry.mu.Lock()
	defer registry.mu.Unlock()
	if registry.commands == nil {
		registry.commands = make(map[string]*registration)
	}
	if _, exists := registry.commands[command.name]; exists {
		return Command[A, R]{}, fmt.Errorf("command %q is already registered", command.name)
	}
	command.registration = &registration{}
	registry.commands[command.name] = command.registration
	return command, nil
}

type Dispatcher struct {
	registry   *Registry
	authorizer Authorizer
	auditor    Auditor
}

func NewDispatcher(registry *Registry, authorizer Authorizer, auditor Auditor) *Dispatcher {
	return &Dispatcher{registry: registry, authorizer: authorizer, auditor: auditor}
}

func Dispatch[A, R any](ctx context.Context, dispatcher *Dispatcher, command Command[A, R], request Request[A]) (R, error) {
	var zero R
	event := AuditEvent{
		Command:    command.name,
		Capability: command.capability,
		Reason:     request.Reason,
		Outcome:    OutcomeDenied,
	}
	if request.Actor != nil {
		event.ActorID = request.Actor.ID
		event.ActorRole = request.Actor.Role
	}
	if dispatcher == nil || dispatcher.auditor == nil {
		return zero, ErrAuditUnavailable
	}

	fail := func(outcome Outcome, err error) (R, error) {
		event.Outcome = outcome
		if err != nil {
			event.Error = err.Error()
		}
		if auditErr := dispatcher.auditor.Record(ctx, event); auditErr != nil {
			return zero, fmt.Errorf("%w: %v (original error: %v)", ErrAuditFailed, auditErr, err)
		}
		return zero, err
	}

	if dispatcher.registry == nil || !isRegistered(dispatcher.registry, command) {
		return fail(OutcomeDenied, ErrNotRegistered)
	}
	if request.Actor == nil || strings.TrimSpace(request.Actor.ID) == "" || !validActorRole(request.Actor.Role) {
		return fail(OutcomeDenied, fmt.Errorf("%w: actor with a known role is required", ErrNotAuthorized))
	}
	if strings.TrimSpace(request.Reason) == "" {
		return fail(OutcomeDenied, fmt.Errorf("%w: reason is required", ErrNotAuthorized))
	}
	if dispatcher.authorizer == nil {
		return fail(OutcomeDenied, fmt.Errorf("%w: policy authorizer is unavailable", ErrNotAuthorized))
	}

	args, err := json.Marshal(request.Args)
	if err != nil {
		return fail(OutcomeDenied, fmt.Errorf("encode command arguments: %w", err))
	}
	if len(args) > command.maxArgs {
		return fail(OutcomeDenied, fmt.Errorf("command arguments exceed maximum size of %d bytes", command.maxArgs))
	}

	allowed, authErr := dispatcher.authorizer.Authorize(ctx, *request.Actor, command.capability, request.Reason)
	if authErr != nil {
		return fail(OutcomeDenied, fmt.Errorf("%w: policy authorization failed: %v", ErrNotAuthorized, authErr))
	}
	if !allowed {
		return fail(OutcomeDenied, ErrNotAuthorized)
	}

	event.Outcome = OutcomeRequested
	if auditErr := dispatcher.auditor.Record(ctx, event); auditErr != nil {
		return zero, fmt.Errorf("%w: %v", ErrAuditFailed, auditErr)
	}
	result, execErr := command.executor(ctx, *request.Actor, request.Reason, request.Args)
	if execErr != nil {
		return fail(OutcomeFailed, execErr)
	}
	event.Outcome = OutcomeSucceeded
	if auditErr := dispatcher.auditor.Record(ctx, event); auditErr != nil {
		return zero, fmt.Errorf("%w: %v", ErrAuditFailed, auditErr)
	}
	return result, nil
}

func isRegistered[A, R any](registry *Registry, command Command[A, R]) bool {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	return command.registration != nil && registry.commands[command.name] == command.registration
}

func validActorRole(role string) bool {
	switch role {
	case "player", "moderator", "admin":
		return true
	default:
		return false
	}
}
