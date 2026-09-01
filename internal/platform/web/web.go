// Package web provides support for building HTTP services.
package web

import (
	"context"
	"log/slog"
	"net/http"
)

type Handler func(ctx context.Context, r *http.Request) Response

func Handle(h Handler, mw ...Middleware) http.HandlerFunc {
	h = wrapMiddleware(mw, h)

	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		resp := h(ctx, r)

		// Client disconnected or the request timed out.
		if err := ctx.Err(); err != nil {
			slog.WarnContext(ctx, "response not written", "err", err)

			return
		}

		if resp.body == nil {
			w.WriteHeader(resp.status)

			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(resp.status)

		if _, err := w.Write(resp.body); err != nil {
			slog.ErrorContext(ctx, "writing response", "err", err)
		}
	}
}
