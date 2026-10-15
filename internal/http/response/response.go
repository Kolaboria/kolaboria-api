package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/syahrulfahmi/kolaboria-api/internal/infrastructure/logger"
)

const TraceIDKey = "trace_id"

// SuccessResponse represents the standardized success response envelope.
type SuccessResponse struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
	Meta    any    `json:"meta,omitempty"`
}

// ErrorData carries a trace identifier and the underlying exception message.
type ErrorData struct {
	TraceID    *string `json:"trace_id"`
	ErrMessage *string `json:"err_message"`
}

// ErrorResponse represents the standardized error response envelope.
type ErrorResponse struct {
	Status  int       `json:"status"`
	Message string    `json:"message"`
	Data    ErrorData `json:"data"`
}

// GetTraceID extracts the trace ID from the gin context or underlying request context,
// or generates a new one if not present.
func GetTraceID(c *gin.Context) string {
	if c == nil {
		return uuid.New().String()
	}
	if id, exists := c.Get(TraceIDKey); exists {
		if s, ok := id.(string); ok && s != "" {
			return s
		}
	}
	if id := c.GetString("TraceID"); id != "" {
		return id
	}
	if id := c.GetHeader("X-Trace-ID"); id != "" {
		c.Set(TraceIDKey, id)
		return id
	}
	if c.Request != nil && c.Request.Context() != nil {
		if id := logger.GetTraceID(c.Request.Context()); id != "" {
			c.Set(TraceIDKey, id)
			return id
		}
	}
	newID := uuid.New().String()
	c.Set(TraceIDKey, newID)
	c.Header("X-Trace-ID", newID)
	return newID
}

// Success writes a standard success JSON response.
func Success(c *gin.Context, statusCode int, message string, data any) {
	c.JSON(statusCode, SuccessResponse{
		Status:  statusCode,
		Message: message,
		Data:    data,
	})
}

// OK writes a 200 OK success JSON response.
func OK(c *gin.Context, message string, data any) {
	Success(c, http.StatusOK, message, data)
}

// SuccessWithMeta writes a success JSON response with pagination metadata.
func SuccessWithMeta(c *gin.Context, statusCode int, message string, data any, meta any) {
	c.JSON(statusCode, SuccessResponse{
		Status:  statusCode,
		Message: message,
		Data:    data,
		Meta:    meta,
	})
}

// Error writes a standard error JSON response. When only a public message is
// available, it is also used as err_message.
func Error(c *gin.Context, statusCode int, message string) {
	ErrorWithException(c, statusCode, message, nil)
}

// ErrorWithException preserves a safe client-facing message while returning
// the underlying exception text in data.err_message for diagnosis.
func ErrorWithException(c *gin.Context, statusCode int, message string, exception error) {
	traceID := GetTraceID(c)
	errMessage := message
	if exception != nil {
		errMessage = exception.Error()
	}
	c.JSON(statusCode, ErrorResponse{
		Status:  statusCode,
		Message: message,
		Data: ErrorData{
			TraceID:    &traceID,
			ErrMessage: &errMessage,
		},
	})
}

// ErrorWithTraceID writes an error JSON response with an explicit trace_id.
func ErrorWithTraceID(c *gin.Context, statusCode int, message string, traceID string) {
	if traceID == "" {
		traceID = GetTraceID(c)
	}
	c.JSON(statusCode, ErrorResponse{
		Status:  statusCode,
		Message: message,
		Data:    ErrorData{TraceID: &traceID, ErrMessage: &message},
	})
}

// ValidationError writes a 400 Bad Request error response with trace_id.
func ValidationError(c *gin.Context, errors any) {
	msg := "Validation Error"
	if s, ok := errors.(string); ok && s != "" {
		msg = s
	}
	Error(c, http.StatusBadRequest, msg)
}
