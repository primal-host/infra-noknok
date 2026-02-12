package server

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/primal-host/noknok/internal/atproto"
	"github.com/primal-host/noknok/internal/config"
)

// handleLoginPage renders the login form.
func (s *Server) handleLoginPage(c echo.Context) error {
	redirect := c.QueryParam("redirect")
	errMsg := c.QueryParam("error")
	return c.HTML(http.StatusOK, loginHTML(redirect, errMsg))
}

// handleLogin processes the login form submission.
func (s *Server) handleLogin(c echo.Context) error {
	handle := strings.TrimSpace(c.FormValue("handle"))
	password := c.FormValue("password")
	redirect := c.FormValue("redirect")

	if handle == "" || password == "" {
		return c.HTML(http.StatusOK, loginHTML(redirect, "Handle and app password are required."))
	}

	// Authenticate via AT Protocol.
	did, resolvedHandle, err := atproto.Authenticate(handle, password)
	if err != nil {
		slog.Warn("login failed", "handle", handle, "error", err)
		return c.HTML(http.StatusOK, loginHTML(redirect, "Authentication failed. Check your handle and app password."))
	}

	// Phase 1: Only the owner DID is allowed.
	if did != s.cfg.OwnerDID {
		slog.Warn("unauthorized DID attempted login", "did", did, "handle", resolvedHandle)
		return c.HTML(http.StatusOK, loginHTML(redirect, "Access denied. You are not authorized."))
	}

	// Create session.
	cookie, err := s.sess.Create(c.Request().Context(), did, resolvedHandle)
	if err != nil {
		slog.Error("failed to create session", "error", err)
		return c.HTML(http.StatusOK, loginHTML(redirect, "Internal error. Please try again."))
	}
	c.SetCookie(cookie)

	slog.Info("login successful", "did", did, "handle", resolvedHandle)

	// Redirect to the original destination or portal.
	dest := s.cfg.PublicURL + "/"
	if redirect != "" && isAllowedRedirect(redirect, s.cfg) {
		dest = redirect
	}
	return c.Redirect(http.StatusFound, dest)
}

// isAllowedRedirect validates the redirect URL to prevent open redirect attacks.
func isAllowedRedirect(rawURL string, cfg *config.Config) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	// Must be absolute HTTP(S) URL.
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	// Host must be under the cookie domain.
	domain := cfg.CookieDomain
	if strings.HasPrefix(domain, ".") {
		// e.g. ".primal.host" — allow "primal.host" and "*.primal.host"
		base := domain[1:]
		return u.Host == base || strings.HasSuffix(u.Host, domain)
	}
	return u.Host == domain
}

func loginHTML(redirect, errMsg string) string {
	errorBlock := ""
	if errMsg != "" {
		errorBlock = `<div class="error">` + errMsg + `</div>`
	}

	redirectInput := ""
	if redirect != "" {
		redirectInput = `<input type="hidden" name="redirect" value="` + redirect + `">`
	}

	return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>noknok — Sign In</title>
<style>
  *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
  body {
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
    background: #0f172a;
    color: #e2e8f0;
    min-height: 100vh;
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .card {
    background: #1e293b;
    border-radius: 12px;
    padding: 2.5rem;
    width: 100%;
    max-width: 400px;
    box-shadow: 0 4px 24px rgba(0,0,0,0.3);
  }
  h1 {
    font-size: 1.5rem;
    font-weight: 600;
    margin-bottom: 0.25rem;
    color: #f8fafc;
  }
  .subtitle {
    color: #94a3b8;
    font-size: 0.875rem;
    margin-bottom: 1.5rem;
  }
  .error {
    background: #7f1d1d;
    color: #fca5a5;
    padding: 0.75rem 1rem;
    border-radius: 8px;
    font-size: 0.875rem;
    margin-bottom: 1rem;
  }
  label {
    display: block;
    font-size: 0.875rem;
    font-weight: 500;
    color: #cbd5e1;
    margin-bottom: 0.375rem;
  }
  input[type="text"], input[type="password"] {
    width: 100%;
    padding: 0.625rem 0.75rem;
    background: #0f172a;
    border: 1px solid #334155;
    border-radius: 8px;
    color: #f8fafc;
    font-size: 0.9375rem;
    margin-bottom: 1rem;
    outline: none;
    transition: border-color 0.15s;
  }
  input[type="text"]:focus, input[type="password"]:focus {
    border-color: #3b82f6;
  }
  input[type="text"]::placeholder, input[type="password"]::placeholder {
    color: #475569;
  }
  button {
    width: 100%;
    padding: 0.625rem;
    background: #3b82f6;
    color: #fff;
    border: none;
    border-radius: 8px;
    font-size: 0.9375rem;
    font-weight: 500;
    cursor: pointer;
    transition: background 0.15s;
  }
  button:hover { background: #2563eb; }
  .footer {
    text-align: center;
    margin-top: 1.5rem;
    font-size: 0.75rem;
    color: #475569;
  }
</style>
</head>
<body>
<div class="card">
  <h1>noknok</h1>
  <p class="subtitle">Sign in with your AT Protocol identity</p>
  ` + errorBlock + `
  <form method="POST" action="/login">
    ` + redirectInput + `
    <label for="handle">Handle</label>
    <input type="text" id="handle" name="handle" placeholder="you.bsky.social" autocomplete="username" autofocus required>
    <label for="password">App Password</label>
    <input type="password" id="password" name="password" placeholder="xxxx-xxxx-xxxx-xxxx" autocomplete="current-password" required>
    <button type="submit">Sign In</button>
  </form>
  <p class="footer">Authentication via AT Protocol (Bluesky)</p>
</div>
</body>
</html>`
}
