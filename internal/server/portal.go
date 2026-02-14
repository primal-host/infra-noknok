package server

import (
	"fmt"
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

	// Load session group for identity dropdown.
	var group []session.Session
	if sess.GroupID != "" {
		group, _ = s.sess.ListGroup(ctx, sess.GroupID)
	}
	if len(group) == 0 {
		// Legacy session or error — show as single identity.
		group = []session.Session{*sess}
	}

	// Check if ?admin is in the URL (works with ?admin, ?admin=, ?admin=1).
	_, adminOpen := c.QueryParams()["admin"]
	adminOpen = adminOpen && isAdmin
	adminTab := c.QueryParam("tab")
	if adminTab == "" {
		adminTab = "users"
	}

	return c.HTML(http.StatusOK, portalHTML(sess, group, svcs, isAdmin, user.Role, adminOpen, adminTab))
}

type identityInfo struct {
	ID     int64
	Handle string
	Active bool
}

func portalHTML(active *session.Session, group []session.Session, svcs []database.Service, isAdmin bool, role string, adminOpen bool, adminTab string) string {
	cards := ""
	for _, svc := range svcs {
		initial := "?"
		if len(svc.Name) > 0 {
			initial = string([]rune(svc.Name)[0])
		}
		disabledDot := ""
		if !svc.Enabled {
			disabledDot = `<div class="disabled-dot"></div>`
		}
		cards += `
      <a href="` + svc.URL + `" target="` + svc.Slug + `" rel="noopener" class="card" data-svc-id="` + fmt.Sprintf("%d", svc.ID) + `" onclick="return openService(this)">
        <div class="icon">` + initial + `</div>
        <div class="info">
          <h3>` + svc.Name + `</h3>
          <p>` + svc.Description + `</p>
        </div>
        ` + disabledDot + `
        <div class="grant-dot" style="display:none"></div>
        <div class="traffic-light" style="display:none"><div class="tl-dot tl-enabled"></div><div class="tl-dot tl-public"></div><div class="tl-dot tl-health"></div></div>
      </a>`
	}

	if cards == "" {
		cards = `<p class="empty">No services configured.</p>`
	}

	// Build identity list.
	identities := make([]identityInfo, 0, len(group))
	for _, s := range group {
		identities = append(identities, identityInfo{
			ID:     s.ID,
			Handle: s.Handle,
			Active: s.Token == active.Token,
		})
	}

	// Identity dropdown items.
	identityItems := ""
	for _, id := range identities {
		if id.Active {
			identityItems += `<div class="dd-item dd-active">` + id.Handle + `</div>`
		} else {
			identityItems += fmt.Sprintf(`<form method="POST" action="/switch" style="margin:0"><input type="hidden" name="id" value="%d"><button type="submit" class="dd-item dd-btn">%s</button></form>`, id.ID, id.Handle)
		}
	}

	// Logout items.
	logoutItems := ""
	for _, id := range identities {
		logoutItems += fmt.Sprintf(`<form method="POST" action="/logout/one" style="margin:0" onsubmit="closeAllTracked()"><input type="hidden" name="id" value="%d"><button type="submit" class="dd-item dd-btn dd-danger">Log out %s</button></form>`, id.ID, id.Handle)
	}

	// Admin item in dropdown (only for admin/owner).
	adminItem := ""
	if isAdmin {
		adminItem = `
      <div class="dd-sep"></div>
      <div class="dd-section">
        <a href="/?admin" class="dd-add">Admin</a>
      </div>`
	}

	adminHTML := ""
	if isAdmin {
		adminHTML = adminPanelHTML(role, adminOpen, adminTab)
	}

	return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>nokNok — Portal</title>
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
    gap: 0.5rem;
    font-size: 0.875rem;
    color: #94a3b8;
    position: relative;
  }
  .dd-trigger {
    background: #334155;
    color: #e2e8f0;
    border: none;
    padding: 0.375rem 0.75rem;
    border-radius: 6px;
    font-size: 0.8125rem;
    cursor: pointer;
    transition: background 0.15s;
    display: flex;
    align-items: center;
    gap: 0.375rem;
  }
  .dd-trigger:hover { background: #475569; }
  .dd-arrow { font-size: 0.625rem; opacity: 0.7; }
  .dd-menu {
    display: none;
    position: absolute;
    top: calc(100% + 0.375rem);
    right: 0;
    background: #1e293b;
    border: 1px solid #334155;
    border-radius: 8px;
    min-width: 240px;
    box-shadow: 0 8px 24px rgba(0,0,0,0.4);
    z-index: 100;
    overflow: hidden;
  }
  .dd-menu.open { display: block; }
  .dd-section { padding: 0.25rem 0; }
  .dd-sep { border-top: 1px solid #334155; margin: 0; }
  .dd-item {
    display: block;
    width: 100%;
    padding: 0.5rem 0.75rem;
    font-size: 0.8125rem;
    color: #e2e8f0;
    text-align: left;
  }
  .dd-active {
    color: #3b82f6;
    font-weight: 500;
  }
  .dd-btn {
    background: none;
    border: none;
    cursor: pointer;
    transition: background 0.15s;
    font-family: inherit;
  }
  .dd-btn:hover { background: #334155; }
  .dd-danger { color: #f87171; }
  .dd-danger:hover { background: #7f1d1d; }
  .dd-add {
    color: #94a3b8;
    text-decoration: none;
    display: block;
    padding: 0.5rem 0.75rem;
    font-size: 0.8125rem;
    transition: background 0.15s;
  }
  .dd-add:hover { background: #334155; color: #e2e8f0; }
  .dd-logout-all {
    display: block;
    width: 100%;
    padding: 0.5rem 0.75rem;
    font-size: 0.8125rem;
    color: #f87171;
    background: none;
    border: none;
    cursor: pointer;
    text-align: left;
    font-family: inherit;
    transition: background 0.15s;
  }
  .dd-logout-all:hover { background: #7f1d1d; }
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
    position: relative;
  }
  .card:hover { background: #334155; transform: translateY(-2px); }
  .disabled-dot {
    position: absolute;
    top: 0.5rem;
    right: 0.5rem;
    width: 1rem;
    height: 1rem;
    border-radius: 4px;
    background: #ef4444;
  }
  .grant-dot {
    position: absolute;
    top: 0.5rem;
    right: 0.5rem;
    width: 1.25rem;
    height: 1.25rem;
    border-radius: 5px;
    cursor: pointer;
    transition: background 0.15s;
  }
  .grant-dot.granted { background: #22c55e; }
  .grant-dot.granted:hover { background: #16a34a; }
  .grant-dot.revoked { background: #475569; }
  .grant-dot.revoked:hover { background: #64748b; }
  .traffic-light {
    position: absolute;
    right: 0.5rem;
    top: 50%;
    transform: translateY(-50%);
    display: flex;
    flex-direction: column;
    gap: 3px;
  }
  .tl-dot {
    width: 1rem;
    height: 1rem;
    border-radius: 4px;
    background: #475569;
    cursor: pointer;
    transition: background 0.15s;
  }
  .tl-dot.tl-off { background: #475569; }
  .tl-dot.tl-red { background: #ef4444; }
  .tl-dot.tl-yellow { background: #eab308; }
  .tl-dot.tl-green { background: #22c55e; }
  .tl-dot.tl-health { cursor: default; }
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
  <h1>nokNok</h1>
  <div class="user">
    <button class="dd-trigger" onclick="toggleDropdown(event)">
      ` + active.Handle + ` <span class="dd-arrow">&#9660;</span>
    </button>
    <div class="dd-menu" id="identity-menu">
      <div class="dd-section">
        ` + identityItems + `
      </div>
      <div class="dd-sep"></div>
      <div class="dd-section">
        <a href="/login" class="dd-add">+ New sign-in...</a>
      </div>
      ` + adminItem + `
      <div class="dd-sep"></div>
      <div class="dd-section">
        ` + logoutItems + `
        <form method="POST" action="/logout" style="margin:0" onsubmit="closeAllTracked()">
          <button type="submit" class="dd-logout-all">Log out all</button>
        </form>
      </div>
    </div>
  </div>
</div>
` + adminHTML + `
<div class="grid">` + cards + `
</div>
<script>
var openWindows = {};
function openService(el) {
  var w = window.open(el.href, el.target);
  if (w) openWindows[el.target] = w;
  return false;
}
function closeTrackedWindow(slug) {
  if (openWindows[slug]) {
    try { openWindows[slug].close(); } catch(e) {}
    delete openWindows[slug];
  }
}
function closeAllTracked() {
  for (var name in openWindows) {
    if (openWindows.hasOwnProperty(name)) {
      try { openWindows[name].close(); } catch(e) {}
    }
  }
  openWindows = {};
}
function toggleDropdown(e) {
  e.stopPropagation();
  document.getElementById('identity-menu').classList.toggle('open');
}
document.addEventListener('click', function(e) {
  var menu = document.getElementById('identity-menu');
  if (!menu.contains(e.target)) menu.classList.remove('open');
});
document.addEventListener('keydown', function(e) {
  if (e.key === 'Escape') document.getElementById('identity-menu').classList.remove('open');
});
// Duplicate-tab detection via BroadcastChannel.
// The first portal tab claims "primary". Any subsequent portal tab
// that arrives (e.g. from a forwardAuth deny redirect) asks the
// primary to focus and then closes itself.
(function() {
  if (typeof BroadcastChannel === 'undefined') return;
  var ch = new BroadcastChannel('noknok_portal');
  var isPrimary = false;
  // Ask if a primary exists.
  ch.postMessage({ type: 'ping' });
  // If no pong within 200ms, claim primary.
  var timer = setTimeout(function() {
    isPrimary = true;
  }, 200);
  ch.onmessage = function(e) {
    if (e.data.type === 'ping' && isPrimary) {
      ch.postMessage({ type: 'pong' });
    } else if (e.data.type === 'pong' && !isPrimary) {
      clearTimeout(timer);
      ch.postMessage({ type: 'focus' });
      window.close();
    } else if (e.data.type === 'focus' && isPrimary) {
      window.focus();
      window.location.reload();
    }
  };
})();
// Reload on tab focus to refresh grants and service cards.
// Only if the tab was hidden for more than 5 seconds, to avoid
// reloading during quick tab switches.
(function() {
  var hiddenAt = 0;
  document.addEventListener('visibilitychange', function() {
    if (document.hidden) {
      hiddenAt = Date.now();
    } else if (hiddenAt && (Date.now() - hiddenAt) > 5000) {
      window.location.reload();
    }
  });
})();
</script>
</body>
</html>`
}
