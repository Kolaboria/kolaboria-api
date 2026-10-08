package test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/project"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestProjectEditorAggregateTransactionsAgainstPostgres(t *testing.T) {
	dsn := os.Getenv("KOLABORIA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set KOLABORIA_TEST_DATABASE_URL to a disposable database with migrations through 000009 applied")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal("open test PostgreSQL database")
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal("get PostgreSQL connection")
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	actorID := uuid.New()
	projectID := uuid.New()
	requestID := uuid.New()
	roleID := uuid.New()
	createdAt := time.Now().UTC()
	_, _ = sqlDB.ExecContext(t.Context(), "DELETE FROM projects WHERE id = $1", projectID)
	t.Cleanup(func() {
		_, _ = sqlDB.ExecContext(context.Background(), "DELETE FROM projects WHERE id = $1", projectID)
		_, _ = sqlDB.ExecContext(context.Background(), "DELETE FROM users WHERE id = $1", actorID)
	})

	if _, err := sqlDB.ExecContext(t.Context(), "INSERT INTO users (id,email,password_hash) VALUES ($1,$2,$3)", actorID, fmt.Sprintf("project-editor-%s@example.test", actorID), "test-hash"); err != nil {
		t.Fatal("insert test actor")
	}
	repository := project.NewProjectRepository(db)
	projectRecord := project.Project{
		ID: projectID, Slug: "project-editor-" + strings.ReplaceAll(projectID.String(), "-", ""),
		CreationMode: project.CreationModePersonal, CreatedByUserID: actorID,
		InitiatorUserID: &actorID, OwnerID: &actorID, Title: "Initial title", ProjectCategory: "product",
		Visibility: "public", Origin: "personal", Availability: "flexible", HoursPerWeek: 5,
		Status: project.ProjectStatusDraft, Version: 1, ClientRequestID: requestID,
		CreateRequestHash: strings.Repeat("a", 64), CreatedAt: createdAt, UpdatedAt: createdAt,
	}
	masterRoleID := uuid.MustParse("5838d3f2-7c00-5cc7-bf80-96e063b006e1")
	skillID := uuid.MustParse("e61e074d-34a6-5235-93d7-c2b47ed74f02")
	toolID := uuid.MustParse("c04676b1-1823-54a3-8d5e-f84bb73305b6")
	aggregate := &project.ProjectAggregate{
		Project: projectRecord,
		Roles: []project.ProjectRoleAggregate{{
			ProjectRole: project.ProjectRole{ID: roleID, ContributionRoleID: &masterRoleID, Description: "Build the interface", Capacity: 2, Position: 0},
			Skills:      []project.MasterReference{{ID: skillID}}, Tools: []project.MasterReference{{ID: toolID}},
		}},
		OwnerMember: &project.ProjectMember{UserID: actorID, MembershipRole: "owner", Status: "active", JoinedAt: createdAt},
	}

	if err := repository.WithTransaction(t.Context(), func(tx project.ProjectTx) error {
		return tx.SaveAggregate(t.Context(), aggregate, true)
	}); err != nil {
		t.Fatal("create project aggregate")
	}

	loaded, err := repository.FindByID(t.Context(), projectID)
	if err != nil {
		t.Fatal("read project aggregate")
	}
	if len(loaded.Roles) != 1 || loaded.Roles[0].ID != roleID || len(loaded.Roles[0].Skills) != 1 || len(loaded.Roles[0].Tools) != 1 {
		t.Fatalf("initial aggregate lost role or master references: %#v", loaded.Roles)
	}
	if loaded.OwnerMember == nil || loaded.OwnerMember.UserID != actorID {
		t.Fatal("personal project owner membership was not persisted")
	}

	loaded.Title = ""
	loaded.ClientAcknowledgement = false
	loaded.Version = 2
	loaded.Roles[0].Description = ""
	loaded.Roles[0].Skills = []project.MasterReference{}
	loaded.Roles[0].Tools = []project.MasterReference{}
	if err := repository.WithTransaction(t.Context(), func(tx project.ProjectTx) error {
		return tx.SaveAggregate(t.Context(), loaded, false)
	}); err != nil {
		t.Fatal("update project aggregate")
	}
	loaded, err = repository.FindByID(t.Context(), projectID)
	if err != nil {
		t.Fatal("read updated project aggregate")
	}
	if loaded.Title != "" || loaded.ClientAcknowledgement || loaded.Version != 2 || loaded.Roles[0].ID != roleID || loaded.Roles[0].Description != "" {
		t.Fatalf("update did not preserve explicit zero values or role identity: %#v", loaded)
	}
	if len(loaded.Roles[0].Skills) != 0 || len(loaded.Roles[0].Tools) != 0 {
		t.Fatal("empty skill/tool arrays did not clear persisted joins")
	}

	loaded.Title = "must roll back"
	loaded.Version = 3
	loaded.Roles[0].Description = "also rolled back"
	loaded.Roles[0].Tools = []project.MasterReference{{ID: uuid.New()}}
	err = repository.WithTransaction(t.Context(), func(tx project.ProjectTx) error {
		return tx.SaveAggregate(t.Context(), loaded, false)
	})
	if err == nil {
		t.Fatal("expected invalid tool foreign key to abort aggregate transaction")
	}
	afterRollback, err := repository.FindByID(t.Context(), projectID)
	if err != nil {
		t.Fatal("read project after rollback")
	}
	if afterRollback.Title != "" || afterRollback.Version != 2 || afterRollback.Roles[0].Description != "" || len(afterRollback.Roles[0].Tools) != 0 {
		t.Fatalf("failed relation write partially mutated project: %#v", afterRollback)
	}
}
