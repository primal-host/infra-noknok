package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/primal-host/noknok/internal/config"
	"github.com/primal-host/noknok/internal/database"
	"github.com/primal-host/noknok/internal/session"
)

const redirectCookieName = "noknok_redirect"

// handleLoginPage renders the login form (handle only, no password).
func (s *Server) handleLoginPage(c echo.Context) error {
	redirect := c.QueryParam("redirect")
	errMsg := c.QueryParam("error")

	svcs, err := s.db.ListPublicServices(c.Request().Context())
	if err != nil {
		slog.Warn("login: failed to load public services", "error", err)
		svcs = nil
	}

	return c.HTML(http.StatusOK, loginHTML(redirect, errMsg, s.hasValidSession(c), svcs))
}

// handleLogin processes the login form — starts the OAuth flow.
func (s *Server) handleLogin(c echo.Context) error {
	handle := strings.TrimSpace(c.FormValue("handle"))
	redirect := c.FormValue("redirect")

	if handle == "" {
		return c.HTML(http.StatusOK, loginHTML(redirect, "Handle is required.", s.hasValidSession(c), nil))
	}

	// Default bare names to .bsky.social.
	if !strings.Contains(handle, ".") {
		handle += ".bsky.social"
	}

	// Store redirect URL in a cookie so we can use it after the OAuth callback.
	if redirect != "" && isAllowedRedirect(redirect, s.cfg) {
		secure := strings.HasPrefix(s.cfg.PublicURL, "https://")
		c.SetCookie(&http.Cookie{
			Name:     redirectCookieName,
			Value:    redirect,
			Path:     "/",
			MaxAge:   600, // 10 minutes
			HttpOnly: true,
			Secure:   secure,
			SameSite: http.SameSiteLaxMode,
		})
	}

	authURL, err := s.oauth.StartLogin(c.Request().Context(), handle)
	if err != nil {
		slog.Warn("OAuth start failed", "handle", handle, "error", err)
		return c.HTML(http.StatusOK, loginHTML(redirect, "Could not start login. Check your handle and try again.", s.hasValidSession(c), nil))
	}

	return c.Redirect(http.StatusFound, authURL)
}

// handleOAuthCallback processes the auth server redirect.
func (s *Server) handleOAuthCallback(c echo.Context) error {
	did, resolvedHandle, err := s.oauth.HandleCallback(c.Request().Context(), c.QueryParams())
	if err != nil {
		slog.Warn("OAuth callback failed", "error", err)
		return c.Redirect(http.StatusFound, s.cfg.PublicURL+"/login?error="+url.QueryEscape("Authentication failed. Please try again."))
	}

	// Check if user exists in the users table.
	exists, err := s.db.UserExists(c.Request().Context(), did)
	if err != nil {
		slog.Error("user lookup failed", "did", did, "error", err)
		return c.Redirect(http.StatusFound, s.cfg.PublicURL+"/login?error="+url.QueryEscape("Internal error. Please try again."))
	}
	if !exists {
		slog.Warn("unauthorized DID attempted login", "did", did, "handle", resolvedHandle)
		return c.Redirect(http.StatusFound, s.cfg.PublicURL+"/login?error="+url.QueryEscape("Access denied. You are not authorized."))
	}

	// Check for existing session group (adding identity to existing browser session).
	var groupID string
	if existing, err := c.Cookie(session.CookieName()); err == nil && existing.Value != "" {
		if existingSess, err := s.sess.Validate(c.Request().Context(), existing.Value); err == nil {
			groupID = existingSess.GroupID

			// If this DID already exists in the group, switch to it instead of creating a duplicate.
			if existingID, _, found := s.sess.GroupHasDID(c.Request().Context(), groupID, did); found {
				switchCookie, switchErr := s.sess.SwitchTo(c.Request().Context(), groupID, existingID)
				if switchErr != nil {
					slog.Warn("failed to switch to existing identity", "did", did, "error", switchErr)
				} else {
					c.SetCookie(switchCookie)
				}
				slog.Info("switched to existing identity in group", "did", did, "handle", resolvedHandle)
				dest := s.cfg.PublicURL + "/"
				if rc, err := c.Cookie(redirectCookieName); err == nil && rc.Value != "" {
					if isAllowedRedirect(rc.Value, s.cfg) {
						dest = rc.Value
					}
					c.SetCookie(&http.Cookie{Name: redirectCookieName, Value: "", Path: "/", MaxAge: -1})
				}
				return c.Redirect(http.StatusFound, dest)
			}
		}
	}

	// Create noknok session.
	cookie, err := s.sess.Create(c.Request().Context(), did, resolvedHandle, groupID)
	if err != nil {
		slog.Error("failed to create session", "error", err)
		return c.Redirect(http.StatusFound, s.cfg.PublicURL+"/login?error="+url.QueryEscape("Internal error. Please try again."))
	}
	c.SetCookie(cookie)

	slog.Info("login successful", "did", did, "handle", resolvedHandle)

	// Redirect to the stored destination or portal.
	dest := s.cfg.PublicURL + "/"
	if rc, err := c.Cookie(redirectCookieName); err == nil && rc.Value != "" {
		if isAllowedRedirect(rc.Value, s.cfg) {
			dest = rc.Value
		}
		// Clear the redirect cookie.
		c.SetCookie(&http.Cookie{
			Name:   redirectCookieName,
			Value:  "",
			Path:   "/",
			MaxAge: -1,
		})
	}
	return c.Redirect(http.StatusFound, dest)
}

