package logger

import (
	"context"
	"log/slog"
)

// TraceHandler wraps an slog.Handler and automatically extracts trace_id and request_id
// from the context and adds them to the log record attributes if not already present.
type TraceHandler struct {
	next slog.Handler
}

// NewTraceHandler wraps the given slog.Handler with trace extraction logic.
func NewTraceHandler(next slog.Handler) *TraceHandler {
	return &TraceHandler{next: next}
}

func (h *TraceHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *TraceHandler) Handle(ctx context.Context, r slog.Record) error {
	if ctx != nil {
		hasTraceID := false
		hasRequestID := false
		r.Attrs(func(a slog.Attr) bool {
			if a.Key == "trace_id" {
				hasTraceID = true
			}
			if a.Key == "request_id" {
				hasRequestID = true
			}
			return true
		})

		if !hasTraceID {
			if traceID := GetTraceID(ctx); traceID != "" {
				r.AddAttrs(slog.String("trace_id", traceID))
			}
		}
		if !hasRequestID {
			if requestID := GetRequestID(ctx); requestID != "" {
				r.AddAttrs(slog.String("request_id", requestID))
			}
		}
	}
	return h.next.Handle(ctx, r)
}

func (h *TraceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &TraceHandler{next: h.next.WithAttrs(attrs)}
}

func (h *TraceHandler) WithGroup(name string) slog.Handler {
	return &TraceHandler{next: h.next.WithGroup(name)}
}
