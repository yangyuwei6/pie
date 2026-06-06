package data

import (
	"context"
	"time"
)

type TokenRepo struct {
	data *Data
}

func NewTokenRepo(data *Data) *TokenRepo {
	return &TokenRepo{data: data}
}

func (r *TokenRepo) Blacklist(ctx context.Context, token string, ttl time.Duration) error {
	if ttl <= 0 {
		return nil
	}
	return r.data.rdb.Set(ctx, blacklistKey(token), "1", ttl).Err()
}

func (r *TokenRepo) IsBlacklisted(ctx context.Context, token string) (bool, error) {
	exists, err := r.data.rdb.Exists(ctx, blacklistKey(token)).Result()
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}

func blacklistKey(token string) string {
	return "jwt:blacklist:" + token
}