// handleClientMetadata serves the OAuth client metadata document.
func (s *Server) handleClientMetadata(c echo.Context) error {
	return c.JSON(http.StatusOK, s.oauth.ClientMetadata())
}

// handleJWKS serves the public JSON Web Key Set.
func (s *Server) handleJWKS(c echo.Context) error {
	return c.JSON(http.StatusOK, s.oauth.PublicJWKS())
}

// isAllowedRedirect validates the redirect URL to prevent open redirect attacks.
func isAllowedRedirect(rawURL string, cfg *config.Config) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	domain := cfg.CookieDomain
	if strings.HasPrefix(domain, ".") {
		base := domain[1:]
		return u.Host == base || strings.HasSuffix(u.Host, domain)
	}
	return u.Host == domain
}

// hasValidSession returns true if the request has a valid session cookie.
func (s *Server) hasValidSession(c echo.Context) bool {
	cookie, err := c.Cookie(session.CookieName())
	if err != nil || cookie.Value == "" {
		return false
	}
	_, err = s.sess.Validate(c.Request().Context(), cookie.Value)
	return err == nil
}

func loginHTML(redirect, errMsg string, hasSession bool, svcs []database.Service) string {
	errorBlock := ""
	if errMsg != "" {
		errorBlock = `<div class="error">` + errMsg + `</div>`
	}

	redirectInput := ""
	if redirect != "" {
		redirectInput = `<input type="hidden" name="redirect" value="` + redirect + `">`
	}

	closeBtn := ""
	if hasSession {
		closeBtn = `<a href="/" class="close-btn" title="Cancel">&times;</a>`
	}

	// Build public service cards.
	serviceCards := ""
	for _, svc := range svcs {
		initial := "?"
		if len(svc.Name) > 0 {
			initial = string([]rune(svc.Name)[0])
		}
		faviconURL := strings.TrimRight(svc.URL, "/") + "/favicon.ico"
		desc := svc.Description
		if len([]rune(desc)) > 20 {
			desc = string([]rune(desc)[:20]) + "..."
		}
		serviceCards += `
      <a href="` + svc.URL + `" target="` + svc.Slug + `" class="card svc-card" rel="noopener">
        <div class="icon"><img src="` + faviconURL + `" onerror="this.style.display='none';this.nextSibling.style.display=''" style="width:28px;height:28px;border-radius:4px"><span style="display:none">` + initial + `</span></div>
        <div class="info">
          <h3>` + svc.Name + `</h3>
          <p>` + desc + `</p>
        </div>
      </a>`
	}

	// Only show service grid section if there are public services.
	serviceSection := ""
	if serviceCards != "" {
		serviceSection = fmt.Sprintf(`
<div class="grid">%s
</div>`, serviceCards)
	}

	return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>nokNok — Sign In</title>
<style>
  *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
  body {
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
    background: #0f172a;
    color: #e2e8f0;
    min-height: 100vh;
    padding: 2rem;
  }
  .login-card {
    background: #1e293b;
    border-radius: 12px;
    padding: 1.25rem;
    max-width: 800px;
    margin: 0 auto 1rem;
    display: flex;
    align-items: flex-start;
    gap: 1rem;
    position: relative;
  }
  .close-btn {
    position: absolute;
    top: 0.75rem;
    right: 0.75rem;
    background: none;
    border: 1.5px solid #475569;
    color: #64748b;
    font-size: 0.875rem;
    cursor: pointer;
    width: 1.75rem;
    height: 1.75rem;
    padding: 0;
    line-height: 1;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: color 0.15s, border-color 0.15s, background 0.15s;
    text-decoration: none;
  }
  .close-btn:hover { color: #fff; border-color: #f97316; background: #f97316; }
  .login-icon {
    width: 48px;
    height: 48px;
    background: #bbbbff;
    border-radius: 10px;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 1.25rem;
    font-weight: 700;
    color: #fff;
    flex-shrink: 0;
  }
  .login-body { flex: 1; min-width: 0; }
  .login-body h2 {
    font-size: 1rem;
    font-weight: 600;
    color: #f8fafc;
    margin-bottom: 0.125rem;
  }
  .error {
    background: #7f1d1d;
    color: #fca5a5;
    padding: 0.75rem 1rem;
    border-radius: 8px;
    font-size: 0.875rem;
    margin-bottom: 1rem;
  }
  input[type="text"] {
    width: 100%;
    padding: 0.625rem 0.75rem;
    background: #0f172a;
    border: 1px solid #334155;
    border-radius: 8px;
    color: #f8fafc;
    font-size: 0.9375rem;
    margin-bottom: 0.75rem;
    outline: none;
    transition: border-color 0.15s;
  }
  input[type="text"]:focus { border-color: #3b82f6; }
  input[type="text"]::placeholder { color: #475569; }
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
  .login-footer {
    text-align: center;
    margin-top: 0.75rem;
    font-size: 0.75rem;
    color: #475569;
  }
  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
    gap: 1rem;
    max-width: 800px;
    margin: 0 auto;
  }
  .svc-card {
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
  .svc-card:hover { background: #334155; transform: translateY(-2px); }
  .icon {
    width: 48px;
    height: 48px;
    background: #bbbbff;
    border-radius: 10px;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 1.25rem;
    font-weight: 700;
    color: #fff;
    flex-shrink: 0;
  }
  .info { flex: 1; min-width: 0; }
  .info h3 {
    font-size: 1rem;
    font-weight: 600;
    color: #f8fafc;
    margin-bottom: 0.125rem;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .info p {
    font-size: 0.8125rem;
    color: #94a3b8;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
</style>
</head>
<body>
<div class="login-card">
  ` + closeBtn + `
  <div class="login-icon">N</div>
  <div class="login-body">
    <h2>nokNok</h2>
    ` + errorBlock + `
    <form method="POST" action="/login">
      ` + redirectInput + `
      <input type="text" id="handle" name="handle" placeholder="you.bsky.social" autocomplete="username" autofocus required>
      <button type="submit">Sign in with Bluesky</button>
    </form>
    <p class="login-footer">You will be redirected to authorize at your provider</p>
  </div>
</div>
` + serviceSection + `
<script>
(function() {
  if (typeof BroadcastChannel === 'undefined') return;
  var ch = new BroadcastChannel('noknok_portal');
  ch.postMessage({ type: 'ping' });
  ch.onmessage = function(e) {
    if (e.data.type === 'pong') {
      ch.postMessage({ type: 'focus' });
      window.close();
    }
  };
  setTimeout(function() { ch.close(); }, 500);
})();
</script>
</body>
</html>`
}
