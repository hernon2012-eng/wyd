package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidInput   = errors.New("invalid identity input")
	ErrAuthentication = errors.New("authentication failed")
)

const (
	maxUsernameLength = 128
	maxPasswordLength = 72
	sessionTokenBytes = 32
)

// Role is stored as a string and accepted only when it is one of the defined values.
type Role string

const (
	RolePlayer    Role = "player"
	RoleModerator Role = "moderator"
	RoleAdmin     Role = "admin"
)

// Admin is the public identity record; password hashes are never included.
type Admin struct {
	ID       int64
	Username string
	Role     Role
}

// Session contains the bearer token only when a session is first created.
type Session struct {
	Token     string
	ExpiresAt time.Time
}

// AuthenticatedSession is returned after a bearer token has been resolved.
type AuthenticatedSession struct {
	Admin     Admin
	ExpiresAt time.Time
}

// Store uses an injected database connection and never applies migrations itself.
type Store struct {
	db *sql.DB
}

func New(db *sql.DB) (*Store, error) {
	if db == nil {
		return nil, ErrInvalidInput
	}
	return &Store{db: db}, nil
}

// CreateCredentials creates an admin identity using a bcrypt password hash.
func (s *Store) CreateCredentials(ctx context.Context, username, password string, role Role) (Admin, error) {
	if s == nil || s.db == nil || ctx == nil || !validUsername(username) || !validRole(role) {
		return Admin{}, ErrInvalidInput
	}
	passwordHash, err := hashPassword(password)
	if err != nil {
		return Admin{}, err
	}

	var admin Admin
	err = s.db.QueryRowContext(ctx,
		`INSERT INTO arko_admin_credentials (username, password_hash, role)
		 VALUES ($1, $2, $3)
		 RETURNING id, username, role`,
		username, passwordHash, string(role),
	).Scan(&admin.ID, &admin.Username, &admin.Role)
	if err != nil {
		return Admin{}, fmt.Errorf("create admin credentials: %w", err)
	}
	return admin, nil
}

// VerifyCredentials returns the same authentication error for an unknown user or
// a password mismatch.
func (s *Store) VerifyCredentials(ctx context.Context, username, password string) (Admin, error) {
	if s == nil || s.db == nil || ctx == nil || !validUsername(username) || !validPassword(password) {
		return Admin{}, ErrAuthentication
	}

	var admin Admin
	var passwordHash []byte
	err := s.db.QueryRowContext(ctx,
		`SELECT id, username, password_hash, role
		 FROM arko_admin_credentials
		 WHERE username = $1
		   AND status = 'active'`,
		username,
	).Scan(&admin.ID, &admin.Username, &passwordHash, &admin.Role)
	if errors.Is(err, sql.ErrNoRows) {
		return Admin{}, ErrAuthentication
	}
	if err != nil {
		return Admin{}, fmt.Errorf("verify admin credentials: %w", err)
	}
	if !validRole(admin.Role) || bcrypt.CompareHashAndPassword(passwordHash, []byte(password)) != nil {
		return Admin{}, ErrAuthentication
	}
	return admin, nil
}

// CreateSession creates an opaque, high-entropy bearer token. Only its SHA-256
// digest is written to the database.
func (s *Store) CreateSession(ctx context.Context, adminID int64, ttl time.Duration) (Session, error) {
	if s == nil || s.db == nil || ctx == nil || adminID <= 0 || ttl <= 0 {
		return Session{}, ErrInvalidInput
	}
	token, tokenHash, err := newSessionToken()
	if err != nil {
		return Session{}, fmt.Errorf("generate session token: %w", err)
	}
	expiresAt := time.Now().UTC().Add(ttl)
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO arko_admin_sessions (token_hash, admin_id, expires_at)
		 VALUES ($1, $2, $3)`,
		tokenHash, adminID, expiresAt,
	)
	if err != nil {
		return Session{}, fmt.Errorf("create admin session: %w", err)
	}
	return Session{Token: token, ExpiresAt: expiresAt}, nil
}

// ResolveSession resolves only unexpired and unrevoked bearer tokens.
func (s *Store) ResolveSession(ctx context.Context, token string) (AuthenticatedSession, error) {
	tokenHash, ok := hashPresentedToken(token)
	if !ok || s == nil || s.db == nil || ctx == nil {
		return AuthenticatedSession{}, ErrAuthentication
	}

	var result AuthenticatedSession
	err := s.db.QueryRowContext(ctx,
		`SELECT a.id, a.username, a.role, s.expires_at
		 FROM arko_admin_sessions AS s
		 JOIN arko_admin_credentials AS a ON a.id = s.admin_id
		 WHERE s.token_hash = $1
		   AND s.expires_at > CURRENT_TIMESTAMP
		   AND s.revoked_at IS NULL
		   AND a.status = 'active'`,
		tokenHash,
	).Scan(&result.Admin.ID, &result.Admin.Username, &result.Admin.Role, &result.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return AuthenticatedSession{}, ErrAuthentication
	}
	if err != nil {
		return AuthenticatedSession{}, fmt.Errorf("resolve admin session: %w", err)
	}
	if !validRole(result.Admin.Role) {
		return AuthenticatedSession{}, ErrAuthentication
	}
	return result, nil
}

// RevokeSession revokes a bearer token without storing or returning the token.
func (s *Store) RevokeSession(ctx context.Context, token string) error {
	tokenHash, ok := hashPresentedToken(token)
	if !ok || s == nil || s.db == nil || ctx == nil {
		return ErrAuthentication
	}
	result, err := s.db.ExecContext(ctx,
		`UPDATE arko_admin_sessions
		 SET revoked_at = COALESCE(revoked_at, CURRENT_TIMESTAMP)
		 WHERE token_hash = $1`,
		tokenHash,
	)
	if err != nil {
		return fmt.Errorf("revoke admin session: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("revoke admin session: %w", err)
	}
	if rowsAffected == 0 {
		return ErrAuthentication
	}
	return nil
}

func hashPassword(password string) ([]byte, error) {
	if !validPassword(password) {
		return nil, ErrInvalidInput
	}
	return bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
}

func validPassword(password string) bool {
	return len(password) > 0 && len(password) <= maxPasswordLength
}

func validUsername(username string) bool {
	if len(username) == 0 || len(username) > maxUsernameLength || strings.TrimSpace(username) != username {
		return false
	}
	for _, r := range username {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func validRole(role Role) bool {
	switch role {
	case RolePlayer, RoleModerator, RoleAdmin:
		return true
	default:
		return false
	}
}

func newSessionToken() (string, []byte, error) {
	raw := make([]byte, sessionTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256([]byte(token))
	return token, hash[:], nil
}

func hashPresentedToken(token string) ([]byte, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != sessionTokenBytes || base64.RawURLEncoding.EncodeToString(raw) != token {
		return nil, false
	}
	hash := sha256.Sum256([]byte(token))
	return hash[:], true
}
