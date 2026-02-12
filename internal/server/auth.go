package server

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/labstack/echo/v4"
	"github.com/primal-host/noknok/internal/session"
)

// handleHealth returns 200 if the server is running.
func (s *Server) handleHealth(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// handleAuth is the Traefik forwardAuth endpoint.
// Valid session → 200 with X-User-DID and X-User-Handle headers.
// No/invalid session → 302 redirect to login page.
func (s *Server) handleAuth(c echo.Context) error {
	cookie, err := c.Cookie(session.CookieName())
	if err == nil && cookie.Value != "" {
		sess, err := s.sess.Validate(c.Request().Context(), cookie.Value)
		if err == nil {
			c.Response().Header().Set("X-User-DID", sess.DID)
			c.Response().Header().Set("X-User-Handle", sess.Handle)
			return c.NoContent(http.StatusOK)
		}
	}

	// Build redirect URL from forwarded headers.
	scheme := c.Request().Header.Get("X-Forwarded-Proto")
	if scheme == "" {
		scheme = "https"
	}
	host := c.Request().Header.Get("X-Forwarded-Host")
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

// handleLogout destroys the session and redirects to login.
func (s *Server) handleLogout(c echo.Context) error {
	cookie, err := c.Cookie(session.CookieName())
	if err == nil && cookie.Value != "" {
		_ = s.sess.Destroy(c.Request().Context(), cookie.Value)
	}
	c.SetCookie(s.sess.ClearCookie())
	return c.Redirect(http.StatusFound, s.cfg.PublicURL+"/login")
}
