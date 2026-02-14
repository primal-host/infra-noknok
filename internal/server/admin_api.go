package server

import (
	"log/slog"
	"net/http"
	"regexp"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/primal-host/noknok/internal/database"
	"github.com/primal-host/noknok/internal/session"
)

var validUsername = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,39}$`)

const ctxKeyUser = "admin_user"

// requireAdmin validates the session and ensures the user is owner or admin.
func (s *Server) requireAdmin(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		cookie, err := c.Cookie(session.CookieName())
		if err != nil || cookie.Value == "" {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		}
		sess, err := s.sess.Validate(c.Request().Context(), cookie.Value)
		if err != nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid session"})
		}
		user, err := s.db.GetUserByDID(c.Request().Context(), sess.DID)
		if err != nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "user not found"})
		}
		if user.Role != "owner" && user.Role != "admin" {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "admin access required"})
		}
		c.Set(ctxKeyUser, user)
		return next(c)
	}
}

func adminUser(c echo.Context) *database.User {
	return c.Get(ctxKeyUser).(*database.User)
}

// --- Users ---

func (s *Server) handleListUsers(c echo.Context) error {
	users, err := s.db.ListUsers(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list users"})
	}
	if users == nil {
		users = []database.User{}
	}
	return c.JSON(http.StatusOK, users)
}

func (s *Server) handleCreateUser(c echo.Context) error {
	caller := adminUser(c)

	var req struct {
		Handle   string `json:"handle"`
		Role     string `json:"role"`
		Username string `json:"username"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if req.Handle == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "handle is required"})
	}
	if req.Role == "" {
		req.Role = "user"
	}

	// Admins can only create users, not other admins/owners.
	if caller.Role != "owner" && req.Role != "user" {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "only owners can assign admin/owner roles"})
	}
	if req.Role != "user" && req.Role != "admin" && req.Role != "owner" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid role"})
	}

	// Resolve handle to DID.
	did, resolvedHandle, err := s.oauth.ResolveHandle(c.Request().Context(), req.Handle)
	if err != nil {
		slog.Warn("handle resolution failed", "handle", req.Handle, "error", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "could not resolve handle"})
	}

	if req.Username != "" && !validUsername.MatchString(req.Username) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid username (alphanumeric, hyphens, underscores, 1-39 chars)"})
	}

	user, err := s.db.CreateUser(c.Request().Context(), did, resolvedHandle, req.Role, req.Username)
	if err != nil {
		slog.Warn("create user failed", "did", did, "error", err)
		return c.JSON(http.StatusConflict, map[string]string{"error": "user already exists"})
	}

	slog.Info("user created", "did", did, "handle", resolvedHandle, "role", req.Role, "by", caller.Handle)
	return c.JSON(http.StatusCreated, user)
}

func (s *Server) handleUpdateUserRole(c echo.Context) error {
	caller := adminUser(c)

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid user ID"})
	}

	var req struct {
		Role string `json:"role"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if req.Role != "user" && req.Role != "admin" && req.Role != "owner" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid role"})
	}

	// Admins can only set role to "user".
	if caller.Role != "owner" && req.Role != "user" {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "only owners can assign admin/owner roles"})
	}

	// Prevent changing the seed owner's role.
	users, err := s.db.ListUsers(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
	}
	for _, u := range users {
		if u.ID == id && u.DID == s.cfg.OwnerDID {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "cannot change seed owner role"})
		}
	}

	if err := s.db.UpdateUserRole(c.Request().Context(), id, req.Role); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update role"})
	}

	slog.Info("user role updated", "user_id", id, "role", req.Role, "by", caller.Handle)
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleUpdateUserUsername(c echo.Context) error {
	caller := adminUser(c)

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid user ID"})
	}

	var req struct {
		Username string `json:"username"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if req.Username != "" && !validUsername.MatchString(req.Username) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid username (alphanumeric, hyphens, underscores, 1-39 chars)"})
	}

	if err := s.db.UpdateUserUsername(c.Request().Context(), id, req.Username); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update username"})
	}

	slog.Info("user username updated", "user_id", id, "username", req.Username, "by", caller.Handle)
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleDeleteUser(c echo.Context) error {
	caller := adminUser(c)

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid user ID"})
	}

	// No self-deletion.
	if id == caller.ID {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "cannot delete yourself"})
	}

	// Protect seed owner.
	users, err := s.db.ListUsers(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
	}
	for _, u := range users {
		if u.ID == id {
			if u.DID == s.cfg.OwnerDID {
				return c.JSON(http.StatusForbidden, map[string]string{"error": "cannot delete seed owner"})
			}
			// Admins can only delete users, not other admins/owners.
			if caller.Role != "owner" && u.Role != "user" {
				return c.JSON(http.StatusForbidden, map[string]string{"error": "only owners can delete admins/owners"})
			}
			break
		}
	}

	if err := s.db.DeleteUser(c.Request().Context(), id); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to delete user"})
	}

	slog.Info("user deleted", "user_id", id, "by", caller.Handle)
	return c.NoContent(http.StatusNoContent)
}

