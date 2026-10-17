package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	sharedErrors "github.com/syahrulfahmi/kolaboria-api/internal/shared/errors"
)

type activityMemoryRepository struct {
	rows        []ActivityProjection
	total       int64
	err         error
	page, limit int
}

func (r *activityMemoryRepository) LoadPage(_ context.Context, _ string, _ uuid.UUID, page, limit int) ([]ActivityProjection, int64, error) {
	r.page, r.limit = page, limit
	return r.rows, r.total, r.err
}

func TestActivityReviewDescribesReviewerWithoutAttributingAssigneeWork(t *testing.T) {
	self := true
	row := ActivityProjection{WorkspaceActivityEvent: WorkspaceActivityEvent{EventType: "task.status_changed", Payload: []byte(`{"from":"review","to":"done"}`)},
		ActorName: "Reviewer", SubjectLabel: "Alur onboarding", SubjectAvailable: true, SelfReview: &self}
	item := mapActivity(row)
	if item.Actor.DisplayName != "Reviewer" || !strings.Contains(item.Description, "menerima hasil review") || !strings.Contains(item.Description, "self-review") {
		t.Fatalf("incorrect review attribution: %#v", item)
	}
	row.Payload = []byte(`{"from":"in_progress","to":"review"}`)
	item = mapActivity(row)
	if item.Changes.SelfReview != nil || strings.Contains(item.Description, "menerima") {
		t.Fatal("request must not inherit a cycle's later acceptance")
	}
}

func TestActivityReferencesSpecificTaskForLegacyEvents(t *testing.T) {
	taskID := uuid.MustParse("3c5d577c-80b5-4683-9984-46a9abcf02ba")
	for _, test := range []struct {
		name, historicalTitle, currentTitle, expected string
	}{
		{"legacy event", "", "Setup Repository FE", "Setup Repository FE"},
		{"historical title retained", "Rancangan awal", "Rancangan terbaru", "Rancangan awal"},
		{"blank historical title", "   ", "Setup Repository FE", "Setup Repository FE"},
		{"missing title identifies task", "", "", "Task #3c5d577c"},
	} {
		t.Run(test.name, func(t *testing.T) {
			row := ActivityProjection{
				WorkspaceActivityEvent: WorkspaceActivityEvent{
					TaskID: &taskID, EventType: "task.status_changed",
					Payload: []byte(`{"from":"todo","to":"in_progress"}`),
				},
				SubjectLabel: test.historicalTitle, CurrentTaskTitle: test.currentTitle, SubjectAvailable: true,
			}
			item := mapActivity(row)
			if item.Subject.Label != test.expected || !strings.Contains(item.Description, "“"+test.expected+"”") {
				t.Fatalf("activity must identify its task: %#v", item)
			}
			row.SubjectDeleted = true
			item = mapActivity(row)
			if item.Subject.Availability != "deleted" || strings.Contains(item.Description, test.expected) {
				t.Fatal("deleted source must not expose its resolved task title")
			}
		})
	}
}

func TestActivityDeletedSourceDoesNotResurfaceContent(t *testing.T) {
	row := ActivityProjection{WorkspaceActivityEvent: WorkspaceActivityEvent{EventType: "discussion.reply_created"},
		SubjectLabel: "Rahasia yang telah dihapus", SubjectAvailable: true, SubjectDeleted: true}
	item := mapActivity(row)
	if strings.Contains(item.Description, "Rahasia") || item.Subject.Availability != "deleted" || item.Actor.DisplayName == "" {
		t.Fatalf("deleted projection leaked content: %#v", item)
	}
	row.EventType = "task.status_changed"
	row.Payload = []byte(`{"from":"todo","to":"closed","close_reason":"Rahasia"}`)
	if strings.Contains(mapActivity(row).Description, "Rahasia") {
		t.Fatal("deleted task must not leak close reason")
	}
}

func TestActivityPreservesPaginationTotalAndAccessErrors(t *testing.T) {
	r := &activityMemoryRepository{rows: []ActivityProjection{}, total: 101}
	service := NewActivityService(r)
	items, meta, err := service.List(context.Background(), uuid.New(), "project", 2, 50)
	if err != nil || items == nil || meta.Total != 101 || meta.TotalPages != 3 || r.page != 2 || r.limit != 50 {
		t.Fatalf("invalid standard pagination: %#v %v", meta, err)
	}
	r.err = sharedErrors.ErrNotFound
	if _, _, err = service.List(context.Background(), uuid.New(), "project", 1, 50); !errors.Is(err, sharedErrors.ErrNotFound) {
		t.Fatal("access error must be preserved")
	}
}

func TestActivityHandlerUsesStandardListAndExceptionEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := &activityMemoryRepository{rows: []ActivityProjection{}, total: 0}
	handler := &Handler{activities: NewActivityService(r)}
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("user_id", uuid.NewString()); c.Next() })
	router.GET("/workspaces/:projectSlug/activities", handler.ListWorkspaceActivities)
	for _, test := range []struct {
		query  string
		status int
	}{{"?page=1&limit=50", 200}, {"?page=0", 400}} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("GET", "/workspaces/project/activities"+test.query, nil))
		var body map[string]json.RawMessage
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if w.Code != test.status {
			t.Fatalf("unexpected status: %s", w.Body.String())
		}
		if w.Code == 200 {
			if string(body["data"]) != "[]" || body["meta"] == nil {
				t.Fatal("list must use array plus standard meta")
			}
		} else if !strings.Contains(string(body["data"]), "err_message") || !strings.Contains(string(body["data"]), "trace_id") {
			t.Fatal("error must preserve exception and trace")
		}
	}
}
