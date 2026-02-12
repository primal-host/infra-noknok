package server

func (s *Server) registerRoutes() {
	s.echo.GET("/health", s.handleHealth)
	s.echo.GET("/auth", s.handleAuth)
	s.echo.GET("/login", s.handleLoginPage)
	s.echo.POST("/login", s.handleLogin)
	s.echo.POST("/logout", s.handleLogout)
	s.echo.GET("/", s.handlePortal)
}
