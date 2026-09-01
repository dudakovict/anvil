package postgres

import (
	"time"

	"github.com/dudakovict/anvil/internal/core/domain"
)

type userRow struct {
	ID           int64
	Email        string
	Name         string
	Role         string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (userRow) TableName() string {
	return "users"
}

func toUserRow(u domain.User) userRow {
	return userRow{
		ID:           u.ID,
		Email:        u.Email,
		Name:         u.Name,
		Role:         string(u.Role),
		PasswordHash: u.PasswordHash,
	}
}

func (r userRow) toDomain() domain.User {
	return domain.User{
		ID:           r.ID,
		Email:        r.Email,
		Name:         r.Name,
		Role:         domain.Role(r.Role),
		PasswordHash: r.PasswordHash,
		CreatedAt:    r.CreatedAt,
		UpdatedAt:    r.UpdatedAt,
	}
}

func toDomainUsers(rows []userRow) []domain.User {
	out := make([]domain.User, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.toDomain())
	}

	return out
}
