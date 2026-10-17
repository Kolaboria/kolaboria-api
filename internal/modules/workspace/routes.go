package workspace

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/syahrulfahmi/kolaboria-api/internal/http/middleware"
	"github.com/syahrulfahmi/kolaboria-api/internal/infrastructure/jwt"
)

func RegisterRoutes(v1 gin.IRouter, handler *Handler, jwtService jwt.JWTService) {
	group := v1.Group("/workspaces")
	group.GET("/:projectSlug/info", middleware.Auth(jwtService), handler.GetProjectInfo)
	if handler.overview != nil {
		admission := handler.overviewAdmission
		if admission == nil {
			admission = NewOverviewAdmission(true, 64, 2*time.Second)
		}
		group.GET("/:projectSlug/overview", middleware.Auth(jwtService), admission.Middleware(), handler.GetWorkspaceOverview)
	}
	if handler.completion != nil {
		group.POST("/:projectSlug/completion", middleware.Auth(jwtService), handler.CompleteWorkspaceProject)
	}
	if handler.activities != nil {
		group.GET("/:projectSlug/activities", middleware.Auth(jwtService), handler.ListWorkspaceActivities)
	}
	if handler.team != nil {
		group.GET("/:projectSlug/members", middleware.Auth(jwtService), handler.ListTeamMembers)
	}
	if handler.tasks != nil {
		tasks := group.Group("/:projectSlug/tasks", middleware.Auth(jwtService))
		tasks.GET("", handler.ListWorkspaceTasks)
		tasks.POST("", handler.CreateWorkspaceTask)
		tasks.GET("/:taskID", handler.GetWorkspaceTask)
		tasks.PATCH("/:taskID", handler.UpdateWorkspaceTask)
		tasks.POST("/:taskID/transitions", handler.TransitionWorkspaceTask)
		tasks.DELETE("/:taskID", handler.DeleteWorkspaceTask)
		tasks.GET("/:taskID/comments", handler.ListWorkspaceTaskComments)
		tasks.POST("/:taskID/comments", handler.CreateWorkspaceTaskComment)
		group.PATCH("/:projectSlug/members/:memberID/task-access", middleware.Auth(jwtService), handler.UpdateWorkspaceTaskAccess)
	}
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
