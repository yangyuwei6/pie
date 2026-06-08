package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

const defaultPath = "configs/config.yaml"

type Config struct {
	HTTP        HTTPConfig        `yaml:"http"`
	MySQL       MySQLConfig       `yaml:"mysql"`
	Redis       RedisConfig       `yaml:"redis"`
	Search      SearchConfig      `yaml:"elasticsearch"`
	ObjectStore ObjectStoreConfig `yaml:"minio"`
	Messaging   MessagingConfig   `yaml:"kafka"`
	AI          AIConfig          `yaml:"ai"`
	JWT         JWTConfig         `yaml:"jwt"`
}

type HTTPConfig struct {
	Addr string `yaml:"addr"`
}

type MySQLConfig struct {
	DSN string `yaml:"dsn"`
}

type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

type SearchConfig struct {
	ElasticsearchURL string `yaml:"url"`
	IndexName        string `yaml:"index_name"`
}

type ObjectStoreConfig struct {
	Endpoint  string `yaml:"endpoint"`
	Bucket    string `yaml:"bucket"`
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
}

type MessagingConfig struct {
	KafkaBrokers []string `yaml:"brokers"`
	Topic        string   `yaml:"topic"`
}

type JWTConfig struct {
	Secret             string `yaml:"secret"`
	ExpireHours        int    `yaml:"expire_hours"`
	RefreshExpireHours int    `yaml:"refresh_expire_hours"`
}

type AIConfig struct {
	LLMBaseURL          string `yaml:"llm_base_url"`
	LLMAPIKey           string `yaml:"llm_api_key"`
	EmbeddingBaseURL    string `yaml:"embedding_base_url"`
	EmbeddingAPIKey     string `yaml:"embedding_api_key"`
	EmbeddingModel      string `yaml:"embedding_model"`
	EmbeddingDimensions int    `yaml:"embedding_dimensions"`
	TikaURL             string `yaml:"tika_url"`
}

func Load() (Config, error) {
	return LoadFile(defaultPath)
}

func LoadFile(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config file: %w", err)
	}

	setDefaults(&cfg)

	if cfg.MySQL.DSN == "" {
		return Config{}, fmt.Errorf("mysql.dsn is empty")
	}

	return cfg, nil
}

func setDefaults(cfg *Config) {
	if cfg.HTTP.Addr == "" {
		cfg.HTTP.Addr = ":8080"
	}
	if cfg.JWT.ExpireHours == 0 {
		cfg.JWT.ExpireHours = 24
	}
	if cfg.JWT.RefreshExpireHours == 0 {
		cfg.JWT.RefreshExpireHours = 24 * 7
	}
	if cfg.Messaging.Topic == "" {
		cfg.Messaging.Topic = "file-processing"
	}
	if cfg.Search.IndexName == "" {
		cfg.Search.IndexName = "knowledge_base"
	}
	if cfg.AI.EmbeddingModel == "" {
		cfg.AI.EmbeddingModel = "text-embedding-v4"
	}
	if cfg.AI.EmbeddingDimensions == 0 {
		cfg.AI.EmbeddingDimensions = 2048
	}
}
