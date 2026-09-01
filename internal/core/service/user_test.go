package service_test

import (
	"context"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/dudakovict/anvil/internal/core/domain"
	"github.com/dudakovict/anvil/internal/core/port"
	mockport "github.com/dudakovict/anvil/internal/core/port/mock"
	"github.com/dudakovict/anvil/internal/core/service"
	"github.com/dudakovict/anvil/internal/platform/argon2id"
	"github.com/dudakovict/anvil/internal/platform/assert"
	"github.com/dudakovict/anvil/internal/platform/query"
)

func TestUserServiceCreate(t *testing.T) {
	ctrl := gomock.NewController(t)

	tests := []struct {
		name     string
		mocks    func(t *testing.T) port.UserRepository
		email    string
		userName string
		password string

		expectedError error
	}{
		{
			name: "invalid email returns ErrInvalidUser without any repo call",
			mocks: func(t *testing.T) port.UserRepository {
				t.Helper()

				return mockport.NewMockUserRepository(ctrl)
			},
			email:         "not-an-email",
			userName:      "John Doe",
			password:      "s3cret-pass",
			expectedError: domain.ErrInvalidUser,
		},
		{
			name: "empty name returns ErrInvalidUser without any repo call",
			mocks: func(t *testing.T) port.UserRepository {
				t.Helper()

				return mockport.NewMockUserRepository(ctrl)
			},
			email:         "john.doe@example.com",
			password:      "s3cret-pass",
			expectedError: domain.ErrInvalidUser,
		},
		{
			name: "short password returns ErrInvalidUser without any repo call",
			mocks: func(t *testing.T) port.UserRepository {
				t.Helper()

				return mockport.NewMockUserRepository(ctrl)
			},
			email:         "john.doe@example.com",
			userName:      "John Doe",
			password:      "short",
			expectedError: domain.ErrInvalidUser,
		},
		{
			name: "duplicate email returns ErrEmailTaken",
			mocks: func(t *testing.T) port.UserRepository {
				t.Helper()

				repo := mockport.NewMockUserRepository(ctrl)

				repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(domain.User{}, domain.ErrEmailTaken)

				return repo
			},
			email:         "john.doe@example.com",
			userName:      "John Doe",
			password:      "s3cret-pass",
			expectedError: domain.ErrEmailTaken,
		},
		{
			name: "success hashes the password and returns the stored user",
			mocks: func(t *testing.T) port.UserRepository {
				t.Helper()

				repo := mockport.NewMockUserRepository(ctrl)

				repo.EXPECT().
					Create(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, u domain.User) (domain.User, error) {
						u.ID = 1

						return u, nil
					})

				return repo
			},
			email:    "john.doe@example.com",
			userName: "John Doe",
			password: "s3cret-pass",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := service.NewUserService(tt.mocks(t))

			u, err := svc.Create(context.Background(), tt.email, tt.userName, tt.password)
			if tt.expectedError != nil {
				assert.ErrorIs(t, err, tt.expectedError)

				return
			}

			assert.Nil(t, err)
			assert.Equal(t, u.Email, tt.email)
			assert.Equal(t, u.Role, domain.RoleUser)
			assert.True(t, u.ID > 0)
			assert.True(t, u.PasswordHash != "")
			assert.NotEqual(t, u.PasswordHash, tt.password)
		})
	}
}

func TestUserServiceAuthenticate(t *testing.T) {
	ctrl := gomock.NewController(t)

	hash, err := argon2id.CreateHash("s3cret-pass", argon2id.DefaultParams)
	assert.Nil(t, err)

	stored := domain.User{ID: 1, Email: "john.doe@example.com", PasswordHash: hash}

	tests := []struct {
		name     string
		mocks    func(t *testing.T) port.UserRepository
		email    string
		password string

		expectedError error
	}{
		{
			name: "unknown email returns ErrInvalidCredentials",
			mocks: func(t *testing.T) port.UserRepository {
				t.Helper()

				repo := mockport.NewMockUserRepository(ctrl)

				repo.EXPECT().GetByEmail(gomock.Any(), "nobody@example.com").Return(domain.User{}, domain.ErrUserNotFound)

				return repo
			},
			email:         "nobody@example.com",
			password:      "s3cret-pass",
			expectedError: domain.ErrInvalidCredentials,
		},
		{
			name: "wrong password returns ErrInvalidCredentials",
			mocks: func(t *testing.T) port.UserRepository {
				t.Helper()

				repo := mockport.NewMockUserRepository(ctrl)

				repo.EXPECT().GetByEmail(gomock.Any(), stored.Email).Return(stored, nil)

				return repo
			},
			email:         stored.Email,
			password:      "wrong-pass",
			expectedError: domain.ErrInvalidCredentials,
		},
		{
			name: "success returns the user",
			mocks: func(t *testing.T) port.UserRepository {
				t.Helper()

				repo := mockport.NewMockUserRepository(ctrl)

				repo.EXPECT().GetByEmail(gomock.Any(), stored.Email).Return(stored, nil)

				return repo
			},
			email:    stored.Email,
			password: "s3cret-pass",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := service.NewUserService(tt.mocks(t))

			u, err := svc.Authenticate(context.Background(), tt.email, tt.password)
			if tt.expectedError != nil {
				assert.ErrorIs(t, err, tt.expectedError)

				return
			}

			assert.Nil(t, err)
			assert.Equal(t, u.ID, stored.ID)
		})
	}
}

