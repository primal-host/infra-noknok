package server

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/primal-host/noknok/internal/session"
)

// handleHealth returns 200 if the server is running.
func (s *Server) handleHealth(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// handleAuth is the Traefik forwardAuth endpoint.
// Valid session → 200 with X-User-DID and X-User-Handle headers.
// Authorization header present → 200 (let backend validate the token).
// No/invalid session → 302 redirect to login page.
func (s *Server) handleAuth(c echo.Context) error {
	host := c.Request().Header.Get("X-Forwarded-Host")

	// Check service status — disabled blocks all, public allows all.
	if host != "" {
		svc, _ := s.db.GetServiceByHost(c.Request().Context(), host)
		if svc != nil && !svc.Enabled {
			accept := c.Request().Header.Get("X-Forwarded-Accept")
			if accept == "" {
				accept = c.Request().Header.Get("Accept")
			}
			if strings.Contains(accept, "text/html") {
				return c.Redirect(http.StatusFound, s.cfg.PublicURL+"/")
			}
			return c.NoContent(http.StatusServiceUnavailable)
		}
		if svc != nil && svc.Public {
			return c.NoContent(http.StatusOK)
		}
	}

	cookie, err := c.Cookie(session.CookieName())
	if err == nil && cookie.Value != "" {
		sess, err := s.sess.Validate(c.Request().Context(), cookie.Value)
		if err == nil {
			// Check if user is owner/admin (full access) or has a grant for this service.
			if host != "" {
				role, roleErr := s.db.GetUserServiceRole(c.Request().Context(), sess.DID, host)
				if roleErr != nil || role == "" {
					// User has no grant for this service — deny access.
					// Redirect browser to portal so they see what they can access.
					accept := c.Request().Header.Get("X-Forwarded-Accept")
					if accept == "" {
						accept = c.Request().Header.Get("Accept")
					}
					if strings.Contains(accept, "text/html") {
						return c.Redirect(http.StatusFound, s.cfg.PublicURL+"/")
					}
					return c.NoContent(http.StatusForbidden)
				}
				c.Response().Header().Set("X-User-Role", role)
			}

			c.Response().Header().Set("X-User-DID", sess.DID)
			c.Response().Header().Set("X-User-Handle", sess.Handle)
			if sess.Username != "" {
				c.Response().Header().Set("X-WEBAUTH-USER", sess.Username)
			}

			return c.NoContent(http.StatusOK)
		}
	}

	// Pass through requests with an Authorization header (e.g. PATs, API tokens)
	// so the backend service can validate them itself.
	if c.Request().Header.Get("X-Forwarded-Authorization") != "" ||
		c.Request().Header.Get("Authorization") != "" {
		return c.NoContent(http.StatusOK)
	}

	// Non-browser clients (git, curl, API) get 401 so they can retry with
	// credentials. The backend (e.g. Gitea) will issue its own WWW-Authenticate
	// challenge once it receives the request.
	accept := c.Request().Header.Get("X-Forwarded-Accept")
	if accept == "" {
		accept = c.Request().Header.Get("Accept")
	}
	if !strings.Contains(accept, "text/html") {
		return c.NoContent(http.StatusUnauthorized)
	}

	// Build redirect URL from forwarded headers.
	scheme := c.Request().Header.Get("X-Forwarded-Proto")
	if scheme == "" {
		scheme = "https"
	}
	if host == "" {
		host = c.Request().Header.Get("X-Forwarded-Host")
	}
	uri := c.Request().Header.Get("X-Forwarded-Uri")

	redirectTarget := ""
	if host != "" {
		redirectTarget = fmt.Sprintf("%s://%s%s", scheme, host, uri)
	}

	loginURL := fmt.Sprintf("%s/login", s.cfg.PublicURL)
	if redirectTarget != "" {
		loginURL += "?redirect=" + url.QueryEscape(redirectTarget)
	}

	return c.Redirect(http.StatusFound, loginURL)
}

// handleLogout destroys the entire session group and redirects to login.
func (s *Server) handleLogout(c echo.Context) error {
	cookie, err := c.Cookie(session.CookieName())
	if err == nil && cookie.Value != "" {
		sess, err := s.sess.Validate(c.Request().Context(), cookie.Value)
		if err == nil && sess.GroupID != "" {
			_ = s.sess.DestroyGroup(c.Request().Context(), sess.GroupID)
		} else {
			_ = s.sess.Destroy(c.Request().Context(), cookie.Value)
		}
	}
	c.SetCookie(s.sess.ClearCookie())
	return c.Redirect(http.StatusFound, s.cfg.PublicURL+"/login")
}
