package auth

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Opt in only against a disposable database with migration 000008 applied.
// All fixtures and changes stay inside a rolled-back transaction.
func TestInitiableOrganizationsAgainstPostgres(t *testing.T) {
	dsn := os.Getenv("KOLABORIA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set KOLABORIA_TEST_DATABASE_URL to a disposable database with migrations through 000008 applied")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal("open disposable PostgreSQL database failed")
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	exec := func(query string, args ...any) {
		t.Helper()
		if err := tx.Exec(query, args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	userID, outsiderID := uuid.New(), uuid.New()
	for _, id := range []uuid.UUID{userID, outsiderID} {
		exec("INSERT INTO users(id, email, password_hash, system_role) VALUES (?, ?, 'test-only', 'admin')", id, id.String()+"@example.test")
	}
	allowedID, deniedID, inactiveID := uuid.New(), uuid.New(), uuid.New()
	for index, id := range []uuid.UUID{allowedID, deniedID, inactiveID} {
		exec("INSERT INTO organizations(id, name, slug, is_active) VALUES (?, ?, ?, ?)", id, "Test organization", "test-"+id.String(), index != 2)
		exec("INSERT INTO organization_members(organization_id, user_id, can_initiate_projects) VALUES (?, ?, ?)", id, userID, index != 1)
	}
	repo := NewAuthRepository(tx)
	assertList := func(id uuid.UUID, expected int) {
		t.Helper()
		got, err := repo.ListInitiableOrganizations(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		if got == nil || len(got) != expected {
			t.Fatalf("expected %d organizations, got %+v", expected, got)
		}
		if expected == 1 && got[0].ID != allowedID {
			t.Fatal("unauthorized organization returned")
		}
	}
	assertList(userID, 1)
	assertList(outsiderID, 0)
	for _, change := range []struct {
		revoke, restore string
		id              uuid.UUID
	}{
		{"UPDATE users SET system_role = 'user' WHERE id = ?", "UPDATE users SET system_role = 'admin' WHERE id = ?", userID},
		{"UPDATE users SET is_active = false WHERE id = ?", "UPDATE users SET is_active = true WHERE id = ?", userID},
		{"UPDATE users SET deleted_at = NOW() WHERE id = ?", "UPDATE users SET deleted_at = NULL WHERE id = ?", userID},
		{"UPDATE organization_members SET is_active = false WHERE organization_id = ?", "UPDATE organization_members SET is_active = true WHERE organization_id = ?", allowedID},
		{"UPDATE organization_members SET can_initiate_projects = false WHERE organization_id = ?", "UPDATE organization_members SET can_initiate_projects = true WHERE organization_id = ?", allowedID},
		{"UPDATE organizations SET is_active = false WHERE id = ?", "UPDATE organizations SET is_active = true WHERE id = ?", allowedID},
	} {
		exec(change.revoke, change.id)
		assertList(userID, 0)
		exec(change.restore, change.id)
		assertList(userID, 1)
	}
}
