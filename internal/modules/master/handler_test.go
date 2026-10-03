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
	calls        int
	query        ListQuery
	err          error
	searchResult []SearchLocationResponse
	detailResult *LocationDetailResponse
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

func (f *readerFake) SearchLocations(_ context.Context, _ string) ([]SearchLocationResponse, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	if f.searchResult == nil {
		return []SearchLocationResponse{}, nil
	}
	return f.searchResult, nil
}

func (f *readerFake) GetLocationDetail(_ context.Context, _ int64) (*LocationDetailResponse, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.detailResult, nil
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

func TestHandlerSearchLocationsSuccess(t *testing.T) {
	reader := &readerFake{
		searchResult: []SearchLocationResponse{
			{
				VillageID:  101,
				Village:    "Dago",
				District:   "Coblong",
				Regency:    "Kota Bandung",
				Province:   "Jawa Barat",
				PostalCode: "40135",
			},
		},
	}
	handler := NewMasterHandler(reader)
	router := newTestMasterRouter(handler)
	request := httptest.NewRequest(http.MethodGet, "/master-data/locations/search?q=Dago", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusOK, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"villageId":101`) || !strings.Contains(response.Body.String(), `"postalCode":"40135"`) {
		t.Fatalf("expected location response body: %s", response.Body.String())
	}
}

func TestHandlerSearchLocationsRejectsRepeatedQuery(t *testing.T) {
	reader := &readerFake{}
	handler := NewMasterHandler(reader)
	router := newTestMasterRouter(handler)
	request := httptest.NewRequest(http.MethodGet, "/master-data/locations/search?q=foo&q=bar", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusBadRequest, response.Body.String())
	}
	if reader.calls != 0 {
		t.Fatalf("service calls = %d, want 0", reader.calls)
	}
}

func TestHandlerSearchLocationsMapsServiceErrors(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{name: "bad request", err: sharedErrors.ErrBadRequest, status: http.StatusBadRequest},
		{name: "service unavailable", err: sharedErrors.ErrServiceUnavailable, status: http.StatusServiceUnavailable},
		{name: "request deadline", err: context.DeadlineExceeded, status: http.StatusGatewayTimeout},
		{name: "request canceled", err: context.Canceled, status: http.StatusRequestTimeout},
		{name: "internal error", err: errors.New("db error"), status: http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := newTestMasterRouter(NewMasterHandler(&readerFake{err: tt.err}))
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/master-data/locations/search?q=Dago", nil))
			if response.Code != tt.status {
				t.Fatalf("status = %d, want %d; body: %s", response.Code, tt.status, response.Body.String())
			}
		})
	}
}

func TestHandlerGetLocationDetailSuccess(t *testing.T) {
	reader := &readerFake{
		detailResult: &LocationDetailResponse{
			Province:   LocationInfo{ID: 1, Name: "Jawa Barat"},
			Regency:    LocationInfo{ID: 10, Name: "Kota Bandung"},
			District:   LocationInfo{ID: 100, Name: "Coblong"},
			Village:    LocationInfo{ID: 1000, Name: "Dago"},
			PostalCode: "40135",
		},
	}
	handler := NewMasterHandler(reader)
	router := newTestMasterRouter(handler)
	request := httptest.NewRequest(http.MethodGet, "/master-data/locations/1000", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusOK, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"postalCode":"40135"`) || !strings.Contains(response.Body.String(), `"name":"Dago"`) {
		t.Fatalf("expected detail response body: %s", response.Body.String())
	}
}

func TestHandlerGetLocationDetailRejectsInvalidID(t *testing.T) {
	for _, id := range []string{"0", "-1", "abc", "12a", ""} {
		t.Run("id="+id, func(t *testing.T) {
			reader := &readerFake{}
			handler := NewMasterHandler(reader)
			router := newTestMasterRouter(handler)
			request := httptest.NewRequest(http.MethodGet, "/master-data/locations/"+id, nil)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			if response.Code != http.StatusBadRequest && response.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 400 or 404; body: %s", response.Code, response.Body.String())
			}
			if id != "" && response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body: %s", response.Code, response.Body.String())
			}
			if reader.calls != 0 {
				t.Fatalf("service calls = %d, want 0", reader.calls)
			}
		})
	}
}

func TestHandlerGetLocationDetailNotFoundAndErrors(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{name: "not found", err: sharedErrors.ErrNotFound, status: http.StatusNotFound},
		{name: "bad request", err: sharedErrors.ErrBadRequest, status: http.StatusBadRequest},
		{name: "service unavailable", err: sharedErrors.ErrServiceUnavailable, status: http.StatusServiceUnavailable},
		{name: "request deadline", err: context.DeadlineExceeded, status: http.StatusGatewayTimeout},
		{name: "request canceled", err: context.Canceled, status: http.StatusRequestTimeout},
		{name: "internal error", err: errors.New("unexpected error"), status: http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := newTestMasterRouter(NewMasterHandler(&readerFake{err: tt.err}))
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/master-data/locations/1000", nil))
			if response.Code != tt.status {
				t.Fatalf("status = %d, want %d; body: %s", response.Code, tt.status, response.Body.String())
			}
		})
	}
}

