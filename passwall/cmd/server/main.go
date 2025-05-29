package main

import (
	"log"
	"net/http"

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
	db, err := repository.InitDB(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// 3. 初始化服务
	services := service.NewServices(db)

	// 4. 初始化调度器
	newScheduler := scheduler.NewScheduler()
	newScheduler.SetServices(services.TaskManager, services.ProxyTester)
	err = newScheduler.Start(cfg.CronJobs)
	if err != nil {
		log.Fatalf("Failed to start scheduler: %v", err)
	}

	// 5. 启动HTTP服务器
	router := api.SetupRouter(cfg, db, services)

	log.Printf("Starting server on %s", cfg.Server.Address)
	if err := http.ListenAndServe(cfg.Server.Address, router); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
