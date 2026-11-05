package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func (clock *testClock) Now() time.Time {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	return clock.now
}

func (clock *testClock) Advance(duration time.Duration) {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	clock.now = clock.now.Add(duration)
}

func newTestLimiter(t *testing.T, config Config) *Limiter {
	t.Helper()
	limiter, err := New(config)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return limiter
}

func TestAllowUsesIndependentBucketsAndExpiresWindow(t *testing.T) {
	clock := &testClock{now: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)}
	limiter := newTestLimiter(t, Config{
		DefaultLimit:  Limit{Requests: 2, Window: time.Minute},
		MaxKeys:       4,
		IdleTTL:       time.Hour,
		CleanupPeriod: time.Hour,
		Clock:         clock.Now,
	})

	for _, want := range []Decision{Allowed, Allowed, RateLimited} {
		if got := limiter.Allow("client-a"); got != want {
			t.Errorf("Allow(client-a) = %v, want %v", got, want)
		}
	}
	if got := limiter.Allow("client-b"); got != Allowed {
		t.Fatalf("Allow(client-b) = %v, want %v", got, Allowed)
	}

	clock.Advance(time.Minute)
	if got := limiter.Allow("client-a"); got != Allowed {
		t.Errorf("Allow(client-a) after window = %v, want %v", got, Allowed)
	}
}

func TestCapacityFailsClosedAndExpiredBucketsAreReclaimed(t *testing.T) {
	clock := &testClock{now: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)}
	limiter := newTestLimiter(t, Config{
		DefaultLimit:  Limit{Requests: 1, Window: time.Minute},
		MaxKeys:       1,
		IdleTTL:       10 * time.Second,
		CleanupPeriod: time.Hour,
		Clock:         clock.Now,
	})

	if got := limiter.Allow("first"); got != Allowed {
		t.Fatalf("Allow(first) = %v, want %v", got, Allowed)
	}
	if got := limiter.Allow("second"); got != CapacityExceeded {
		t.Fatalf("Allow(second) at capacity = %v, want %v", got, CapacityExceeded)
	}

	clock.Advance(10 * time.Second)
	if got := limiter.Allow("second"); got != Allowed {
		t.Fatalf("Allow(second) after expiry = %v, want %v", got, Allowed)
	}
	if got := len(limiter.buckets); got != 1 {
		t.Errorf("bucket count = %d, want 1", got)
	}
}

func TestAllowAppliesPerKeyPolicyAndFailsClosedOnInvalidLimit(t *testing.T) {
	limiter := newTestLimiter(t, Config{
		DefaultLimit: Limit{Requests: 3, Window: time.Minute},
		LimitForKey: func(key string) Limit {
			switch key {
			case "restricted":
				return Limit{Requests: 1, Window: time.Minute}
			case "invalid":
				return Limit{}
			default:
				return Limit{Requests: 3, Window: time.Minute}
			}
		},
		MaxKeys: 4,
		IdleTTL: time.Minute,
		Clock:   time.Now,
	})

	if got := limiter.Allow("restricted"); got != Allowed {
		t.Fatalf("first restricted request = %v, want %v", got, Allowed)
	}
	if got := limiter.Allow("restricted"); got != RateLimited {
		t.Errorf("second restricted request = %v, want %v", got, RateLimited)
	}
	if got := limiter.Allow("invalid"); got != FailClosed {
		t.Errorf("invalid policy result = %v, want %v", got, FailClosed)
	}
}

func TestMiddlewareIgnoresIdentityHeadersAndUsesAuthenticatedPrincipal(t *testing.T) {
	limiter := newTestLimiter(t, Config{
		DefaultLimit: Limit{Requests: 1, Window: time.Minute},
		MaxKeys:      8,
		IdleTTL:      time.Minute,
	})
	handler := limiter.Middleware(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))

	makeRequest := func(remoteAddr, forwardedFor, principal string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		request.RemoteAddr = remoteAddr
		request.Header.Set("X-Forwarded-For", forwardedFor)
		if principal != "" {
			request = request.WithContext(WithAuthenticatedPrincipal(request.Context(), principal))
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}

	if got := makeRequest("192.0.2.1:1234", "198.51.100.1", "").Code; got != http.StatusNoContent {
		t.Fatalf("first remote request status = %d", got)
	}
	if got := makeRequest("192.0.2.1:1234", "203.0.113.1", "").Code; got != http.StatusTooManyRequests {
		t.Errorf("request with spoofed identity header status = %d, want %d", got, http.StatusTooManyRequests)
	}
	if got := makeRequest("192.0.2.2:1234", "", "user-7").Code; got != http.StatusNoContent {
		t.Fatalf("first authenticated request status = %d", got)
	}
	if got := makeRequest("192.0.2.3:1234", "", "user-7").Code; got != http.StatusTooManyRequests {
		t.Errorf("same principal from another address status = %d, want %d", got, http.StatusTooManyRequests)
	}
}

func TestMiddlewareFailsClosedForInvalidAddressAndCapacity(t *testing.T) {
	limiter := newTestLimiter(t, Config{
		DefaultLimit: Limit{Requests: 1, Window: time.Minute},
		MaxKeys:      1,
		IdleTTL:      time.Hour,
	})
	handler := limiter.Middleware(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "not-an-address"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Errorf("invalid address status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}

	for index, address := range []string{"192.0.2.1:1234", "192.0.2.2:1234"} {
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		request.RemoteAddr = address
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		want := http.StatusNoContent
		if index == 1 {
			want = http.StatusServiceUnavailable
		}
		if response.Code != want {
			t.Errorf("address %q status = %d, want %d", address, response.Code, want)
		}
	}
	if got := len(limiter.buckets); got > 1 {
		t.Errorf("bucket count = %d, exceeds cap 1", got)
	}
}

func TestConcurrentRequestsRespectLimitAndBucketCap(t *testing.T) {
	const (
		workers        = 200
		requestsPerKey = 10
		limit          = 37
		maxKeys        = 5
	)
	limiter := newTestLimiter(t, Config{
		DefaultLimit: Limit{Requests: limit, Window: time.Minute},
		MaxKeys:      maxKeys,
		IdleTTL:      time.Hour,
	})

	var allowed atomic.Int64
	var rateLimited atomic.Int64
	var capacityExceeded atomic.Int64
	var wait sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wait.Add(1)
		go func(worker int) {
			defer wait.Done()
			for request := 0; request < requestsPerKey; request++ {
				key := "client-" + string(rune('a'+worker%10))
				switch limiter.Allow(key) {
				case Allowed:
					allowed.Add(1)
				case RateLimited:
					rateLimited.Add(1)
				case CapacityExceeded:
					capacityExceeded.Add(1)
				default:
					t.Errorf("unexpected fail-closed result for valid key")
				}
			}
		}(worker)
	}
	wait.Wait()

	if got := allowed.Load(); got != limit*maxKeys {
		t.Errorf("allowed requests = %d, want exactly %d", got, limit*maxKeys)
	}
	if got := rateLimited.Load() + capacityExceeded.Load(); got != workers*requestsPerKey-limit*maxKeys {
		t.Errorf("rejected requests = %d, want %d", got, workers*requestsPerKey-limit*maxKeys)
	}
	if got := len(limiter.buckets); got > maxKeys {
		t.Errorf("bucket count = %d, exceeds cap %d", got, maxKeys)
	}
}
