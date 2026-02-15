package database

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DB wraps a pgx connection pool.
type DB struct {
	Pool *pgxpool.Pool
}

// Open creates a connection pool and bootstraps the schema.
func Open(ctx context.Context, dsn string) (*DB, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	db := &DB{Pool: pool}
	if err := db.bootstrap(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("bootstrap: %w", err)
	}
	return db, nil
}

// Close shuts down the connection pool.
func (db *DB) Close() {
	db.Pool.Close()
}

// SeedOwner ensures the owner user exists with the given DID and username.
// Creates both a user record and a primary identity in user_identities.
func (db *DB) SeedOwner(ctx context.Context, did, username string) error {
	// Check if this DID already has an identity.
	var userID int64
	err := db.Pool.QueryRow(ctx,
		`SELECT user_id FROM user_identities WHERE did = $1`, did).Scan(&userID)
	if err == nil {
		// Identity exists — update user role and username.
		_, err = db.Pool.Exec(ctx, `
			UPDATE users SET role = 'owner',
				username = CASE WHEN $2 != '' THEN $2 ELSE username END,
				updated_at = now()
			WHERE id = $1`, userID, username)
		return err
	}

	// Create new user + identity.
	err = db.Pool.QueryRow(ctx, `
		INSERT INTO users (role, username)
		VALUES ('owner', $1)
		RETURNING id`, username).Scan(&userID)
	if err != nil {
		return fmt.Errorf("create owner user: %w", err)
	}

	_, err = db.Pool.Exec(ctx, `
		INSERT INTO user_identities (user_id, did, is_primary)
		VALUES ($1, $2, true)`, userID, did)
	if err != nil {
		return fmt.Errorf("create owner identity: %w", err)
	}
	return nil
}

func (db *DB) bootstrap(ctx context.Context) error {
	if _, err := db.Pool.Exec(ctx, schema); err != nil {
		return err
	}
	return db.migrateIdentities(ctx)
}

// migrateIdentities moves did/handle from users to user_identities (one-time).
func (db *DB) migrateIdentities(ctx context.Context) error {
	// Check if users.did column still exists (pre-migration state).
	var colExists bool
	err := db.Pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'users' AND column_name = 'did'
		)`).Scan(&colExists)
	if err != nil {
		return fmt.Errorf("check users.did column: %w", err)
	}
	if !colExists {
		return nil // Already migrated.
	}

	// Migrate rows that aren't already in user_identities.
	result, err := db.Pool.Exec(ctx, `
		INSERT INTO user_identities (user_id, did, handle, is_primary)
		SELECT id, did, handle, true FROM users
		WHERE did != '' AND NOT EXISTS (
			SELECT 1 FROM user_identities WHERE user_identities.did = users.did
		)`)
	if err != nil {
		return fmt.Errorf("migrate identities: %w", err)
	}
	if result.RowsAffected() > 0 {
		slog.Info("migrated user identities", "count", result.RowsAffected())
	}

	// Backfill sessions.user_id from user_identities.
	_, err = db.Pool.Exec(ctx, `
		UPDATE sessions SET user_id = ui.user_id
		FROM user_identities ui
		WHERE sessions.did = ui.did AND sessions.user_id = 0`)
	if err != nil {
		return fmt.Errorf("backfill sessions.user_id: %w", err)
	}

	// Drop did/handle columns from users.
	_, err = db.Pool.Exec(ctx, `
		ALTER TABLE users DROP COLUMN IF EXISTS did;
		ALTER TABLE users DROP COLUMN IF EXISTS handle`)
	if err != nil {
		return fmt.Errorf("drop users.did/handle: %w", err)
	}

	slog.Info("identity migration complete")
	return nil
}

// SeedServices reads a JSON file of services and upserts them into the database.
func (db *DB) SeedServices(ctx context.Context, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	var svcs []struct {
		Slug        string `json:"slug"`
		Name        string `json:"name"`
		Description string `json:"description"`
		URL         string `json:"url"`
		IconURL     string `json:"icon_url"`
		AdminRole   string `json:"admin_role"`
	}
	if err := json.Unmarshal(data, &svcs); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}

	for _, s := range svcs {
		if s.AdminRole == "" {
			s.AdminRole = "admin"
		}
		_, err := db.Pool.Exec(ctx, `
			INSERT INTO services (slug, name, description, url, icon_url, admin_role)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (slug) DO UPDATE SET
				name = EXCLUDED.name,
				description = EXCLUDED.description,
				url = EXCLUDED.url,
				icon_url = EXCLUDED.icon_url,
				admin_role = EXCLUDED.admin_role`,
			s.Slug, s.Name, s.Description, s.URL, s.IconURL, s.AdminRole)
		if err != nil {
			return fmt.Errorf("seed service %s: %w", s.Slug, err)
		}
	}
	return nil
}

// GrantOwnerAllServices grants the owner access to every service.
func (db *DB) GrantOwnerAllServices(ctx context.Context, ownerDID string) error {
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO grants (user_id, service_id, granted_by)
		SELECT ui.user_id, s.id, ui.user_id
		FROM user_identities ui, services s
		WHERE ui.did = $1
		ON CONFLICT (user_id, service_id) DO NOTHING`, ownerDID)
	return err
}
