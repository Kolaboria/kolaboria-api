package workspace

import (
	"github.com/gin-gonic/gin"
	"github.com/syahrulfahmi/kolaboria-api/internal/http/middleware"
	"github.com/syahrulfahmi/kolaboria-api/internal/infrastructure/jwt"
)

func RegisterRoutes(v1 gin.IRouter, handler *Handler, jwtService jwt.JWTService) {
	group := v1.Group("/workspaces")
	group.GET("/:projectSlug/info", middleware.Auth(jwtService), handler.GetProjectInfo)
	if handler.discussions == nil {
		return
	}
	discussions := group.Group("/:projectSlug/discussions", middleware.Auth(jwtService))
	discussions.GET("", handler.ListDiscussions)
	discussions.POST("", handler.CreateDiscussion)
	discussions.GET("/:discussionID", handler.GetDiscussion)
	discussions.PATCH("/:discussionID", handler.UpdateDiscussion)
	discussions.DELETE("/:discussionID", handler.DeleteDiscussion)
	discussions.GET("/:discussionID/replies", handler.ListDiscussionReplies)
	discussions.POST("/:discussionID/replies", handler.CreateDiscussionReply)
	discussions.PATCH("/:discussionID/replies/:replyID", handler.UpdateDiscussionReply)
	discussions.DELETE("/:discussionID/replies/:replyID", handler.DeleteDiscussionReply)
}
