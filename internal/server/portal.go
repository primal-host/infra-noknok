package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"sync"

	"github.com/labstack/echo/v4"
	"github.com/primal-host/noknok/internal/session"
)

// Service represents a protected service shown in the portal.
type Service struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	URL         string `json:"url"`
	IconURL     string `json:"icon_url"`
}

var (
	services     []Service
	servicesOnce sync.Once
)

func loadServices() []Service {
	servicesOnce.Do(func() {
		data, err := os.ReadFile("services.json")
		if err != nil {
			slog.Warn("failed to load services.json", "error", err)
			return
		}
		if err := json.Unmarshal(data, &services); err != nil {
			slog.Warn("failed to parse services.json", "error", err)
		}
	})
	return services
}

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

	svcs := loadServices()
	return c.HTML(http.StatusOK, portalHTML(sess.Handle, svcs))
}

func portalHTML(handle string, svcs []Service) string {
	cards := ""
	for _, svc := range svcs {
		initial := "?"
		if len(svc.Name) > 0 {
			initial = string([]rune(svc.Name)[0])
		}
		cards += `
      <a href="` + svc.URL + `" class="card">
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
    <span>` + handle + `</span>
    <form method="POST" action="/logout" style="margin:0">
      <button class="logout" type="submit">Sign Out</button>
    </form>
  </div>
</div>
<div class="grid">` + cards + `
</div>
</body>
</html>`
}
