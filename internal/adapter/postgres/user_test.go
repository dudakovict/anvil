package postgres_test

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/dudakovict/anvil/internal/adapter/postgres"
	"github.com/dudakovict/anvil/internal/core/domain"
	"github.com/dudakovict/anvil/internal/platform/assert"
	"github.com/dudakovict/anvil/internal/platform/database"
	"github.com/dudakovict/anvil/internal/platform/docker"
	"github.com/dudakovict/anvil/internal/platform/query"
	"github.com/dudakovict/anvil/migrations"
)

var testDB *gorm.DB

func TestMain(m *testing.M) {
	flag.Parse()

	if testing.Short() {
		os.Exit(m.Run())
	}

	if _, err := exec.LookPath("docker"); err != nil {
		os.Exit(m.Run())
	}

	c, err := docker.StartContainer(
		"postgres:17-alpine", "anvil-test-db", "5432",
		[]string{"-e", "POSTGRES_USER=test", "-e", "POSTGRES_PASSWORD=test", "-e", "POSTGRES_DB=test"},
		nil,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "starting postgres container: %s\n", err)
		os.Exit(1)
	}

	code := func() int {
		dsn := fmt.Sprintf("postgres://test:test@%s/test?sslmode=disable", c.HostPort)

		db, err := database.Open(database.Config{DSN: dsn})
		if err != nil {
			fmt.Fprintf(os.Stderr, "opening database: %s\n", err)

			return 1
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := database.Ping(ctx, db); err != nil {
			fmt.Fprintf(os.Stderr, "pinging database: %s\n", err)

			return 1
		}

		if err := migrations.Up(ctx, dsn); err != nil {
			fmt.Fprintf(os.Stderr, "migrating: %s\n", err)

			return 1
		}

		testDB = db

		return m.Run()
	}()

	if err := docker.StopContainer(c.Name); err != nil {
		fmt.Fprintf(os.Stderr, "stopping container: %s\n", err)
	}

	os.Exit(code)
}

func newTestRepo(t *testing.T) *postgres.UserRepository {
	t.Helper()

	if testDB == nil {
		t.Skip("database not available")
	}

	assert.Nil(t, testDB.Exec("TRUNCATE users RESTART IDENTITY").Error)

	return postgres.NewUserRepository(testDB)
}

func seedUser(t *testing.T, repo *postgres.UserRepository, email, name string) domain.User {
	t.Helper()

	u, err := repo.Create(t.Context(), domain.User{Email: email, Name: name, Role: domain.RoleUser, PasswordHash: "hash"})
	assert.Nil(t, err)

	return u
}

func TestUserRepositoryCreate(t *testing.T) {
	tests := []struct {
		name string
		seed func(t *testing.T, repo *postgres.UserRepository)
		user domain.User

		expectedError error
	}{
		{
			name: "success stores the user and returns DB timestamps",
			user: domain.User{Email: "john.doe@example.com", Name: "John Doe", Role: domain.RoleUser, PasswordHash: "hash"},
		},
		{
			name: "duplicate email returns ErrEmailTaken",
			seed: func(t *testing.T, repo *postgres.UserRepository) {
				t.Helper()
				seedUser(t, repo, "john.doe@example.com", "John Doe")
			},
			user:          domain.User{Email: "john.doe@example.com", Name: "Another John", Role: domain.RoleUser, PasswordHash: "hash"},
			expectedError: domain.ErrEmailTaken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newTestRepo(t)

			if tt.seed != nil {
				tt.seed(t, repo)
			}

			got, err := repo.Create(t.Context(), tt.user)
			if tt.expectedError != nil {
				assert.ErrorIs(t, err, tt.expectedError)

				return
			}

			assert.Nil(t, err)
			assert.Equal(t, got.Email, tt.user.Email)
			assert.Equal(t, got.Role, domain.RoleUser)
			assert.True(t, got.ID > 0)
			assert.False(t, got.CreatedAt.IsZero())
		})
	}
}

func TestUserRepositoryGet(t *testing.T) {
	tests := []struct {
		name string
		seed bool

		expectedError error
	}{
		{
			name: "success returns the stored user",
			seed: true,
		},
		{
			name:          "missing user returns ErrUserNotFound",
			expectedError: domain.ErrUserNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newTestRepo(t)

			id := int64(99)
			if tt.seed {
				id = seedUser(t, repo, "john.doe@example.com", "John Doe").ID
			}

			got, err := repo.Get(t.Context(), id)
			if tt.expectedError != nil {
				assert.ErrorIs(t, err, tt.expectedError)

				return
			}

			assert.Nil(t, err)
			assert.Equal(t, got.ID, id)
			assert.Equal(t, got.Email, "john.doe@example.com")
		})
	}
}

