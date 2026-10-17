package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestOverviewAdmissionRejectsSaturatedRequestWithStandardErrorEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	admission := NewOverviewAdmission(true, 1, time.Second)
	admission.limit <- struct{}{}
	router := gin.New()
	router.GET("/overview", admission.Middleware(), func(c *gin.Context) { c.Status(http.StatusOK) })
	request := httptest.NewRequest(http.MethodGet, "/overview", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable || response.Header().Get("Retry-After") != "1" || response.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("unexpected overload response: status=%d headers=%v", response.Code, response.Header())
	}
	var body struct {
		Status int `json:"status"`
		Data   struct {
			TraceID    string `json:"trace_id"`
			ErrMessage string `json:"err_message"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status != http.StatusServiceUnavailable || body.Data.TraceID == "" || body.Data.ErrMessage == "" {
		t.Fatalf("error envelope is incomplete: %#v", body)
	}
}

type overviewRepositoryFunc func(context.Context, string, uuid.UUID) (*OverviewSnapshot, error)

func (repository overviewRepositoryFunc) LoadSnapshot(ctx context.Context, slug string, actorID uuid.UUID) (*OverviewSnapshot, error) {
	return repository(ctx, slug, actorID)
}

func TestOverviewCoordinatorRejectsDistinctWorkWhenSnapshotPoolIsFull(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	repository := overviewRepositoryFunc(func(ctx context.Context, _ string, _ uuid.UUID) (*OverviewSnapshot, error) {
		close(started)
		select {
		case <-release:
			return &OverviewSnapshot{Access: &WorkspaceProjectAccess{}}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})
	coordinator := NewOverviewCoordinator(repository, 1, time.Second)
	first := make(chan error, 1)
	go func() {
		_, err := coordinator.LoadSnapshot(context.Background(), "project-a", uuid.New())
		first <- err
	}()
	<-started
	if _, err := coordinator.LoadSnapshot(context.Background(), "project-b", uuid.New()); !errors.Is(err, ErrOverviewOverloaded) {
		t.Fatalf("expected bounded worker rejection, got %v", err)
	}
	close(release)
	select {
	case err := <-first:
		if err != nil {
			t.Fatalf("first snapshot failed: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("first snapshot did not release its worker")
	}
}
