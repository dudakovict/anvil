// Package requestid provides support for propagating request ids.
package requestid

import "context"

const (
	Header = "X-Request-Id"

	key ctxKey = 0
)

type ctxKey int

func WithContext(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, key, id)
}

func FromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(key).(string)

	return id, ok
}
