package session

import (
	"testing"
	"time"
)

func TestSessionExpires(t *testing.T) {
	manager := New(time.Minute)
	now := time.Now()
	manager.now = func() time.Time { return now }
	sess, err := manager.Create("mock:demo", "MOCK")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Get(sess.Token); err != nil {
		t.Fatalf("new session rejected: %v", err)
	}
	now = now.Add(time.Minute)
	if _, err := manager.Get(sess.Token); err != ErrInvalid {
		t.Fatalf("expired session error = %v, want %v", err, ErrInvalid)
	}
}

func TestSessionRevokeAndCapacity(t *testing.T) {
	manager := New(time.Minute)
	manager.maxItems = 1
	sess, err := manager.Create("mock:demo", "MOCK")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Create("mock:demo", "MOCK"); err != ErrCapacity {
		t.Fatalf("capacity error = %v, want %v", err, ErrCapacity)
	}
	if err := manager.Revoke(sess.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Get(sess.Token); err != ErrInvalid {
		t.Fatalf("revoked session error = %v, want %v", err, ErrInvalid)
	}
}
