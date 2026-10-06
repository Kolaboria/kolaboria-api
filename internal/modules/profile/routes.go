package profile

import "github.com/gin-gonic/gin"

func RegisterProfileRoutes(v1 gin.IRouter, handler *ProfileHandler, authMiddleware gin.HandlerFunc) {
	group := v1.Group("/profiles")
	group.GET("/onboarding/status", authMiddleware, handler.CheckOnboardingStatus)
	group.POST("/onboarding", authMiddleware, handler.SubmitOnboarding)
	group.GET("/me", authMiddleware, handler.GetMyProfile)
	group.PATCH("/me", authMiddleware, handler.UpdateMyProfile)
	group.GET("/me/skills", authMiddleware, handler.ListMySkills)
	group.POST("/me/skills", authMiddleware, handler.AddMySkill)
	group.DELETE("/me/skills/:id", authMiddleware, handler.RemoveMySkill)
	group.PATCH("/me/skills/:id/primary", authMiddleware, handler.SetPrimaryMySkill)
	group.GET("/me/tools", authMiddleware, handler.ListMyTools)
	group.POST("/me/tools", authMiddleware, handler.AddMyTool)
	group.DELETE("/me/tools/:id", authMiddleware, handler.RemoveMyTool)
	group.GET("/me/careers", authMiddleware, handler.ListMyCareers)
	group.POST("/me/careers", authMiddleware, handler.CreateMyCareer)
	group.PUT("/me/careers/:id", authMiddleware, handler.UpdateMyCareer)
	group.DELETE("/me/careers/:id", authMiddleware, handler.DeleteMyCareer)
}
