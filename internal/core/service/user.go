package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"

	"github.com/dudakovict/anvil/internal/core/domain"
	"github.com/dudakovict/anvil/internal/core/port"
	"github.com/dudakovict/anvil/internal/platform/argon2id"
	"github.com/dudakovict/anvil/internal/platform/query"
)

const minPasswordLength = 8

type UserService struct {
	repo port.UserRepository
}

var _ port.UserService = (*UserService)(nil)

func NewUserService(repo port.UserRepository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) Create(ctx context.Context, email, name, password string) (domain.User, error) {
	if err := validateUser(email, name); err != nil {
		return domain.User{}, err
	}

	if len(password) < minPasswordLength {
		return domain.User{}, fmt.Errorf("%w: password must have at least %d characters", domain.ErrInvalidUser, minPasswordLength)
	}

	hash, err := argon2id.CreateHash(password, argon2id.DefaultParams)
	if err != nil {
		return domain.User{}, fmt.Errorf("hashing password: %w", err)
	}

	u, err := s.repo.Create(ctx, domain.User{
		Email:        email,
		Name:         name,
		Role:         domain.RoleUser,
		PasswordHash: hash,
	})
	if err != nil {
		return domain.User{}, err
	}

	slog.DebugContext(ctx, "user created", "user_id", u.ID)

	return u, nil
}

// Authenticate returns ErrInvalidCredentials for both an unknown email and a
// wrong password so callers cannot probe which emails exist.
func (s *UserService) Authenticate(ctx context.Context, email, password string) (domain.User, error) {
	u, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return domain.User{}, domain.ErrInvalidCredentials
		}

		return domain.User{}, err
	}

	match, err := argon2id.ComparePasswordAndHash(password, u.PasswordHash)
	if err != nil {
		return domain.User{}, fmt.Errorf("comparing password: %w", err)
	}

	if !match {
		return domain.User{}, domain.ErrInvalidCredentials
	}

	return u, nil
}

func (s *UserService) Get(ctx context.Context, id int64) (domain.User, error) {
	return s.repo.Get(ctx, id)
}

func (s *UserService) List(ctx context.Context, filter domain.UserFilter, page query.Page, order query.OrderBy) ([]domain.User, int64, error) {
	return s.repo.List(ctx, filter, page, order)
}

func (s *UserService) Update(ctx context.Context, id int64, email, name string) (domain.User, error) {
	if err := validateUser(email, name); err != nil {
		return domain.User{}, err
	}

	return s.repo.Update(ctx, domain.User{
		ID:    id,
		Email: email,
		Name:  name,
	})
}

func (s *UserService) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

func validateUser(email, name string) error {
	if _, err := mail.ParseAddress(email); err != nil {
		return fmt.Errorf("%w: invalid email", domain.ErrInvalidUser)
	}

	if name == "" {
		return fmt.Errorf("%w: name must not be empty", domain.ErrInvalidUser)
	}

	return nil
}
