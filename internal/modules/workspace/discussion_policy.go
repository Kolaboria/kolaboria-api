package workspace

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/project"
	sharedErrors "github.com/syahrulfahmi/kolaboria-api/internal/shared/errors"
)

func canReadWorkspaceDiscussions(isActiveMember, isProjectOwner bool) bool {
	return isActiveMember || isProjectOwner
}

func canWriteWorkspaceDiscussions(status project.ProjectStatus, isActiveMember, isProjectOwner bool) bool {
	return status == project.ProjectStatusInProgress && canReadWorkspaceDiscussions(isActiveMember, isProjectOwner)
}

func canEditDiscussion(authorID, actorID uuid.UUID) bool {
	return actorID != uuid.Nil && authorID == actorID
}

func canDeleteDiscussion(authorID, actorID, ownerID uuid.UUID) bool {
	return actorID != uuid.Nil && (authorID == actorID || (ownerID != uuid.Nil && ownerID == actorID))
}

func validateDiscussionParent(parent *DiscussionReply) error {
	if parent == nil || parent.ParentReplyID == nil {
		return nil
	}
	return fmt.Errorf("%w: follow-up replies cannot have child replies", sharedErrors.ErrBadRequest)
}
