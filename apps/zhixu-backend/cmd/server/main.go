package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"zhixu-backend/internal/config"
	"zhixu-backend/internal/database"
	"zhixu-backend/internal/handler"
	"zhixu-backend/internal/log"
	"zhixu-backend/internal/middleware"
	"zhixu-backend/internal/repository"
	"zhixu-backend/internal/router"
	"zhixu-backend/internal/service"
	"zhixu-backend/internal/utils"
)

func main() {
	cfg := config.MustLoad()

	log.Init(cfg.LogLevel, cfg.AppEnv)
	database.InitDB(cfg)
	utils.InitJWT(cfg)
	r := router.SetUpRouter(cfg)
	router.RegisterServiceStatusRouter(r, service.NewServiceStatusService(cfg))
	credentials, err := service.NewModelCredentials(cfg.ModelConfigEncryptionKey)
	if err != nil {
		log.Fatal().Msg("模型密钥加密配置无效")
	}
	router.RegisterAIConfigRouter(r, &service.AIConfigService{
		Store: &repository.AIConfigRepository{DB: database.DB}, Credentials: credentials,
	})
	chat := &handler.ChatHandler{
		Service: &service.ChatService{Store: &repository.AIConfigRepository{DB: database.DB}, Credentials: credentials, AllowedPrivateEndpoints: cfg.ModelAllowedPrivateEndpoints},
		AIURL:   cfg.ChatAIURL, InternalBackendURL: cfg.ChatBackendURL, InternalAPISecret: cfg.InternalAPISecret,
	}
	chatSourceRepo := repository.NewChatSourceRepository(database.DB)
	conversations := &service.ConversationService{
		AIURL: cfg.ChatAIURL, Secret: cfg.InternalAPISecret, Sources: chatSourceRepo,
	}
	chat.Conversations = conversations
	conversationHandler := &handler.ConversationHandler{Conversations: conversations}
	qdrant := &service.QdrantClient{URL: cfg.QdrantURL, APIKey: cfg.QdrantAPIKey}
	chat.Rag = &service.RagSearcher{
		Chat:     chat.Service,
		Embedder: &service.MastraEmbedder{AIURL: cfg.ChatAIURL, Secret: cfg.InternalAPISecret},
		Vectors:  qdrant, Chunks: &repository.RagRepository{DB: database.DB},
		Logs: repository.NewRetrievalLogRepository(database.DB),
	}
	r.POST("/chat", middleware.AuthMiddleware(), chat.Send)
	r.GET("/chat/conversations", middleware.AuthMiddleware(), conversationHandler.List)
	r.GET("/chat/conversations/:id", middleware.AuthMiddleware(), conversationHandler.Detail)
	r.PATCH("/chat/conversations/:id", middleware.AuthMiddleware(), conversationHandler.Rename)
	r.DELETE("/chat/conversations/:id", middleware.AuthMiddleware(), conversationHandler.Delete)
	r.POST("/internal/tools/v1/search", chat.RagSearch)
	knowledgeService := service.NewKnowledgeService(repository.NewKnowledgeRepository(database.DB), cfg.FileStoragePath)
	aiTasks := service.NewAiTaskService(&repository.AiTaskRepository{DB: database.DB})
	knowledgeService.Tasks = aiTasks
	vectorStore := &repository.VectorizeRepository{DB: database.DB}
	knowledgeService.Vectors = vectorStore
	knowledgeService.Logs = repository.NewOperationLogRepository(database.DB)
	knowledgeService.Fetcher = service.NewWebFetcher()
	vectorService := &service.VectorizeService{
		Store: vectorStore, Knowledge: repository.NewKnowledgeRepository(database.DB), Chat: chat.Service,
		Embedder:  &service.MastraEmbedder{AIURL: cfg.ChatAIURL, Secret: cfg.InternalAPISecret},
		Vectors:   qdrant,
		Relations: aiTasks,
	}
	vectorCleanup := &service.VectorCleanupService{
		Store: &repository.VectorCleanupRepository{DB: database.DB}, Vectors: qdrant,
	}
	router.RegisterKnowledgeRouter(r, knowledgeService)
	tagService := service.NewTagService(repository.NewTagRepository(database.DB))
	router.RegisterTagRouter(r, &handler.TagHandler{Service: tagService})
	relationRepo := repository.NewRelationRepository(database.DB)
	relationService := service.NewRelationService(relationRepo, knowledgeService, aiTasks)
	router.RegisterRelationRouter(r, &handler.RelationHandler{Service: relationService})
	// 采纳 AI 整理结果时需要追加标签：只增不删，不覆盖用户手工标签。
	knowledgeService.Tags = tagService
	// AI 整理工作流：Go worker 直接调用 AI 服务 /organize，草稿只写 ai_tasks.payload，等用户确认后才落正式字段。
	aiTasks.Register(&service.OrganizeProcessor{
		Chat: chat.Service, Knowledge: knowledgeService, Tags: tagService,
		Client: &service.OrganizeClient{AIURL: cfg.ChatAIURL, Secret: cfg.InternalAPISecret},
	})
	// 关联建议工作流：向量化就绪后入队，候选由向量召回，模型只负责关系判定。
	aiTasks.Register(&service.RelationProcessor{
		Chat: chat.Service, Embedding: chat.Service, Knowledge: knowledgeService, Relations: relationRepo,
		Embedder: &service.MastraEmbedder{AIURL: cfg.ChatAIURL, Secret: cfg.InternalAPISecret},
		Vectors:  qdrant,
		Client:   &service.RelateClient{AIURL: cfg.ChatAIURL, Secret: cfg.InternalAPISecret},
	})
	workerCtx, stopWorker := context.WithCancel(context.Background())
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		knowledgeService.Run(workerCtx)
	}()
	vectorDone := make(chan struct{})
	go func() {
		defer close(vectorDone)
		vectorService.Run(workerCtx)
	}()

	vectorCleanupDone := make(chan struct{})
	go func() {
		defer close(vectorCleanupDone)
		vectorCleanup.Run(workerCtx)
	}()
	conversationCleanupDone := make(chan struct{})
	go func() {
		defer close(conversationCleanupDone)
		conversations.Run(workerCtx)
	}()
	aiTaskDone := make(chan struct{})
	go func() {
		defer close(aiTaskDone)
		aiTasks.Run(workerCtx)
	}()

	port := cfg.Port
	if port == "" {
		port = "9000"
	}

	addr := ":" + port
	srv := &http.Server{Addr: addr, Handler: r}

	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			<-ticker.C
			repo := repository.NewFileRepository(database.DB)
			if err := repo.DeleteExpiredChunks(24*time.Hour, cfg.ChunkStoragePath); err != nil {
				log.Error().Err(err).Msg("failed to delete expired upload chunks")
			}
		}
	}()

	go func() {
		log.Info().Str("port", port).Msg("服务器启动")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("服务器启动失败")
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Info().Msg("正在关闭服务器")
	stopWorker()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal().Err(err).Msg("服务器关闭失败")
	}
	select {
	case <-workerDone:
	case <-ctx.Done():
		log.Warn().Msg("知识导入任务将在下次启动恢复")
	}
	select {
	case <-vectorDone:
	case <-ctx.Done():
		log.Warn().Msg("向量化任务将在下次启动恢复")
	}
	select {
	case <-vectorCleanupDone:
	case <-ctx.Done():
		log.Warn().Msg("向量清理任务将在下次启动恢复")
	}
	select {
	case <-conversationCleanupDone:
	case <-ctx.Done():
		log.Warn().Msg("会话清理任务将在下次启动恢复")
	}
	select {
	case <-aiTaskDone:
	case <-ctx.Done():
		log.Warn().Msg("AI 任务将在下次启动恢复")
	}
	log.Info().Msg("服务器已退出")
}
