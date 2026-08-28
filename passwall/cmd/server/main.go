package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

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
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("Failed to get database connection pool: %v", err)
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

	if err := services.StatisticsService.Restart(mergedConfig.ClashAPI); err != nil {
		log.Fatalf("Failed to start traffic statistics service: %v", err)
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
	server := newHTTPServer(cfg.Server.Address, router)
	runCtx, stopSignals := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stopSignals()

	// 在goroutine中启动服务器，这样就不会阻塞
	serverErr := make(chan error, 1)
	go func() {
		log.Printf("Starting server on %s", cfg.Server.Address)
		err := server.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		serverErr <- err
	}()

	// 等待中断信号以优雅地关闭服务器
	var listenErr error
	select {
	case <-runCtx.Done():
	case listenErr = <-serverErr:
	}
	log.Println("Shutting down server...")
	cronStopCtx := newScheduler.BeginStop()

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 30*time.Second)
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Failed to shut down HTTP server: %v", err)
	}
	cancelShutdown()

	taskCtx, cancelTasks := context.WithTimeout(context.Background(), 30*time.Second)
	if err := services.TaskManager.Shutdown(taskCtx); err != nil {
		log.Printf("Failed to stop background tasks: %v", err)
	}
	cancelTasks()

	<-cronStopCtx.Done()
	const trafficStopAttempts = 5
	for attempt := 1; attempt <= trafficStopAttempts; attempt++ {
		if err := services.StatisticsService.Stop(); err != nil {
			if attempt == trafficStopAttempts {
				log.Printf("Failed to stop traffic statistics service after %d attempts: %v", attempt, err)
				break
			}
			log.Printf("Failed to stop traffic statistics service, retrying (%d/%d): %v", attempt, trafficStopAttempts, err)
			time.Sleep(time.Second)
			continue
		}
		break
	}
	if err := sqlDB.Close(); err != nil {
		log.Printf("Failed to close database connection pool: %v", err)
	}

	log.Println("Server exiting")
	if listenErr != nil {
		log.Fatalf("HTTP server stopped unexpectedly: %v", listenErr)
	}
}

func newHTTPServer(address string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
}
