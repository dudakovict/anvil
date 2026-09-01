package domain

import (
	"errors"
	"time"
)

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidUser        = errors.New("invalid user")
	ErrEmailTaken         = errors.New("email already taken")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrForbidden          = errors.New("forbidden")
)

type Role string

const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
)

type User struct {
	ID           int64
	Email        string
	Name         string
	Role         Role
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type UserFilter struct {
	Email *string
	Name  *string
}
