package rest_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"github.com/dudakovict/anvil/internal/adapter/rest"
	"github.com/dudakovict/anvil/internal/config"
	"github.com/dudakovict/anvil/internal/core/domain"
	"github.com/dudakovict/anvil/internal/core/port"
	mockport "github.com/dudakovict/anvil/internal/core/port/mock"
	"github.com/dudakovict/anvil/internal/platform/assert"
	"github.com/dudakovict/anvil/internal/platform/auth"
)

func TestAuthorization(t *testing.T) {
	ctrl := gomock.NewController(t)

	tokens := auth.New(auth.Config{Secret: "test-secret", TTL: time.Minute})

	userToken, err := tokens.Generate(1, string(domain.RoleUser))
	assert.Nil(t, err)

	adminToken, err := tokens.Generate(2, string(domain.RoleAdmin))
	assert.Nil(t, err)

	tests := []struct {
		name  string
		token string
		path  string
		mocks func(t *testing.T) port.UserService

		expectedStatus int
	}{
		{
			name: "no token is unauthorized",
			path: "/api/v1/users",
			mocks: func(t *testing.T) port.UserService {
				t.Helper()

				return mockport.NewMockUserService(ctrl)
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:  "user cannot list users",
			token: userToken,
			path:  "/api/v1/users",
			mocks: func(t *testing.T) port.UserService {
				t.Helper()

				return mockport.NewMockUserService(ctrl)
			},
			expectedStatus: http.StatusForbidden,
		},
		{
			name:  "admin lists users",
			token: adminToken,
			path:  "/api/v1/users",
			mocks: func(t *testing.T) port.UserService {
				t.Helper()

				svc := mockport.NewMockUserService(ctrl)

				svc.EXPECT().
					List(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return([]domain.User{}, int64(0), nil)

				return svc
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:  "user cannot get another account",
			token: userToken,
			path:  "/api/v1/users/2",
			mocks: func(t *testing.T) port.UserService {
				t.Helper()

				return mockport.NewMockUserService(ctrl)
			},
			expectedStatus: http.StatusForbidden,
		},
		{
			name:  "user gets their own account",
			token: userToken,
			path:  "/api/v1/users/1",
			mocks: func(t *testing.T) port.UserService {
				t.Helper()

				svc := mockport.NewMockUserService(ctrl)

				svc.EXPECT().Get(gomock.Any(), int64(1)).Return(domain.User{ID: 1}, nil)

				return svc
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Config{HTTP: config.HTTP{RequestTimeout: time.Minute}}
			router := rest.NewRouter(rest.Deps{Cfg: cfg, Users: tt.mocks(t), Auth: tokens})

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, tt.path, nil)
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			assert.Equal(t, rec.Code, tt.expectedStatus)
		})
	}
}

func TestCORSPreflight(t *testing.T) {
	tests := []struct {
		name   string
		origin string

		expectedAllowOrigin string
	}{
		{
			name:                "allowed origin gets CORS headers",
			origin:              "https://app.example.com",
			expectedAllowOrigin: "https://app.example.com",
		},
		{
			name:   "unknown origin gets no CORS headers",
			origin: "https://evil.example.com",
		},
	}

	cfg := config.Config{
		HTTP: config.HTTP{CORSAllowedOrigins: []string{"https://app.example.com"}},
	}
	router := rest.NewRouter(rest.Deps{Cfg: cfg})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), http.MethodOptions, "/api/v1/users", nil)
			req.Header.Set("Origin", tt.origin)
			req.Header.Set("Access-Control-Request-Method", http.MethodPost)

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			assert.Equal(t, rec.Header().Get("Access-Control-Allow-Origin"), tt.expectedAllowOrigin)
		})
	}
}
