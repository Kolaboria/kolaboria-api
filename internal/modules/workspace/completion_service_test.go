package workspace

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/syahrulfahmi/kolaboria-api/internal/infrastructure/workspaceactivity"
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/project"
	sharedErrors "github.com/syahrulfahmi/kolaboria-api/internal/shared/errors"
)

type completionMemoryRepository struct {
	access   WorkspaceProjectAccess
	summary  ProjectTaskSummaryResponse
	stored   *WorkspaceProjectCompletion
	events   []workspaceactivity.Input
	eventErr error
}

// Stages writes, like a database transaction: an event failure must discard the
// completion and project status together, not just return an error afterwards.
func (repository *completionMemoryRepository) WithTransaction(_ context.Context, operation func(CompletionTx) error) error {
	staged := *repository
	staged.events = append([]workspaceactivity.Input(nil), repository.events...)
	if err := operation(&staged); err != nil {
		return err
	}
	*repository = staged
	return nil
}
func (repository *completionMemoryRepository) LockProject(context.Context, string, uuid.UUID) (*WorkspaceProjectAccess, error) {
	return &repository.access, nil
}
func (repository *completionMemoryRepository) LoadTaskSummary(uuid.UUID) (ProjectTaskSummaryResponse, error) {
	return repository.summary, nil
}
func (repository *completionMemoryRepository) FindCompletion(uuid.UUID) (*WorkspaceProjectCompletion, error) {
	return repository.stored, nil
}
func (repository *completionMemoryRepository) InsertCompletion(record *WorkspaceProjectCompletion) error {
	repository.stored = record
	return nil
}
func (repository *completionMemoryRepository) MarkCompleted(_ uuid.UUID, version int64, _ time.Time) error {
	if repository.access.Project.Version != version {
		return ErrCompletionStale
	}
	repository.access.Project.Status = project.ProjectStatusCompleted
	repository.access.Project.Version++
	return nil
}
func (repository *completionMemoryRepository) AppendActivity(input workspaceactivity.Input) error {
	if repository.eventErr != nil {
		return repository.eventErr
	}
	repository.events = append(repository.events, input)
	return nil
}

func completionFixture() (*completionMemoryRepository, uuid.UUID, CompleteProjectRequest) {
	actor := uuid.New()
	repository := &completionMemoryRepository{
		access:  WorkspaceProjectAccess{IsOwner: true, CanManage: true, Project: project.Project{ID: uuid.New(), OwnerID: &actor, Status: project.ProjectStatusInProgress, Version: 7}},
		summary: ProjectTaskSummaryResponse{Version: 18, TotalTasks: 3, DoneTasks: 2, ClosedTasks: 1, DeletedTasks: 2},
	}
	return repository, actor, CompleteProjectRequest{Version: 7, TaskVersion: 18, ResultSummary: "Hasil sudah ditinjau", ReviewBasis: "Review bersama tim"}
}

