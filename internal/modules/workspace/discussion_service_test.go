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
	"gorm.io/gorm"
)

type memoryDiscussionRepository struct {
	events      []workspaceactivity.Input
	eventErr    error
	access      *DiscussionProjectAccess
	resolveErr  error
	listItems   []DiscussionProjection
	discussions map[uuid.UUID]Discussion
	replies     map[uuid.UUID]DiscussionReply
}

func newMemoryDiscussionRepository(access *DiscussionProjectAccess) *memoryDiscussionRepository {
	return &memoryDiscussionRepository{access: access, discussions: map[uuid.UUID]Discussion{}, replies: map[uuid.UUID]DiscussionReply{}}
}

func (repository *memoryDiscussionRepository) ResolveDiscussionProject(context.Context, string, uuid.UUID) (*DiscussionProjectAccess, error) {
	return repository.access, repository.resolveErr
}

func (repository *memoryDiscussionRepository) ListDiscussions(context.Context, uuid.UUID, int, int) ([]DiscussionProjection, int64, error) {
	return repository.listItems, int64(len(repository.listItems)), nil
}

func (repository *memoryDiscussionRepository) GetDiscussion(_ context.Context, projectID, discussionID uuid.UUID) (*DiscussionProjection, error) {
	return memoryGetDiscussion(repository.discussions, projectID, discussionID)
}

func (repository *memoryDiscussionRepository) ListDiscussionReplies(context.Context, uuid.UUID, uuid.UUID, int, int) ([]DiscussionReplyProjection, int64, error) {
	return nil, 0, nil
}

func (repository *memoryDiscussionRepository) WithDiscussionTransaction(_ context.Context, fn func(DiscussionTx) error) error {
	discussions := make(map[uuid.UUID]Discussion, len(repository.discussions))
	for id, record := range repository.discussions {
		discussions[id] = record
	}
	replies := make(map[uuid.UUID]DiscussionReply, len(repository.replies))
	for id, record := range repository.replies {
		replies[id] = record
	}
	eventCount := len(repository.events)
	err := fn(&memoryDiscussionTx{repository: repository})
	if err != nil {
		repository.discussions = discussions
		repository.replies = replies
		repository.events = repository.events[:eventCount]
	}
	return err
}

type memoryDiscussionTx struct{ repository *memoryDiscussionRepository }

func (tx *memoryDiscussionTx) ValidateDiscussionWriter(_ context.Context, projectID, actorID uuid.UUID) (*DiscussionProjectAccess, error) {
	access := tx.repository.access
	if access.ID != projectID || !canReadWorkspaceDiscussions(access.IsActiveMember, access.IsOwner(actorID)) {
		return nil, gorm.ErrRecordNotFound
	}
	return access, nil
}

func (tx *memoryDiscussionTx) GetDiscussion(_ context.Context, projectID, discussionID uuid.UUID, _ bool) (*DiscussionProjection, error) {
	return memoryGetDiscussion(tx.repository.discussions, projectID, discussionID)
}

