package project

import (
	"github.com/gin-gonic/gin"
	"github.com/syahrulfahmi/kolaboria-api/internal/http/middleware"
	"github.com/syahrulfahmi/kolaboria-api/internal/infrastructure/jwt"
)

func RegisterProjectRoutes(v1 gin.IRouter, handler *ProjectHandler, jwtService jwt.JWTService) {
	group := v1.Group("/projects")
	group.GET("", handler.ListPublic)
	group.GET("/my-projects", middleware.Auth(jwtService), handler.ListMine)
	group.GET("/slug/:slug", middleware.OptionalAuth(jwtService), handler.GetBySlug)
	group.GET("/:id", middleware.OptionalAuth(jwtService), handler.GetByID)
	group.POST("", middleware.Auth(jwtService), handler.CreateDraft)
	group.PUT("/:id", middleware.Auth(jwtService), handler.UpdateDefinition)
	group.POST("/:id/publish", middleware.Auth(jwtService), handler.Publish)
}
