package rest

import (
	"time"

	"github.com/dudakovict/anvil/internal/core/domain"
	"github.com/dudakovict/anvil/internal/platform/query"
)

type LoginRequest struct {
	Email    string `json:"email"    validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
}

type CreateUserRequest struct {
	Email    string `json:"email"    validate:"required,email"`
	Name     string `json:"name"     validate:"required"`
	Password string `json:"password" validate:"required,min=8"`
}

type UpdateUserRequest struct {
	Email string `json:"email" validate:"required,email"`
	Name  string `json:"name"  validate:"required"`
}

type ListUsersQuery struct {
	Email *string `form:"email" validate:"omitempty,email"`
	Name  *string `form:"name"`
}

type UserResponse struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UsersPage = query.Result[UserResponse]

func toUserResponse(u domain.User) UserResponse {
	return UserResponse{
		ID:        u.ID,
		Email:     u.Email,
		Name:      u.Name,
		Role:      string(u.Role),
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

func toUserResponses(users []domain.User) []UserResponse {
	out := make([]UserResponse, 0, len(users))
	for _, u := range users {
		out = append(out, toUserResponse(u))
	}

	return out
}
