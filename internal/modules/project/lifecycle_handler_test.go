package project

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestProjectLifecycleHTTPContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name, method, body               string
		status                           ProjectStatus
		authenticated, outsider, history bool
		expected                         int
	}{
		{"archive", "PATCH", `{"version":3,"status":"archived"}`, ProjectStatusOpen, true, false, true, 200},
		{"draft with history", "PATCH", `{"version":3,"status":"draft"}`, ProjectStatusOpen, true, false, true, 409},
		{"guest", "PATCH", `{"version":3,"status":"draft"}`, ProjectStatusOpen, false, false, false, 401},
		{"outsider", "DELETE", `{"version":3}`, ProjectStatusDraft, true, true, false, 404},
		{"missing version", "PATCH", `{"status":"draft"}`, ProjectStatusOpen, true, false, false, 400},
		{"unsupported target", "PATCH", `{"version":3,"status":"completed"}`, ProjectStatusOpen, true, false, false, 400},
		{"unknown field", "DELETE", `{"version":3,"force":true}`, ProjectStatusDraft, true, false, false, 400},
		{"delete open", "DELETE", `{"version":3}`, ProjectStatusOpen, true, false, false, 409},
		{"delete draft", "DELETE", `{"version":3}`, ProjectStatusDraft, true, false, false, 200},
	} {
		t.Run(test.name, func(t *testing.T) {
			service, tx := lifecycleFixture(test.status)
			tx.hasHistory = test.history
			router := gin.New()
			if test.authenticated {
				router.Use(func(c *gin.Context) {
					actorID := tx.actor.ID
					if test.outsider {
						actorID = uuid.New()
					}
					c.Set("user_id", actorID.String())
					c.Next()
				})
			}
			handler := NewProjectHandler(service)
			router.PATCH("/projects/:id/status", handler.ChangeStatus)
			router.DELETE("/projects/:id", handler.DeleteDraft)
			path := "/projects/" + tx.project.ID.String()
			if test.method == "PATCH" {
				path += "/status"
			}
			request := httptest.NewRequest(test.method, path, strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			if recorder.Code != test.expected {
				t.Fatalf("unexpected response %d: %s", recorder.Code, recorder.Body.String())
			}
			if test.expected >= 400 && (tx.updated || tx.deleted) {
				t.Fatal("rejected request mutated project")
			}
			if test.expected >= 400 && !strings.Contains(recorder.Body.String(), `"trace_id"`) {
				t.Fatal("error must include trace_id")
			}
		})
	}
}