func TestUserServiceGet(t *testing.T) {
	ctrl := gomock.NewController(t)

	tests := []struct {
		name  string
		mocks func(t *testing.T) port.UserRepository
		id    int64

		expectedError error
	}{
		{
			name: "missing user returns ErrUserNotFound",
			mocks: func(t *testing.T) port.UserRepository {
				t.Helper()

				repo := mockport.NewMockUserRepository(ctrl)

				repo.EXPECT().Get(gomock.Any(), int64(99)).Return(domain.User{}, domain.ErrUserNotFound)

				return repo
			},
			id:            99,
			expectedError: domain.ErrUserNotFound,
		},
		{
			name: "success returns the user",
			mocks: func(t *testing.T) port.UserRepository {
				t.Helper()

				repo := mockport.NewMockUserRepository(ctrl)

				repo.EXPECT().Get(gomock.Any(), int64(1)).Return(domain.User{ID: 1, Email: "john.doe@example.com"}, nil)

				return repo
			},
			id: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := service.NewUserService(tt.mocks(t))

			u, err := svc.Get(context.Background(), tt.id)
			if tt.expectedError != nil {
				assert.ErrorIs(t, err, tt.expectedError)

				return
			}

			assert.Nil(t, err)
			assert.Equal(t, u.ID, tt.id)
		})
	}
}

func TestUserServiceList(t *testing.T) {
	ctrl := gomock.NewController(t)

	page := query.Page{Limit: 2}
	users := []domain.User{
		{ID: 1, Email: "john.doe@example.com"},
		{ID: 2, Email: "jane.doe@example.com"},
	}

	tests := []struct {
		name  string
		mocks func(t *testing.T) port.UserRepository

		expectedUsers []domain.User
		expectedTotal int64
	}{
		{
			name: "returns users and total from the repository",
			mocks: func(t *testing.T) port.UserRepository {
				t.Helper()

				repo := mockport.NewMockUserRepository(ctrl)

				repo.EXPECT().
					List(gomock.Any(), domain.UserFilter{}, page, gomock.Nil()).
					Return(users, int64(3), nil)

				return repo
			},
			expectedUsers: users,
			expectedTotal: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := service.NewUserService(tt.mocks(t))

			got, total, err := svc.List(context.Background(), domain.UserFilter{}, page, nil)
			assert.Nil(t, err)
			assert.Equal(t, got, tt.expectedUsers)
			assert.Equal(t, total, tt.expectedTotal)
		})
	}
}

func TestUserServiceUpdate(t *testing.T) {
	ctrl := gomock.NewController(t)

	tests := []struct {
		name     string
		mocks    func(t *testing.T) port.UserRepository
		id       int64
		email    string
		userName string

		expectedError error
	}{
		{
			name: "invalid email returns ErrInvalidUser without any repo call",
			mocks: func(t *testing.T) port.UserRepository {
				t.Helper()

				return mockport.NewMockUserRepository(ctrl)
			},
			id:            1,
			email:         "nope",
			userName:      "John Doe",
			expectedError: domain.ErrInvalidUser,
		},
		{
			name: "missing user returns ErrUserNotFound",
			mocks: func(t *testing.T) port.UserRepository {
				t.Helper()

				repo := mockport.NewMockUserRepository(ctrl)

				repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(domain.User{}, domain.ErrUserNotFound)

				return repo
			},
			id:            99,
			email:         "john.doe@example.com",
			userName:      "John Doe",
			expectedError: domain.ErrUserNotFound,
		},
		{
			name: "success returns the updated user",
			mocks: func(t *testing.T) port.UserRepository {
				t.Helper()

				repo := mockport.NewMockUserRepository(ctrl)

				repo.EXPECT().
					Update(gomock.Any(), domain.User{ID: 1, Email: "john.doe@example.com", Name: "John Doe"}).
					Return(domain.User{ID: 1, Email: "john.doe@example.com", Name: "John Doe"}, nil)

				return repo
			},
			id:       1,
			email:    "john.doe@example.com",
			userName: "John Doe",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := service.NewUserService(tt.mocks(t))

			u, err := svc.Update(context.Background(), tt.id, tt.email, tt.userName)
			if tt.expectedError != nil {
				assert.ErrorIs(t, err, tt.expectedError)

				return
			}

			assert.Nil(t, err)
			assert.Equal(t, u.Email, tt.email)
		})
	}
}

func TestUserServiceDelete(t *testing.T) {
	ctrl := gomock.NewController(t)

	tests := []struct {
		name  string
		mocks func(t *testing.T) port.UserRepository
		id    int64

		expectedError error
	}{
		{
			name: "missing user returns ErrUserNotFound",
			mocks: func(t *testing.T) port.UserRepository {
				t.Helper()

				repo := mockport.NewMockUserRepository(ctrl)

				repo.EXPECT().Delete(gomock.Any(), int64(99)).Return(domain.ErrUserNotFound)

				return repo
			},
			id:            99,
			expectedError: domain.ErrUserNotFound,
		},
		{
			name: "success deletes the user",
			mocks: func(t *testing.T) port.UserRepository {
				t.Helper()

				repo := mockport.NewMockUserRepository(ctrl)

				repo.EXPECT().Delete(gomock.Any(), int64(1)).Return(nil)

				return repo
			},
			id: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := service.NewUserService(tt.mocks(t))

			err := svc.Delete(context.Background(), tt.id)
			if tt.expectedError != nil {
				assert.ErrorIs(t, err, tt.expectedError)

				return
			}

			assert.Nil(t, err)
		})
	}
}
