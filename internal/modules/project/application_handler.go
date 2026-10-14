package project

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/syahrulfahmi/kolaboria-api/internal/http/response"
	sharedErrors "github.com/syahrulfahmi/kolaboria-api/internal/shared/errors"
	"gorm.io/gorm"
)

const maxApplicationRequestBytes = 192 * 1024

type ApplicationHandler struct{ service *ApplicationService }

func NewApplicationHandler(service *ApplicationService) *ApplicationHandler {
	return &ApplicationHandler{service: service}
}

// ApplicantSummary returns global application counts and current role capacity for the project owner.
// @Summary Get applicant management summary
// @Tags Applications
// @Produce json
// @Security BearerAuth
// @Param id path string true "Project UUID"
// @Success 200 {object} response.SuccessResponse{data=ApplicantSummaryResponse}
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/projects/{id}/applicants/summary [get]
func (h *ApplicationHandler) ApplicantSummary(c *gin.Context) {
	actorID, ok := requiredActorID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Authentication is required")
		return
	}
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil || projectID == uuid.Nil {
		response.Error(c, http.StatusBadRequest, "Invalid project identifier")
		return
	}
	markPrivateResponse(c)
	result, err := h.service.GetApplicantSummary(c.Request.Context(), actorID, projectID)
	if err != nil {
		writeApplicationError(c, err)
		return
	}
	response.OK(c, "Applicant summary retrieved", result)
}

// ListProjectApplicants returns a filtered page of private application data to the project owner.
// @Summary List project applicants
// @Tags Applications
// @Produce json
// @Security BearerAuth
// @Param id path string true "Project UUID"
// @Param page query int false "Page number"
// @Param limit query int false "Items per page (max 100; default 5)"
// @Param search query string false "Search application and role data"
// @Param statuses query string false "CSV: pending,accepted,rejected,withdrawn"
// @Param project_role_ids query string false "CSV project role UUIDs"
// @Param sort query string false "newest or oldest"
// @Success 200 {object} response.SuccessResponse{data=[]ApplicantListItemResponse,meta=ProjectPaginationMeta}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/projects/{id}/applicants [get]
func (h *ApplicationHandler) ListProjectApplicants(c *gin.Context) {
	actorID, ok := requiredActorID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Authentication is required")
		return
	}
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil || projectID == uuid.Nil {
		response.Error(c, http.StatusBadRequest, "Invalid project identifier")
		return
	}
	filter, ok := parseApplicationFilter(c, true)
	if !ok {
		response.Error(c, http.StatusBadRequest, "Invalid application filters")
		return
	}
	markPrivateResponse(c)
	items, meta, err := h.service.ListProjectApplicants(c.Request.Context(), actorID, projectID, filter)
	if err != nil {
		writeApplicationError(c, err)
		return
	}
	response.SuccessWithMeta(c, http.StatusOK, "Applicants retrieved", items, meta)
}

// ApplicationDetail returns one application to its applicant or the project owner.
// @Summary Get application detail
// @Tags Applications
// @Produce json
// @Security BearerAuth
// @Param id path string true "Application UUID"
// @Success 200 {object} response.SuccessResponse{data=ApplicationDetailResponse}
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/applications/{id} [get]
func (h *ApplicationHandler) ApplicationDetail(c *gin.Context) {
	actorID, ok := requiredActorID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Authentication is required")
		return
	}
	applicationID, err := uuid.Parse(c.Param("id"))
	if err != nil || applicationID == uuid.Nil {
		response.Error(c, http.StatusBadRequest, "Invalid application identifier")
		return
	}
	markPrivateResponse(c)
	result, err := h.service.GetApplicationDetail(c.Request.Context(), actorID, applicationID)
	if err != nil {
		writeApplicationError(c, err)
		return
	}
	response.OK(c, "Application retrieved", result)
}

