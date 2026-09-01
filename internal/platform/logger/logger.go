// Package logger provides support for context-aware structured logging.
package logger

import (
	"context"
	"io"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel/trace"

	"github.com/dudakovict/anvil/internal/platform/requestid"
)

func New(w io.Writer, level slog.Level) *slog.Logger {
	if w == nil {
		w = os.Stderr
	}

	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})

	return slog.New(ContextHandler{handler})
}

// ContextHandler enriches each record with the trace and request ids.
type ContextHandler struct {
	slog.Handler
}

func (h ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		r.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}

	if id, ok := requestid.FromContext(ctx); ok {
		r.AddAttrs(slog.String("request_id", id))
	}

	return h.Handler.Handle(ctx, r)
}

func (h ContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return ContextHandler{h.Handler.WithAttrs(attrs)}
}

func (h ContextHandler) WithGroup(name string) slog.Handler {
	return ContextHandler{h.Handler.WithGroup(name)}
}
