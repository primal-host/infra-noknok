package database

import (
	"context"
	"encoding/json"
	"fmt"
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

// SeedOwner ensures the owner user exists with the given DID.
func (db *DB) SeedOwner(ctx context.Context, did string) error {
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO users (did, handle, role)
		VALUES ($1, '', 'owner')
		ON CONFLICT (did) DO UPDATE SET role = 'owner', updated_at = now()
	`, did)
	return err
}

func (db *DB) bootstrap(ctx context.Context) error {
	_, err := db.Pool.Exec(ctx, schema)
	return err
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
			ON CONFLICT (slug) DO UPDATE SET admin_role = EXCLUDED.admin_role`,
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
		SELECT u.id, s.id, u.id
		FROM users u, services s
		WHERE u.did = $1
		ON CONFLICT (user_id, service_id) DO NOTHING`, ownerDID)
	return err
}
