package workspace

import (
	"fmt"
	"strings"
	"unicode/utf8"

	sharedErrors "github.com/syahrulfahmi/kolaboria-api/internal/shared/errors"
)

const (
	DiscussionTitleMaxLength  = 160
	DiscussionBodyMaxLength   = 10000
	DiscussionReplyMaxLength  = 2000
	DiscussionDefaultPageSize = 20
	DiscussionMaxPageSize     = 50
)

type CreateDiscussionRequest struct {
	Title string `json:"title" binding:"required"`
	Body  string `json:"body" binding:"required"`
}

type UpdateDiscussionRequest struct {
	Title *string `json:"title,omitempty"`
	Body  *string `json:"body,omitempty"`
}

type CreateDiscussionReplyRequest struct {
	Body          string  `json:"body" binding:"required"`
	ParentReplyID *string `json:"parent_reply_id"`
}

type UpdateDiscussionReplyRequest struct {
	Body string `json:"body" binding:"required"`
}

type DiscussionAuthorResponse struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	AvatarURL *string `json:"avatar_url,omitempty"`
}

type DiscussionListItemResponse struct {
	ID               string                   `json:"id"`
	Title            string                   `json:"title"`
	BodyPreview      string                   `json:"body_preview"`
	Author           DiscussionAuthorResponse `json:"author"`
	ReplyCount       int                      `json:"reply_count"`
	ParticipantCount int                      `json:"participant_count"`
	LastActivityAt   string                   `json:"last_activity_at"`
	CreatedAt        string                   `json:"created_at"`
	UpdatedAt        string                   `json:"updated_at"`
	CanEdit          bool                     `json:"can_edit"`
	CanDelete        bool                     `json:"can_delete"`
}

type DiscussionResponse struct {
	ID               string                   `json:"id"`
	ProjectID        string                   `json:"project_id"`
	ProjectTitle     string                   `json:"project_title"`
	Title            string                   `json:"title"`
	Body             string                   `json:"body"`
	Author           DiscussionAuthorResponse `json:"author"`
	ReplyCount       int                      `json:"reply_count"`
	ParticipantCount int                      `json:"participant_count"`
	LastActivityAt   string                   `json:"last_activity_at"`
	CreatedAt        string                   `json:"created_at"`
	UpdatedAt        string                   `json:"updated_at"`
	CanEdit          bool                     `json:"can_edit"`
	CanDelete        bool                     `json:"can_delete"`
}

type DiscussionReplyResponse struct {
	ID            string                    `json:"id"`
	DiscussionID  string                    `json:"discussion_id"`
	ParentReplyID *string                   `json:"parent_reply_id"`
	Body          string                    `json:"body"`
	Author        DiscussionAuthorResponse  `json:"author"`
	CreatedAt     string                    `json:"created_at"`
	UpdatedAt     string                    `json:"updated_at"`
	IsDeleted     bool                      `json:"is_deleted"`
	CanEdit       bool                      `json:"can_edit"`
	CanDelete     bool                      `json:"can_delete"`
	Replies       []DiscussionReplyResponse `json:"replies"`
}

func normalizeDiscussionTitle(value string) (string, error) {
	return normalizeDiscussionText(value, DiscussionTitleMaxLength, "title")
}

func normalizeDiscussionBody(value string) (string, error) {
	return normalizeDiscussionText(value, DiscussionBodyMaxLength, "body")
}

func normalizeDiscussionReplyBody(value string) (string, error) {
	return normalizeDiscussionText(value, DiscussionReplyMaxLength, "reply body")
}

func normalizeDiscussionText(value string, maxLength int, field string) (string, error) {
	value = strings.TrimSpace(value)
	if !utf8.ValidString(value) {
		return "", fmt.Errorf("%w: %s must be valid UTF-8", sharedErrors.ErrBadRequest, field)
	}
	if value == "" {
		return "", fmt.Errorf("%w: %s is required", sharedErrors.ErrBadRequest, field)
	}
	if length := utf8.RuneCountInString(value); length > maxLength {
		return "", fmt.Errorf("%w: %s must contain at most %d characters", sharedErrors.ErrBadRequest, field, maxLength)
	}
	return value, nil
}
