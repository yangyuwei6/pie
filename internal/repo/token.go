package repo

import (
	"context"
	"time"
)

type TokenRepo interface {
	Blacklist(ctx context.Context, token string, ttl time.Duration) error
	IsBlacklisted(ctx context.Context, token string) (bool, error)
}
