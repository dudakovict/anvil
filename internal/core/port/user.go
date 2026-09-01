package port

import (
	"context"

	"github.com/dudakovict/anvil/internal/core/domain"
	"github.com/dudakovict/anvil/internal/platform/query"
)

type UserService interface {
	Create(ctx context.Context, email, name, password string) (domain.User, error)
	Authenticate(ctx context.Context, email, password string) (domain.User, error)
	Get(ctx context.Context, id int64) (domain.User, error)
	List(ctx context.Context, filter domain.UserFilter, page query.Page, order query.OrderBy) ([]domain.User, int64, error)
	Update(ctx context.Context, id int64, email, name string) (domain.User, error)
	Delete(ctx context.Context, id int64) error
}

type UserRepository interface {
	Create(ctx context.Context, u domain.User) (domain.User, error)
	Get(ctx context.Context, id int64) (domain.User, error)
	GetByEmail(ctx context.Context, email string) (domain.User, error)
	List(ctx context.Context, filter domain.UserFilter, page query.Page, order query.OrderBy) ([]domain.User, int64, error)
	Update(ctx context.Context, u domain.User) (domain.User, error)
	Delete(ctx context.Context, id int64) error
}
