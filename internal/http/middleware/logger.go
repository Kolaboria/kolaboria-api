package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(startTime)
		statusCode := c.Writer.Status()
		clientIP := c.ClientIP()
		clientUserAgent := c.Request.UserAgent()
		requestID := c.GetString(RequestIDKey)
		traceID := c.GetString(TraceIDKey)
		if traceID == "" {
			traceID = c.Writer.Header().Get("X-Trace-ID")
		}

		ctx := c.Request.Context()
		attrs := []any{
			"status", statusCode,
			"method", c.Request.Method,
			"path", path,
			"query", query,
			"ip", clientIP,
			"latency", latency.String(),
			"user_agent", clientUserAgent,
			"request_id", requestID,
			"trace_id", traceID,
		}

		if statusCode >= 500 {
			slog.ErrorContext(ctx, "HTTP Request", attrs...)
		} else if statusCode >= 400 {
			slog.WarnContext(ctx, "HTTP Request", attrs...)
		} else {
			slog.InfoContext(ctx, "HTTP Request", attrs...)
		}
	}
}
