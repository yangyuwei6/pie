package data

import (
	"fmt"

	"pie/internal/config"
	"pie/internal/data/query"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type Data struct {
	q *query.Query
}

func NewDB(cfg config.MySQLConfig) (*gorm.DB, error) {
	db, err := gorm.Open(mysql.Open(cfg.DSN), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("connect mysql failed: %w", err)
	}
	return db, nil
}

func NewData(db *gorm.DB) *Data {
	return &Data{
		q: query.Use(db),
	}
}
