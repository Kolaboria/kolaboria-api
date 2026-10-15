package workspace

import (
	"time"

	"github.com/google/uuid"
)

type Discussion struct {
	ID             uuid.UUID  `gorm:"column:id;type:uuid;primaryKey"`
	ProjectID      uuid.UUID  `gorm:"column:project_id;type:uuid;not null"`
	AuthorID       uuid.UUID  `gorm:"column:author_id;type:uuid;not null"`
	Title          string     `gorm:"column:title;not null"`
	Body           string     `gorm:"column:body;not null"`
	LastActivityAt time.Time  `gorm:"column:last_activity_at;not null"`
	IdempotencyKey *string    `gorm:"column:idempotency_key"`
	RequestHash    *string    `gorm:"column:request_hash"`
	CreatedAt      time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt      time.Time  `gorm:"column:updated_at;not null"`
	DeletedAt      *time.Time `gorm:"column:deleted_at"`
}

func (Discussion) TableName() string { return "workspace_discussions" }

type DiscussionReply struct {
	ID             uuid.UUID  `gorm:"column:id;type:uuid;primaryKey"`
	ProjectID      uuid.UUID  `gorm:"column:project_id;type:uuid;not null"`
	DiscussionID   uuid.UUID  `gorm:"column:discussion_id;type:uuid;not null"`
	ParentReplyID  *uuid.UUID `gorm:"column:parent_reply_id;type:uuid"`
	AuthorID       uuid.UUID  `gorm:"column:author_id;type:uuid;not null"`
	Body           string     `gorm:"column:body;not null"`
	IdempotencyKey *string    `gorm:"column:idempotency_key"`
	RequestHash    *string    `gorm:"column:request_hash"`
	CreatedAt      time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt      time.Time  `gorm:"column:updated_at;not null"`
	DeletedAt      *time.Time `gorm:"column:deleted_at"`
}

func (DiscussionReply) TableName() string { return "workspace_discussion_replies" }
