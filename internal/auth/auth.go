// Package auth implements the minimal authentication slice for the workbench:
// username/password login with PBKDF2 password hashing, bearer tokens with a
// fixed TTL, and the two roles (operator, client) used for data isolation.
package auth

import (
	"context"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"tiktok-trend-shop/internal/id"
)

// Roles supported by the workbench.
const (
	RoleOperator = "operator"
	RoleClient   = "client"
)

const (
	// TokenTTL is how long an issued bearer token stays valid.
	TokenTTL = 7 * 24 * time.Hour

	pbkdf2Iterations = 210_000
	pbkdf2KeyLength  = 32
	saltLength       = 16
	tokenBytes       = 32
)

var (
	// ErrInvalidCredentials is returned by Login for unknown usernames, wrong
	// passwords, and disabled accounts alike.
	ErrInvalidCredentials = errors.New("invalid username or password")
	// ErrTokenInvalid is returned when a token is unknown, expired, or belongs
	// to a disabled user.
	ErrTokenInvalid = errors.New("token is invalid or expired")
	// ErrUsernameTaken is returned when creating a user with a duplicate name.
	ErrUsernameTaken = errors.New("username already exists")
)

// User is one account. PasswordHash is never serialized to JSON.
type User struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	Role         string `json:"role"`
	DisplayName  string `json:"display_name"`
	Active       bool   `json:"active"`
	CreatedAt    string `json:"created_at"`
	PasswordHash string `json:"-"`
}

// Service bundles the repository with password and token handling.
type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// CreateUser validates the input, hashes the password, and stores the user.
func (s *Service) CreateUser(ctx context.Context, username, password, role, displayName string) (User, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return User{}, fmt.Errorf("username is required")
	}
	if password == "" {
		return User{}, fmt.Errorf("password is required")
	}
	if role != RoleOperator && role != RoleClient {
		return User{}, fmt.Errorf("role must be %q or %q", RoleOperator, RoleClient)
	}
	hash, err := HashPassword(password)
	if err != nil {
		return User{}, err
	}
	user := User{
		ID:           id.New("usr"),
		Username:     username,
		Role:         role,
		DisplayName:  strings.TrimSpace(displayName),
		Active:       true,
		CreatedAt:    utcNow(),
		PasswordHash: hash,
	}
	if err := s.repo.CreateUser(ctx, user); err != nil {
		if isUniqueViolation(err) {
			return User{}, ErrUsernameTaken
		}
		return User{}, err
	}
	return user, nil
}

// ListUsers returns all accounts for operator management views.
func (s *Service) ListUsers(ctx context.Context) ([]User, error) {
	return s.repo.ListUsers(ctx)
}

// Login verifies the credentials and issues a fresh bearer token.
func (s *Service) Login(ctx context.Context, username, password string) (string, User, error) {
	user, err := s.repo.GetByUsername(ctx, strings.TrimSpace(username))
	if err == sql.ErrNoRows {
		return "", User{}, ErrInvalidCredentials
	}
	if err != nil {
		return "", User{}, err
	}
	if !user.Active || !VerifyPassword(password, user.PasswordHash) {
		return "", User{}, ErrInvalidCredentials
	}
	token, err := generateToken()
	if err != nil {
		return "", User{}, err
	}
	if err := s.repo.SaveToken(ctx, token, user.ID, utcNow(), time.Now().UTC().Add(TokenTTL).Format(time.RFC3339)); err != nil {
		return "", User{}, err
	}
	return token, user, nil
}

// Authenticate resolves a bearer token to its user, enforcing expiry and the
// active flag.
func (s *Service) Authenticate(ctx context.Context, token string) (User, error) {
	user, err := s.repo.ResolveToken(ctx, token, utcNow())
	if err == sql.ErrNoRows {
		return User{}, ErrTokenInvalid
	}
	if err != nil {
		return User{}, err
	}
	return user, nil
}

// Logout deletes the bearer token so it can no longer be used.
func (s *Service) Logout(ctx context.Context, token string) error {
	return s.repo.DeleteToken(ctx, token)
}

// SeedOperator creates the initial operator account when the users table is
// empty. Credentials come from TTS_ADMIN_USERNAME / TTS_ADMIN_PASSWORD with
// development defaults; falling back to the default password prints a warning
// to stderr.
func (s *Service) SeedOperator(ctx context.Context) error {
	users, err := s.repo.ListUsers(ctx)
	if err != nil {
		return err
	}
	if len(users) > 0 {
		return nil
	}
	username := envOr("TTS_ADMIN_USERNAME", "admin")
	password := os.Getenv("TTS_ADMIN_PASSWORD")
	if password == "" {
		password = "admin123"
		fmt.Fprintf(os.Stderr,
			"WARNING: seeding operator account %q with the default password; set TTS_ADMIN_PASSWORD to override\n",
			username)
	}
	_, err = s.CreateUser(ctx, username, password, RoleOperator, "Operator")
	return err
}

// HashPassword derives a PBKDF2-SHA256 hash, stored as "saltHex$hashHex".
func HashPassword(password string) (string, error) {
	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, pbkdf2Iterations, pbkdf2KeyLength)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(salt) + "$" + hex.EncodeToString(key), nil
}

// VerifyPassword compares a candidate against a stored "saltHex$hashHex" hash
// in constant time.
func VerifyPassword(password, stored string) bool {
	parts := strings.Split(stored, "$")
	if len(parts) != 2 {
		return false
	}
	salt, err := hex.DecodeString(parts[0])
	if err != nil {
		return false
	}
	want, err := hex.DecodeString(parts[1])
	if err != nil || len(want) == 0 {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, pbkdf2Iterations, len(want))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(got, want) == 1
}

func generateToken() (string, error) {
	buf := make([]byte, tokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func utcNow() string {
	return time.Now().UTC().Format(time.RFC3339)
}
