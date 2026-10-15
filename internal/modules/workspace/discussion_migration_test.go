package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscussionMigrationPreservesProjectAndReplyIntegrity(t *testing.T) {
	path := filepath.Join("..", "..", "..", "migrations", "000012_create_workspace_discussions.up.sql")
	schema, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read discussion migration: %v", err)
	}
	sql := strings.ToLower(string(schema))
	for _, expected := range []string{
		"create table workspace_discussions",
		"create table workspace_discussion_replies",
		"foreign key (discussion_id, project_id)",
		"foreign key (parent_reply_id, discussion_id, project_id)",
		"unique index workspace_discussions_idempotency_idx",
		"unique index workspace_discussion_replies_idempotency_idx",
		"deleted_at",
	} {
		if !strings.Contains(sql, expected) {
			t.Errorf("migration must include %q", expected)
		}
	}
}
