package server

import (
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/primal-host/noknok/internal/database"
	"github.com/primal-host/noknok/internal/session"
)

// handlePortal renders the service catalog page (requires valid session).
func (s *Server) handlePortal(c echo.Context) error {
	cookie, err := c.Cookie(session.CookieName())
	if err != nil || cookie.Value == "" {
		return c.Redirect(http.StatusFound, s.cfg.PublicURL+"/login")
	}

	sess, err := s.sess.Validate(c.Request().Context(), cookie.Value)
	if err != nil {
		return c.Redirect(http.StatusFound, s.cfg.PublicURL+"/login")
	}

	ctx := c.Request().Context()

	user, err := s.db.GetUserByDID(ctx, sess.DID)
	if err != nil {
		slog.Warn("portal: user lookup failed", "did", sess.DID, "error", err)
		return c.Redirect(http.StatusFound, s.cfg.PublicURL+"/login")
	}

	isAdmin := user.Role == "owner" || user.Role == "admin"

	var svcs []database.Service
	if isAdmin {
		svcs, err = s.db.ListServices(ctx)
	} else {
		svcs, err = s.db.ListServicesForUser(ctx, user.ID)
	}
	if err != nil {
		slog.Error("portal: failed to load services", "error", err)
		svcs = nil
	}

	return c.HTML(http.StatusOK, portalHTML(sess.Handle, svcs, isAdmin, user.Role))
}

func portalHTML(handle string, svcs []database.Service, isAdmin bool, role string) string {
	cards := ""
	for _, svc := range svcs {
		initial := "?"
		if len(svc.Name) > 0 {
			initial = string([]rune(svc.Name)[0])
		}
		cards += `
      <a href="` + svc.URL + `" target="_blank" rel="noopener" class="card">
        <div class="icon">` + initial + `</div>
        <div class="info">
          <h3>` + svc.Name + `</h3>
          <p>` + svc.Description + `</p>
        </div>
      </a>`
	}

	if cards == "" {
		cards = `<p class="empty">No services configured.</p>`
	}

	// Username display: clickable for admins/owners, plain for users.
	handleHTML := `<span>` + handle + `</span>`
	if isAdmin {
		handleHTML = `<span style="cursor:pointer;text-decoration:underline;text-decoration-style:dotted;text-underline-offset:3px" onclick="openAdmin()">` + handle + `</span>`
	}

	adminHTML := ""
	if isAdmin {
		adminHTML = adminPanelHTML(role)
	}

	return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>noknok — Portal</title>
<style>
  *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
  body {
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
    background: #0f172a;
    color: #e2e8f0;
    min-height: 100vh;
    padding: 2rem;
  }
  .header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    max-width: 800px;
    margin: 0 auto 2rem;
  }
  h1 { font-size: 1.5rem; color: #f8fafc; }
  .user {
    display: flex;
    align-items: center;
    gap: 1rem;
    font-size: 0.875rem;
    color: #94a3b8;
  }
  .logout {
    background: #334155;
    color: #e2e8f0;
    border: none;
    padding: 0.375rem 0.75rem;
    border-radius: 6px;
    font-size: 0.8125rem;
    cursor: pointer;
    transition: background 0.15s;
  }
  .logout:hover { background: #475569; }
  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
    gap: 1rem;
    max-width: 800px;
    margin: 0 auto;
  }
  .card {
    display: flex;
    align-items: center;
    gap: 1rem;
    background: #1e293b;
    border-radius: 12px;
    padding: 1.25rem;
    text-decoration: none;
    color: inherit;
    transition: background 0.15s, transform 0.1s;
  }
  .card:hover { background: #334155; transform: translateY(-2px); }
  .icon {
    width: 48px;
    height: 48px;
    background: #3b82f6;
    border-radius: 10px;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 1.25rem;
    font-weight: 700;
    color: #fff;
    flex-shrink: 0;
  }
  .info h3 {
    font-size: 1rem;
    font-weight: 600;
    color: #f8fafc;
    margin-bottom: 0.125rem;
  }
  .info p {
    font-size: 0.8125rem;
    color: #94a3b8;
  }
  .empty {
    color: #475569;
    text-align: center;
    grid-column: 1 / -1;
    padding: 3rem;
  }
</style>
</head>
<body>
<div class="header">
  <h1>noknok</h1>
  <div class="user">
    ` + handleHTML + `
    <form method="POST" action="/logout" style="margin:0">
      <button class="logout" type="submit">Sign Out</button>
    </form>
  </div>
</div>
<div class="grid">` + cards + `
</div>
` + adminHTML + `
</body>
</html>`
}
