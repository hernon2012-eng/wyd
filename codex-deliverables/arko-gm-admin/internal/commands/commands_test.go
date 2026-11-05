package commands

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type testArgs struct {
	Value string `json:"value"`
}

type testAuthorizer struct {
	allowed    bool
	err        error
	calls      int
	actor      Actor
	capability Capability
	reason     string
}

func (a *testAuthorizer) Authorize(_ context.Context, actor Actor, capability Capability, reason string) (bool, error) {
	a.calls++
	a.actor = actor
	a.capability = capability
	a.reason = reason
	return a.allowed, a.err
}

type testAuditor struct {
	events []AuditEvent
	err    error
}

func (a *testAuditor) Record(_ context.Context, event AuditEvent) error {
	a.events = append(a.events, event)
	return a.err
}

func registerTestCommand(t *testing.T, registry *Registry, executor Executor[testArgs, string], maxArgs int) Command[testArgs, string] {
	t.Helper()
	command, err := Register(registry, "grant-item", "inventory.grant", maxArgs, executor)
	if err != nil {
		t.Fatal(err)
	}
	return command
}

func TestDispatchDeniesWithoutPolicyOrRequiredAttribution(t *testing.T) {
	executions := 0
	tests := []struct {
		name       string
		authorizer Authorizer
		request    Request[testArgs]
		wantAuth   int
	}{
		{
			name:     "missing authorizer",
			request:  Request[testArgs]{Actor: &Actor{ID: "gm-1", Role: "moderator"}, Reason: "ticket 123"},
			wantAuth: 0,
		},
		{
			name:       "policy denies",
			authorizer: &testAuthorizer{allowed: false},
			request:    Request[testArgs]{Actor: &Actor{ID: "gm-1", Role: "moderator"}, Reason: "ticket 123"},
			wantAuth:   1,
		},
		{
			name:       "policy errors",
			authorizer: &testAuthorizer{err: errors.New("policy backend unavailable")},
			request:    Request[testArgs]{Actor: &Actor{ID: "gm-1", Role: "moderator"}, Reason: "ticket 123"},
			wantAuth:   1,
		},
		{
			name:       "missing actor",
			authorizer: &testAuthorizer{allowed: true},
			request:    Request[testArgs]{Reason: "ticket 123"},
			wantAuth:   0,
		},
		{
			name:       "empty actor",
			authorizer: &testAuthorizer{allowed: true},
			request:    Request[testArgs]{Actor: &Actor{}, Reason: "ticket 123"},
			wantAuth:   0,
		},
		{
			name:       "missing reason",
			authorizer: &testAuthorizer{allowed: true},
			request:    Request[testArgs]{Actor: &Actor{ID: "gm-1", Role: "moderator"}, Reason: " \t "},
			wantAuth:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewRegistry()
			command := registerTestCommand(t, registry, func(context.Context, Actor, string, testArgs) (string, error) {
				executions++
				return "stored", nil
			}, 128)
			auditor := &testAuditor{}
			dispatcher := NewDispatcher(registry, tt.authorizer, auditor)

			result, err := Dispatch(context.Background(), dispatcher, command, tt.request)
			if !errors.Is(err, ErrNotAuthorized) {
				t.Fatalf("Dispatch error = %v, want ErrNotAuthorized", err)
			}
			if result != "" {
				t.Fatalf("Dispatch result = %q, want empty result", result)
			}
			if executions != 0 {
				t.Fatalf("executor called %d times, want 0", executions)
			}
			if len(auditor.events) != 1 || auditor.events[0].Outcome != OutcomeDenied {
				t.Fatalf("audit events = %#v, want one denied event", auditor.events)
			}
			if auth, ok := tt.authorizer.(*testAuthorizer); ok && auth.calls != tt.wantAuth {
				t.Fatalf("authorizer called %d times, want %d", auth.calls, tt.wantAuth)
			}
		})
	}
}

