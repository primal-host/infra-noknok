package database

import (
	"context"
	"time"
)

// User represents a row in the users table.
type User struct {
	ID        int64     `json:"id"`
	DID       string    `json:"did"`
	Handle    string    `json:"handle"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Service represents a row in the services table.
type Service struct {
	ID          int64     `json:"id"`
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	URL         string    `json:"url"`
	IconURL     string    `json:"icon_url"`
	AdminRole   string    `json:"admin_role"`
	CreatedAt   time.Time `json:"created_at"`
}

// Grant represents a row in the grants table with joined user/service info.
type Grant struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"user_id"`
	ServiceID   int64     `json:"service_id"`
	Role        string    `json:"role"`
	GrantedBy   *int64    `json:"granted_by"`
	CreatedAt   time.Time `json:"created_at"`
	UserHandle  string    `json:"user_handle,omitempty"`
	ServiceName string    `json:"service_name,omitempty"`
}

// --- Users ---

func (db *DB) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT id, did, handle, role, created_at, updated_at
		FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.DID, &u.Handle, &u.Role, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (db *DB) GetUserByDID(ctx context.Context, did string) (*User, error) {
	var u User
	err := db.Pool.QueryRow(ctx, `
		SELECT id, did, handle, role, created_at, updated_at
		FROM users WHERE did = $1`, did).
		Scan(&u.ID, &u.DID, &u.Handle, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (db *DB) GetUserRole(ctx context.Context, did string) (string, error) {
	var role string
	err := db.Pool.QueryRow(ctx, `SELECT role FROM users WHERE did = $1`, did).Scan(&role)
	return role, err
}

func (db *DB) CreateUser(ctx context.Context, did, handle, role string) (*User, error) {
	var u User
	err := db.Pool.QueryRow(ctx, `
		INSERT INTO users (did, handle, role)
		VALUES ($1, $2, $3)
		RETURNING id, did, handle, role, created_at, updated_at`,
		did, handle, role).
		Scan(&u.ID, &u.DID, &u.Handle, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (db *DB) UpdateUserRole(ctx context.Context, id int64, role string) error {
	_, err := db.Pool.Exec(ctx, `
		UPDATE users SET role = $1, updated_at = now() WHERE id = $2`, role, id)
	return err
}

func (db *DB) DeleteUser(ctx context.Context, id int64) error {
	_, err := db.Pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	return err
}

func (db *DB) UserExists(ctx context.Context, did string) (bool, error) {
	var exists bool
	err := db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE did = $1)`, did).Scan(&exists)
	return exists, err
}

// --- Services ---

func (db *DB) ListServices(ctx context.Context) ([]Service, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT id, slug, name, description, url, COALESCE(icon_url, ''), admin_role, created_at
		FROM services ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var svcs []Service
	for rows.Next() {
		var s Service
		if err := rows.Scan(&s.ID, &s.Slug, &s.Name, &s.Description, &s.URL, &s.IconURL, &s.AdminRole, &s.CreatedAt); err != nil {
			return nil, err
		}
		svcs = append(svcs, s)
	}
	return svcs, rows.Err()
}

func (db *DB) ListServicesForUser(ctx context.Context, userID int64) ([]Service, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT s.id, s.slug, s.name, s.description, s.url, COALESCE(s.icon_url, ''), s.admin_role, s.created_at
		FROM services s
		JOIN grants g ON g.service_id = s.id
		WHERE g.user_id = $1
		ORDER BY s.name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var svcs []Service
	for rows.Next() {
		var s Service
		if err := rows.Scan(&s.ID, &s.Slug, &s.Name, &s.Description, &s.URL, &s.IconURL, &s.AdminRole, &s.CreatedAt); err != nil {
			return nil, err
		}
		svcs = append(svcs, s)
	}
	return svcs, rows.Err()
}

func (db *DB) CreateService(ctx context.Context, slug, name, description, url, iconURL, adminRole string) (*Service, error) {
	if adminRole == "" {
		adminRole = "admin"
	}
	var s Service
	err := db.Pool.QueryRow(ctx, `
		INSERT INTO services (slug, name, description, url, icon_url, admin_role)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, slug, name, description, url, COALESCE(icon_url, ''), admin_role, created_at`,
		slug, name, description, url, iconURL, adminRole).
		Scan(&s.ID, &s.Slug, &s.Name, &s.Description, &s.URL, &s.IconURL, &s.AdminRole, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (db *DB) UpdateService(ctx context.Context, id int64, name, description, url, iconURL, adminRole string) error {
	if adminRole == "" {
		adminRole = "admin"
	}
	_, err := db.Pool.Exec(ctx, `
		UPDATE services SET name = $1, description = $2, url = $3, icon_url = $4, admin_role = $5
		WHERE id = $6`, name, description, url, iconURL, adminRole, id)
	return err
}

func (db *DB) DeleteService(ctx context.Context, id int64) error {
	_, err := db.Pool.Exec(ctx, `DELETE FROM services WHERE id = $1`, id)
	return err
}

// --- Grants ---

func (db *DB) ListGrants(ctx context.Context) ([]Grant, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT g.id, g.user_id, g.service_id, g.role, g.granted_by, g.created_at,
		       u.handle, s.name
		FROM grants g
		JOIN users u ON u.id = g.user_id
		JOIN services s ON s.id = g.service_id
		ORDER BY u.handle, s.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var grants []Grant
	for rows.Next() {
		var g Grant
		if err := rows.Scan(&g.ID, &g.UserID, &g.ServiceID, &g.Role, &g.GrantedBy, &g.CreatedAt,
			&g.UserHandle, &g.ServiceName); err != nil {
			return nil, err
		}
		grants = append(grants, g)
	}
	return grants, rows.Err()
}

func (db *DB) CreateGrant(ctx context.Context, userID, serviceID, grantedBy int64, role string) (*Grant, error) {
	if role == "" {
		role = "user"
	}
	var g Grant
	err := db.Pool.QueryRow(ctx, `
		INSERT INTO grants (user_id, service_id, role, granted_by)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, service_id) DO UPDATE SET role = EXCLUDED.role
		RETURNING id, user_id, service_id, role, granted_by, created_at`,
		userID, serviceID, role, grantedBy).
		Scan(&g.ID, &g.UserID, &g.ServiceID, &g.Role, &g.GrantedBy, &g.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &g, nil
}

func (db *DB) DeleteGrant(ctx context.Context, id int64) error {
	_, err := db.Pool.Exec(ctx, `DELETE FROM grants WHERE id = $1`, id)
	return err
}

func (db *DB) DeleteGrantByUserService(ctx context.Context, userID, serviceID int64) error {
	_, err := db.Pool.Exec(ctx, `DELETE FROM grants WHERE user_id = $1 AND service_id = $2`, userID, serviceID)
	return err
}

// GetUserServiceRole returns the role a user has for a service whose URL
// contains the given host. For owner/admin users, returns the service's
// admin_role. For regular users, returns the grant's role.
func (db *DB) GetUserServiceRole(ctx context.Context, did, host string) (string, error) {
	// Match service by checking if the url contains the host.
	var userRole, grantRole, adminRole string
	err := db.Pool.QueryRow(ctx, `
		SELECT u.role,
		       COALESCE(g.role, ''),
		       COALESCE(s.admin_role, 'admin')
		FROM users u
		LEFT JOIN services s ON s.url LIKE '%' || $2 || '%'
		LEFT JOIN grants g ON g.user_id = u.id AND g.service_id = s.id
		WHERE u.did = $1
		LIMIT 1`, did, host).Scan(&userRole, &grantRole, &adminRole)
	if err != nil {
		return "", err
	}
	if userRole == "owner" || userRole == "admin" {
		return adminRole, nil
	}
	if grantRole != "" {
		return grantRole, nil
	}
	return "", nil
}

func (db *DB) GrantAllServices(ctx context.Context, userID, grantedBy int64) error {
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO grants (user_id, service_id, granted_by)
		SELECT $1, id, $2 FROM services
		ON CONFLICT (user_id, service_id) DO NOTHING`, userID, grantedBy)
	return err
}
