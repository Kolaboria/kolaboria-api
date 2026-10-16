package test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/syahrulfahmi/kolaboria-api/config"
	httpapi "github.com/syahrulfahmi/kolaboria-api/internal/http"
	"github.com/syahrulfahmi/kolaboria-api/internal/infrastructure/jwt"
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/auth"
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/health"
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/master"
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/profile"
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/project"
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/workspace"
)

func TestAPIRoutesUseV1Prefix(t *testing.T) {
	router := httpapi.NewRouter(
		&config.Config{App: config.AppConfig{Env: "test"}},
		health.NewHealthHandler(),
		jwt.NewJWTService(config.JWTConfig{}),
		master.NewMasterHandler(nil),
		auth.NewAuthHandler(nil),
		profile.NewProfileHandler(nil),
		project.NewProjectHandler(nil, project.NewApplicationHandler(nil)),
		workspace.NewHandler(nil, workspace.NewDiscussionService(nil)),
	)

	for _, route := range router.Routes() {
		if route.Path == "/health" || strings.HasPrefix(route.Path, "/swagger/") {
			continue
		}
		if !strings.HasPrefix(route.Path, "/v1/") {
			t.Errorf("%s %s must use the /v1 prefix", route.Method, route.Path)
		}
	}

	// Protected routes must still enforce authentication at their new URLs.
	for _, path := range []string{
		"/v1/auth/me",
		"/v1/profiles/me",
		"/v1/projects/my-projects",
		"/v1/applications/my-applications",
		"/v1/workspaces/example/info",
		"/v1/workspaces/example/discussions",
		"/v1/workspaces/example/discussions/discussion-id",
		"/v1/workspaces/example/discussions/discussion-id/replies",
	} {
		t.Run(path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
			if recorder.Code != http.StatusUnauthorized {
				t.Errorf("GET %s status = %d, want 401", path, recorder.Code)
			}
		})
	}

	for path, wantStatus := range map[string]int{
		"/health":           http.StatusOK,
		"/swagger/doc.json": http.StatusOK,
		"/api/v1/auth/me":   http.StatusNotFound,
		"/v1/unknown-route": http.StatusNotFound,
	} {
		t.Run(path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
			if recorder.Code != wantStatus {
				t.Errorf("GET %s status = %d, want %d", path, recorder.Code, wantStatus)
			}
		})
	}
}
