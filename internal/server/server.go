package server

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/primal-host/noknok/internal/atproto"
	"github.com/primal-host/noknok/internal/config"
	"github.com/primal-host/noknok/internal/database"
	"github.com/primal-host/noknok/internal/session"
)

// Server wraps the Echo instance and dependencies.
type Server struct {
	echo       *echo.Echo
	db         *database.DB
	sess       *session.Manager
	cfg        *config.Config
	oauth      *atproto.OAuthClient
	addr       string
	healthMu   sync.RWMutex
	healthData map[int64]bool
	healthStop chan struct{}
}

// New creates a configured Echo server.
func New(db *database.DB, sess *session.Manager, cfg *config.Config, oauth *atproto.OAuthClient) *Server {
	s := &Server{
		echo:  echo.New(),
		db:    db,
		sess:  sess,
		cfg:   cfg,
		oauth: oauth,
		addr:  cfg.ListenAddr,
	}

	s.echo.HideBanner = true
	s.echo.HidePort = true

	s.echo.Use(middleware.Recover())
	s.echo.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogStatus: true,
		LogURI:    true,
		LogMethod: true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			slog.Info("request",
				"method", v.Method,
				"uri", v.URI,
				"status", v.Status,
			)
			return nil
		},
	}))

	s.registerRoutes()
	s.startHealthPoller()

	return s
}

// Start begins listening for HTTP requests.
func (s *Server) Start() error {
	slog.Info("server listening", "addr", s.addr)
	return s.echo.Start(s.addr)
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	close(s.healthStop)
	return s.echo.Shutdown(ctx)
}

// startHealthPoller runs service health checks every 60 seconds in the background.
func (s *Server) startHealthPoller() {
	s.healthStop = make(chan struct{})
	go func() {
		s.refreshHealth()
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.refreshHealth()
			case <-s.healthStop:
				return
			}
		}
	}()
}

func (s *Server) refreshHealth() {
	svcs, err := s.db.ListServices(context.Background())
	if err != nil {
		slog.Error("health poller: failed to list services", "error", err)
		return
	}
	health := s.checkServicesHealth(svcs)
	s.healthMu.Lock()
	s.healthData = health
	s.healthMu.Unlock()
}

func (s *Server) cachedHealth() map[int64]bool {
	s.healthMu.RLock()
	defer s.healthMu.RUnlock()
	m := make(map[int64]bool, len(s.healthData))
	for k, v := range s.healthData {
		m[k] = v
	}
	return m
}
