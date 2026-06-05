package main

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
	"gorm.io/driver/mysql"
	"gorm.io/gen"
	"gorm.io/gorm"
)

const configPath = "../../configs/config.yaml"

func main() {
	dsn, err := loadDSN(configPath)
	if err != nil {
		panic(err)
	}

	db, err := gorm.Open(mysql.Open(dsn))
	if err != nil {
		panic(fmt.Errorf("connect mysql: %w", err))
	}

	g := gen.NewGenerator(gen.Config{
		OutPath:           "../../internal/data/query",
		ModelPkgPath:      "model",
		Mode:              gen.WithDefaultQuery | gen.WithQueryInterface,
		FieldNullable:     true,
		FieldWithIndexTag: true,
		FieldWithTypeTag:  true,
	})

	g.UseDB(db)
	g.ApplyBasic(g.GenerateAllTable()...)
	g.Execute()
}

func loadDSN(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read config file: %w", err)
	}

	var cfg struct {
		MySQL struct {
			DSN string `yaml:"dsn"`
		} `yaml:"mysql"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return "", fmt.Errorf("parse config file: %w", err)
	}

	if cfg.MySQL.DSN == "" {
		return "", errors.New("mysql.dsn is empty")
	}

	return cfg.MySQL.DSN, nil
}
