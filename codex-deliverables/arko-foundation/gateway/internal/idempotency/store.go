package idempotency

import (
	"crypto/sha256"
	"errors"
	"sync"
	"time"
)

var (
	ErrConflict = errors.New("idempotency key was already used with a different request")
	ErrRequired = errors.New("Idempotency-Key header is required")
)

type Result struct {
	Fingerprint [32]byte
	Body        []byte
	Status      int
	ExpiresAt   time.Time
}

type Store struct {
	mu       sync.Mutex
	entries  map[string]Result
	ttl      time.Duration
	now      func() time.Time
	maxItems int
}

func New(ttl time.Duration) *Store {
	return &Store{entries: make(map[string]Result), ttl: ttl, now: time.Now, maxItems: 10000}
}

func Fingerprint(body []byte) [32]byte { return sha256.Sum256(body) }

func (s *Store) Get(key string, fingerprint [32]byte) (Result, bool, error) {
	if key == "" {
		return Result{}, false, ErrRequired
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[key]
	if !ok {
		return Result{}, false, nil
	}
	if !entry.ExpiresAt.After(s.now()) {
		delete(s.entries, key)
		return Result{}, false, nil
	}
	if entry.Fingerprint != fingerprint {
		return Result{}, false, ErrConflict
	}
	entry.Body = append([]byte(nil), entry.Body...)
	return entry, true, nil
}

func (s *Store) Put(key string, fingerprint [32]byte, status int, body []byte) error {
	if key == "" {
		return ErrRequired
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for existingKey, entry := range s.entries {
		if !entry.ExpiresAt.After(now) {
			delete(s.entries, existingKey)
		}
	}
	if existing, ok := s.entries[key]; ok && existing.ExpiresAt.After(s.now()) {
		if existing.Fingerprint != fingerprint {
			return ErrConflict
		}
		return nil
	}
	if len(s.entries) >= s.maxItems {
		return errors.New("idempotency capacity reached")
	}
	s.entries[key] = Result{
		Fingerprint: fingerprint,
		Body:        append([]byte(nil), body...),
		Status:      status,
		ExpiresAt:   now.Add(s.ttl),
	}
	return nil
}
