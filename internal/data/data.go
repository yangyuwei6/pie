package data

import (
	"context"
	"fmt"

	"pie/internal/config"
	"pie/internal/data/query"

	"github.com/redis/go-redis/v9"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type Data struct {
	q   *query.Query
	rdb *redis.Client
}

func NewDB(cfg config.MySQLConfig) (*gorm.DB, error) {
	db, err := gorm.Open(mysql.Open(cfg.DSN), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("connect mysql failed: %w", err)
	}
	return db, nil
}

func NewRedis(cfg config.RedisConfig) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("connect redis failed: %w", err)
	}

	return rdb, nil
}

func NewData(db *gorm.DB, rdb *redis.Client) *Data {
	return &Data{
		q:   query.Use(db),
		rdb: rdb,
	}
}
