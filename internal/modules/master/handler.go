package master

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/syahrulfahmi/kolaboria-api/internal/http/response"
	sharedErrors "github.com/syahrulfahmi/kolaboria-api/internal/shared/errors"
)

type MasterReader interface {
	List(context.Context, Catalog, ListQuery) ([]ItemResponse, PaginationMeta, error)
	SearchLocations(context.Context, string) ([]SearchLocationResponse, error)
	GetLocationDetail(context.Context, int64) (*LocationDetailResponse, error)
}

type MasterHandler struct {
	service MasterReader
}

func NewMasterHandler(service MasterReader) *MasterHandler {
	return &MasterHandler{service: service}
}

func (h *MasterHandler) list(c *gin.Context, catalog Catalog) {
	query, err := parseListQuery(c.Request.URL.RawQuery)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid query parameters")
		return
	}
	items, meta, err := h.service.List(c.Request.Context(), catalog, query)
	if err != nil {
		switch {
		case errors.Is(err, sharedErrors.ErrBadRequest):
			response.Error(c, http.StatusBadRequest, "Invalid query parameters")
		case errors.Is(err, sharedErrors.ErrServiceUnavailable):
			slog.ErrorContext(c.Request.Context(), "master service unavailable", "catalog", string(catalog), "error", err)
			response.Error(c, http.StatusServiceUnavailable, "Service unavailable")
		case errors.Is(err, context.DeadlineExceeded):
			slog.WarnContext(c.Request.Context(), "master request deadline exceeded", "catalog", string(catalog), "error", err)
			response.Error(c, http.StatusGatewayTimeout, "Request timed out")
		case errors.Is(err, context.Canceled):
			slog.WarnContext(c.Request.Context(), "master request canceled", "catalog", string(catalog), "error", err)
			response.Error(c, http.StatusRequestTimeout, "Request canceled")
		default:
			slog.ErrorContext(c.Request.Context(), "master service internal error", "catalog", string(catalog), "error", err)
			response.Error(c, http.StatusInternalServerError, "Internal server error")
		}
		return
	}
	response.SuccessWithMeta(c, http.StatusOK, "Success", items, meta)
}

// ListContributionRoles godoc
// @Summary List contribution roles
// @Tags master-data
// @Produce json
// @Param page query int false "Page number" default(1) minimum(1)
// @Param limit query int false "Page size" default(50) minimum(1) maximum(100)
// @Param q query string false "Literal case-insensitive substring search"
// @Success 200 {object} ListResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 408 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Failure 503 {object} response.ErrorResponse
// @Failure 504 {object} response.ErrorResponse
// @Router /api/v1/master-data/contribution-roles [get]
func (h *MasterHandler) ListContributionRoles(c *gin.Context) {
	h.list(c, CatalogContributionRoles)
}

// ListSkills godoc
// @Summary List skills
// @Tags master-data
// @Produce json
// @Param page query int false "Page number" default(1) minimum(1)
// @Param limit query int false "Page size" default(50) minimum(1) maximum(100)
// @Param q query string false "Literal case-insensitive substring search"
// @Success 200 {object} ListResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 408 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Failure 503 {object} response.ErrorResponse
// @Failure 504 {object} response.ErrorResponse
// @Router /api/v1/master-data/skills [get]
func (h *MasterHandler) ListSkills(c *gin.Context) {
	h.list(c, CatalogSkills)
}

// ListTools godoc
// @Summary List tools
// @Tags master-data
// @Produce json
// @Param page query int false "Page number" default(1) minimum(1)
// @Param limit query int false "Page size" default(50) minimum(1) maximum(100)
// @Param q query string false "Literal case-insensitive substring search"
// @Success 200 {object} ListResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 408 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Failure 503 {object} response.ErrorResponse
// @Failure 504 {object} response.ErrorResponse
// @Router /api/v1/master-data/tools [get]
func (h *MasterHandler) ListTools(c *gin.Context) {
	h.list(c, CatalogTools)
}

// SearchLocations godoc
// @Summary Search locations
// @Description Search locations (villages, districts, regencies, provinces) by keyword
// @Tags master-data
// @Produce json
// @Param q query string true "Search keyword (min 2 characters)"
// @Success 200 {object} SearchLocationListResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 408 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Failure 503 {object} response.ErrorResponse
// @Failure 504 {object} response.ErrorResponse
// @Router /api/v1/master-data/locations/search [get]
func (h *MasterHandler) SearchLocations(c *gin.Context) {
	values, err := url.ParseQuery(c.Request.URL.RawQuery)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid query parameters")
		return
	}
	if len(values["q"]) > 1 {
		response.Error(c, http.StatusBadRequest, "Invalid query parameters")
		return
	}
	q := c.Query("q")
	results, err := h.service.SearchLocations(c.Request.Context(), q)
	if err != nil {
		switch {
		case errors.Is(err, sharedErrors.ErrBadRequest):
			response.Error(c, http.StatusBadRequest, "Invalid query parameters")
		case errors.Is(err, sharedErrors.ErrServiceUnavailable):
			slog.ErrorContext(c.Request.Context(), "master locations search service unavailable", "error", err)
			response.Error(c, http.StatusServiceUnavailable, "Service unavailable")
		case errors.Is(err, context.DeadlineExceeded):
			slog.WarnContext(c.Request.Context(), "master locations search request deadline exceeded", "error", err)
			response.Error(c, http.StatusGatewayTimeout, "Request timed out")
		case errors.Is(err, context.Canceled):
			slog.WarnContext(c.Request.Context(), "master locations search request canceled", "error", err)
			response.Error(c, http.StatusRequestTimeout, "Request canceled")
		default:
			slog.ErrorContext(c.Request.Context(), "master locations search internal error", "error", err)
			response.Error(c, http.StatusInternalServerError, "Internal server error")
		}
		return
	}
	response.Success(c, http.StatusOK, "Locations retrieved successfully", results)
}

