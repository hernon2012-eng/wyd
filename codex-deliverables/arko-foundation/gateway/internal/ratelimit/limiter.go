package ratelimit

import (
	"sync"
	"time"
)

type bucket struct {
	tokens float64
	last   time.Time
}

type Limiter struct {
	mu       sync.Mutex
	buckets  map[string]bucket
	rate     float64
	burst    float64
	now      func() time.Time
	maxPeers int
}

func New(rate float64, burst int) *Limiter {
	return &Limiter{buckets: make(map[string]bucket), rate: rate, burst: float64(burst), now: time.Now, maxPeers: 4096}
}

func (l *Limiter) Allow(key string) bool {
	if l.rate <= 0 || l.burst <= 0 {
		return true
	}
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	entry, ok := l.buckets[key]
	if !ok {
		if len(l.buckets) >= l.maxPeers {
			for peer, candidate := range l.buckets {
				if now.Sub(candidate.last) > time.Minute {
					delete(l.buckets, peer)
				}
			}
			if len(l.buckets) >= l.maxPeers {
				return false
			}
		}
		entry = bucket{tokens: l.burst, last: now}
	}
	entry.tokens += now.Sub(entry.last).Seconds() * l.rate
	if entry.tokens > l.burst {
		entry.tokens = l.burst
	}
	entry.last = now
	if entry.tokens < 1 {
		l.buckets[key] = entry
		return false
	}
	entry.tokens--
	l.buckets[key] = entry
	return true
}