// ReviewApplication accepts or rejects a pending application as the verified project owner.
// @Summary Review application
// @Tags Applications
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Application UUID"
// @Param request body ReviewApplicationRequest true "Decision and optional applicant-visible note"
// @Success 200 {object} response.SuccessResponse{data=ApplicationDetailResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Router /api/v1/applications/{id}/review [patch]
func (h *ApplicationHandler) ReviewApplication(c *gin.Context) {
	actorID, ok := requiredActorID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Authentication is required")
		return
	}
	applicationID, err := uuid.Parse(c.Param("id"))
	if err != nil || applicationID == uuid.Nil {
		response.Error(c, http.StatusBadRequest, "Invalid application identifier")
		return
	}
	body, err := readApplicationBody(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid application review request")
		return
	}
	var request ReviewApplicationRequest
	if err := decodeSingleJSON(body, &request); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid application review request")
		return
	}
	markPrivateResponse(c)
	result, err := h.service.ReviewApplication(c.Request.Context(), actorID, applicationID, request)
	if err != nil {
		writeApplicationError(c, err)
		return
	}
	response.OK(c, "Application reviewed", result)
}

// MyApplicationSummary returns application counts for the authenticated applicant.
// @Summary Get my application summary
// @Tags Applications
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.SuccessResponse{data=MyApplicationSummaryResponse}
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Router /api/v1/applications/my-applications/summary [get]
func (h *ApplicationHandler) MyApplicationSummary(c *gin.Context) {
	actorID, ok := requiredActorID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Authentication is required")
		return
	}
	markPrivateResponse(c)
	result, err := h.service.GetMyApplicationSummary(c.Request.Context(), actorID)
	if err != nil {
		writeApplicationError(c, err)
		return
	}
	response.OK(c, "My application summary retrieved", result)
}

// ListMyApplications returns the authenticated applicant's filtered application history.
// @Summary List my applications
// @Tags Applications
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number"
// @Param limit query int false "Items per page (max 100; default 6)"
// @Param search query string false "Search project, role, author, and submission"
// @Param statuses query string false "CSV: pending,accepted,rejected,withdrawn"
// @Param origins query string false "CSV: personal,community,experiment,client"
// @Param sort query string false "newest or oldest"
// @Success 200 {object} response.SuccessResponse{data=[]MyApplicationListItemResponse,meta=ProjectPaginationMeta}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Router /api/v1/applications/my-applications [get]
func (h *ApplicationHandler) ListMyApplications(c *gin.Context) {
	actorID, ok := requiredActorID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Authentication is required")
		return
	}
	filter, ok := parseApplicationFilter(c, false)
	if !ok {
		response.Error(c, http.StatusBadRequest, "Invalid application filters")
		return
	}
	markPrivateResponse(c)
	items, meta, err := h.service.ListMyApplications(c.Request.Context(), actorID, filter)
	if err != nil {
		writeApplicationError(c, err)
		return
	}
	response.SuccessWithMeta(c, http.StatusOK, "My applications retrieved", items, meta)
}

// WithdrawApplication lets an applicant withdraw their own pending application.
// @Summary Withdraw application
// @Tags Applications
// @Produce json
// @Security BearerAuth
// @Param id path string true "Application UUID"
// @Success 200 {object} response.SuccessResponse{data=ApplicationDetailResponse}
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Router /api/v1/applications/{id}/withdraw [post]
func (h *ApplicationHandler) WithdrawApplication(c *gin.Context) {
	actorID, ok := requiredActorID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Authentication is required")
		return
	}
	applicationID, err := uuid.Parse(c.Param("id"))
	if err != nil || applicationID == uuid.Nil {
		response.Error(c, http.StatusBadRequest, "Invalid application identifier")
		return
	}
	body, err := readApplicationBody(c)
	if err != nil || len(bytes.TrimSpace(body)) > 0 {
		response.Error(c, http.StatusBadRequest, "Withdraw request body must be empty")
		return
	}
	markPrivateResponse(c)
	result, err := h.service.WithdrawApplication(c.Request.Context(), actorID, applicationID)
	if err != nil {
		writeApplicationError(c, err)
		return
	}
	response.OK(c, "Application withdrawn", result)
}

