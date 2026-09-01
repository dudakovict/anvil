---
name: go-tests
description: Write Go tests in this repo's house style — table-driven with gomock for services, container-backed integration tests for repositories. Use whenever writing or modifying _test.go files.
---

# Writing tests

Every test is table-driven, even with a single case. External test packages
(`package x_test`), `platform/assert` for assertions, gomock for ports,
John Doe fixtures (`john.doe@example.com`, `jane.doe@example.com`,
`richard.roe@example.com`).

## Rules

- Table fields: `name` first, inputs, then a blank line, then `expected*`
  fields; loop as `for _, tt := range tests { t.Run(tt.name, ...) }`
- Case names read as behavior: "duplicate email returns ErrEmailTaken",
  "success returns the stored user"
- Assertions: `assert.Equal(t, got, want)` (got first), `assert.ErrorIs`,
  `assert.Nil`; error cases assert and `return` early after a blank line
- Mocks: one `ctrl := gomock.NewController(t)` per test function; each case
  builds its own repo via `mocks func(t *testing.T) port.UserRepository`
  closure with `t.Helper()`; regenerate with `make mocks` when port
  interfaces change
- Use `t.Context()` for request contexts in integration tests,
  `context.Background()` in unit tests
- Never use testify; never assert on log output

## Unit test skeleton (service layer)

```go
func TestUserServiceCreate(t *testing.T) {
	ctrl := gomock.NewController(t)

	tests := []struct {
		name  string
		mocks func(t *testing.T) port.UserRepository
		email string

		expectedError error
	}{
		{
			name: "duplicate email returns ErrEmailTaken",
			mocks: func(t *testing.T) port.UserRepository {
				t.Helper()

				repo := mockport.NewMockUserRepository(ctrl)

				repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(domain.User{}, domain.ErrEmailTaken)

				return repo
			},
			email:         "john.doe@example.com",
			expectedError: domain.ErrEmailTaken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := service.NewUserService(tt.mocks(t))

			u, err := svc.Create(context.Background(), tt.email, "John Doe", "s3cret-pass")
			if tt.expectedError != nil {
				assert.ErrorIs(t, err, tt.expectedError)

				return
			}

			assert.Nil(t, err)
			assert.Equal(t, u.Email, tt.email)
		})
	}
}
```

## Integration test skeleton (postgres repository)

`TestMain` in `internal/adapter/postgres/user_test.go` already provisions one
throwaway postgres container per package run (skipped under `-short` or
without docker) — new tests only need the helpers:

```go
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
			repo := newTestRepo(t) // skips without DB + truncates the table

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
		})
	}
}
```

For a new entity, add a `seedProduct`-style helper next to the tests and
extend `newTestRepo`'s TRUNCATE if the table differs.

## HTTP-level tests (router)

Authorization/middleware behavior is tested through the real router with a
mocked `port.UserService` and a real `auth.Auth` — see `TestAuthorization` in
`internal/adapter/rest/router_test.go`. Remember to set
`config.HTTP{RequestTimeout: time.Minute}` in Deps or every request 503s.

## Verify

`make test` locally (integration needs docker), `make lint` — wsl_v5/nlreturn
enforce the blank-line layout shown above.
