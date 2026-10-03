package master

import "github.com/gin-gonic/gin"

func RegisterMasterRoutes(v1 gin.IRouter, handler *MasterHandler) {
	group := v1.Group("/master-data")
	group.GET("/contribution-roles", handler.ListContributionRoles)
	group.GET("/skills", handler.ListSkills)
	group.GET("/tools", handler.ListTools)
	group.GET("/locations/search", handler.SearchLocations)
	group.GET("/locations/:village_id", handler.GetLocationDetail)
}

