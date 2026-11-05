package idempotency

import (
	"testing"
	"time"
)

func TestStoreReplayConflictAndExpiry(t *testing.T) {
	store := New(time.Minute)
	now := time.Now()
	store.now = func() time.Time { return now }
	fingerprint := Fingerprint([]byte("request"))
	if err := store.Put("key", fingerprint, 201, []byte("response")); err != nil {
		t.Fatal(err)
	}
	if result, found, err := store.Get("key", fingerprint); err != nil || !found || result.Status != 201 {
		t.Fatalf("replay = %#v, %v, %v", result, found, err)
	}
	if _, _, err := store.Get("key", Fingerprint([]byte("other"))); err != ErrConflict {
		t.Fatalf("conflict error = %v", err)
	}
	now = now.Add(time.Minute)
	if _, found, err := store.Get("key", fingerprint); err != nil || found {
		t.Fatalf("expired key found=%v err=%v", found, err)
	}
}

func TestStoreCopiesResponseBytes(t *testing.T) {
	store := New(time.Minute)
	fingerprint := Fingerprint([]byte("request"))
	body := []byte("response")
	if err := store.Put("key", fingerprint, 200, body); err != nil {
		t.Fatal(err)
	}
	body[0] = 'X'
	result, _, err := store.Get("key", fingerprint)
	if err != nil || string(result.Body) != "response" {
		t.Fatalf("stored response = %q, err=%v", result.Body, err)
	}
}

func TestStoreCapacity(t *testing.T) {
	store := New(time.Minute)
	store.maxItems = 1
	if err := store.Put("first", Fingerprint([]byte("one")), 200, []byte("ok")); err != nil {
		t.Fatal(err)
	}
	if err := store.Put("second", Fingerprint([]byte("two")), 200, []byte("ok")); err == nil {
		t.Fatal("expected capacity error")
	}
}
