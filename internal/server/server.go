package server

import (
	"context"
	"log/slog"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/primal-host/noknok/internal/config"
	"github.com/primal-host/noknok/internal/database"
	"github.com/primal-host/noknok/internal/session"
)

// Server wraps the Echo instance and dependencies.
type Server struct {
	echo    *echo.Echo
	db      *database.DB
	sess    *session.Manager
	cfg     *config.Config
	addr    string
}

// New creates a configured Echo server.
func New(db *database.DB, sess *session.Manager, cfg *config.Config) *Server {
	s := &Server{
		echo: echo.New(),
		db:   db,
		sess: sess,
		cfg:  cfg,
		addr: cfg.ListenAddr,
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

	return s
}

// Start begins listening for HTTP requests.
func (s *Server) Start() error {
	slog.Info("server listening", "addr", s.addr)
	return s.echo.Start(s.addr)
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.echo.Shutdown(ctx)
}
