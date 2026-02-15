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
	ID        int64
	Token     string
	DID       string
	Handle    string
	Username  string
	GroupID   string
	UserID    int64
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
// If groupID is empty, a new group is created.
func (m *Manager) Create(ctx context.Context, userID int64, did, handle, groupID string) (*http.Cookie, error) {
	token, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	if groupID == "" {
		groupID, err = generateUUID()
		if err != nil {
			return nil, fmt.Errorf("generate group id: %w", err)
		}
	}

	// Look up username from users table.
	var username string
	_ = m.pool.QueryRow(ctx, `SELECT username FROM users WHERE id = $1`, userID).Scan(&username)

	expiresAt := time.Now().Add(m.ttl)
	_, err = m.pool.Exec(ctx, `
		INSERT INTO sessions (token, did, handle, username, group_id, user_id, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, token, did, handle, username, groupID, userID, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("insert session: %w", err)
	}

	// Update the identity's handle if it changed.
	_, err = m.pool.Exec(ctx, `
		UPDATE user_identities SET handle = $2 WHERE did = $1
	`, did, handle)
	if err != nil {
		slog.Warn("failed to update identity handle", "did", did, "error", err)
	}

	return m.makeCookie(token, expiresAt), nil
}

// Validate checks a session token and returns the session if valid.
func (m *Manager) Validate(ctx context.Context, token string) (*Session, error) {
	var s Session
	err := m.pool.QueryRow(ctx, `
		SELECT id, token, did, handle, username, COALESCE(group_id, ''), user_id, expires_at FROM sessions
		WHERE token = $1 AND expires_at > now()
	`, token).Scan(&s.ID, &s.Token, &s.DID, &s.Handle, &s.Username, &s.GroupID, &s.UserID, &s.ExpiresAt)
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

// ListGroup returns all non-expired sessions in a group, ordered by creation time.
func (m *Manager) ListGroup(ctx context.Context, groupID string) ([]Session, error) {
	if groupID == "" {
		return nil, nil
	}
	rows, err := m.pool.Query(ctx, `
		SELECT id, token, did, handle, username, group_id, user_id, expires_at FROM sessions
		WHERE group_id = $1 AND expires_at > now()
		ORDER BY created_at
	`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var s Session
		if err := rows.Scan(&s.ID, &s.Token, &s.DID, &s.Handle, &s.Username, &s.GroupID, &s.UserID, &s.ExpiresAt); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

// GroupHasDID checks if a DID already exists in a group and returns the session ID if so.
func (m *Manager) GroupHasDID(ctx context.Context, groupID, did string) (int64, string, bool) {
	if groupID == "" {
		return 0, "", false
	}
	var id int64
	var token string
	err := m.pool.QueryRow(ctx, `
		SELECT id, token FROM sessions
		WHERE group_id = $1 AND did = $2 AND expires_at > now()
	`, groupID, did).Scan(&id, &token)
	if err != nil {
		return 0, "", false
	}
	return id, token, true
}

// SwitchTo switches the active session within a group. Returns a cookie for the target session.
func (m *Manager) SwitchTo(ctx context.Context, groupID string, sessionID int64) (*http.Cookie, error) {
	var token string
	var expiresAt time.Time
	err := m.pool.QueryRow(ctx, `
		SELECT token, expires_at FROM sessions
		WHERE id = $1 AND group_id = $2 AND expires_at > now()
	`, sessionID, groupID).Scan(&token, &expiresAt)
	if err != nil {
		return nil, fmt.Errorf("session not found in group: %w", err)
	}
	return m.makeCookie(token, expiresAt), nil
}

// DestroyOne deletes one session from a group. If wasActive is true, returns a cookie
// for the next session in the group, or ClearCookie if none remain.
func (m *Manager) DestroyOne(ctx context.Context, groupID string, sessionID int64, wasActive bool) (*http.Cookie, error) {
	_, err := m.pool.Exec(ctx, `
		DELETE FROM sessions WHERE id = $1 AND group_id = $2
	`, sessionID, groupID)
	if err != nil {
		return nil, fmt.Errorf("delete session: %w", err)
	}

	if !wasActive {
		return nil, nil // no cookie change needed
	}

	// Find the next session in the group.
	var token string
	var expiresAt time.Time
	err = m.pool.QueryRow(ctx, `
		SELECT token, expires_at FROM sessions
		WHERE group_id = $1 AND expires_at > now()
		ORDER BY created_at LIMIT 1
	`, groupID).Scan(&token, &expiresAt)
	if err != nil {
		// No sessions left — clear cookie.
		return m.ClearCookie(), nil
	}
	return m.makeCookie(token, expiresAt), nil
}

// DestroyGroup deletes all sessions in a group.
func (m *Manager) DestroyGroup(ctx context.Context, groupID string) error {
	if groupID == "" {
		return nil
	}
	_, err := m.pool.Exec(ctx, `DELETE FROM sessions WHERE group_id = $1`, groupID)
	return err
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

func (m *Manager) makeCookie(token string, expiresAt time.Time) *http.Cookie {
	return &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		Domain:   m.cookieDomain,
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteLaxMode,
	}
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func generateUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	// Set version 4 and variant bits.
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
