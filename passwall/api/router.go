package api

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"passwall/api/handler"
	"passwall/api/middleware"
	"passwall/config"
	"passwall/internal/scheduler"
	"passwall/internal/service"
)

// SetupRouter 设置API路由
func SetupRouter(cfg *config.Config, db *gorm.DB, services *service.Services, scheduler *scheduler.Scheduler) *gin.Engine {
	// 创建Gin路由
	router := gin.Default()

	// 添加中间件
	router.Use(middleware.Cors())
	router.Use(middleware.Logger())
	router.Use(middleware.Recovery())

	// 添加API路由
	apiGroup := router.Group("/api")

	// 认证中间件
	authMiddleware := middleware.Auth(cfg.Token)

	// 需要认证的API
	apiGroup.Use(authMiddleware)
	{
		// 公开API
		apiGroup.POST("/create_proxy", handler.CreateProxy(db, services.ParserFactory, services.ProxyTester))
		apiGroup.POST("/test_proxy_server", handler.TestProxyServer(db, services.TaskManager, services.ProxyTester))

		apiGroup.GET("/subscribe", handler.GetSubscribe(db, cfg.Token, services.GeneratorFactory))
		apiGroup.POST("/reload_subscription", handler.ReloadSubscription(services.ProxyTester))

		// 添加任务状态API
		apiGroup.GET("/task_status", func(c *gin.Context) {
			c.JSON(200, services.TaskManager.GetAllTaskStatus())
		})

		// 添加调度器状态API
		apiGroup.GET("/scheduler_status", func(c *gin.Context) {
			c.JSON(200, scheduler.GetStatus())
		})
	}

	// 添加API路由
	webGroup := router.Group("/web/api")

	// 认证中间件
	webAuthMiddleware := middleware.Auth(cfg.Token)

	// 需要认证的API
	webGroup.Use(webAuthMiddleware)
	{
		webGroup.POST("/create_proxy", handler.CreateProxy(db, services.ParserFactory, services.ProxyTester))
		webGroup.GET("/subscriptions", handler.GetSubscriptions(db))
		webGroup.GET("/get_proxies", handler.GetProxies(db))
		webGroup.GET("/proxy/:id/history", handler.GetProxyHistory(db))
		webGroup.GET("/subscribe", handler.GetSubscribe(db, cfg.Token, services.GeneratorFactory))
		webGroup.GET("/get_types", handler.GetTypes(db))
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
