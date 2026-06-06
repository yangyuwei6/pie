package data

import (
	"context"

	"pie/internal/data/model"
)

type UserRepo struct {
	data *Data
}

func NewUserRepo(data *Data) *UserRepo {
	return &UserRepo{data: data}
}

func (r *UserRepo) Create(ctx context.Context, user *model.User) error {
	return r.data.q.User.WithContext(ctx).Create(user)
}

func (r *UserRepo) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	u := r.data.q.User

	return u.WithContext(ctx).Where(u.Username.Eq(username)).First()
}

func (r *UserRepo) FindByID(ctx context.Context, userID int64) (*model.User, error) {

	return r.data.q.User.WithContext(ctx).Where(r.data.q.User.ID.Eq(userID)).First()
}

func (r *UserRepo) Update(ctx context.Context, user *model.User) error {
	return r.data.q.User.WithContext(ctx).Save(user)
}

func (r *UserRepo) FindAll(ctx context.Context) ([]*model.User, error) {
	return r.data.q.User.WithContext(ctx).Find()
}

func (r *UserRepo) FindWithPagination(ctx context.Context, offset, limit int) ([]*model.User, int64, error) {

	total, err := r.data.q.User.WithContext(ctx).Count()
	if err != nil {
		return nil, 0, err
	}

	users, err := r.data.q.User.WithContext(ctx).
		Offset(offset).
		Limit(limit).
		Find()
	if err != nil {
		return nil, 0, err
	}

	return users, total, nil
}
