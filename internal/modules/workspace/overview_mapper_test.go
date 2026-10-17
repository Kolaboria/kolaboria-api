package workspace

import (
	"testing"

	"github.com/google/uuid"
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/project"
)

func TestMapWorkspaceOverviewPreservesDiscussionAccessAndPreviewLimits(t *testing.T) {
	memberID := uuid.New()
	actorID := uuid.New()
	projectID := uuid.New()
	projectRecord := project.Project{ID: projectID, Slug: "sample", Title: "Sample", Status: project.ProjectStatusInProgress, Version: 3}
	access := &WorkspaceProjectAccess{Project: projectRecord, Member: &project.ProjectMember{ID: memberID, UserID: actorID}, ActiveMember: true}
	snapshot := &OverviewSnapshot{
		Access:             access,
		Tasks:              OverviewTaskSummaryResponse{TodoTasks: 1, InProgressTasks: 1, ReviewTasks: 1, DoneTasks: 2, ClosedTasks: 1, TotalTasks: 6, OutstandingTasks: 3},
		ActiveMemberCount:  4,
		MyTasks:            make([]OverviewTaskPreviewResponse, 7),
		Discussions:        make([]OverviewDiscussionPreviewResponse, 4),
		DiscussionReadable: true,
		Activities:         make([]ActivityResponse, 5),
	}
	got := mapWorkspaceOverview(snapshot)
	if got.Project.ID != projectID || got.Viewer.MembershipID == nil || *got.Viewer.MembershipID != memberID {
		t.Fatalf("project/viewer identity was not mapped: %#v", got)
	}
	if got.TaskSummary.TotalTasks != 6 || got.TaskSummary.OutstandingTasks != 3 || got.TeamSummary.ActiveMemberCount != 4 {
		t.Fatalf("summary was not preserved: %#v", got)
	}
	if len(got.MyTasks) != WorkspaceOverviewTaskPreviewLimit || len(got.RecentDiscussions) != WorkspaceOverviewDiscussionPreviewLimit || len(got.RecentActivities) != WorkspaceOverviewActivityPreviewLimit {
		t.Fatalf("previews exceeded their public limits: tasks=%d discussions=%d activities=%d", len(got.MyTasks), len(got.RecentDiscussions), len(got.RecentActivities))
	}
}

func TestMapWorkspaceOverviewKeepsUnreadableDiscussionDistinctFromEmpty(t *testing.T) {
	access := &WorkspaceProjectAccess{Project: project.Project{ID: uuid.New(), Slug: "sample", Status: project.ProjectStatusInProgress}}
	got := mapWorkspaceOverview(&OverviewSnapshot{Access: access, DiscussionReadable: false})
	if got.RecentDiscussions != nil {
		t.Fatalf("unreadable discussions must map to null, got %#v", got.RecentDiscussions)
	}
	access.ActiveMember = true
	got = mapWorkspaceOverview(&OverviewSnapshot{Access: access, DiscussionReadable: true})
	if got.RecentDiscussions == nil || len(got.RecentDiscussions) != 0 {
		t.Fatalf("readable empty discussions must map to an empty array, got %#v", got.RecentDiscussions)
	}
}