func TestUnconfiguredCommandCannotExecute(t *testing.T) {
	registry := NewRegistry()
	auditor := &testAuditor{}
	dispatcher := NewDispatcher(registry, &testAuthorizer{allowed: true}, auditor)

	result, err := Dispatch(context.Background(), dispatcher, Command[testArgs, string]{}, Request[testArgs]{
		Actor:  &Actor{ID: "gm-1", Role: "moderator"},
		Reason: "ticket 123",
	})
	if !errors.Is(err, ErrNotRegistered) {
		t.Fatalf("Dispatch error = %v, want ErrNotRegistered", err)
	}
	if result != "" || len(auditor.events) != 1 || auditor.events[0].Outcome != OutcomeDenied {
		t.Fatalf("result or audit event indicates unexpected execution: result=%q events=%#v", result, auditor.events)
	}
	if _, err := Register[testArgs, string](registry, "unconfigured", "inventory.grant", 128, nil); err == nil {
		t.Fatal("Register accepted an unconfigured executor")
	}
}

func TestDispatchAttributesActorReasonAndCapability(t *testing.T) {
	registry := NewRegistry()
	authorizer := &testAuthorizer{allowed: true}
	auditor := &testAuditor{}
	command := registerTestCommand(t, registry, func(_ context.Context, actor Actor, reason string, args testArgs) (string, error) {
		if actor.ID != "gm-7" || actor.Role != "moderator" || reason != "support case 42" || args.Value != "potion" {
			t.Fatalf("executor received actor=%#v reason=%q args=%#v", actor, reason, args)
		}
		if len(auditor.events) != 1 || auditor.events[0].Outcome != OutcomeRequested {
			t.Fatalf("command executed before intent audit: %#v", auditor.events)
		}
		return "saved", nil
	}, 128)

	result, err := Dispatch(context.Background(), NewDispatcher(registry, authorizer, auditor), command, Request[testArgs]{
		Actor:  &Actor{ID: "gm-7", Role: "moderator"},
		Reason: "support case 42",
		Args:   testArgs{Value: "potion"},
	})
	if err != nil || result != "saved" {
		t.Fatalf("Dispatch = (%q, %v), want (saved, nil)", result, err)
	}
	if authorizer.actor.ID != "gm-7" || authorizer.capability != "inventory.grant" || authorizer.reason != "support case 42" {
		t.Fatalf("authorizer received actor=%#v capability=%q reason=%q", authorizer.actor, authorizer.capability, authorizer.reason)
	}
	if len(auditor.events) != 2 {
		t.Fatalf("audit event count = %d, want 2", len(auditor.events))
	}
	event := auditor.events[1]
	if event.ActorID != "gm-7" || event.ActorRole != "moderator" || event.Reason != "support case 42" || event.Outcome != OutcomeSucceeded || event.Capability != "inventory.grant" {
		t.Fatalf("audit event = %#v", event)
	}
}

func TestDispatchAuditsFailedExecution(t *testing.T) {
	registry := NewRegistry()
	auditor := &testAuditor{}
	command := registerTestCommand(t, registry, func(context.Context, Actor, string, testArgs) (string, error) {
		return "", errors.New("storage unavailable")
	}, 128)

	result, err := Dispatch(context.Background(), NewDispatcher(registry, &testAuthorizer{allowed: true}, auditor), command, Request[testArgs]{
		Actor:  &Actor{ID: "gm-1", Role: "moderator"},
		Reason: "ticket 123",
	})
	if err == nil || result != "" {
		t.Fatalf("Dispatch = (%q, %v), want error and empty result", result, err)
	}
	if len(auditor.events) != 2 || auditor.events[0].Outcome != OutcomeRequested ||
		auditor.events[1].Outcome != OutcomeFailed || !strings.Contains(auditor.events[1].Error, "storage unavailable") {
		t.Fatalf("audit event = %#v, want failed storage attempt", auditor.events)
	}
}

