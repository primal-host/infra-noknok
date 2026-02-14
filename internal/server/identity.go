package server

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/primal-host/noknok/internal/session"
)

// handleSwitchIdentity switches the active identity within the session group.
func (s *Server) handleSwitchIdentity(c echo.Context) error {
	cookie, err := c.Cookie(session.CookieName())
	if err != nil || cookie.Value == "" {
		return c.Redirect(http.StatusFound, s.cfg.PublicURL+"/login")
	}

	sess, err := s.sess.Validate(c.Request().Context(), cookie.Value)
	if err != nil {
		return c.Redirect(http.StatusFound, s.cfg.PublicURL+"/login")
	}

	targetID, err := strconv.ParseInt(c.FormValue("id"), 10, 64)
	if err != nil {
		return c.Redirect(http.StatusFound, s.cfg.PublicURL+"/")
	}

	newCookie, err := s.sess.SwitchTo(c.Request().Context(), sess.GroupID, targetID)
	if err != nil {
		return c.Redirect(http.StatusFound, s.cfg.PublicURL+"/")
	}

	c.SetCookie(newCookie)
	return c.Redirect(http.StatusFound, s.cfg.PublicURL+"/")
}

// handleLogoutOne logs out a single identity from the session group.
func (s *Server) handleLogoutOne(c echo.Context) error {
	cookie, err := c.Cookie(session.CookieName())
	if err != nil || cookie.Value == "" {
		return c.Redirect(http.StatusFound, s.cfg.PublicURL+"/login")
	}

	sess, err := s.sess.Validate(c.Request().Context(), cookie.Value)
	if err != nil {
		return c.Redirect(http.StatusFound, s.cfg.PublicURL+"/login")
	}

	targetID, err := strconv.ParseInt(c.FormValue("id"), 10, 64)
	if err != nil {
		return c.Redirect(http.StatusFound, s.cfg.PublicURL+"/")
	}

	wasActive := targetID == sess.ID
	newCookie, err := s.sess.DestroyOne(c.Request().Context(), sess.GroupID, targetID, wasActive)
	if err != nil {
		return c.Redirect(http.StatusFound, s.cfg.PublicURL+"/")
	}

	if newCookie != nil {
		c.SetCookie(newCookie)
	}

	// If no sessions remain, redirect to login.
	if wasActive && newCookie != nil && newCookie.MaxAge == -1 {
		return c.Redirect(http.StatusFound, s.cfg.PublicURL+"/login")
	}

	return c.Redirect(http.StatusFound, s.cfg.PublicURL+"/")
}

// handleListIdentities returns all identities in the current session group as JSON.
func (s *Server) handleListIdentities(c echo.Context) error {
	cookie, err := c.Cookie(session.CookieName())
	if err != nil || cookie.Value == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
	}

	sess, err := s.sess.Validate(c.Request().Context(), cookie.Value)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid session"})
	}

	group, err := s.sess.ListGroup(c.Request().Context(), sess.GroupID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list group"})
	}

	type identity struct {
		ID     int64  `json:"id"`
		DID    string `json:"did"`
		Handle string `json:"handle"`
		Active bool   `json:"active"`
	}

	result := make([]identity, 0, len(group))
	for _, g := range group {
		result = append(result, identity{
			ID:     g.ID,
			DID:    g.DID,
			Handle: g.Handle,
			Active: g.Token == sess.Token,
		})
	}

	return c.JSON(http.StatusOK, result)
}
