package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCORSAllowsIdempotencyKeyForDiscussionPreflight(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(CORS(nil))
	router.POST("/v1/workspaces/:projectSlug/discussions", func(c *gin.Context) {
		c.Status(http.StatusCreated)
	})

	request := httptest.NewRequest(http.MethodOptions, "/v1/workspaces/test-project/discussions", nil)
	request.Header.Set("Origin", "http://localhost:3000")
	request.Header.Set("Access-Control-Request-Method", http.MethodPost)
	request.Header.Set("Access-Control-Request-Headers", "content-type,idempotency-key")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("expected preflight status %d, got %d", http.StatusNoContent, response.Code)
	}
	if response.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("expected wildcard allow origin in development, got %q", response.Header().Get("Access-Control-Allow-Origin"))
	}

	allowedHeaders := strings.Split(response.Header().Get("Access-Control-Allow-Headers"), ",")
	for _, header := range allowedHeaders {
		if strings.EqualFold(strings.TrimSpace(header), "Idempotency-Key") {
			return
		}
	}
	t.Fatalf("expected Access-Control-Allow-Headers to include Idempotency-Key, got %q", response.Header().Get("Access-Control-Allow-Headers"))
}
