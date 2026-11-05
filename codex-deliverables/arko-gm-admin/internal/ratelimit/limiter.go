package ratelimit

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"
)

// Limit allows Requests requests in each fixed Window.
type Limit struct {
	Requests int
	Window   time.Duration
}

// Config controls the limiter's storage, cleanup, and rate policies.
type Config struct {
	DefaultLimit  Limit
	LimitForKey   func(key string) Limit
	MaxKeys       int
	MaxKeyLength  int
	IdleTTL       time.Duration
	CleanupPeriod time.Duration
	Clock         func() time.Time
}

// Decision is the outcome of checking a request against the limiter.
type Decision uint8

const (
	Allowed Decision = iota
	RateLimited
	CapacityExceeded
	FailClosed
)

var errInvalidConfig = errors.New("ratelimit: invalid configuration")

type bucket struct {
	limit       Limit
	windowStart time.Time
	lastSeen    time.Time
	requests    int
}

// Limiter maintains one bounded fixed-window bucket per key.
type Limiter struct {
	mu            sync.Mutex
	buckets       map[string]*bucket
	defaultLimit  Limit
	limitForKey   func(string) Limit
	maxKeys       int
	maxKeyLength  int
	idleTTL       time.Duration
	cleanupPeriod time.Duration
	clock         func() time.Time
	nextCleanup   time.Time
}

// New constructs a limiter with bounded storage and periodic idle cleanup.
func New(config Config) (*Limiter, error) {
	if !validLimit(config.DefaultLimit) || config.MaxKeys <= 0 || config.IdleTTL <= 0 {
		return nil, errInvalidConfig
	}
	if config.MaxKeyLength == 0 {
		config.MaxKeyLength = 256
	}
	if config.MaxKeyLength < 1 {
		return nil, errInvalidConfig
	}
	if config.CleanupPeriod == 0 {
		config.CleanupPeriod = config.IdleTTL / 2
		if config.CleanupPeriod == 0 {
			config.CleanupPeriod = config.IdleTTL
		}
	}
	if config.CleanupPeriod < 0 {
		return nil, errInvalidConfig
	}
	if config.Clock == nil {
		config.Clock = time.Now
	}

	return &Limiter{
		buckets:       make(map[string]*bucket),
		defaultLimit:  config.DefaultLimit,
		limitForKey:   config.LimitForKey,
		maxKeys:       config.MaxKeys,
		maxKeyLength:  config.MaxKeyLength,
		idleTTL:       config.IdleTTL,
		cleanupPeriod: config.CleanupPeriod,
		clock:         config.Clock,
	}, nil
}

func validLimit(limit Limit) bool {
	return limit.Requests > 0 && limit.Window > 0
}

// Allow checks and records one request for key. Invalid keys or policies fail
// closed, and no new bucket is created when storage remains at capacity.
func (l *Limiter) Allow(key string) Decision {
	if key == "" || len(key) > l.maxKeyLength {
		return FailClosed
	}

	limit := l.defaultLimit
	if l.limitForKey != nil {
		limit = l.limitForKey(key)
	}
	if !validLimit(limit) {
		return FailClosed
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.clock()
	if l.nextCleanup.IsZero() || !now.Before(l.nextCleanup) {
		l.cleanupLocked(now)
		l.nextCleanup = now.Add(l.cleanupPeriod)
	}

	current, ok := l.buckets[key]
	if !ok {
		if len(l.buckets) >= l.maxKeys {
			l.cleanupLocked(now)
			if len(l.buckets) >= l.maxKeys {
				return CapacityExceeded
			}
		}
		current = &bucket{limit: limit, windowStart: now, lastSeen: now}
		l.buckets[key] = current
	} else if current.limit.Window != limit.Window {
		current.limit = limit
		current.windowStart = now
		current.requests = 0
	} else {
		current.limit = limit
	}

	if now.Before(current.windowStart) || !now.Before(current.windowStart.Add(current.limit.Window)) {
		current.windowStart = now
		current.requests = 0
	}
	current.lastSeen = now
	if current.requests >= current.limit.Requests {
		return RateLimited
	}
	current.requests++
	return Allowed
}

func (l *Limiter) cleanupLocked(now time.Time) {
	for key, current := range l.buckets {
		if now.Sub(current.lastSeen) >= l.idleTTL {
			delete(l.buckets, key)
		}
	}
}

type principalContextKey struct{}

// WithAuthenticatedPrincipal stores an authenticated principal for use by
// Middleware. Call this only after the principal has been authenticated.
func WithAuthenticatedPrincipal(ctx context.Context, principal string) context.Context {
	return context.WithValue(ctx, principalContextKey{}, principal)
}

func requestKey(request *http.Request) (string, bool) {
	if principal, ok := request.Context().Value(principalContextKey{}).(string); ok {
		if principal = strings.TrimSpace(principal); principal != "" {
			return "principal:" + principal, true
		}
	}

	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err != nil {
		host = request.RemoteAddr
	}
	address, err := netip.ParseAddr(host)
	if err != nil {
		return "", false
	}
	return "remote:" + address.Unmap().String(), true
}

// Middleware enforces the configured limit using only an authenticated
// principal from context or the server-provided RemoteAddr. It does not inspect
// client-supplied identity headers.
func (l *Limiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		key, ok := requestKey(request)
		if !ok {
			http.Error(writer, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
			return
		}

		switch l.Allow(key) {
		case Allowed:
			next.ServeHTTP(writer, request)
		case RateLimited:
			http.Error(writer, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
		case CapacityExceeded, FailClosed:
			http.Error(writer, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		}
	})
}