func TestDispatchReturnsAuditFailuresWithoutClaimingSuccess(t *testing.T) {
	registry := NewRegistry()
	auditor := &testAuditor{err: errors.New("audit storage unavailable")}
	executions := 0
	command := registerTestCommand(t, registry, func(context.Context, Actor, string, testArgs) (string, error) {
		executions++
		return "stored", nil
	}, 128)

	result, err := Dispatch(context.Background(), NewDispatcher(registry, &testAuthorizer{allowed: true}, auditor), command, Request[testArgs]{
		Actor:  &Actor{ID: "gm-1", Role: "moderator"},
		Reason: "ticket 123",
	})
	if !errors.Is(err, ErrAuditFailed) || result != "" {
		t.Fatalf("Dispatch = (%q, %v), want empty result and ErrAuditFailed", result, err)
	}
	if executions != 0 {
		t.Fatalf("executor called %d times, want 0 when the intent cannot be audited", executions)
	}
	if len(auditor.events) != 1 || auditor.events[0].Outcome != OutcomeRequested {
		t.Fatalf("audit events = %#v, want one failed intent record", auditor.events)
	}
}

func TestDispatchReturnsAuditFailureForDeniedAttempt(t *testing.T) {
	registry := NewRegistry()
	auditor := &testAuditor{err: errors.New("audit storage unavailable")}
	executions := 0
	command := registerTestCommand(t, registry, func(context.Context, Actor, string, testArgs) (string, error) {
		executions++
		return "stored", nil
	}, 128)

	result, err := Dispatch(context.Background(), NewDispatcher(registry, &testAuthorizer{allowed: false}, auditor), command, Request[testArgs]{
		Actor:  &Actor{ID: "gm-1", Role: "moderator"},
		Reason: "ticket 123",
	})
	if !errors.Is(err, ErrAuditFailed) || result != "" || executions != 0 {
		t.Fatalf("Dispatch = (%q, %v), executor calls=%d; want audit failure and no execution", result, err, executions)
	}
	if len(auditor.events) != 1 || auditor.events[0].Outcome != OutcomeDenied {
		t.Fatalf("audit events = %#v, want one attempted denial record", auditor.events)
	}
}

func TestDispatchEnforcesArgumentSizeLimit(t *testing.T) {
	registry := NewRegistry()
	auditor := &testAuditor{}
	authorizer := &testAuthorizer{allowed: true}
	executions := 0
	command := registerTestCommand(t, registry, func(context.Context, Actor, string, testArgs) (string, error) {
		executions++
		return "stored", nil
	}, 8)

	_, err := Dispatch(context.Background(), NewDispatcher(registry, authorizer, auditor), command, Request[testArgs]{
		Actor:  &Actor{ID: "gm-1", Role: "moderator"},
		Reason: "ticket 123",
		Args:   testArgs{Value: "a value longer than eight bytes"},
	})
	if err == nil || !strings.Contains(err.Error(), "maximum size") {
		t.Fatalf("Dispatch error = %v, want argument size error", err)
	}
	if authorizer.calls != 0 || executions != 0 {
		t.Fatalf("authorizer called %d times and executor %d times, want neither called", authorizer.calls, executions)
	}
	if len(auditor.events) != 1 || auditor.events[0].Outcome != OutcomeDenied {
		t.Fatalf("audit events = %#v, want one denied event", auditor.events)
	}
}

func TestDispatchFailsClosedWhenAuditUnavailable(t *testing.T) {
	registry := NewRegistry()
	executions := 0
	command := registerTestCommand(t, registry, func(context.Context, Actor, string, testArgs) (string, error) {
		executions++
		return "stored", nil
	}, 128)

	result, err := Dispatch(context.Background(), NewDispatcher(registry, &testAuthorizer{allowed: true}, nil), command, Request[testArgs]{
		Actor:  &Actor{ID: "gm-1", Role: "moderator"},
		Reason: "ticket 123",
	})
	if !errors.Is(err, ErrAuditUnavailable) || result != "" || executions != 0 {
		t.Fatalf("Dispatch = (%q, %v), executor calls=%d; want audit unavailable and no execution", result, err, executions)
	}
}
