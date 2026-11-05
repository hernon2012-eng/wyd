package session

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

var (
	ErrInvalid  = errors.New("session is invalid or expired")
	ErrCapacity = errors.New("session capacity reached")
)

type Session struct {
	ID        string    `json:"session_id"`
	Token     string    `json:"access_token"`
	Principal string    `json:"principal_id"`
	Backend   string    `json:"backend_mode"`
	ExpiresAt time.Time `json:"expires_at"`
}

type Manager struct {
	mu       sync.Mutex
	sessions map[string]Session
	ttl      time.Duration
	now      func() time.Time
	maxItems int
}

func New(ttl time.Duration) *Manager {
	return &Manager{sessions: make(map[string]Session), ttl: ttl, now: time.Now, maxItems: 10000}
}

func (m *Manager) Create(principal, backend string) (Session, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return Session{}, err
	}
	now := m.now()
	sess := Session{
		ID:        hex.EncodeToString(bytes[:8]),
		Token:     hex.EncodeToString(bytes),
		Principal: principal,
		Backend:   backend,
		ExpiresAt: now.Add(m.ttl),
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for token, existing := range m.sessions {
		if !existing.ExpiresAt.After(now) {
			delete(m.sessions, token)
		}
	}
	if len(m.sessions) >= m.maxItems {
		return Session{}, ErrCapacity
	}
	m.sessions[sess.Token] = sess
	return sess, nil
}

func (m *Manager) Get(token string) (Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sess, ok := m.sessions[token]
	if !ok || !sess.ExpiresAt.After(m.now()) {
		delete(m.sessions, token)
		return Session{}, ErrInvalid
	}
	return sess, nil
}

func (m *Manager) Revoke(token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	sess, ok := m.sessions[token]
	if !ok || !sess.ExpiresAt.After(m.now()) {
		delete(m.sessions, token)
		return ErrInvalid
	}
	delete(m.sessions, token)
	return nil
}
