package workspace

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func loadOverviewActivities(tx *gorm.DB, projectID uuid.UUID) ([]ActivityResponse, error) {
	types := visibleActivityTypes()
	base := tx.Table("workspace_activity_events e").
		Where("e.project_id = ? AND e.event_type IN ?", projectID, types).
		Select("e.*").
		Order("e.occurred_at DESC, e.operation_id DESC, e.event_order DESC, e.id DESC").
		Limit(WorkspaceOverviewActivityPreviewLimit)
	rows := make([]ActivityProjection, 0, WorkspaceOverviewActivityPreviewLimit)
	query := tx.Table("(?) AS e", base)
	if err := workspaceActivityProjectionQuery(query).Scan(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]ActivityResponse, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapActivity(row))
	}
	return items, nil
}
