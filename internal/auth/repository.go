package auth

import (
	"context"
	"database/sql"
)

// Repository persists users and bearer tokens.
type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// CreateUser inserts a user row; the caller supplies the hashed password.
func (r *Repository) CreateUser(ctx context.Context, user User) error {
	active := 0
	if user.Active {
		active = 1
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO users(id, username, password_hash, role, display_name, active, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, user.ID, user.Username, user.PasswordHash, user.Role, user.DisplayName, active, user.CreatedAt)
	return err
}

// GetByUsername returns the user with the given username or sql.ErrNoRows.
func (r *Repository) GetByUsername(ctx context.Context, username string) (User, error) {
	row := r.db.QueryRowContext(ctx, userSelectColumns+` WHERE username = ?`, username)
	return scanUser(row)
}

// GetByID returns the user with the given id or sql.ErrNoRows.
func (r *Repository) GetByID(ctx context.Context, userID string) (User, error) {
	row := r.db.QueryRowContext(ctx, userSelectColumns+` WHERE id = ?`, userID)
	return scanUser(row)
}

// ListUsers returns every account ordered by username.
func (r *Repository) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := r.db.QueryContext(ctx, userSelectColumns+` ORDER BY username ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := []User{}
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

// SaveToken stores a freshly issued bearer token for a user.
func (r *Repository) SaveToken(ctx context.Context, token, userID, createdAt, expiresAt string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO auth_tokens(token, user_id, created_at, expires_at)
		VALUES (?, ?, ?, ?)
	`, token, userID, createdAt, expiresAt)
	return err
}

// ResolveToken returns the token's user when the token exists, has not
// expired, and the user is active; otherwise sql.ErrNoRows.
func (r *Repository) ResolveToken(ctx context.Context, token, now string) (User, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT u.id, u.username, u.role, u.display_name, u.active, u.created_at, u.password_hash
		FROM auth_tokens t
		JOIN users u ON u.id = t.user_id
		WHERE t.token = ? AND t.expires_at > ? AND u.active = 1
	`, token, now)
	return scanUser(row)
}

// DeleteToken removes a token; a missing token is not an error.
func (r *Repository) DeleteToken(ctx context.Context, token string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM auth_tokens WHERE token = ?`, token)
	return err
}

const userSelectColumns = `
	SELECT id, username, role, display_name, active, created_at, password_hash
	FROM users
`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(row rowScanner) (User, error) {
	var user User
	var active int
	if err := row.Scan(
		&user.ID,
		&user.Username,
		&user.Role,
		&user.DisplayName,
		&active,
		&user.CreatedAt,
		&user.PasswordHash,
	); err != nil {
		return User{}, err
	}
	user.Active = active != 0
	return user, nil
}
