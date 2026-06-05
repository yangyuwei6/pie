package main

import (
	"pie/internal/auth"
	"pie/internal/biz"
	"pie/internal/config"
	"pie/internal/data"
	"pie/internal/log"
	"pie/internal/middleware"
	"pie/internal/router"
	"pie/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func main() {
	logger, err := log.NewLogger()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("load config failed", zap.Error(err))
	}

	r := gin.Default()

	db, err := data.NewDB(cfg.MySQL)
	if err != nil {
		logger.Fatal("init database failed", zap.Error(err))
	}
	sqlDB, err := db.DB()
	if err != nil {
		logger.Fatal("get sql db failed", zap.Error(err))
	}
	defer sqlDB.Close()

	dataStore := data.NewData(db)
	userRepo := data.NewUserRepo(dataStore, logger)
	jwtManager := auth.NewJWTManager(cfg.JWT)
	userBiz := biz.NewUserUsecase(userRepo, jwtManager, logger)
	userService := service.NewUserService(userBiz, logger)
	jwtMiddleware := middleware.JWT(jwtManager)
	router.Register(r, userService, jwtMiddleware)

	if err := r.Run(cfg.HTTP.Addr); err != nil {
		logger.Fatal("failed to run server", zap.Error(err))
	}
}
