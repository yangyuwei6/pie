package data

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"

	"pie/internal/config"
	"pie/internal/data/query"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type Data struct {
	q           *query.Query
	rdb         *redis.Client
	minioClient *minio.Client
	esClient    *elasticsearch.TypedClient
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

func NewMinIO(cfg config.ObjectStoreConfig) (*minio.Client, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: false,
	})
	if err != nil {
		return nil, fmt.Errorf("connect minio failed: %w", err)
	}

	exists, err := client.BucketExists(context.Background(), cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("check minio bucket failed: %w", err)
	}
	if !exists {
		if err := client.MakeBucket(context.Background(), cfg.Bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("create minio bucket failed: %w", err)
		}
	}

	return client, nil
}

func NewElasticsearch(cfg config.SearchConfig) (*elasticsearch.TypedClient, error) {
	client, err := elasticsearch.NewTypedClient(elasticsearch.Config{
		Addresses: []string{cfg.ElasticsearchURL},
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("connect elasticsearch failed: %w", err)
	}
	return client, nil
}

func NewData(db *gorm.DB, rdb *redis.Client, minioClient *minio.Client, esClient *elasticsearch.TypedClient) *Data {
	return &Data{
		q:           query.Use(db),
		rdb:         rdb,
		minioClient: minioClient,
		esClient:    esClient,
	}
}
