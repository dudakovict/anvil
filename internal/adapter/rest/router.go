package rest

import (
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
	"github.com/riandyrn/otelchi"
	otelchimetric "github.com/riandyrn/otelchi/metric"
	httpSwagger "github.com/swaggo/http-swagger/v2"

	"github.com/dudakovict/anvil/internal/config"
	"github.com/dudakovict/anvil/internal/core/domain"
	"github.com/dudakovict/anvil/internal/core/port"
	"github.com/dudakovict/anvil/internal/platform/auth"
	"github.com/dudakovict/anvil/internal/platform/requestid"
	"github.com/dudakovict/anvil/internal/platform/web"
)

const loginRateLimit = 10

type Deps struct {
	Cfg   config.Config
	DB    Pinger
	Users port.UserService
	Auth  *auth.Auth
}

func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()

	if len(d.Cfg.HTTP.CORSAllowedOrigins) > 0 {
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins: d.Cfg.HTTP.CORSAllowedOrigins,
			AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders: []string{"Authorization", "Content-Type", requestid.Header},
			MaxAge:         300,
		}))
	}

	metricCfg := otelchimetric.NewBaseConfig(d.Cfg.OTel.ServiceName)
	r.Use(otelchi.Middleware(d.Cfg.OTel.ServiceName, otelchi.WithChiRoutes(r)))
	r.Use(otelchimetric.NewServerRequestDuration(metricCfg))
	r.Use(otelchimetric.NewServerActiveRequests(metricCfg))
	r.Use(web.RequestID())
	r.Use(web.SecureHeaders())
	r.Use(web.Log())
	r.Use(web.Recover())
	r.Use(web.MaxBytes(d.Cfg.HTTP.MaxBodyBytes))
	r.Use(web.Timeout(d.Cfg.HTTP.RequestTimeout))

	health := &healthHandler{db: d.DB}
	r.Get("/healthz", web.Handle(health.live))
	r.Get("/readyz", web.Handle(health.ready))

	users := &userHandler{svc: d.Users}
	sessions := &authHandler{svc: d.Users, auth: d.Auth}

	r.Route("/api/v1", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(httprate.LimitBy(
				loginRateLimit, time.Minute,
				keyByRemoteAddr,
				httprate.WithLimitHandler(tooManyRequests),
			))
			sessions.routes(r)
		})

		users.publicRoutes(r)

		r.Group(func(r chi.Router) {
			r.Use(web.Auth(d.Auth))
			users.protectedRoutes(r)

			r.Group(func(r chi.Router) {
				r.Use(web.Authorize(string(domain.RoleAdmin)))
				users.adminRoutes(r)
			})
		})
	})

	r.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))

	return r
}

func keyByRemoteAddr(r *http.Request) (string, error) {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)

	return ip, err
}

func tooManyRequests(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusTooManyRequests)
	_, _ = w.Write([]byte(`{"error":"too many requests"}`))
}
