package workspaceactivity

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Input describes one successful domain change. Record must be called with the
// transaction that changes the source; authorization and retry guards belong to
// that domain operation, before Record is invoked.
type Input struct {
	ProjectID   uuid.UUID
	ActorID     uuid.UUID
	SubjectType string
	SubjectID   uuid.UUID
	TaskID      *uuid.UUID
	SnapshotID  uuid.UUID
	OperationID uuid.UUID
	Order       int16
	Type        string
	Payload     any
	At          time.Time
}

func CaptureContext(tx *gorm.DB, projectID uuid.UUID, at time.Time) (*ContextSnapshot, error) {
	var source struct {
		Title                   string
		Summary                 string
		ProjectCategory         string
		Version                 int64
		InitiatorOrganizationID *uuid.UUID
		OrganizationName        *string
	}
	if err := tx.Table("projects p").Select("p.title, p.summary, p.project_category, p.version, p.initiator_organization_id, o.name AS organization_name").
		Joins("LEFT JOIN organizations o ON o.id = p.initiator_organization_id").Where("p.id = ?", projectID).Take(&source).Error; err != nil {
		return nil, fmt.Errorf("load workspace context: %w", err)
	}
	canonical, err := json.Marshal(struct {
		Title            string     `json:"title"`
		Summary          string     `json:"summary"`
		Category         string     `json:"category"`
		OrganizationID   *uuid.UUID `json:"organization_id"`
		OrganizationName *string    `json:"organization_name"`
	}{source.Title, source.Summary, source.ProjectCategory, source.InitiatorOrganizationID, source.OrganizationName})
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(canonical)
	snapshot := ContextSnapshot{ID: uuid.New(), ProjectID: projectID, ProjectVersion: source.Version,
		ContextHash: hex.EncodeToString(hash[:]), Title: source.Title, Summary: source.Summary,
		ProjectCategory: source.ProjectCategory, InitiatorOrganizationID: source.InitiatorOrganizationID,
		InitiatorOrganizationName: source.OrganizationName, CapturedAt: at.UTC(), SchemaVersion: 1}
	// RETURNING returns the existing ID on conflict without a second lookup or
	// replacing the original snapshot timestamp.
	err = tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "project_id"}, {Name: "project_version"}, {Name: "context_hash"}},
		DoUpdates: clause.Set{{Column: clause.Column{Name: "captured_at"}, Value: gorm.Expr("workspace_project_context_snapshots.captured_at")}},
	}, clause.Returning{Columns: []clause.Column{{Name: "id"}}}).Create(&snapshot).Error
	return &snapshot, err
}

func Record(tx *gorm.DB, input Input) error {
	if input.ProjectID == uuid.Nil || input.ActorID == uuid.Nil || input.SubjectID == uuid.Nil || input.Type == "" || input.SubjectType == "" {
		return fmt.Errorf("workspace activity requires project, actor, subject and event type")
	}
	if input.At.IsZero() {
		input.At = time.Now().UTC()
	}
	if input.OperationID == uuid.Nil {
		input.OperationID = uuid.New()
	}
	if input.Order < 1 {
		input.Order = 1
	}
	if input.SnapshotID == uuid.Nil {
		snapshot, err := CaptureContext(tx, input.ProjectID, input.At)
		if err != nil {
			return err
		}
		input.SnapshotID = snapshot.ID
	}
	var actor struct {
		MemberID *uuid.UUID
		RoleID   *uuid.UUID
		RoleName *string
		Scope    string
	}
	err := tx.Table("projects p").Select(`pm.id AS member_id, cr.id AS role_id, cr.name AS role_name,
		CASE WHEN p.owner_id = ? OR pm.membership_role = 'owner' THEN 'owner'
		WHEN p.owner_id IS NULL AND p.initiator_organization_id IS NOT NULL AND p.created_by_user_id = ? THEN 'organization_initiator'
		ELSE 'contributor' END AS scope`, input.ActorID, input.ActorID).
		Joins("LEFT JOIN project_members pm ON pm.project_id = p.id AND pm.user_id = ? AND pm.status = 'active'", input.ActorID).
		Joins("LEFT JOIN project_roles pr ON pr.id = pm.project_role_id AND pr.project_id = p.id").
		Joins("LEFT JOIN contribution_roles cr ON cr.id = COALESCE(pm.contribution_role_id, pr.contribution_role_id)").
		Where("p.id = ?", input.ProjectID).Take(&actor).Error
	if err != nil {
		return fmt.Errorf("resolve workspace activity actor: %w", err)
	}
	encoded, err := json.Marshal(input.Payload)
	if err != nil {
		return fmt.Errorf("encode workspace activity: %w", err)
	}
	if input.Payload == nil {
		encoded = []byte("{}")
	}
	event := Event{ID: uuid.New(), ProjectID: input.ProjectID, SubjectType: input.SubjectType, SubjectID: input.SubjectID,
		TaskID: input.TaskID, ProjectContextSnapshotID: input.SnapshotID, OperationID: input.OperationID,
		EventOrder: input.Order, EventType: input.Type, ActorUserID: input.ActorID, ActorMemberID: actor.MemberID,
		ActorScope: actor.Scope, ActorRoleID: actor.RoleID, ActorRoleName: actor.RoleName,
		OccurredAt: input.At.UTC(), SchemaVersion: 1, Payload: encoded}
	if err := tx.Create(&event).Error; err != nil {
		return fmt.Errorf("append workspace activity: %w", err)
	}
	return nil
}
