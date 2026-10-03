package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"github.com/syahrulfahmi/kolaboria-api/internal/http/response"
)

func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				requestID := c.GetString(RequestIDKey)
				traceID := c.GetString(TraceIDKey)
				if traceID == "" {
					traceID = c.Writer.Header().Get("X-Trace-ID")
				}

				slog.ErrorContext(c.Request.Context(), "panic recovered",
					"error", err,
					"request_id", requestID,
					"trace_id", traceID,
					"stack", string(debug.Stack()),
				)

				response.Error(c, http.StatusInternalServerError, "Internal Server Error")
				c.Abort()
			}
		}()
		c.Next()
	}
}
