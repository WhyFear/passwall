package api

import (
	"context"
	"passwall/api/handler"
	"passwall/api/middleware"
	"passwall/config"
	"passwall/internal/scheduler"
	"passwall/internal/service"

	"github.com/gin-gonic/gin"
)

// SetupRouter 设置API路由
func SetupRouter(cfg *config.Config, services *service.Services, scheduler *scheduler.Scheduler) *gin.Engine {
	ctx := context.Background()
	ctx = context.WithValue(ctx, "concurrent", cfg.Concurrent)

	// 创建Gin路由
	router := gin.Default()
	// 添加中间件
	router.Use(middleware.Cors())
	router.Use(middleware.Recovery())

	openApiGroup := router.Group("/api")
	openAuthMiddleware := middleware.AuthReq(cfg.Token)
	openApiGroup.Use(openAuthMiddleware)
	{
		openApiGroup.GET("/subscribe", handler.GetSubscribe(services.ProxyService, services.GeneratorFactory))
	}

	apiGroup := router.Group("/api/v1")
	authMiddleware := middleware.Auth(cfg.Token)
	apiGroup.Use(authMiddleware)
	{
		// 公开API
		apiGroup.POST("/create_proxy", handler.CreateProxy(services.ProxyService, services.SubscriptionManager, services.ParserFactory, services.ProxyTester, services.IPDetectorService))
		apiGroup.POST("/test_proxy_server", handler.TestProxyServer(services.TaskManager, services.ProxyTester))
		apiGroup.POST("/stop_task", handler.StopTask(services.TaskManager))
		apiGroup.POST("/reload_subscription", handler.ReloadSubscription(ctx, services.SubscriptionManager))

		// 添加任务状态API
		apiGroup.GET("/task_all_status", func(c *gin.Context) {
			c.JSON(200, services.TaskManager.GetAllStatus())
		})

		// 添加调度器状态API
		apiGroup.GET("/scheduler_status", func(c *gin.Context) {
			c.JSON(200, scheduler.GetStatus())
		})
		// IP质量检测API
		apiGroup.POST("/batch_detect_ip", handler.BatchDetectIPQuality(cfg.IPCheck, services.IPDetectorService))
	}

	webGroup := router.Group("/web/api")
	webAuthMiddleware := middleware.Auth(cfg.Token)
	webGroup.Use(webAuthMiddleware)
	{
		// 新增订阅
		webGroup.POST("/create_proxy", handler.CreateProxy(services.ProxyService, services.SubscriptionManager, services.ParserFactory, services.ProxyTester, services.IPDetectorService))
		// 获取订阅链接
		webGroup.GET("/subscriptions", handler.GetSubscriptions(services.SubscriptionManager, services.ProxyService))
		// 刷新订阅
		webGroup.POST("/reload_subscription", handler.ReloadSubscription(ctx, services.SubscriptionManager))

		// 获取代理信息
		webGroup.GET("/get_proxies", handler.GetProxies(services.ProxyService, services.SubscriptionManager, services.SpeedTestHistoryService, services.StatisticsService, services.IPDetectorService))
		// 获取代理历史测速记录
		webGroup.GET("/proxy/:id/history", handler.GetProxyHistory(services.SpeedTestHistoryService))
		// 生成代理分享链接
		webGroup.GET("/subscribe", handler.GetSubscribe(services.ProxyService, services.GeneratorFactory))
		// 测试代理服务器
		webGroup.POST("/test_proxy_server", handler.TestProxy(ctx, services.NewTester))
		// 获取所有代理类型
		webGroup.GET("/get_types", handler.GetTypes(services.ProxyService))
		// 置顶代理
		webGroup.POST("/pin_proxy", handler.PinProxy(services.ProxyService))
		// 禁用代理
		webGroup.POST("/ban_proxy", handler.BanProxy(ctx, services.ProxyService))

		// 获取指定任务状态
		webGroup.GET("/get_task_status", handler.GetTaskStatus(services.TaskManager))
		// 停止任务
		webGroup.POST("/stop_task", handler.StopTask(services.TaskManager))

		// IP质量检测API
		webGroup.POST("/detect_ip", handler.DetectIPQuality(cfg.IPCheck, services.IPDetectorService))
		webGroup.GET("/get_ip_info", handler.GetIPQuality(services.IPDetectorService))
		webGroup.GET("/get_country_codes", handler.GetCountryCodeList(services.IPDetectorService))
	}

	// 添加静态文件服务 - 修改为最后添加，避免与API路由冲突
	router.Static("/static", "./web/build/static")
	router.StaticFile("/", "./web/build/index.html")
	router.StaticFile("/favicon.ico", "./web/build/favicon.ico")
	// 处理其他前端路由
	router.NoRoute(func(c *gin.Context) {
		c.File("./web/build/index.html")
	})

	return router
}
