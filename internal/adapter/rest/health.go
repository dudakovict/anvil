package rest

import (
	"context"
	"net/http"
	"time"

	"github.com/dudakovict/anvil/internal/platform/web"
)

type Pinger interface {
	PingContext(ctx context.Context) error
}

type healthHandler struct {
	db Pinger
}

type healthStatus struct {
	Status string `json:"status"`
}

// live godoc
//
//	@Summary	Liveness probe
//	@Tags		health
//	@Produce	json
//	@Success	200	{object}	rest.healthStatus
//	@Router		/healthz [get]
func (h *healthHandler) live(ctx context.Context, _ *http.Request) web.Response {
	return web.Respond(ctx, http.StatusOK, healthStatus{Status: "ok"})
}

// ready godoc
//
//	@Summary	Readiness probe (checks database connectivity)
//	@Tags		health
//	@Produce	json
//	@Success	200	{object}	rest.healthStatus
//	@Failure	503	{object}	web.ErrorResponse
//	@Router		/readyz [get]
func (h *healthHandler) ready(ctx context.Context, _ *http.Request) web.Response {
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err := h.db.PingContext(pingCtx); err != nil {
		return web.RespondError(ctx, http.StatusServiceUnavailable, "database unreachable")
	}

	return web.Respond(ctx, http.StatusOK, healthStatus{Status: "ok"})
}
