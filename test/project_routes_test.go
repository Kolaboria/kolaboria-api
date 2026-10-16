package test

import (
	"os"
	"strings"
	"testing"

	"github.com/syahrulfahmi/kolaboria-api/config"
	httpapi "github.com/syahrulfahmi/kolaboria-api/internal/http"
	"github.com/syahrulfahmi/kolaboria-api/internal/infrastructure/jwt"
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/health"
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/project"
)

func TestNewRouterRegistersProjectEditorEndpoints(t *testing.T) {
	router := httpapi.NewRouter(
		&config.Config{App: config.AppConfig{Env: "test"}},
		health.NewHealthHandler(),
		jwt.NewJWTService(config.JWTConfig{}),
		nil,
		nil,
		nil,
		project.NewProjectHandler(nil),
	)

	registered := make(map[string]bool)
	for _, route := range router.Routes() {
		registered[route.Method+" "+route.Path] = true
	}
	for _, endpoint := range []string{
		"POST /v1/projects",
		"PUT /v1/projects/:id",
		"PATCH /v1/projects/:id/status",
		"DELETE /v1/projects/:id",
		"POST /v1/projects/:id/publish",
		"GET /v1/projects/:id",
		"GET /v1/projects/slug/:slug",
		"GET /v1/projects/my-projects",
		"GET /v1/projects",
	} {
		if !registered[endpoint] {
			t.Errorf("router did not register %s", endpoint)
		}
	}
}

func TestProjectSwaggerDocumentsRegisteredAPIPaths(t *testing.T) {
	swagger, err := os.ReadFile("../docs/swagger.yaml")
	if err != nil {
		t.Fatal(err)
	}
	document := string(swagger)
	for _, endpoint := range []string{
		"/v1/projects:",
		"/v1/projects/{id}:",
		"/v1/projects/{id}/publish:",
		"/v1/projects/{id}/status:",
		"/v1/projects/slug/{slug}:",
		"/v1/projects/my-projects:",
	} {
		if !strings.Contains(document, endpoint) {
			t.Errorf("generated Swagger is missing registered API path %s", endpoint)
		}
	}
	if strings.Contains(document, "\n  /projects:") {
		t.Error("generated Swagger exposes project routes without the registered /v1 prefix")
	}
}
