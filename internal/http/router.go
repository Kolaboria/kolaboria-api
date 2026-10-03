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
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/health"
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/master"
)

func NewRouter(
	cfg *config.Config,
	healthHandler *health.HealthHandler,
	jwtService jwt.JWTService,
	masterHandler *master.MasterHandler,
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
	master.RegisterMasterRoutes(v1, masterHandler)
	r.NoRoute(func(c *gin.Context) { response.Error(c, 404, "Not Found") })
	r.NoMethod(func(c *gin.Context) { response.Error(c, 405, "Method Not Allowed") })

	return r
}
