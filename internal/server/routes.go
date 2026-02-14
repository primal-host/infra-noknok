package server

func (s *Server) registerRoutes() {
	s.echo.GET("/health", s.handleHealth)
	s.echo.GET("/auth", s.handleAuth)
	s.echo.GET("/login", s.handleLoginPage)
	s.echo.POST("/login", s.handleLogin)
	s.echo.POST("/logout", s.handleLogout)
	s.echo.GET("/", s.handlePortal)

	// OAuth endpoints.
	s.echo.GET("/oauth/callback", s.handleOAuthCallback)
	s.echo.GET("/.well-known/oauth-client-metadata", s.handleClientMetadata)
	s.echo.GET("/oauth/jwks.json", s.handleJWKS)

	// Admin API (protected by requireAdmin middleware).
	admin := s.echo.Group("/admin/api", s.requireAdmin)
	admin.GET("/users", s.handleListUsers)
	admin.POST("/users", s.handleCreateUser)
	admin.PUT("/users/:id/role", s.handleUpdateUserRole)
	admin.PUT("/users/:id/username", s.handleUpdateUserUsername)
	admin.DELETE("/users/:id", s.handleDeleteUser)
	admin.GET("/services", s.handleListServicesAdmin)
	admin.POST("/services", s.handleCreateService)
	admin.PUT("/services/:id", s.handleUpdateService)
	admin.DELETE("/services/:id", s.handleDeleteService)
	admin.GET("/grants", s.handleListGrants)
	admin.POST("/grants", s.handleCreateGrant)
	admin.DELETE("/grants/:id", s.handleDeleteGrant)
}
