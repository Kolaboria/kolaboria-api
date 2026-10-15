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
	group.POST("/publish", middleware.Auth(jwtService), handler.CreateAndPublish)
	group.POST("", middleware.Auth(jwtService), handler.CreateDraft)
	group.PUT("/:id", middleware.Auth(jwtService), handler.UpdateDefinition)
	group.PATCH("/:id/status", middleware.Auth(jwtService), handler.ChangeStatus)
	group.DELETE("/:id", middleware.Auth(jwtService), handler.DeleteDraft)
	group.POST("/:id/publish", middleware.Auth(jwtService), handler.Publish)
	group.POST("/:id/apply", middleware.Auth(jwtService), handler.Apply)
	if handler.applications != nil {
		RegisterApplicationRoutes(v1, handler.applications, jwtService)
	}
}