// --- Services ---

func (s *Server) handleListServicesAdmin(c echo.Context) error {
	svcs, err := s.db.ListServices(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list services"})
	}
	if svcs == nil {
		svcs = []database.Service{}
	}
	return c.JSON(http.StatusOK, svcs)
}

func (s *Server) handleCreateService(c echo.Context) error {
	caller := adminUser(c)

	var req struct {
		Slug        string `json:"slug"`
		Name        string `json:"name"`
		Description string `json:"description"`
		URL         string `json:"url"`
		IconURL     string `json:"icon_url"`
		AdminRole   string `json:"admin_role"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if req.Slug == "" || req.Name == "" || req.URL == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "slug, name, and url are required"})
	}

	svc, err := s.db.CreateService(c.Request().Context(), req.Slug, req.Name, req.Description, req.URL, req.IconURL, req.AdminRole)
	if err != nil {
		return c.JSON(http.StatusConflict, map[string]string{"error": "service slug already exists"})
	}

	slog.Info("service created", "slug", req.Slug, "by", caller.Handle)
	return c.JSON(http.StatusCreated, svc)
}

func (s *Server) handleUpdateService(c echo.Context) error {
	caller := adminUser(c)

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid service ID"})
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		URL         string `json:"url"`
		IconURL     string `json:"icon_url"`
		AdminRole   string `json:"admin_role"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if req.Name == "" || req.URL == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "name and url are required"})
	}

	if err := s.db.UpdateService(c.Request().Context(), id, req.Name, req.Description, req.URL, req.IconURL, req.AdminRole); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update service"})
	}

	slog.Info("service updated", "service_id", id, "by", caller.Handle)
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleDeleteService(c echo.Context) error {
	caller := adminUser(c)

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid service ID"})
	}

	if err := s.db.DeleteService(c.Request().Context(), id); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to delete service"})
	}

	slog.Info("service deleted", "service_id", id, "by", caller.Handle)
	return c.NoContent(http.StatusNoContent)
}

// --- Grants ---

func (s *Server) handleListGrants(c echo.Context) error {
	grants, err := s.db.ListGrants(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list grants"})
	}
	if grants == nil {
		grants = []database.Grant{}
	}
	return c.JSON(http.StatusOK, grants)
}

func (s *Server) handleCreateGrant(c echo.Context) error {
	caller := adminUser(c)

	var req struct {
		UserID    int64  `json:"user_id"`
		ServiceID int64  `json:"service_id"`
		Role      string `json:"role"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if req.UserID == 0 || req.ServiceID == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "user_id and service_id are required"})
	}

	grant, err := s.db.CreateGrant(c.Request().Context(), req.UserID, req.ServiceID, caller.ID, req.Role)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create grant"})
	}

	slog.Info("grant created", "user_id", req.UserID, "service_id", req.ServiceID, "by", caller.Handle)
	return c.JSON(http.StatusCreated, grant)
}

func (s *Server) handleDeleteGrant(c echo.Context) error {
	caller := adminUser(c)

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid grant ID"})
	}

	if err := s.db.DeleteGrant(c.Request().Context(), id); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to delete grant"})
	}

	slog.Info("grant deleted", "grant_id", id, "by", caller.Handle)
	return c.NoContent(http.StatusNoContent)
}
