package workspace

func mapWorkspaceOverview(snapshot *OverviewSnapshot) WorkspaceOverviewResponse {
	var result WorkspaceOverviewResponse
	access := snapshot.Access
	result.Project.ID = access.Project.ID
	result.Project.Slug = access.Project.Slug
	result.Project.Title = access.Project.Title
	result.Project.Status = access.Project.Status
	result.Project.Version = access.Project.Version
	result.Viewer.Scope = "member"
	if access.IsOwner {
		result.Viewer.Scope = "owner"
	} else if access.CanInitiate && access.CanManage {
		result.Viewer.Scope = "organization_initiator"
	}
	if access.Member != nil {
		membershipID := access.Member.ID
		result.Viewer.MembershipID = &membershipID
	}
	result.Viewer.Capabilities.CanManageWorkspace = access.CanManage
	result.Viewer.Capabilities.CanCreateTask = access.Project.Status == "in_progress" && (access.CanManage || (access.Member != nil && access.Member.TaskManagementGranted))
	result.TaskSummary = snapshot.Tasks
	result.TeamSummary.ActiveMemberCount = snapshot.ActiveMemberCount
	result.MyTasks = snapshot.MyTasks
	if len(result.MyTasks) > WorkspaceOverviewTaskPreviewLimit {
		result.MyTasks = result.MyTasks[:WorkspaceOverviewTaskPreviewLimit]
	}
	if result.MyTasks == nil {
		result.MyTasks = []OverviewTaskPreviewResponse{}
	}
	if snapshot.DiscussionReadable {
		result.RecentDiscussions = snapshot.Discussions
		if len(result.RecentDiscussions) > WorkspaceOverviewDiscussionPreviewLimit {
			result.RecentDiscussions = result.RecentDiscussions[:WorkspaceOverviewDiscussionPreviewLimit]
		}
		if result.RecentDiscussions == nil {
			result.RecentDiscussions = []OverviewDiscussionPreviewResponse{}
		}
	}
	result.RecentActivities = snapshot.Activities
	if len(result.RecentActivities) > WorkspaceOverviewActivityPreviewLimit {
		result.RecentActivities = result.RecentActivities[:WorkspaceOverviewActivityPreviewLimit]
	}
	if result.RecentActivities == nil {
		result.RecentActivities = []ActivityResponse{}
	}
	return result
}
