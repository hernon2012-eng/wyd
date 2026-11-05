package ratelimit

import (
	"testing"
	"time"
)

func TestLimiterConsumesAndRefillsTokens(t *testing.T) {
	limiter := New(1, 2)
	now := time.Now()
	limiter.now = func() time.Time { return now }
	if !limiter.Allow("peer") {
		t.Fatal("first token was not allowed")
	}
	if !limiter.Allow("peer") {
		t.Fatal("second token was not allowed")
	}
	if limiter.Allow("peer") {
		t.Fatal("third request exceeded burst capacity")
	}
	now = now.Add(time.Second)
	if !limiter.Allow("peer") {
		t.Fatal("token was not replenished")
	}
}
