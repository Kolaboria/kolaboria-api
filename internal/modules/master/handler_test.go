package master

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	sharedErrors "github.com/syahrulfahmi/kolaboria-api/internal/shared/errors"
)

type readerFake struct {
	calls int
	query ListQuery
	err   error
}

func newTestMasterRouter(handler *MasterHandler) *gin.Engine {
	router := gin.New()
	RegisterMasterRoutes(router, handler)
	return router
}

func (f *readerFake) List(_ context.Context, _ Catalog, query ListQuery) ([]ItemResponse, PaginationMeta, error) {
	f.calls++
	f.query = query
	return []ItemResponse{}, PaginationMeta{Page: query.Page, Limit: query.Limit}, f.err
}

func TestHandlerMapsServiceFailuresWithoutLeakingDatabaseDetails(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{name: "database unavailable", err: sharedErrors.ErrServiceUnavailable, status: http.StatusServiceUnavailable},
		{name: "database schema failure", err: errors.New("relation secret_table does not exist"), status: http.StatusInternalServerError},
		{name: "request deadline", err: context.DeadlineExceeded, status: http.StatusGatewayTimeout},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := newTestMasterRouter(NewMasterHandler(&readerFake{err: tt.err}))
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/master-data/skills", nil))
			if response.Code != tt.status {
				t.Fatalf("status = %d, want %d: %s", response.Code, tt.status, response.Body.String())
			}
			if strings.Contains(response.Body.String(), "secret_table") {
				t.Fatalf("response leaked internal database detail: %s", response.Body.String())
			}
		})
	}
}

func TestHandlerRejectsRepeatedOrMalformedQueryBeforeCallingService(t *testing.T) {
	for _, query := range []string{"page=1&page=2", "limit=0", "page=999999999999999999999999999999", "q=%FF", "q=%00", "q=" + strings.Repeat("a", 101)} {
		t.Run(query, func(t *testing.T) {
			reader := &readerFake{}
			handler := NewMasterHandler(reader)
			router := newTestMasterRouter(handler)
			request := httptest.NewRequest(http.MethodGet, "/master-data/skills?"+query, nil)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusBadRequest, response.Body.String())
			}
			if reader.calls != 0 {
				t.Fatalf("service calls = %d, want 0", reader.calls)
			}
		})
	}
}

func TestHandlerAppliesDefaultsAndTrimsSearch(t *testing.T) {
	reader := &readerFake{}
	handler := NewMasterHandler(reader)
	router := newTestMasterRouter(handler)
	request := httptest.NewRequest(http.MethodGet, "/master-data/skills?q=%20%20Vue%20%20", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusOK, response.Body.String())
	}
	if reader.query != (ListQuery{Page: 1, Limit: 50, Q: "Vue"}) {
		t.Fatalf("query = %+v", reader.query)
	}
}