func (tx *memoryDiscussionTx) FindDiscussionByIdempotency(_ context.Context, projectID, actorID uuid.UUID, key string) (*Discussion, error) {
	for _, discussion := range tx.repository.discussions {
		if discussion.ProjectID == projectID && discussion.AuthorID == actorID && discussion.IdempotencyKey != nil && *discussion.IdempotencyKey == key {
			copy := discussion
			return &copy, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (tx *memoryDiscussionTx) InsertDiscussion(_ context.Context, discussion *Discussion) (bool, error) {
	if _, err := tx.FindDiscussionByIdempotency(context.Background(), discussion.ProjectID, discussion.AuthorID, *discussion.IdempotencyKey); err == nil {
		return false, nil
	}
	tx.repository.discussions[discussion.ID] = *discussion
	return true, nil
}

func (tx *memoryDiscussionTx) UpdateDiscussion(_ context.Context, projectID, discussionID uuid.UUID, title, body string, now time.Time) error {
	discussion, ok := tx.repository.discussions[discussionID]
	if !ok || discussion.ProjectID != projectID {
		return gorm.ErrRecordNotFound
	}
	discussion.Title, discussion.Body, discussion.UpdatedAt, discussion.LastActivityAt = title, body, now, now
	tx.repository.discussions[discussionID] = discussion
	return nil
}

func (tx *memoryDiscussionTx) SoftDeleteDiscussion(_ context.Context, projectID, discussionID uuid.UUID, now time.Time) error {
	discussion, ok := tx.repository.discussions[discussionID]
	if !ok || discussion.ProjectID != projectID {
		return gorm.ErrRecordNotFound
	}
	discussion.Title, discussion.Body, discussion.DeletedAt, discussion.UpdatedAt = "", "", &now, now
	tx.repository.discussions[discussionID] = discussion
	return nil
}

func (*memoryDiscussionTx) SoftDeleteDiscussionReplies(context.Context, uuid.UUID, uuid.UUID, time.Time) error {
	return nil
}
func (tx *memoryDiscussionTx) GetDiscussionReply(_ context.Context, _, _, replyID uuid.UUID, _ bool) (*DiscussionReplyProjection, error) {
	reply, ok := tx.repository.replies[replyID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return &DiscussionReplyProjection{DiscussionReply: reply, AuthorName: "Ayu"}, nil
}
func (tx *memoryDiscussionTx) FindReplyByIdempotency(context.Context, uuid.UUID, uuid.UUID, string) (*DiscussionReply, error) {
	return nil, gorm.ErrRecordNotFound
}
func (tx *memoryDiscussionTx) InsertDiscussionReply(_ context.Context, reply *DiscussionReply) (bool, error) {
	tx.repository.replies[reply.ID] = *reply
	return true, nil
}
func (tx *memoryDiscussionTx) UpdateDiscussionReply(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string, time.Time) error {
	return nil
}
func (tx *memoryDiscussionTx) SoftDeleteDiscussionReply(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, time.Time) error {
	return nil
}
func (tx *memoryDiscussionTx) TouchDiscussion(_ context.Context, projectID, discussionID uuid.UUID, now time.Time) error {
	discussion, ok := tx.repository.discussions[discussionID]
	if !ok || discussion.ProjectID != projectID {
		return gorm.ErrRecordNotFound
	}
	discussion.LastActivityAt = now
	tx.repository.discussions[discussionID] = discussion
	return nil
}
func memoryGetDiscussion(records map[uuid.UUID]Discussion, projectID, discussionID uuid.UUID) (*DiscussionProjection, error) {
	discussion, ok := records[discussionID]
	if !ok || discussion.ProjectID != projectID || discussion.DeletedAt != nil {
		return nil, gorm.ErrRecordNotFound
	}
	return &DiscussionProjection{Discussion: discussion, AuthorName: "Ayu", ParticipantCount: 1}, nil
}

func TestCreateDiscussionIsIdempotentAndRejectsKeyReuseWithDifferentContent(t *testing.T) {
	actorID, projectID, ownerID := uuid.New(), uuid.New(), uuid.New()
	repository := newMemoryDiscussionRepository(&DiscussionProjectAccess{ID: projectID, Status: project.ProjectStatusInProgress, OwnerID: &ownerID, IsActiveMember: true})
	service := NewDiscussionService(repository)
	request := CreateDiscussionRequest{Title: "  Rencana rilis  ", Body: "  Perlu sinkronisasi  "}

	first, created, err := service.Create(context.Background(), actorID, "platform", "request-1", request)
	if err != nil || !created {
		t.Fatalf("first create should persist: created=%v error=%v", created, err)
	}
	second, created, err := service.Create(context.Background(), actorID, "platform", "request-1", request)
	if err != nil || created || first.ID != second.ID {
		t.Fatalf("retry should return original record: first=%#v second=%#v created=%v error=%v", first, second, created, err)
	}
	if _, _, err := service.Create(context.Background(), actorID, "platform", "request-1", CreateDiscussionRequest{Title: "Konten lain", Body: "Isi lain"}); !errors.Is(err, sharedErrors.ErrConflict) {
		t.Fatalf("reusing key with different content should conflict, got %v", err)
	}
	if len(repository.discussions) != 1 {
		t.Fatalf("retry must not create duplicates, got %d records", len(repository.discussions))
	}
}

func TestCreateDiscussionRejectsClosedProjectAndOutsider(t *testing.T) {
	actorID, projectID := uuid.New(), uuid.New()
	service := NewDiscussionService(newMemoryDiscussionRepository(&DiscussionProjectAccess{ID: projectID, Status: project.ProjectStatusCompleted, IsActiveMember: true}))
	if _, _, err := service.Create(context.Background(), actorID, "platform", "request-1", CreateDiscussionRequest{Title: "Topik", Body: "Isi"}); !errors.Is(err, sharedErrors.ErrConflict) {
		t.Fatalf("completed project should reject writes, got %v", err)
	}

	outsider := NewDiscussionService(newMemoryDiscussionRepository(&DiscussionProjectAccess{ID: projectID, Status: project.ProjectStatusInProgress}))
	if _, _, err := outsider.Create(context.Background(), actorID, "platform", "request-2", CreateDiscussionRequest{Title: "Topik", Body: "Isi"}); !errors.Is(err, sharedErrors.ErrNotFound) {
		t.Fatalf("outsider should not be able to create discussion, got %v", err)
	}
}

func TestDiscussionPageAndPageSizeDefaults(t *testing.T) {
	if got := normalizeDiscussionPage(0); got != 1 {
		t.Fatalf("expected page one as default, got %d", got)
	}
	if got := normalizeDiscussionPage(3); got != 3 {
		t.Fatalf("expected explicit page to be preserved, got %d", got)
	}
	if got := normalizeDiscussionPageSize(0); got != DiscussionDefaultPageSize {
		t.Fatalf("expected default page size, got %d", got)
	}
	if got := normalizeDiscussionPageSize(999); got != DiscussionMaxPageSize {
		t.Fatalf("expected max page size, got %d", got)
	}
}

func (tx *memoryDiscussionTx) AppendActivity(_ context.Context, input workspaceactivity.Input) error {
	if tx.repository.eventErr != nil {
		return tx.repository.eventErr
	}
	tx.repository.events = append(tx.repository.events, input)
	return nil
}

func TestDiscussionActivitiesAreIdempotentAndIgnoreNoopEdits(t *testing.T) {
	actor := uuid.New()
	repository := newMemoryDiscussionRepository(&DiscussionProjectAccess{ID: uuid.New(), Status: project.ProjectStatusInProgress, OwnerID: &actor, IsActiveMember: true})
	service := NewDiscussionService(repository)
	request := CreateDiscussionRequest{Title: "Rencana rilis", Body: "Diskusikan alur rilis"}
	topic, _, err := service.Create(context.Background(), actor, "project", "same-key", request)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = service.Create(context.Background(), actor, "project", "same-key", request); err != nil {
		t.Fatal(err)
	}
	if len(repository.events) != 1 {
		t.Fatal("retried create must retain one event")
	}
	if _, err = service.Update(context.Background(), actor, "project", topic.ID, UpdateDiscussionRequest{Title: &request.Title}); err != nil {
		t.Fatal(err)
	}
	if len(repository.events) != 1 {
		t.Fatal("noop update must not create activity")
	}
	nextTitle := "Jadwal rilis baru"
	if _, err = service.Update(context.Background(), actor, "project", topic.ID, UpdateDiscussionRequest{Title: &nextTitle}); err != nil {
		t.Fatal(err)
	}
	if len(repository.events) != 2 || repository.events[1].Type != "discussion.updated" {
		t.Fatal("real update must record an event")
	}
}

func TestDiscussionRecorderFailureRollsBackSource(t *testing.T) {
	actor := uuid.New()
	repository := newMemoryDiscussionRepository(&DiscussionProjectAccess{ID: uuid.New(), Status: project.ProjectStatusInProgress, OwnerID: &actor, IsActiveMember: true})
	repository.eventErr = errors.New("event insert failed")
	service := NewDiscussionService(repository)
	_, _, err := service.Create(context.Background(), actor, "project", "request", CreateDiscussionRequest{Title: "Rencana rilis", Body: "Diskusikan alur rilis"})
	if err == nil || len(repository.discussions) != 0 || len(repository.events) != 0 {
		t.Fatalf("source and event must roll back together: %v", err)
	}
}
