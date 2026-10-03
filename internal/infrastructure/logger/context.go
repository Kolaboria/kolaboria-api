package logger

import (
	"context"
	"log/slog"
)

type contextKey string

const (
	TraceIDContextKey   contextKey = "trace_id"
	RequestIDContextKey contextKey = "request_id"
)

// ContextWithTraceID returns a new context containing the trace ID.
func ContextWithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, TraceIDContextKey, traceID)
}

// GetTraceID extracts the trace ID from the context. Returns empty string if not found.
func GetTraceID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(TraceIDContextKey).(string); ok {
		return v
	}
	return ""
}

// ContextWithRequestID returns a new context containing the request ID.
func ContextWithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, RequestIDContextKey, requestID)
}

// GetRequestID extracts the request ID from the context. Returns empty string if not found.
func GetRequestID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(RequestIDContextKey).(string); ok {
		return v
	}
	return ""
}

// FromContext returns an *slog.Logger with trace_id and request_id pre-bound as attributes.
func FromContext(ctx context.Context) *slog.Logger {
	l := slog.Default()
	if ctx == nil {
		return l
	}
	if traceID := GetTraceID(ctx); traceID != "" {
		l = l.With("trace_id", traceID)
	}
	if requestID := GetRequestID(ctx); requestID != "" {
		l = l.With("request_id", requestID)
	}
	return l
}
