package workspace

import (
	"strings"

	"github.com/google/uuid"
	sharedErrors "github.com/syahrulfahmi/kolaboria-api/internal/shared/errors"
)

type TaskAccess struct {
	ActorActive           bool
	ActiveMember          bool
	MemberID              *uuid.UUID
	IsManager             bool
	TaskManagementGranted bool
}

type TaskPolicyState struct {
	Status           TaskStatus
	AssigneeMemberID *uuid.UUID
	Version          int64
	HasPendingReview bool
}

type TaskMutation struct {
	HasContentChange  bool
	HasMetadataChange bool
	AssignmentChanged bool
	ToStatus          *TaskStatus
	CloseReason       *string
	ReopenConfirmed   bool
	IsDelete          bool
	IsComment         bool
}

type TaskMutationDecision struct {
	ToStatus             TaskStatus
	ChangesReviewContext bool
	CreateReviewCycle    bool
	ResolveReviewAs      string
	SelfReview           bool
}

func EvaluateTaskMutation(access TaskAccess, current TaskPolicyState, mutation TaskMutation) (TaskMutationDecision, error) {
	if !access.ActorActive {
		return TaskMutationDecision{}, sharedErrors.ErrUnauthorized
	}
	if !access.ActiveMember && !access.IsManager {
		return TaskMutationDecision{}, sharedErrors.ErrNotFound
	}
	if mutation.IsComment {
		return TaskMutationDecision{ToStatus: current.Status}, nil
	}
	if mutation.IsDelete {
		if !access.IsManager && !access.TaskManagementGranted {
			return TaskMutationDecision{}, sharedErrors.ErrForbidden
		}
		if !isActiveTaskStatus(current.Status) {
			return TaskMutationDecision{}, sharedErrors.ErrConflict
		}
		decision := TaskMutationDecision{ToStatus: current.Status}
		if current.Status == TaskStatusReview {
			decision.ResolveReviewAs = "withdrawn"
		}
		return decision, nil
	}
	if mutation.HasContentChange && !access.IsManager && !isAssignedTo(access, current) {
		return TaskMutationDecision{}, sharedErrors.ErrForbidden
	}
	if mutation.HasMetadataChange && !access.IsManager {
		return TaskMutationDecision{}, sharedErrors.ErrForbidden
	}
	if !isActiveTaskStatus(current.Status) && !mutation.ReopenConfirmed {
		return TaskMutationDecision{}, sharedErrors.ErrConflict
	}

	to := current.Status
	if mutation.ToStatus != nil {
		to = *mutation.ToStatus
	}
	if !validTaskStatus(to) {
		return TaskMutationDecision{}, sharedErrors.ErrBadRequest
	}
	if current.Status == TaskStatusClosed || current.Status == TaskStatusDone {
		if !access.IsManager || !mutation.ReopenConfirmed || (to != TaskStatusTodo && to != TaskStatusInProgress) {
			return TaskMutationDecision{}, sharedErrors.ErrForbidden
		}
	}
	if to == TaskStatusClosed {
		if !access.IsManager || mutation.CloseReason == nil || strings.TrimSpace(*mutation.CloseReason) == "" {
			return TaskMutationDecision{}, sharedErrors.ErrForbidden
		}
		if len([]rune(*mutation.CloseReason)) > 1000 {
			return TaskMutationDecision{}, sharedErrors.ErrBadRequest
		}
	}
	if to == TaskStatusDone && current.Status != TaskStatusReview {
		return TaskMutationDecision{}, sharedErrors.ErrConflict
	}
	if to == TaskStatusReview {
		if current.Status != TaskStatusReview && !access.IsManager && (access.MemberID == nil || !isAssignedTo(access, current)) {
			return TaskMutationDecision{}, sharedErrors.ErrForbidden
		}
	}
	if current.Status == TaskStatusReview && to == TaskStatusTodo && !access.IsManager && !isAssignedTo(access, current) {
		return TaskMutationDecision{}, sharedErrors.ErrForbidden
	}
	if (to == TaskStatusDone || to == TaskStatusInProgress) && current.Status == TaskStatusReview && !access.ActiveMember {
		return TaskMutationDecision{}, sharedErrors.ErrForbidden
	}
	if current.HasPendingReview && (mutation.HasContentChange || mutation.AssignmentChanged) && mutation.ToStatus != nil && (*mutation.ToStatus == TaskStatusDone || *mutation.ToStatus == TaskStatusInProgress) {
		return TaskMutationDecision{}, sharedErrors.ErrConflict
	}
	if current.Status == TaskStatusTodo && to == TaskStatusInProgress && !access.IsManager && !isAssignedTo(access, current) {
		return TaskMutationDecision{}, sharedErrors.ErrForbidden
	}
	if current.Status == TaskStatusInProgress && to == TaskStatusTodo && !access.IsManager && !isAssignedTo(access, current) {
		return TaskMutationDecision{}, sharedErrors.ErrForbidden
	}
	if current.Status == TaskStatusInProgress && to == TaskStatusReview && !access.IsManager && !isAssignedTo(access, current) {
		return TaskMutationDecision{}, sharedErrors.ErrForbidden
	}
	if current.Status == TaskStatusTodo && to == TaskStatusReview && !access.IsManager && !isAssignedTo(access, current) {
		return TaskMutationDecision{}, sharedErrors.ErrForbidden
	}
	if to == current.Status && !mutation.HasContentChange && !mutation.HasMetadataChange {
		return TaskMutationDecision{}, sharedErrors.ErrBadRequest
	}
	decision := TaskMutationDecision{ToStatus: to}
	if current.Status == TaskStatusReview && to == TaskStatusClosed {
		decision.ResolveReviewAs = "withdrawn"
	}
	if current.HasPendingReview && (mutation.HasContentChange || mutation.AssignmentChanged) {
		decision.ChangesReviewContext = true
		decision.ResolveReviewAs = "superseded"
		if mutation.ToStatus == nil {
			decision.ToStatus = TaskStatusInProgress
		}
	}
	if current.Status == TaskStatusReview && to == TaskStatusDone {
		decision.ResolveReviewAs = "accepted"
		decision.SelfReview = isAssignedTo(access, current)
	}
	if current.Status == TaskStatusReview && to == TaskStatusInProgress {
		decision.ResolveReviewAs = "changes_requested"
	}
	if current.Status == TaskStatusReview && to == TaskStatusTodo {
		decision.ResolveReviewAs = "withdrawn"
	}
	if to == TaskStatusReview && (current.Status != TaskStatusReview || decision.ChangesReviewContext) {
		decision.CreateReviewCycle = true
	}
	if mutation.ReopenConfirmed && (current.Status == TaskStatusClosed || current.Status == TaskStatusDone) {
		decision.ChangesReviewContext = false
		decision.ResolveReviewAs = ""
	}
	return decision, nil
}

func isAssignedTo(access TaskAccess, state TaskPolicyState) bool {
	return access.MemberID != nil && state.AssigneeMemberID != nil && *access.MemberID == *state.AssigneeMemberID
}

func isActiveTaskStatus(status TaskStatus) bool {
	return status == TaskStatusTodo || status == TaskStatusInProgress || status == TaskStatusReview
}

func validTaskStatus(status TaskStatus) bool {
	return status == TaskStatusTodo || status == TaskStatusInProgress || status == TaskStatusReview || status == TaskStatusDone || status == TaskStatusClosed
}