// GetLocationDetail godoc
// @Summary Get location detail
// @Description Get detailed hierarchy (province, regency, district, village) by village ID
// @Tags master-data
// @Produce json
// @Param village_id path int true "Village ID"
// @Success 200 {object} LocationDetailResultResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 408 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Failure 503 {object} response.ErrorResponse
// @Failure 504 {object} response.ErrorResponse
// @Router /api/v1/master-data/locations/{village_id} [get]
func (h *MasterHandler) GetLocationDetail(c *gin.Context) {
	villageIDStr := c.Param("village_id")
	villageID, err := parsePositiveInt64(villageIDStr)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid village ID")
		return
	}

	detail, err := h.service.GetLocationDetail(c.Request.Context(), villageID)
	if err != nil {
		switch {
		case errors.Is(err, sharedErrors.ErrNotFound):
			response.Error(c, http.StatusNotFound, "Location not found")
		case errors.Is(err, sharedErrors.ErrBadRequest):
			response.Error(c, http.StatusBadRequest, "Invalid village ID")
		case errors.Is(err, sharedErrors.ErrServiceUnavailable):
			slog.ErrorContext(c.Request.Context(), "master location detail service unavailable", "village_id", villageID, "error", err)
			response.Error(c, http.StatusServiceUnavailable, "Service unavailable")
		case errors.Is(err, context.DeadlineExceeded):
			slog.WarnContext(c.Request.Context(), "master location detail request deadline exceeded", "village_id", villageID, "error", err)
			response.Error(c, http.StatusGatewayTimeout, "Request timed out")
		case errors.Is(err, context.Canceled):
			slog.WarnContext(c.Request.Context(), "master location detail request canceled", "village_id", villageID, "error", err)
			response.Error(c, http.StatusRequestTimeout, "Request canceled")
		default:
			slog.ErrorContext(c.Request.Context(), "master location detail internal error", "village_id", villageID, "error", err)
			response.Error(c, http.StatusInternalServerError, "Internal server error")
		}
		return
	}
	response.Success(c, http.StatusOK, "Location detail retrieved successfully", detail)
}

func parsePositiveInt64(value string) (int64, error) {
	if value == "" {
		return 0, sharedErrors.ErrBadRequest
	}
	for _, digit := range value {
		if digit < '0' || digit > '9' {
			return 0, sharedErrors.ErrBadRequest
		}
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 1 {
		return 0, sharedErrors.ErrBadRequest
	}
	return parsed, nil
}

func parseListQuery(rawQuery string) (ListQuery, error) {
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return ListQuery{}, err
	}
	for _, key := range []string{"page", "limit", "q"} {
		if len(values[key]) > 1 {
			return ListQuery{}, sharedErrors.ErrBadRequest
		}
	}
	query := ListQuery{Page: defaultPage, Limit: defaultLimit}
	if values.Has("page") {
		query.Page, err = parsePositiveInteger(values.Get("page"))
		if err != nil {
			return ListQuery{}, err
		}
	}
	if values.Has("limit") {
		query.Limit, err = parsePositiveInteger(values.Get("limit"))
		if err != nil || query.Limit > maxLimit {
			return ListQuery{}, sharedErrors.ErrBadRequest
		}
	}
	maxInt := int(^uint(0) >> 1)
	if query.Page-1 > maxInt/query.Limit {
		return ListQuery{}, sharedErrors.ErrBadRequest
	}
	if values.Has("q") {
		query.Q = strings.TrimSpace(values.Get("q"))
		if !utf8.ValidString(query.Q) || strings.ContainsRune(query.Q, '\x00') || utf8.RuneCountInString(query.Q) > maxQuerySize {
			return ListQuery{}, sharedErrors.ErrBadRequest
		}
	}
	return query, nil
}

func parsePositiveInteger(value string) (int, error) {
	if value == "" {
		return 0, sharedErrors.ErrBadRequest
	}
	for _, digit := range value {
		if digit < '0' || digit > '9' {
			return 0, sharedErrors.ErrBadRequest
		}
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return 0, sharedErrors.ErrBadRequest
	}
	return parsed, nil
}
