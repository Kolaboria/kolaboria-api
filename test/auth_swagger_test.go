package test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/syahrulfahmi/kolaboria-api/docs"
)

func TestSwaggerAuthenticatedOperationsHaveBearerDefinition(t *testing.T) {
	file, err := os.ReadFile("../docs/swagger.json")
	if err != nil {
		t.Fatal(err)
	}
	for name, raw := range map[string][]byte{"swagger.json": file, "compiled Swagger": []byte(docs.SwaggerInfo.ReadDoc())} {
		t.Run(name, func(t *testing.T) {
			var spec struct {
				SecurityDefinitions map[string]struct {
					Type string `json:"type"`
					In   string `json:"in"`
					Name string `json:"name"`
				} `json:"securityDefinitions"`
				Paths map[string]map[string]struct {
					Security []map[string][]string `json:"security"`
				} `json:"paths"`
			}
			if err := json.Unmarshal(raw, &spec); err != nil {
				t.Fatal(err)
			}
			bearer, exists := spec.SecurityDefinitions["BearerAuth"]
			if !exists || bearer.Type != "apiKey" || bearer.In != "header" || bearer.Name != "Authorization" {
				t.Fatal("BearerAuth must define the Authorization header for Swagger Authorize")
			}
			me, exists := spec.Paths["/api/v1/auth/me"]["get"]
			if !exists || len(me.Security) == 0 {
				t.Fatal("GET /api/v1/auth/me must require BearerAuth")
			}
			if _, exists := me.Security[0]["BearerAuth"]; !exists {
				t.Fatal("GET /api/v1/auth/me bearer requirement missing")
			}
			for path, operations := range spec.Paths {
				for method, operation := range operations {
					for _, requirement := range operation.Security {
						for scheme := range requirement {
							if _, exists := spec.SecurityDefinitions[scheme]; !exists {
								t.Errorf("%s %s references undefined security scheme %s", method, path, scheme)
							}
						}
					}
				}
			}
		})
	}
}
