package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrEmailExists        = errors.New("email already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidSession     = errors.New("invalid session")
)

const sessionLifetime = 30 * 24 * time.Hour

type User struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	CreatedAt   string `json:"createdAt"`
}

type Session struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expiresAt"`
	User      User   `json:"user"`
}

type Store interface {
	Register(context.Context, string, string, string, string) (Session, error)
	Login(context.Context, string, string, string) (Session, error)
	Authenticate(context.Context, string) (User, error)
	Logout(context.Context, string) error
	Preferences(context.Context, string) (json.RawMessage, error)
	SavePreferences(context.Context, string, json.RawMessage) (json.RawMessage, error)
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Register(
	ctx context.Context, email, displayName, password, userAgent string,
) (Session, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return Session{}, fmt.Errorf("hash password: %w", err)
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Session{}, fmt.Errorf("begin registration: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var user User
	var created time.Time
	err = tx.QueryRow(ctx, `
		INSERT INTO app_users(email,password_hash,display_name)
		VALUES($1,$2,$3)
		RETURNING id::text,email,display_name,created_at`,
		normalizeEmail(email), string(hash), strings.TrimSpace(displayName),
	).Scan(&user.ID, &user.Email, &user.DisplayName, &created)
	var databaseError *pgconn.PgError
	if errors.As(err, &databaseError) && databaseError.Code == "23505" {
		return Session{}, ErrEmailExists
	}
	if err != nil {
		return Session{}, fmt.Errorf("create user: %w", err)
	}
	user.CreatedAt = created.UTC().Format(time.RFC3339)
	session, tokenHash, expiresAt, err := newSession(user)
	if err != nil {
		return Session{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO auth_sessions(token_hash,user_id,expires_at,user_agent)
		VALUES($1,$2,$3,$4)`, tokenHash, user.ID, expiresAt,
		nullableTrimmed(userAgent)); err != nil {
		return Session{}, fmt.Errorf("create registration session: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO user_preferences(user_id) VALUES($1)`, user.ID); err != nil {
		return Session{}, fmt.Errorf("initialize preferences: %w", err)
	}
	return session, tx.Commit(ctx)
}

func (r *Repository) Login(
	ctx context.Context, email, password, userAgent string,
) (Session, error) {
	var user User
	var passwordHash string
	var created time.Time
	err := r.pool.QueryRow(ctx, `
		SELECT id::text,email,display_name,created_at,password_hash
		FROM app_users WHERE email=$1 AND disabled_at IS NULL`,
		normalizeEmail(email)).Scan(&user.ID, &user.Email, &user.DisplayName,
		&created, &passwordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		// Keep the expensive path for unknown users to reduce timing leakage.
		_ = bcrypt.CompareHashAndPassword(
			[]byte("$2a$10$7EqJtq98hPqEX7fNZaFWoO5uS6jUIK7QXG8kcW1AFwVZfh5b8P5aK"),
			[]byte(password))
		return Session{}, ErrInvalidCredentials
	}
	if err != nil {
		return Session{}, fmt.Errorf("find login user: %w", err)
	}
	if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)) != nil {
		return Session{}, ErrInvalidCredentials
	}
	user.CreatedAt = created.UTC().Format(time.RFC3339)
	session, tokenHash, expiresAt, err := newSession(user)
	if err != nil {
		return Session{}, err
	}
	if _, err := r.pool.Exec(ctx, `
		INSERT INTO auth_sessions(token_hash,user_id,expires_at,user_agent)
		VALUES($1,$2,$3,$4)`, tokenHash, user.ID, expiresAt,
		nullableTrimmed(userAgent)); err != nil {
		return Session{}, fmt.Errorf("create login session: %w", err)
	}
	return session, nil
}

func (r *Repository) Authenticate(ctx context.Context, token string) (User, error) {
	hash := sha256.Sum256([]byte(token))
	var user User
	var created time.Time
	err := r.pool.QueryRow(ctx, `
		SELECT u.id::text,u.email,u.display_name,u.created_at
		FROM auth_sessions session
		JOIN app_users u ON u.id=session.user_id
		WHERE session.token_hash=$1 AND session.expires_at>now()
		  AND u.disabled_at IS NULL`, hash[:]).Scan(
		&user.ID, &user.Email, &user.DisplayName, &created)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrInvalidSession
	}
	if err != nil {
		return User{}, fmt.Errorf("authenticate session: %w", err)
	}
	user.CreatedAt = created.UTC().Format(time.RFC3339)
	return user, nil
}

func (r *Repository) Logout(ctx context.Context, token string) error {
	hash := sha256.Sum256([]byte(token))
	_, err := r.pool.Exec(ctx, `DELETE FROM auth_sessions WHERE token_hash=$1`, hash[:])
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (r *Repository) Preferences(ctx context.Context, userID string) (json.RawMessage, error) {
	var preferences json.RawMessage
	if err := r.pool.QueryRow(ctx,
		`SELECT preferences FROM user_preferences WHERE user_id=$1`,
		userID).Scan(&preferences); err != nil {
		return nil, fmt.Errorf("load preferences: %w", err)
	}
	return preferences, nil
}

func (r *Repository) SavePreferences(
	ctx context.Context, userID string, preferences json.RawMessage,
) (json.RawMessage, error) {
	var saved json.RawMessage
	err := r.pool.QueryRow(ctx, `
		INSERT INTO user_preferences(user_id,preferences,updated_at)
		VALUES($1,$2,now())
		ON CONFLICT(user_id) DO UPDATE
		SET preferences=excluded.preferences,updated_at=excluded.updated_at
		RETURNING preferences`, userID, preferences).Scan(&saved)
	if err != nil {
		return nil, fmt.Errorf("save preferences: %w", err)
	}
	return saved, nil
}

func newSession(user User) (Session, []byte, time.Time, error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return Session{}, nil, time.Time{}, fmt.Errorf("generate session token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(random)
	hash := sha256.Sum256([]byte(token))
	expiresAt := time.Now().UTC().Add(sessionLifetime)
	return Session{
		Token: token, ExpiresAt: expiresAt.Format(time.RFC3339), User: user,
	}, hash[:], expiresAt, nil
}

func normalizeEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func nullableTrimmed(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if len(value) > 500 {
		value = value[:500]
	}
	return value
}
