package config

import (
	"fmt"
	"time"
)

type WorkspaceConfig struct {
	OverviewEnabled            bool `mapstructure:"WORKSPACE_OVERVIEW_ENABLED"`
	OverviewMaxRequests        int  `mapstructure:"WORKSPACE_OVERVIEW_MAX_REQUESTS"`
	OverviewMaxSnapshots       int  `mapstructure:"WORKSPACE_OVERVIEW_MAX_SNAPSHOTS"`
	OverviewRequestTimeoutMS   int  `mapstructure:"WORKSPACE_OVERVIEW_REQUEST_TIMEOUT_MS"`
	OverviewSnapshotTimeoutMS  int  `mapstructure:"WORKSPACE_OVERVIEW_SNAPSHOT_TIMEOUT_MS"`
	OverviewStatementTimeoutMS int  `mapstructure:"WORKSPACE_OVERVIEW_STATEMENT_TIMEOUT_MS"`
	OverviewLockTimeoutMS      int  `mapstructure:"WORKSPACE_OVERVIEW_LOCK_TIMEOUT_MS"`
}

func (c WorkspaceConfig) Validate(databaseOpenConns int) error {
	values := []struct {
		name  string
		value int
	}{
		{"WORKSPACE_OVERVIEW_MAX_REQUESTS", c.OverviewMaxRequests},
		{"WORKSPACE_OVERVIEW_MAX_SNAPSHOTS", c.OverviewMaxSnapshots},
		{"WORKSPACE_OVERVIEW_REQUEST_TIMEOUT_MS", c.OverviewRequestTimeoutMS},
		{"WORKSPACE_OVERVIEW_SNAPSHOT_TIMEOUT_MS", c.OverviewSnapshotTimeoutMS},
		{"WORKSPACE_OVERVIEW_STATEMENT_TIMEOUT_MS", c.OverviewStatementTimeoutMS},
		{"WORKSPACE_OVERVIEW_LOCK_TIMEOUT_MS", c.OverviewLockTimeoutMS},
	}
	for _, item := range values {
		if item.value < 1 {
			return fmt.Errorf("%s must be positive", item.name)
		}
	}
	if !(c.OverviewLockTimeoutMS < c.OverviewStatementTimeoutMS && c.OverviewStatementTimeoutMS < c.OverviewSnapshotTimeoutMS && c.OverviewSnapshotTimeoutMS < c.OverviewRequestTimeoutMS) {
		return fmt.Errorf("workspace overview timeouts must satisfy lock < statement < snapshot < request")
	}
	if databaseOpenConns < 1 || c.OverviewMaxSnapshots > databaseOpenConns/4 {
		return fmt.Errorf("WORKSPACE_OVERVIEW_MAX_SNAPSHOTS must not exceed one quarter of DB_MAX_OPEN_CONNS")
	}
	return nil
}

func (c WorkspaceConfig) OverviewDurations() (time.Duration, time.Duration, time.Duration, time.Duration) {
	return time.Duration(c.OverviewRequestTimeoutMS) * time.Millisecond,
		time.Duration(c.OverviewSnapshotTimeoutMS) * time.Millisecond,
		time.Duration(c.OverviewStatementTimeoutMS) * time.Millisecond,
		time.Duration(c.OverviewLockTimeoutMS) * time.Millisecond
}
