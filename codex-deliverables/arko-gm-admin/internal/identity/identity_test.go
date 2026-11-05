package identity

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func TestHashPasswordUsesBcrypt(t *testing.T) {
	password := "correct horse battery staple"
	hash, err := hashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(hash, []byte(password)) {
		t.Fatal("password was stored without hashing")
	}
	if err := bcrypt.CompareHashAndPassword(hash, []byte(password)); err != nil {
		t.Fatalf("bcrypt hash did not verify: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword(hash, []byte("wrong password")); err == nil {
		t.Fatal("bcrypt hash accepted an incorrect password")
	}
}

func TestSessionTokenIsOpaqueAndHashable(t *testing.T) {
	token, storedHash, err := newSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(token) != base64.RawURLEncoding.EncodedLen(sessionTokenBytes) {
		t.Fatalf("token length = %d, want %d", len(token), base64.RawURLEncoding.EncodedLen(sessionTokenBytes))
	}
	expectedHash := sha256.Sum256([]byte(token))
	if bytes.Equal(storedHash, []byte(token)) || !bytes.Equal(storedHash, expectedHash[:]) {
		t.Fatal("stored session value is not an opaque SHA-256 digest")
	}
	gotHash, ok := hashPresentedToken(token)
	if !ok || !bytes.Equal(gotHash, storedHash) {
		t.Fatal("presented token did not resolve to its stored digest")
	}
	if _, ok := hashPresentedToken(token + "x"); ok {
		t.Fatal("malformed bearer token was accepted")
	}
}

func TestInputValidation(t *testing.T) {
	if _, err := New(nil); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("New(nil) error = %v, want ErrInvalidInput", err)
	}
	for _, password := range []string{"", string(bytes.Repeat([]byte("x"), maxPasswordLength+1))} {
		if _, err := hashPassword(password); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("hashPassword(%d bytes) error = %v, want ErrInvalidInput", len(password), err)
		}
	}
	for _, username := range []string{"", " leading", "trailing ", "control\n", string(bytes.Repeat([]byte("x"), maxUsernameLength+1))} {
		if validUsername(username) {
			t.Errorf("validUsername(%q) = true, want false", username)
		}
	}
	for _, role := range []Role{RolePlayer, RoleModerator, RoleAdmin} {
		if !validRole(role) {
			t.Errorf("validRole(%q) = false, want true", role)
		}
	}
	for _, role := range []Role{"", "owner", "ADMIN"} {
		if validRole(role) {
			t.Errorf("validRole(%q) = true, want false", role)
		}
	}
}

func TestSessionInputValidationBeforeDatabaseAccess(t *testing.T) {
	store, err := New(nil)
	if !errors.Is(err, ErrInvalidInput) || store != nil {
		t.Fatalf("New(nil) = (%v, %v), want (nil, ErrInvalidInput)", store, err)
	}

	var nilStore *Store
	if _, err := nilStore.CreateSession(context.Background(), 1, time.Hour); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("CreateSession on nil store error = %v, want ErrInvalidInput", err)
	}
	if _, err := nilStore.ResolveSession(context.Background(), "invalid"); !errors.Is(err, ErrAuthentication) {
		t.Fatalf("ResolveSession on nil store error = %v, want ErrAuthentication", err)
	}
	if err := nilStore.RevokeSession(context.Background(), "invalid"); !errors.Is(err, ErrAuthentication) {
		t.Fatalf("RevokeSession on nil store error = %v, want ErrAuthentication", err)
	}
}

func TestMigrationsAreAvailableButNotApplied(t *testing.T) {
	migrations, err := Migrations()
	if err != nil {
		t.Fatal(err)
	}
	if len(migrations) != 1 || migrations[0].Version != 1 {
		t.Fatalf("migrations = %#v, want one version 1 migration", migrations)
	}
	if !bytes.Contains([]byte(migrations[0].SQL), []byte("password_hash BYTEA")) ||
		!bytes.Contains([]byte(migrations[0].SQL), []byte("token_hash BYTEA")) {
		t.Fatal("embedded migration is missing credential or session hash columns")
	}
}