func parseApplicationFilter(c *gin.Context, owner bool) (ApplicationListFilter, bool) {
	allowed := map[string]bool{"page": true, "limit": true, "search": true, "statuses": true, "sort": true}
	if owner {
		allowed["project_role_ids"] = true
	} else {
		allowed["origins"] = true
	}
	for key, values := range c.Request.URL.Query() {
		if !allowed[key] || len(values) != 1 {
			return ApplicationListFilter{}, false
		}
	}
	filter := ApplicationListFilter{Page: 1, Search: c.Query("search"), Sort: ApplicationSortNewest}
	if owner {
		filter.Limit = 5
	} else {
		filter.Limit = 6
	}
	if raw, present := c.Request.URL.Query()["page"]; present {
		if raw[0] == "" {
			return ApplicationListFilter{}, false
		}
		value, err := strconv.Atoi(raw[0])
		if err != nil || value < 1 {
			return ApplicationListFilter{}, false
		}
		filter.Page = value
	}
	if raw, present := c.Request.URL.Query()["limit"]; present {
		if raw[0] == "" {
			return ApplicationListFilter{}, false
		}
		value, err := strconv.Atoi(raw[0])
		if err != nil || value < 1 {
			return ApplicationListFilter{}, false
		}
		filter.Limit = value
	}
	if raw, present := c.Request.URL.Query()["sort"]; present {
		if raw[0] == "" {
			return ApplicationListFilter{}, false
		}
		filter.Sort = ApplicationSort(raw[0])
	}
	statuses, ok := parseCSV(c.Query("statuses"))
	if !ok {
		return ApplicationListFilter{}, false
	}
	seenStatus := map[ApplicationStatus]bool{}
	for _, value := range statuses {
		status := ApplicationStatus(value)
		if status != ApplicationStatusPending && status != ApplicationStatusAccepted && status != ApplicationStatusRejected && status != ApplicationStatusWithdrawn {
			return ApplicationListFilter{}, false
		}
		if !seenStatus[status] {
			seenStatus[status] = true
			filter.Statuses = append(filter.Statuses, status)
		}
	}
	if owner {
		values, ok := parseCSV(c.Query("project_role_ids"))
		if !ok {
			return ApplicationListFilter{}, false
		}
		seen := map[uuid.UUID]bool{}
		for _, value := range values {
			id, err := uuid.Parse(value)
			if err != nil || id == uuid.Nil {
				return ApplicationListFilter{}, false
			}
			if !seen[id] {
				seen[id] = true
				filter.ProjectRoleIDs = append(filter.ProjectRoleIDs, id)
			}
		}
	} else {
		values, ok := parseCSV(c.Query("origins"))
		if !ok {
			return ApplicationListFilter{}, false
		}
		seen := map[string]bool{}
		for _, value := range values {
			if value != "personal" && value != "community" && value != "experiment" && value != "client" {
				return ApplicationListFilter{}, false
			}
			if !seen[value] {
				seen[value] = true
				filter.Origins = append(filter.Origins, value)
			}
		}
	}
	return filter, true
}

func parseCSV(raw string) ([]string, bool) {
	if strings.TrimSpace(raw) == "" {
		return []string{}, true
	}
	reader := csv.NewReader(strings.NewReader(raw))
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1
	values, err := reader.Read()
	if err != nil || len(values) == 0 {
		return nil, false
	}
	if _, err := reader.Read(); err != io.EOF {
		return nil, false
	}
	for i := range values {
		values[i] = strings.TrimSpace(values[i])
		if values[i] == "" {
			return nil, false
		}
	}
	return values, true
}

func readApplicationBody(c *gin.Context) ([]byte, error) {
	if c.Request.Body == nil {
		return []byte{}, nil
	}
	return io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, maxApplicationRequestBytes))
}

func decodeSingleJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return errors.New("request must contain one JSON value")
	}
	return nil
}

func markPrivateResponse(c *gin.Context) { c.Header("Cache-Control", "private, no-store") }

func writeApplicationError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, sharedErrors.ErrUnauthorized):
		response.Error(c, http.StatusUnauthorized, "Authentication is required")
	case errors.Is(err, sharedErrors.ErrForbidden):
		response.Error(c, http.StatusForbidden, "You are not allowed to access this application data")
	case errors.Is(err, sharedErrors.ErrNotFound), errors.Is(err, gorm.ErrRecordNotFound):
		response.Error(c, http.StatusNotFound, "Application not found")
	case errors.Is(err, sharedErrors.ErrBadRequest):
		response.Error(c, http.StatusBadRequest, "Application data is invalid")
	case errors.Is(err, sharedErrors.ErrConflict):
		response.Error(c, http.StatusConflict, "Application state changed or conflicts with current project state")
	default:
		response.Error(c, http.StatusInternalServerError, "Unable to process application request")
	}
}
