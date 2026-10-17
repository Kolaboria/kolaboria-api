package workspace

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCompletionHandlerRetainsExceptionAndTraceID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repository, actor, _ := completionFixture()
	repository.eventErr = errors.New("completion event write failed")
	handler := NewHandler(NewService(infoRepositoryFake{})).WithCompletion(NewCompletionService(repository))
	router := gin.New()
	router.POST("/workspaces/:projectSlug/completion", func(c *gin.Context) { c.Set("user_id", actor.String()); handler.CompleteWorkspaceProject(c) })
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/workspaces/platform/completion", strings.NewReader(`{"version":7,"task_version":18,"result_summary":"Hasil ditinjau"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	if recorder.Code != 500 || !strings.Contains(recorder.Body.String(), `"trace_id"`) || !strings.Contains(recorder.Body.String(), `"err_message":"complete workspace project: completion event write failed"`) {
		t.Fatalf("expected existing error envelope, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestCompletionHandlerRejectsMemberEvenWithTaskManagementGrant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repository, actor, _ := completionFixture()
	repository.access.IsOwner = false
	repository.access.ActiveMember = true
	handler := NewHandler(NewService(infoRepositoryFake{})).WithCompletion(NewCompletionService(repository))
	router := gin.New()
	router.POST("/workspaces/:projectSlug/completion", func(c *gin.Context) { c.Set("user_id", actor.String()); handler.CompleteWorkspaceProject(c) })
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/workspaces/platform/completion", strings.NewReader(`{"version":7,"task_version":18,"result_summary":"Hasil ditinjau"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	if recorder.Code != 403 {
		t.Fatalf("expected owner-only closure, got %d: %s", recorder.Code, recorder.Body.String())
	}
}
