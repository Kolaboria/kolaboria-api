package workspace

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/project"
)

func TestWorkspaceInfoHandlerUsesSnakeCaseContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	actorID := uuid.New()
	ownerID := actorID
	roleID := uuid.New()
	repository := infoRepositoryFake{project: &project.ProjectAggregate{
		Project: project.Project{ID: uuid.New(), Slug: "platform", Version: 9, Status: project.ProjectStatusOpen, OwnerID: &ownerID, Title: "Platform"},
		Roles:   []project.ProjectRoleAggregate{{ProjectRole: project.ProjectRole{ID: roleID, Capacity: 3}, FilledCapacity: 2}},
	}}
	router := gin.New()
	router.GET("/workspaces/:projectSlug/info", func(c *gin.Context) {
		c.Set("user_id", actorID.String())
		NewHandler(NewService(repository)).GetProjectInfo(c)
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest("GET", "/workspaces/platform/info", nil))
	if recorder.Code != 200 {
		t.Fatalf("expected success response, got %d: %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, key := range []string{`"position_capacity_summary"`, `"active_contributor_count":2`, `"remaining_capacity":1`, `"readiness_state":"to_fill"`, `"ownership"`, `"owner_id"`} {
		if !strings.Contains(body, key) {
			t.Fatalf("response must include %s: %s", key, body)
		}
	}
	for _, forbidden := range []string{`"result_summary"`, `"task_count"`, `"can_start_project":false`} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("response must not include %s: %s", forbidden, body)
		}
	}
}

func TestWorkspaceInfoHandlerReturnsExceptionAndTraceID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	actorID := uuid.New()
	repository := infoRepositoryFake{findErr: errors.New("database connection refused")}
	router := gin.New()
	router.GET("/workspaces/:projectSlug/info", func(c *gin.Context) {
		c.Set("user_id", actorID.String())
		NewHandler(NewService(repository)).GetProjectInfo(c)
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest("GET", "/workspaces/platform/info", nil))
	if recorder.Code != 500 {
		t.Fatalf("expected internal server error, got %d: %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"trace_id"`) || !strings.Contains(body, `"err_message":"load workspace project by slug: database connection refused"`) {
		t.Fatalf("expected exception details and trace ID in error data: %s", body)
	}
	if !strings.Contains(body, `"message":"Unable to load workspace project info"`) {
		t.Fatalf("expected safe public error message: %s", body)
	}
}

func TestWorkspaceInfoHandlerRequiresActorIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/workspaces/:projectSlug/info", NewHandler(NewService(infoRepositoryFake{})).GetProjectInfo)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest("GET", "/workspaces/platform/info", nil))
	if recorder.Code != 401 || !strings.Contains(recorder.Body.String(), `"err_message"`) {
		t.Fatalf("expected unauthorized error envelope with err_message: %d %s", recorder.Code, recorder.Body.String())
	}
}