func TestCompletionStoresOwnerDecisionAndReplaysWithoutDuplicateEvent(t *testing.T) {
	repository, actor, request := completionFixture()
	service := NewCompletionService(repository)
	first, err := service.Complete(context.Background(), actor, "platform", request)
	if err != nil {
		t.Fatal(err)
	}
	again, err := service.Complete(context.Background(), actor, "platform", request)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != again.ID || repository.access.Project.Version != 8 || len(repository.events) != 1 {
		t.Fatal("retry changed the closure or duplicated its event")
	}
	if first.TaskSummary.DoneTasks != 2 || first.TaskSummary.ClosedTasks != 1 || first.TaskSummary.DeletedTasks != 2 {
		t.Fatal("scope exclusions must remain distinct from completed work")
	}
	if repository.events[0].Type != "project.completed" {
		t.Fatal("missing completion activity")
	}
	request.ResultSummary = "Keputusan berbeda"
	if _, err := service.Complete(context.Background(), actor, "platform", request); !errors.Is(err, sharedErrors.ErrConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestCompletionValidatesLatestTasksStatusAndAuthority(t *testing.T) {
	cases := []struct {
		name   string
		change func(*completionMemoryRepository, *CompleteProjectRequest)
		want   error
	}{
		{"contributor task grant is not project authority", func(r *completionMemoryRepository, _ *CompleteProjectRequest) {
			r.access.IsOwner = false
			r.access.CanManage = false
			r.access.ActiveMember = true
		}, sharedErrors.ErrForbidden},
		{"project not started", func(r *completionMemoryRepository, _ *CompleteProjectRequest) {
			r.access.Project.Status = project.ProjectStatusOpen
		}, sharedErrors.ErrConflict},
		{"archived", func(r *completionMemoryRepository, _ *CompleteProjectRequest) {
			r.access.Project.Status = project.ProjectStatusArchived
		}, sharedErrors.ErrConflict},
		{"review still pending", func(r *completionMemoryRepository, _ *CompleteProjectRequest) { r.summary.OutstandingTasks = 1 }, ErrCompletionBlocked},
		{"closed without scope reason", func(r *completionMemoryRepository, _ *CompleteProjectRequest) { r.summary.InvalidClosedTasks = 1 }, ErrCompletionBlocked},
		{"task edited with same counts", func(r *completionMemoryRepository, _ *CompleteProjectRequest) { r.summary.Version++ }, ErrCompletionStale},
		{"project edited", func(r *completionMemoryRepository, _ *CompleteProjectRequest) { r.access.Project.Version++ }, ErrCompletionStale},
		{"empty result", func(_ *completionMemoryRepository, q *CompleteProjectRequest) { q.ResultSummary = "  " }, sharedErrors.ErrBadRequest},
		{"no tasks without review basis", func(r *completionMemoryRepository, q *CompleteProjectRequest) {
			r.summary = ProjectTaskSummaryResponse{}
			q.TaskVersion = 0
			q.ReviewBasis = ""
		}, sharedErrors.ErrBadRequest},
	}
	for _, example := range cases {
		t.Run(example.name, func(t *testing.T) {
			repository, actor, request := completionFixture()
			example.change(repository, &request)
			if _, err := NewCompletionService(repository).Complete(context.Background(), actor, "platform", request); !errors.Is(err, example.want) {
				t.Fatalf("expected %v, got %v", example.want, err)
			}
			if repository.stored != nil || len(repository.events) != 0 {
				t.Fatal("rejected closure persisted facts")
			}
		})
	}
}

func TestCompletionWithoutStructuredTasksRequiresExplicitReviewBasis(t *testing.T) {
	repository, actor, request := completionFixture()
	repository.summary = ProjectTaskSummaryResponse{Version: 4, DeletedTasks: 2}
	request.TaskVersion = 4
	request.ReviewBasis = "Tinjauan demo; tidak ada task terstruktur untuk memverifikasi pembagian pekerjaan"
	result, err := NewCompletionService(repository).Complete(context.Background(), actor, "platform", request)
	if err != nil || result.TaskSummary.TotalTasks != 0 || result.ReviewBasis != request.ReviewBasis {
		t.Fatalf("unexpected result: %#v, %v", result, err)
	}
}

func TestCompletionWithAllTasksExcludedDoesNotClaimCompletedTaskResults(t *testing.T) {
	repository, actor, request := completionFixture()
	repository.summary.DoneTasks = 0
	repository.summary.ClosedTasks = repository.summary.TotalTasks
	result, err := NewCompletionService(repository).Complete(context.Background(), actor, "platform", request)
	if err != nil || result.TaskSummary.DoneTasks != 0 || result.TaskSummary.ClosedTasks != 3 {
		t.Fatalf("unexpected result: %#v, %v", result, err)
	}
}

func TestCompletionRollsBackStatusAndDecisionWhenActivityFails(t *testing.T) {
	repository, actor, request := completionFixture()
	repository.eventErr = errors.New("event insert failed")
	if _, err := NewCompletionService(repository).Complete(context.Background(), actor, "platform", request); err == nil {
		t.Fatal("expected event error")
	}
	if repository.stored != nil || repository.access.Project.Status != project.ProjectStatusInProgress || repository.access.Project.Version != 7 {
		t.Fatal("closure and lifecycle update must roll back together")
	}
}
