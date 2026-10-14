package project

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestApplicationFilterUsesSearchQueryParameter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest("GET", "/?search=kolaboria", nil)

	filter, ok := parseApplicationFilter(context, false)
	if !ok {
		t.Fatal("expected search query to be accepted")
	}
	if filter.Search != "kolaboria" {
		t.Fatalf("expected search value kolaboria, got %q", filter.Search)
	}

	context.Request = httptest.NewRequest("GET", "/?q=kolaboria", nil)
	if _, ok := parseApplicationFilter(context, false); ok {
		t.Fatal("expected legacy q query to be rejected")
	}
}

func TestProjectListFilterUsesSearchQueryParameter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest("GET", "/?search=kolaboria", nil)

	filter, ok := parseProjectListFilter(context)
	if !ok {
		t.Fatal("expected search query to be accepted")
	}
	if filter.Search != "kolaboria" {
		t.Fatalf("expected search value kolaboria, got %q", filter.Search)
	}

	context.Request = httptest.NewRequest("GET", "/?q=kolaboria", nil)
	if _, ok := parseProjectListFilter(context); ok {
		t.Fatal("expected legacy q query to be rejected")
	}
}
