package master

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"net"
	"strings"
	"unicode/utf8"

	sharedErrors "github.com/syahrulfahmi/kolaboria-api/internal/shared/errors"
)

const (
	defaultPage  = 1
	defaultLimit = 50
	maxLimit     = 100
	maxQuerySize = 100
)

type MasterService struct {
	repository MasterRepository
}

func NewMasterService(repository MasterRepository) *MasterService {
	return &MasterService{repository: repository}
}

func (s *MasterService) List(ctx context.Context, catalog Catalog, query ListQuery) ([]ItemResponse, PaginationMeta, error) {
	if _, ok := catalogTable(catalog); !ok {
		return nil, PaginationMeta{}, sharedErrors.ErrBadRequest
	}
	if query.Page < 1 || query.Limit < 1 || query.Limit > maxLimit {
		return nil, PaginationMeta{}, sharedErrors.ErrBadRequest
	}
	search := strings.TrimSpace(query.Q)
	if !utf8.ValidString(search) || strings.ContainsRune(search, '\x00') || utf8.RuneCountInString(search) > maxQuerySize {
		return nil, PaginationMeta{}, sharedErrors.ErrBadRequest
	}
	maxInt := int(^uint(0) >> 1)
	if query.Page-1 > maxInt/query.Limit {
		return nil, PaginationMeta{}, sharedErrors.ErrBadRequest
	}
	entities, total, err := s.repository.List(ctx, catalog, ListFilter{
		Offset: (query.Page - 1) * query.Limit,
		Limit:  query.Limit,
		Q:      search,
	})
	if err != nil {
		if isDatabaseUnavailable(err) {
			return nil, PaginationMeta{}, fmt.Errorf("%w: %w", sharedErrors.ErrServiceUnavailable, err)
		}
		return nil, PaginationMeta{}, err
	}
	var totalPages int64
	if total > 0 {
		totalPages = (total-1)/int64(query.Limit) + 1
	}
	return mapCatalogItems(entities), PaginationMeta{Page: query.Page, Limit: query.Limit, Total: total, TotalPages: totalPages}, nil
}

func (s *MasterService) SearchLocations(ctx context.Context, query string) ([]SearchLocationResponse, error) {
	search := strings.TrimSpace(query)
	if !utf8.ValidString(search) || strings.ContainsRune(search, '\x00') || utf8.RuneCountInString(search) > maxQuerySize {
		return nil, sharedErrors.ErrBadRequest
	}
	if utf8.RuneCountInString(search) < 2 {
		return []SearchLocationResponse{}, nil
	}
	results, err := s.repository.SearchLocations(ctx, search)
	if err != nil {
		if isDatabaseUnavailable(err) {
			return nil, fmt.Errorf("%w: %w", sharedErrors.ErrServiceUnavailable, err)
		}
		return nil, err
	}
	if results == nil {
		results = []SearchLocationResponse{}
	}
	return results, nil
}

func (s *MasterService) GetLocationDetail(ctx context.Context, villageID int64) (*LocationDetailResponse, error) {
	if villageID <= 0 {
		return nil, sharedErrors.ErrBadRequest
	}
	detail, err := s.repository.GetLocationDetail(ctx, villageID)
	if err != nil {
		if isDatabaseUnavailable(err) {
			return nil, fmt.Errorf("%w: %w", sharedErrors.ErrServiceUnavailable, err)
		}
		return nil, err
	}
	return detail, nil
}


func isDatabaseUnavailable(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if errors.Is(err, driver.ErrBadConn) || errors.Is(err, sql.ErrConnDone) {
		return true
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "database is closed") || strings.Contains(message, "connection is already closed") {
		return true
	}
	var networkError net.Error
	if errors.As(err, &networkError) {
		return true
	}
	var stateError interface{ SQLState() string }
	if errors.As(err, &stateError) {
		state := stateError.SQLState()
		return strings.HasPrefix(state, "08") || state == "57P01" || state == "57P02" || state == "57P03"
	}
	return false
}
