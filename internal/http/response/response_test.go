package response

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestErrorIncludesExceptionMessageAndTraceIDInData(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest("GET", "/", nil)
	context.Set(TraceIDKey, "trace-123")

	Error(context, 400, "Invalid project status request")

	var payload struct {
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if payload.Data["trace_id"] != "trace-123" {
		t.Fatalf("expected trace id in data, got %#v", payload.Data)
	}
	if payload.Data["err_message"] != "Invalid project status request" {
		t.Fatalf("expected err_message in data, got %#v", payload.Data)
	}
	if _, exists := payload.Data["message"]; exists {
		t.Fatalf("legacy data.message must not be present, got %#v", payload.Data)
	}
}

func TestErrorWithTraceIDIncludesSafeMessageInData(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest("GET", "/", nil)

	ErrorWithTraceID(context, 409, "Project has changed", "trace-456")

	var payload struct {
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if payload.Data["trace_id"] != "trace-456" || payload.Data["err_message"] != "Project has changed" {
		t.Fatalf("expected explicit trace id and err_message in data, got %#v", payload.Data)
	}
}

func TestErrorWithExceptionKeepsPublicMessageAndReturnsExceptionInData(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest("GET", "/", nil)

	ErrorWithException(context, 500, "Unable to process request", errors.New("database connection refused"))

	var payload struct {
		Message string            `json:"message"`
		Data    map[string]string `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if payload.Message != "Unable to process request" {
		t.Fatalf("public message should stay safe, got %q", payload.Message)
	}
	if payload.Data["err_message"] != "database connection refused" {
		t.Fatalf("expected exception details in err_message, got %#v", payload.Data)
	}
}
