package main

import (
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"passwall/api"
	"passwall/config"
	"passwall/internal/repository"
	"passwall/internal/scheduler"
	"passwall/internal/service"
)

func main() {
	// 1. 加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 2. 初始化数据库
	// 注意：这里会自动迁移数据库结构
	db, err := repository.InitDB(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// 3. 初始化服务
	services := service.NewServices(db, cfg)

	// 获取合并后的配置（数据库覆盖文件配置）
	mergedConfig, err := services.ConfigService.GetConfig()
	if err != nil {
		log.Printf("Failed to get merged config, using file config: %v", err)
		mergedConfig = cfg
	}
	// 确保 token 和 database 等关键信息存在 (虽然 ConfigService 应该已经处理了，但为了安全起见)
	if mergedConfig.Token == "" {
		mergedConfig.Token = cfg.Token
	}

	if mergedConfig.ClashAPI.Enable {
		_ = services.StatisticsService.Start()
	}

	// 4. 初始化调度器
	newScheduler := scheduler.NewScheduler()
	newScheduler.SetServices(services.TaskManager, services.NewTester, services.SubscriptionManager, services.ProxyService, services.IPDetectorService)
	err = newScheduler.Init(*mergedConfig)
	if err != nil {
		log.Fatalf("Failed to start scheduler: %v", err)
	}

	// 将调度器注入到 ConfigService，以便后续更新配置时能重载调度器
	services.ConfigService.SetScheduler(newScheduler)
	services.ConfigService.SetStatisticsService(services.StatisticsService)

	// 5. 启动HTTP服务器
	router := api.SetupRouter(mergedConfig, services, newScheduler)

	// 创建HTTP服务器
	server := &http.Server{
		Addr:    cfg.Server.Address,
		Handler: router,
	}

	// 在goroutine中启动服务器，这样就不会阻塞
	go func() {
		log.Printf("Starting server on %s", cfg.Server.Address)
		if err := server.ListenAndServe(); err != nil && !errors.Is(http.ErrServerClosed, err) {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// 等待中断信号以优雅地关闭服务器
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// 停止调度器
	newScheduler.Stop()
	if cfg.ClashAPI.Enable {
		services.StatisticsService.Stop()
	}

	log.Println("Server exiting")
}
