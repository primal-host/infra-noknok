package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const cookieName = "noknok_session"

// Session represents an active user session.
type Session struct {
	Token     string
	DID       string
	Handle    string
	ExpiresAt time.Time
}

// Manager handles session creation, validation, and cleanup.
type Manager struct {
	pool         *pgxpool.Pool
	ttl          time.Duration
	cookieDomain string
	secure       bool
	stopCleanup  chan struct{}
}

// NewManager creates a session manager.
func NewManager(pool *pgxpool.Pool, ttl time.Duration, cookieDomain string, secure bool) *Manager {
	return &Manager{
		pool:         pool,
		ttl:          ttl,
		cookieDomain: cookieDomain,
		secure:       secure,
		stopCleanup:  make(chan struct{}),
	}
}

// Create inserts a new session and returns a cookie to set on the response.
func (m *Manager) Create(ctx context.Context, did, handle string) (*http.Cookie, error) {
	token, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	expiresAt := time.Now().Add(m.ttl)
	_, err = m.pool.Exec(ctx, `
		INSERT INTO sessions (token, did, handle, expires_at)
		VALUES ($1, $2, $3, $4)
	`, token, did, handle, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("insert session: %w", err)
	}

	// Update the user's handle if it changed.
	_, err = m.pool.Exec(ctx, `
		UPDATE users SET handle = $2, updated_at = now() WHERE did = $1
	`, did, handle)
	if err != nil {
		slog.Warn("failed to update user handle", "did", did, "error", err)
	}

	return &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		Domain:   m.cookieDomain,
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteLaxMode,
	}, nil
}

// Validate checks a session token and returns the session if valid.
func (m *Manager) Validate(ctx context.Context, token string) (*Session, error) {
	var s Session
	err := m.pool.QueryRow(ctx, `
		SELECT token, did, handle, expires_at FROM sessions
		WHERE token = $1 AND expires_at > now()
	`, token).Scan(&s.Token, &s.DID, &s.Handle, &s.ExpiresAt)
	if err != nil {
		return nil, err
	}

	// Update last_seen asynchronously.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = m.pool.Exec(ctx, `UPDATE sessions SET last_seen = now() WHERE token = $1`, token)
	}()

	return &s, nil
}

// Destroy removes a session (logout).
func (m *Manager) Destroy(ctx context.Context, token string) error {
	_, err := m.pool.Exec(ctx, `DELETE FROM sessions WHERE token = $1`, token)
	return err
}

// ClearCookie returns a cookie that clears the session cookie.
func (m *Manager) ClearCookie() *http.Cookie {
	return &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		Domain:   m.cookieDomain,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteLaxMode,
	}
}

// CookieName returns the session cookie name.
func CookieName() string {
	return cookieName
}

// StartCleanup starts a background goroutine that deletes expired sessions.
func (m *Manager) StartCleanup() {
	go func() {
		ticker := time.NewTicker(15 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				result, err := m.pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at <= now()`)
				cancel()
				if err != nil {
					slog.Error("session cleanup failed", "error", err)
				} else if result.RowsAffected() > 0 {
					slog.Info("cleaned up expired sessions", "count", result.RowsAffected())
				}
			case <-m.stopCleanup:
				return
			}
		}
	}()
}

// StopCleanup signals the cleanup goroutine to stop.
func (m *Manager) StopCleanup() {
	close(m.stopCleanup)
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
