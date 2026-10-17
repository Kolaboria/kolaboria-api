// Package workspaceactivity persists source-linked workspace facts in the
// caller's transaction. It does not decide domain permissions or publish events.
package workspaceactivity

import (
	"time"

	"github.com/google/uuid"
)

type Event struct {
	ID                       uuid.UUID  `gorm:"column:id;type:uuid;primaryKey"`
	ProjectID                uuid.UUID  `gorm:"column:project_id;type:uuid;not null"`
	SubjectType              string     `gorm:"column:subject_type;not null"`
	SubjectID                uuid.UUID  `gorm:"column:subject_id;type:uuid;not null"`
	TaskID                   *uuid.UUID `gorm:"column:task_id;type:uuid"`
	ProjectContextSnapshotID uuid.UUID  `gorm:"column:project_context_snapshot_id;type:uuid;not null"`
	OperationID              uuid.UUID  `gorm:"column:operation_id;type:uuid;not null"`
	EventOrder               int16      `gorm:"column:event_order;not null"`
	EventType                string     `gorm:"column:event_type;not null"`
	ActorUserID              uuid.UUID  `gorm:"column:actor_user_id;type:uuid;not null"`
	ActorMemberID            *uuid.UUID `gorm:"column:actor_member_id;type:uuid"`
	ActorScope               string     `gorm:"column:actor_scope;not null"`
	ActorRoleID              *uuid.UUID `gorm:"column:actor_role_id;type:uuid"`
	ActorRoleName            *string    `gorm:"column:actor_role_name"`
	OccurredAt               time.Time  `gorm:"column:occurred_at;not null"`
	SchemaVersion            int16      `gorm:"column:schema_version;not null"`
	Payload                  []byte     `gorm:"column:payload;type:jsonb;not null"`
}

func (Event) TableName() string { return "workspace_activity_events" }

type ContextSnapshot struct {
	ID                        uuid.UUID  `gorm:"column:id;type:uuid;primaryKey"`
	ProjectID                 uuid.UUID  `gorm:"column:project_id;type:uuid;not null"`
	ProjectVersion            int64      `gorm:"column:project_version;not null"`
	ContextHash               string     `gorm:"column:context_hash;not null"`
	Title                     string     `gorm:"column:title;not null"`
	Summary                   string     `gorm:"column:summary;not null"`
	ProjectCategory           string     `gorm:"column:project_category;not null"`
	InitiatorOrganizationID   *uuid.UUID `gorm:"column:initiator_organization_id;type:uuid"`
	InitiatorOrganizationName *string    `gorm:"column:initiator_organization_name"`
	CapturedAt                time.Time  `gorm:"column:captured_at;not null"`
	SchemaVersion             int16      `gorm:"column:schema_version;not null"`
}

func (ContextSnapshot) TableName() string { return "workspace_project_context_snapshots" }
