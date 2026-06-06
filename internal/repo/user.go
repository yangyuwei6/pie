package repo

import (
	"context"

	"pie/internal/data/model"
)

type UserRepo interface {
	Create(ctx context.Context, user *model.User) error
	FindByUsername(ctx context.Context, username string) (*model.User, error)
	FindByID(ctx context.Context, userID int64) (*model.User, error)
	Update(ctx context.Context, user *model.User) error
	FindAll(ctx context.Context) ([]*model.User, error)
	FindWithPagination(ctx context.Context, offset, limit int) ([]*model.User, int64, error)
}
