package workspace

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/project"
)

func TestDiscussionHandlerUsesSnakeCaseAndCurrentPermissions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	actorID, projectID, ownerID, authorID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	repository := newMemoryDiscussionRepository(&DiscussionProjectAccess{ID: projectID, Status: project.ProjectStatusCompleted, OwnerID: &ownerID, IsActiveMember: true})
	repository.listItems = []DiscussionProjection{{
		Discussion: Discussion{ID: uuid.New(), ProjectID: projectID, AuthorID: authorID, Title: "Topik", Body: "Isi", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), LastActivityAt: time.Now().UTC()},
		AuthorName: "Ayu", ReplyCount: 2, ParticipantCount: 3,
	}}
	handler := NewHandler(NewService(infoRepositoryFake{}), NewDiscussionService(repository))
	router := gin.New()
	router.GET("/workspaces/:projectSlug/discussions", func(c *gin.Context) {
		c.Set("user_id", actorID.String())
		handler.ListDiscussions(c)
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest("GET", "/workspaces/platform/discussions", nil))
	if recorder.Code != 200 {
		t.Fatalf("expected discussion list success, got %d: %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, expected := range []string{`"body_preview"`, `"reply_count":2`, `"participant_count":3`, `"can_edit":false`, `"can_delete":false`, `"meta"`} {
		if !strings.Contains(body, expected) {
			t.Errorf("discussion response must include %s: %s", expected, body)
		}
	}
	for _, expected := range []string{`"page":1`, `"limit":20`, `"total":1`, `"total_pages":1`} {
		if !strings.Contains(body, expected) {
			t.Errorf("discussion list meta must include %s: %s", expected, body)
		}
	}
	for _, removed := range []string{`"next_cursor"`, `"has_more"`, `"can_create"`, `"can_reply"`} {
		if strings.Contains(body, removed) {
			t.Errorf("discussion list meta must not include %s: %s", removed, body)
		}
	}
	if strings.Contains(body, `"projectId"`) || strings.Contains(body, `"replyCount"`) {
		t.Fatalf("response fields must be snake_case: %s", body)
	}
}

func TestDiscussionHandlerReturnsExistingExceptionAndTraceID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	actorID := uuid.New()
	repository := newMemoryDiscussionRepository(nil)
	repository.resolveErr = errors.New("database read timeout")
	handler := NewHandler(NewService(infoRepositoryFake{}), NewDiscussionService(repository))
	router := gin.New()
	router.GET("/workspaces/:projectSlug/discussions/:discussionID", func(c *gin.Context) {
		c.Set("user_id", actorID.String())
		handler.GetDiscussion(c)
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest("GET", "/workspaces/platform/discussions/"+uuid.NewString(), nil))
	if recorder.Code != 500 {
		t.Fatalf("expected internal server error, got %d: %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"message":"Unable to process workspace discussion"`) || !strings.Contains(body, `"err_message":"resolve workspace discussion project: database read timeout"`) || !strings.Contains(body, `"trace_id"`) {
		t.Fatalf("expected safe message and exception trace envelope: %s", body)
	}
}
