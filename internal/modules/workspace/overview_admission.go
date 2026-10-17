package workspace

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/syahrulfahmi/kolaboria-api/internal/http/response"
	"golang.org/x/sync/singleflight"
)

var ErrOverviewOverloaded = errors.New("workspace overview capacity is temporarily full")

type OverviewAdmission struct {
	enabled bool
	limit   chan struct{}
	timeout time.Duration
}

func NewOverviewAdmission(enabled bool, maxRequests int, requestTimeout time.Duration) *OverviewAdmission {
	if maxRequests < 1 {
		maxRequests = 64
	}
	if requestTimeout <= 0 {
		requestTimeout = 2 * time.Second
	}
	return &OverviewAdmission{enabled: enabled, limit: make(chan struct{}, maxRequests), timeout: requestTimeout}
}

func (admission *OverviewAdmission) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "private, no-store")
		if admission == nil || !admission.enabled {
			c.Header("Retry-After", "1")
			response.ErrorWithException(c, http.StatusServiceUnavailable, "Workspace overview is temporarily unavailable", errors.New("workspace overview is disabled"))
			c.Abort()
			return
		}
		select {
		case admission.limit <- struct{}{}:
			defer func() { <-admission.limit }()
		default:
			c.Header("Retry-After", "1")
			response.ErrorWithException(c, http.StatusServiceUnavailable, "Workspace overview is busy; try again shortly", ErrOverviewOverloaded)
			c.Abort()
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), admission.timeout)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

type overviewCoordinator struct {
	repository OverviewRepository
	workers    chan struct{}
	timeout    time.Duration
	requests   singleflight.Group
}

func NewOverviewCoordinator(repository OverviewRepository, maxSnapshots int, snapshotTimeout time.Duration) OverviewRepository {
	if maxSnapshots < 1 {
		maxSnapshots = 8
	}
	if snapshotTimeout <= 0 {
		snapshotTimeout = 1500 * time.Millisecond
	}
	return &overviewCoordinator{repository: repository, workers: make(chan struct{}, maxSnapshots), timeout: snapshotTimeout}
}

func (coordinator *overviewCoordinator) LoadSnapshot(ctx context.Context, slug string, actorID uuid.UUID) (*OverviewSnapshot, error) {
	key := actorID.String() + ":" + slug
	result := coordinator.requests.DoChan(key, func() (any, error) {
		select {
		case coordinator.workers <- struct{}{}:
			defer func() { <-coordinator.workers }()
		default:
			return nil, ErrOverviewOverloaded
		}
		workerCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), coordinator.timeout)
		defer cancel()
		return coordinator.repository.LoadSnapshot(workerCtx, slug, actorID)
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case value := <-result:
		if value.Err != nil {
			return nil, value.Err
		}
		snapshot, ok := value.Val.(*OverviewSnapshot)
		if !ok || snapshot == nil {
			return nil, fmt.Errorf("workspace overview coordinator received invalid snapshot")
		}
		return snapshot, nil
	}
}
