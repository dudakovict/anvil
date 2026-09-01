package web

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/dudakovict/anvil/internal/platform/auth"
	"github.com/dudakovict/anvil/internal/platform/requestid"
)

type Middleware func(Handler) Handler

// wrapMiddleware wraps h so the first middleware in mw runs outermost.
func wrapMiddleware(mw []Middleware, h Handler) Handler {
	for _, m := range slices.Backward(mw) {
		if m != nil {
			h = m(h)
		}
	}

	return h
}

func RequestID() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get(requestid.Header)
			if id == "" {
				id = uuid.NewString()
				r.Header.Set(requestid.Header, id)
			}

			w.Header().Set(requestid.Header, id)

			ctx := requestid.WithContext(r.Context(), id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func Log() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := newStatusResponseWriter(w)
			ctx := r.Context()

			defer func() {
				slog.LogAttrs(ctx, slog.LevelInfo, "request",
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.String("query", r.URL.RawQuery),
					slog.Int("status", sw.status),
					slog.Float64("duration_s", time.Since(start).Seconds()),
					slog.String("remote_addr", r.RemoteAddr),
					slog.String("x_forwarded_for", r.Header.Get("X-Forwarded-For")),
					slog.String("user_agent", r.UserAgent()),
				)
			}()

			next.ServeHTTP(sw, r)
		})
	}
}

func SecureHeaders() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Content-Security-Policy", "frame-ancestors 'none'")
			w.Header().Set("Cache-Control", "no-store")

			next.ServeHTTP(w, r)
		})
	}
}

func Auth(a *auth.Auth) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok {
				unauthorized(w)

				return
			}

			claims, err := a.Verify(token)
			if err != nil {
				unauthorized(w)

				return
			}

			ctx := auth.WithContext(r.Context(), claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func Authorize(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := auth.FromContext(r.Context())
			if !ok || !slices.Contains(roles, claims.Role) {
				forbidden(w)

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
}

func forbidden(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(`{"error":"forbidden"}`))
}

func MaxBytes(n int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, n)
			next.ServeHTTP(w, r)
		})
	}
}

func Recover() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			defer func() {
				rec := recover()
				if rec == nil {
					return
				}

				if rec == http.ErrAbortHandler { //nolint:errorlint // sentinel comparison per net/http docs
					panic(rec)
				}

				slog.ErrorContext(ctx, "panic recovered",
					"panic", rec,
					"stack", string(debug.Stack()),
				)
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error":"internal server error"}`))
			}()

			next.ServeHTTP(w, r)
		})
	}
}

func Timeout(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.TimeoutHandler(next, d, `{"error":"request timed out"}`)
	}
}

type statusResponseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func newStatusResponseWriter(w http.ResponseWriter) *statusResponseWriter {
	return &statusResponseWriter{
		ResponseWriter: w,
		status:         http.StatusOK,
	}
}

func (w *statusResponseWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.status = code
		w.wroteHeader = true
	}

	w.ResponseWriter.WriteHeader(code)
}

func (w *statusResponseWriter) Write(b []byte) (int, error) {
	w.wroteHeader = true

	return w.ResponseWriter.Write(b)
}

func (w *statusResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
