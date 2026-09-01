package web

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/dudakovict/anvil/internal/platform/validate"
)

type Response struct {
	status int
	body   []byte
}

type ErrorResponse struct {
	Error  string               `json:"error"`
	Fields validate.FieldErrors `json:"fields,omitempty"`
}

func Respond(ctx context.Context, status int, body any) Response {
	if body == nil {
		return Response{status: status}
	}

	b, err := json.Marshal(body)
	if err != nil {
		slog.ErrorContext(ctx, "encoding response body", "err", err)

		return RespondError(ctx, http.StatusInternalServerError, "internal server error")
	}

	return Response{
		status: status,
		body:   b,
	}
}

func RespondError(ctx context.Context, status int, msg string, cause ...error) Response {
	if len(cause) > 0 {
		slog.ErrorContext(ctx, "handler error", "err", cause[0], "status", status)
	}

	b, _ := json.Marshal(ErrorResponse{Error: msg})

	return Response{
		status: status,
		body:   b,
	}
}
