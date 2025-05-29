package api

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"passwall/api/handler"
	"passwall/api/middleware"
	"passwall/config"
	"passwall/internal/service"
)

// SetupRouter 设置API路由
func SetupRouter(cfg *config.Config, db *gorm.DB, services *service.Services) *gin.Engine {
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

	// 公开API
	apiGroup.POST("/create_proxy", handler.CreateProxy(db, services.ParserFactory, services.ProxyTester))
	apiGroup.POST("/test_proxy_server", handler.TestProxyServer(db, services.TaskManager, services.ProxyTester))
	apiGroup.GET("/proxy/:id/history", handler.GetProxyHistory(db))

	// 需要认证的API
	apiGroup.Use(authMiddleware)
	{

		apiGroup.GET("/subscribe", handler.GetSubscribe(db, cfg.Token, services.GeneratorFactory))
		apiGroup.POST("/reload_subscription", handler.ReloadSubscription(services.TaskManager, services.ProxyTester))

		// 添加任务状态API
		apiGroup.GET("/task_status", func(c *gin.Context) {
			c.JSON(200, services.TaskManager.GetAllTaskStatus())
		})
	}

	return router
}
