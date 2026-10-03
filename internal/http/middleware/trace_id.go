package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/syahrulfahmi/kolaboria-api/internal/infrastructure/logger"
)

const TraceIDKey = "trace_id"

// TraceID middleware extracts the trace ID from the X-Trace-ID header or generates a new UUID.
// It sets the trace ID in the gin context, response header, and request context.
func TraceID() gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := c.GetHeader("X-Trace-ID")
		if traceID == "" {
			traceID = uuid.New().String()
		}

		c.Set(TraceIDKey, traceID)
		c.Header("X-Trace-ID", traceID)

		ctx := logger.ContextWithTraceID(c.Request.Context(), traceID)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}
