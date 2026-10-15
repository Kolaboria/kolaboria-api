package workspace

import (
	"testing"

	"github.com/google/uuid"
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/project"
)

func TestDiscussionReadAndWriteRequireWorkspaceMembership(t *testing.T) {
	if !canReadWorkspaceDiscussions(true, false) || !canReadWorkspaceDiscussions(false, true) {
		t.Fatal("active member and owner should be able to read discussions")
	}
	if canReadWorkspaceDiscussions(false, false) {
		t.Fatal("non-member must not read project discussions")
	}
	if !canWriteWorkspaceDiscussions(project.ProjectStatusInProgress, true, false) {
		t.Fatal("active member should be able to discuss while the project is in progress")
	}
	if !canWriteWorkspaceDiscussions(project.ProjectStatusInProgress, false, true) {
		t.Fatal("project owner should be able to discuss while the project is in progress")
	}
	for _, status := range []project.ProjectStatus{project.ProjectStatusOpen, project.ProjectStatusCompleted, project.ProjectStatusArchived} {
		if canWriteWorkspaceDiscussions(status, true, false) {
			t.Errorf("discussion mutation must be disabled for project status %q", status)
		}
	}
}

func TestDiscussionEditAndDeletePermissionsAreSeparate(t *testing.T) {
	authorID, ownerID, otherID := uuid.New(), uuid.New(), uuid.New()
	if !canEditDiscussion(authorID, authorID) || canEditDiscussion(authorID, ownerID) {
		t.Fatal("only the author may edit")
	}
	if !canDeleteDiscussion(authorID, ownerID, ownerID) || !canDeleteDiscussion(authorID, authorID, ownerID) || canDeleteDiscussion(authorID, otherID, ownerID) {
		t.Fatal("only the author or project owner may delete")
	}
}

func TestDiscussionRepliesCannotNestUnderAnotherReply(t *testing.T) {
	root := &DiscussionReply{ID: uuid.New()}
	childID := uuid.New()
	child := &DiscussionReply{ID: childID, ParentReplyID: &root.ID}
	if err := validateDiscussionParent(root); err != nil {
		t.Fatalf("root reply should accept one follow-up level: %v", err)
	}
	if err := validateDiscussionParent(child); err == nil {
		t.Fatal("follow-up replies must not be nested further")
	}
}