func TestUserRepositoryGetByEmail(t *testing.T) {
	tests := []struct {
		name string
		seed bool

		expectedError error
	}{
		{
			name: "success returns the stored user",
			seed: true,
		},
		{
			name:          "missing user returns ErrUserNotFound",
			expectedError: domain.ErrUserNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newTestRepo(t)

			if tt.seed {
				seedUser(t, repo, "john.doe@example.com", "John Doe")
			}

			got, err := repo.GetByEmail(t.Context(), "john.doe@example.com")
			if tt.expectedError != nil {
				assert.ErrorIs(t, err, tt.expectedError)

				return
			}

			assert.Nil(t, err)
			assert.Equal(t, got.Email, "john.doe@example.com")
		})
	}
}

func TestUserRepositoryList(t *testing.T) {
	richard := "Richard"
	jane := "jane.doe@example.com"

	tests := []struct {
		name   string
		filter domain.UserFilter
		page   query.Page

		expectedEmails []string
		expectedTotal  int64
	}{
		{
			name:           "returns every user without a filter",
			page:           query.Page{Limit: 10},
			expectedEmails: []string{"john.doe@example.com", "jane.doe@example.com", "richard.roe@example.com"},
			expectedTotal:  3,
		},
		{
			name:           "paginates while total counts all matches",
			page:           query.Page{Limit: 2},
			expectedEmails: []string{"john.doe@example.com", "jane.doe@example.com"},
			expectedTotal:  3,
		},
		{
			name:           "filters by name containment",
			filter:         domain.UserFilter{Name: &richard},
			page:           query.Page{Limit: 10},
			expectedEmails: []string{"richard.roe@example.com"},
			expectedTotal:  1,
		},
		{
			name:           "filters by exact email",
			filter:         domain.UserFilter{Email: &jane},
			page:           query.Page{Limit: 10},
			expectedEmails: []string{"jane.doe@example.com"},
			expectedTotal:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newTestRepo(t)

			seedUser(t, repo, "john.doe@example.com", "John Doe")
			seedUser(t, repo, "jane.doe@example.com", "Jane Doe")
			seedUser(t, repo, "richard.roe@example.com", "Richard Roe")

			users, total, err := repo.List(t.Context(), tt.filter, tt.page, query.OrderBy{{Field: "id"}})
			assert.Nil(t, err)
			assert.Equal(t, total, tt.expectedTotal)
			assert.Equal(t, len(users), len(tt.expectedEmails))

			for i, email := range tt.expectedEmails {
				assert.Equal(t, users[i].Email, email)
			}
		})
	}
}

func TestUserRepositoryUpdate(t *testing.T) {
	tests := []struct {
		name string
		user func(t *testing.T, repo *postgres.UserRepository) domain.User

		expectedError error
	}{
		{
			name: "success returns the updated user",
			user: func(t *testing.T, repo *postgres.UserRepository) domain.User {
				t.Helper()
				u := seedUser(t, repo, "john.doe@example.com", "John Doe")

				return domain.User{ID: u.ID, Email: "john.d@example.com", Name: "John D"}
			},
		},
		{
			name: "duplicate email returns ErrEmailTaken",
			user: func(t *testing.T, repo *postgres.UserRepository) domain.User {
				t.Helper()
				seedUser(t, repo, "john.doe@example.com", "John Doe")
				u := seedUser(t, repo, "jane.doe@example.com", "Jane Doe")

				return domain.User{ID: u.ID, Email: "john.doe@example.com", Name: "Jane Doe"}
			},
			expectedError: domain.ErrEmailTaken,
		},
		{
			name: "missing user returns ErrUserNotFound",
			user: func(t *testing.T, _ *postgres.UserRepository) domain.User {
				t.Helper()

				return domain.User{ID: 99, Email: "new@example.com", Name: "New"}
			},
			expectedError: domain.ErrUserNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newTestRepo(t)

			user := tt.user(t, repo)

			got, err := repo.Update(t.Context(), user)
			if tt.expectedError != nil {
				assert.ErrorIs(t, err, tt.expectedError)

				return
			}

			assert.Nil(t, err)
			assert.Equal(t, got.Email, user.Email)
			assert.Equal(t, got.Name, user.Name)
		})
	}
}

func TestUserRepositoryDelete(t *testing.T) {
	tests := []struct {
		name string
		seed bool

		expectedError error
	}{
		{
			name: "success removes the user",
			seed: true,
		},
		{
			name:          "missing user returns ErrUserNotFound",
			expectedError: domain.ErrUserNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newTestRepo(t)

			id := int64(99)
			if tt.seed {
				id = seedUser(t, repo, "john.doe@example.com", "John Doe").ID
			}

			err := repo.Delete(t.Context(), id)
			if tt.expectedError != nil {
				assert.ErrorIs(t, err, tt.expectedError)

				return
			}

			assert.Nil(t, err)

			_, err = repo.Get(t.Context(), id)
			assert.ErrorIs(t, err, domain.ErrUserNotFound)
		})
	}
}
