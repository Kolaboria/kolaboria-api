package project

import (
	"context"

	"github.com/syahrulfahmi/kolaboria-api/internal/infrastructure/workspaceactivity"
)

func (tx *gormProjectTx) AppendActivity(ctx context.Context, input workspaceactivity.Input) error {
	return workspaceactivity.Record(tx.db.WithContext(ctx), input)
}
