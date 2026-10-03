package auth

import "github.com/gin-gonic/gin"

func RegisterAuthRoutes(v1 gin.IRouter, handler *AuthHandler, authMiddleware gin.HandlerFunc) {
	group := v1.Group("/auth")
	{
		group.POST("/register", handler.Register)
		group.POST("/login", handler.Login)
		group.POST("/logout", handler.Logout)
		group.POST("/refresh", handler.RefreshToken)
		group.GET("/verify-email", handler.VerifyEmail)
		group.POST("/resend-verification", handler.ResendVerification)
		group.POST("/forgot-password", handler.ForgotPassword)
		group.POST("/reset-password", handler.ResetPassword)
		group.POST("/google", handler.GoogleLogin)

		// Protected routes
		group.GET("/me", authMiddleware, handler.Me)
	}
}
