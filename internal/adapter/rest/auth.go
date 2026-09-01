package rest

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/dudakovict/anvil/internal/core/port"
	"github.com/dudakovict/anvil/internal/platform/auth"
	"github.com/dudakovict/anvil/internal/platform/web"
)

type authHandler struct {
	svc  port.UserService
	auth *auth.Auth
}

func (h *authHandler) routes(r chi.Router) {
	r.Post("/auth/login", web.Handle(h.login))
}

// login godoc
//
//	@Summary	Log in with email and password
//	@Tags		auth
//	@Accept		json
//	@Produce	json
//	@Param		request	body		rest.LoginRequest	true	"credentials"
//	@Success	200		{object}	rest.TokenResponse
//	@Failure	400		{object}	web.ErrorResponse
//	@Failure	401		{object}	web.ErrorResponse
//	@Failure	429		{object}	web.ErrorResponse
//	@Failure	500		{object}	web.ErrorResponse
//	@Router		/auth/login [post]
func (h *authHandler) login(ctx context.Context, r *http.Request) web.Response {
	req, err := web.Decode[LoginRequest](r)
	if err != nil {
		return decodeError(ctx, err)
	}

	u, err := h.svc.Authenticate(ctx, req.Email, req.Password)
	if err != nil {
		return errorResponse(ctx, err)
	}

	token, err := h.auth.Generate(u.ID, string(u.Role))
	if err != nil {
		return errorResponse(ctx, err)
	}

	return web.Respond(ctx, http.StatusOK, TokenResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   int64(h.auth.TTL().Seconds()),
	})
}
