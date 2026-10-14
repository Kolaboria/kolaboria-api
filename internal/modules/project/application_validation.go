package project

import (
	"net/url"
	"strings"

	"github.com/google/uuid"
	sharedErrors "github.com/syahrulfahmi/kolaboria-api/internal/shared/errors"
)

func validateOwnerApplicationFilter(filter ApplicationListFilter) (ApplicationListFilter, error) {
	filter, err := normalizeApplicationFilter(filter, 5)
	if err != nil {
		return ApplicationListFilter{}, err
	}
	if len(filter.Origins) > 0 {
		return ApplicationListFilter{}, sharedErrors.ErrBadRequest
	}
	filter.ProjectRoleIDs = uniqueUUIDs(filter.ProjectRoleIDs)
	if len(filter.ProjectRoleIDs) > 100 || !uniqueNonZeroUUIDs(filter.ProjectRoleIDs) {
		return ApplicationListFilter{}, sharedErrors.ErrBadRequest
	}
	return filter, nil
}

func validateMyApplicationFilter(filter ApplicationListFilter) (ApplicationListFilter, error) {
	filter, err := normalizeApplicationFilter(filter, 6)
	if err != nil {
		return ApplicationListFilter{}, err
	}
	if len(filter.ProjectRoleIDs) > 0 {
		return ApplicationListFilter{}, sharedErrors.ErrBadRequest
	}
	for _, origin := range filter.Origins {
		if origin != "personal" && origin != "community" && origin != "experiment" && origin != "client" {
			return ApplicationListFilter{}, sharedErrors.ErrBadRequest
		}
	}
	filter.Origins = uniqueApplicationStrings(filter.Origins)
	return filter, nil
}

func normalizeApplicationFilter(filter ApplicationListFilter, defaultLimit int) (ApplicationListFilter, error) {
	if filter.Page == 0 {
		filter.Page = 1
	}
	if filter.Limit == 0 {
		filter.Limit = defaultLimit
	}
	if filter.Sort == "" {
		filter.Sort = ApplicationSortNewest
	}
	if filter.Page < 1 || filter.Limit < 1 || filter.Limit > 100 || runeLength(filter.Search) > 200 ||
		(filter.Sort != ApplicationSortNewest && filter.Sort != ApplicationSortOldest) {
		return ApplicationListFilter{}, sharedErrors.ErrBadRequest
	}
	filter.Search = strings.TrimSpace(filter.Search)
	filter.Statuses = uniqueStatuses(filter.Statuses)
	for _, status := range filter.Statuses {
		if status != ApplicationStatusPending && status != ApplicationStatusAccepted && status != ApplicationStatusRejected && status != ApplicationStatusWithdrawn {
			return ApplicationListFilter{}, sharedErrors.ErrBadRequest
		}
	}
	return filter, nil
}

func normalizeReviewRequest(request ReviewApplicationRequest) (ReviewApplicationRequest, error) {
	if request.Status != ApplicationStatusAccepted && request.Status != ApplicationStatusRejected {
		return ReviewApplicationRequest{}, sharedErrors.ErrBadRequest
	}
	if request.ReviewerNote == nil {
		return request, nil
	}
	note := strings.TrimSpace(*request.ReviewerNote)
	if runeLength(note) > 2000 {
		return ReviewApplicationRequest{}, sharedErrors.ErrBadRequest
	}
	if note == "" {
		request.ReviewerNote = nil
	} else {
		request.ReviewerNote = &note
	}
	return request, nil
}

func requireApplicationReader(actor *ProjectActor) error {
	if err := requireEligibleActor(actor); err != nil {
		return err
	}
	if actor.AccountType == nil || *actor.AccountType != "talent" {
		return sharedErrors.ErrForbidden
	}
	return nil
}

func requireApplicationWriter(actor *ProjectActor) error {
	if err := requireApplicationReader(actor); err != nil {
		return err
	}
	if actor.EmailVerifiedAt == nil {
		return sharedErrors.ErrForbidden
	}
	return nil
}

func requireApplicationManager(actorID uuid.UUID, project *Project) error {
	if project == nil || project.OwnerID == nil || *project.OwnerID != actorID {
		return sharedErrors.ErrNotFound
	}
	return nil
}

func validateApplicationPortfolio(links []string) error {
	if len(links) > 10 {
		return sharedErrors.ErrBadRequest
	}
	for _, link := range links {
		value := strings.TrimSpace(link)
		parsed, err := url.ParseRequestURI(value)
		if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" || len(value) > 2048 {
			return sharedErrors.ErrBadRequest
		}
	}
	return nil
}

func uniqueStatuses(values []ApplicationStatus) []ApplicationStatus {
	seen := make(map[ApplicationStatus]struct{}, len(values))
	result := make([]ApplicationStatus, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func uniqueApplicationStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
