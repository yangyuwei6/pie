package main

import (
	"context"

	"pie/internal/auth"
	"pie/internal/config"
	"pie/internal/data"
	"pie/internal/embedding"
	uploadhandler "pie/internal/handler/upload"
	userhandler "pie/internal/handler/user"
	"pie/internal/log"
	kafkamessaging "pie/internal/messaging/kafka"
	"pie/internal/middleware"
	"pie/internal/pipeline"
	"pie/internal/router"
	searches "pie/internal/search/es"
	"pie/internal/service"
	"pie/internal/tika"

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

	rdb, err := data.NewRedis(cfg.Redis)
	if err != nil {
		logger.Fatal("init redis failed", zap.Error(err))
	}
	defer rdb.Close()

	minioClient, err := data.NewMinIO(cfg.ObjectStore)
	if err != nil {
		logger.Fatal("init minio failed", zap.Error(err))
	}

	dataStore := data.NewData(db, rdb, minioClient, cfg.ObjectStore.Bucket)
	fileTaskProducer := kafkamessaging.NewProducer(cfg.Messaging)
	defer fileTaskProducer.Close()

	userRepo := data.NewUserRepo(dataStore)
	tokenRepo := data.NewTokenRepo(dataStore)
	orgTagRepo := data.NewOrgTagRepo(dataStore)
	uploadRepo := data.NewUploadRepo(dataStore)
	documentVectorRepo := data.NewDocumentVectorRepo(dataStore)
	jwtManager := auth.NewJWTManager(cfg.JWT)
	userService := service.NewUserService(userRepo, tokenRepo, orgTagRepo, jwtManager, logger)
	uploadService := service.NewUploadService(uploadRepo, userRepo, fileTaskProducer, logger)
	userHandler := userhandler.NewHandler(userService, logger)
	uploadHandler := uploadhandler.NewHandler(uploadService, logger)
	jwtMiddleware := middleware.JWT(jwtManager, userService)
	router.Register(r, userHandler, uploadHandler, jwtMiddleware)

	tikaClient := tika.NewClient(cfg.AI.TikaURL)
	embeddingClient := embedding.NewClient(cfg.AI)
	searchIndexer := searches.NewIndexer(cfg.Search)
	if err := searchIndexer.EnsureIndex(context.Background()); err != nil {
		logger.Warn("ensure elasticsearch index failed", zap.Error(err))
	}
	processor := pipeline.NewProcessor(
		uploadRepo,
		documentVectorRepo,
		tikaClient,
		embeddingClient,
		searchIndexer,
		logger,
	)
	consumer := kafkamessaging.NewConsumer(cfg.Messaging, processor, logger)
	go func() {
		if err := consumer.Run(context.Background()); err != nil {
			logger.Error("kafka consumer stopped", zap.Error(err))
		}
	}()

	if err := r.Run(cfg.HTTP.Addr); err != nil {
		logger.Fatal("failed to run server", zap.Error(err))
	}
}
