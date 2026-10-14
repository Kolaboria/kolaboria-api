package project

import (
	"github.com/gin-gonic/gin"
	"github.com/syahrulfahmi/kolaboria-api/internal/http/middleware"
	"github.com/syahrulfahmi/kolaboria-api/internal/infrastructure/jwt"
)

func RegisterApplicationRoutes(v1 gin.IRouter, handler *ApplicationHandler, jwtService jwt.JWTService) {
	projects := v1.Group("/projects")
	projects.Use(middleware.Auth(jwtService))
	projects.GET("/:id/applicants/summary", handler.ApplicantSummary)
	projects.GET("/:id/applicants", handler.ListProjectApplicants)

	applications := v1.Group("/applications")
	applications.Use(middleware.Auth(jwtService))
	applications.GET("/my-applications/summary", handler.MyApplicationSummary)
	applications.GET("/my-applications", handler.ListMyApplications)
	applications.GET("/:id", handler.ApplicationDetail)
	applications.PATCH("/:id/review", handler.ReviewApplication)
	applications.POST("/:id/withdraw", handler.WithdrawApplication)
}
