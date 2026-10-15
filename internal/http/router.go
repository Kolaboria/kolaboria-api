package http

import (
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/syahrulfahmi/kolaboria-api/config"
	_ "github.com/syahrulfahmi/kolaboria-api/docs"
	"github.com/syahrulfahmi/kolaboria-api/internal/http/middleware"
	"github.com/syahrulfahmi/kolaboria-api/internal/http/response"
	"github.com/syahrulfahmi/kolaboria-api/internal/infrastructure/jwt"
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/auth"
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/health"
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/master"
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/profile"
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/project"
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/workspace"
)

func NewRouter(
	cfg *config.Config,
	healthHandler *health.HealthHandler,
	jwtService jwt.JWTService,
	masterHandler *master.MasterHandler,
	authHandler *auth.AuthHandler,
	profileHandler *profile.ProfileHandler,
	projectHandler *project.ProjectHandler,
	workspaceHandlers ...*workspace.Handler,
) *gin.Engine {
	if cfg.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.HandleMethodNotAllowed = true

	// Apply global middlewares
	r.Use(middleware.TraceID())
	r.Use(middleware.RequestID())
	r.Use(middleware.Logger())
	r.Use(middleware.Recovery())
	r.Use(middleware.CORS(cfg.App.AllowOrigins))

	// Swagger UI
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Register routes
	health.RegisterHealthRoutes(r, healthHandler)
	v1 := r.Group("/api/v1")
	if authHandler != nil {
		auth.RegisterAuthRoutes(v1, authHandler, middleware.Auth(jwtService))
	}
	master.RegisterMasterRoutes(v1, masterHandler)
	if profileHandler != nil {
		profile.RegisterProfileRoutes(v1, profileHandler, middleware.Auth(jwtService))
	}
	if projectHandler != nil {
		project.RegisterProjectRoutes(v1, projectHandler, jwtService)
	}
	if len(workspaceHandlers) > 0 && workspaceHandlers[0] != nil {
		workspace.RegisterRoutes(v1, workspaceHandlers[0], jwtService)
	}
	r.NoRoute(func(c *gin.Context) { response.Error(c, 404, "Not Found") })
	r.NoMethod(func(c *gin.Context) { response.Error(c, 405, "Method Not Allowed") })

	return r
}
