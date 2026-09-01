package postgres

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/dudakovict/anvil/internal/core/domain"
	"github.com/dudakovict/anvil/internal/core/port"
	"github.com/dudakovict/anvil/internal/platform/query"
)

type UserRepository struct {
	db *gorm.DB
}

var _ port.UserRepository = (*UserRepository)(nil)

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, u domain.User) (domain.User, error) {
	row := toUserRow(u)

	if err := r.db.WithContext(ctx).Clauses(clause.Returning{}).Create(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return domain.User{}, domain.ErrEmailTaken
		}

		return domain.User{}, fmt.Errorf("inserting user: %w", err)
	}

	return row.toDomain(), nil
}

func (r *UserRepository) Get(ctx context.Context, id int64) (domain.User, error) {
	var row userRow

	if err := r.db.WithContext(ctx).First(&row, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.User{}, domain.ErrUserNotFound
		}

		return domain.User{}, fmt.Errorf("selecting user: %w", err)
	}

	return row.toDomain(), nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (domain.User, error) {
	var row userRow

	if err := r.db.WithContext(ctx).Where("email = ?", email).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.User{}, domain.ErrUserNotFound
		}

		return domain.User{}, fmt.Errorf("selecting user: %w", err)
	}

	return row.toDomain(), nil
}

func (r *UserRepository) List(ctx context.Context, filter domain.UserFilter, page query.Page, order query.OrderBy) ([]domain.User, int64, error) {
	filtered := func() *gorm.DB {
		q := r.db.WithContext(ctx).Model(&userRow{})

		if filter.Email != nil {
			q = q.Where("email = ?", *filter.Email)
		}

		if filter.Name != nil {
			q = q.Where("name ILIKE ?", "%"+*filter.Name+"%")
		}

		return q
	}

	var total int64
	if err := filtered().Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("counting users: %w", err)
	}

	var rows []userRow

	if err := filtered().
		Order(order.SQL("id")).
		Limit(page.Limit).
		Offset(page.Offset).
		Find(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("selecting users: %w", err)
	}

	return toDomainUsers(rows), total, nil
}

func (r *UserRepository) Update(ctx context.Context, u domain.User) (domain.User, error) {
	var row userRow

	res := r.db.WithContext(ctx).
		Model(&row).
		Clauses(clause.Returning{}).
		Where("id = ?", u.ID).
		Updates(map[string]any{
			"email": u.Email,
			"name":  u.Name,
		})
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
			return domain.User{}, domain.ErrEmailTaken
		}

		return domain.User{}, fmt.Errorf("updating user: %w", res.Error)
	}

	if res.RowsAffected == 0 {
		return domain.User{}, domain.ErrUserNotFound
	}

	return row.toDomain(), nil
}

func (r *UserRepository) Delete(ctx context.Context, id int64) error {
	res := r.db.WithContext(ctx).Delete(&userRow{}, id)
	if res.Error != nil {
		return fmt.Errorf("deleting user: %w", res.Error)
	}

	if res.RowsAffected == 0 {
		return domain.ErrUserNotFound
	}

	return nil
}
