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
}

type ObjectStoreConfig struct {
	Endpoint  string `yaml:"endpoint"`
	Bucket    string `yaml:"bucket"`
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
}

type MessagingConfig struct {
	KafkaBrokers []string `yaml:"brokers"`
}

type JWTConfig struct {
	Secret      string `yaml:"secret"`
	ExpireHours int    `yaml:"expire_hours"`
}

type AIConfig struct {
	LLMBaseURL       string `yaml:"llm_base_url"`
	LLMAPIKey        string `yaml:"llm_api_key"`
	EmbeddingBaseURL string `yaml:"embedding_base_url"`
	EmbeddingAPIKey  string `yaml:"embedding_api_key"`
	TikaURL          string `yaml:"tika_url"`
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
}
