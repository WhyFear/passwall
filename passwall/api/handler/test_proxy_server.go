package handler

import (
	"net/http"
	"passwall/internal/service/task"

	"github.com/gin-gonic/gin"

	"passwall/internal/service"
)

// TestProxyServerRequest 测试代理服务器请求
type TestProxyServerRequest struct {
	ReloadSubscribeConfig bool `form:"reload_subscribe_config" json:"reload_subscribe_config"`
	TestAll               bool `form:"test_all" json:"test_all"`
	TestNew               bool `form:"test_new" json:"test_new"`
	TestFailed            bool `form:"test_failed" json:"test_failed"`
	TestSpeed             bool `form:"test_speed" json:"test_speed"`
	Concurrent            int  `form:"concurrent" json:"concurrent"`
	AutoBan               bool `form:"auto_ban" json:"auto_ban"`
}

// TestProxyServer 测试代理服务器处理器
func TestProxyServer(taskManager task.TaskManager, proxyTester service.ProxyTester) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req TestProxyServerRequest

		// 绑定请求参数
		if err := c.ShouldBind(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"result":      "fail",
				"status_code": http.StatusBadRequest,
				"status_msg":  "Invalid request parameters",
			})
			return
		}

		// 设置默认值
		if req.Concurrent <= 0 {
			req.Concurrent = 5
		}

		// 检查是否有任务在运行
		if taskManager.IsAnyRunning() {
			c.JSON(http.StatusConflict, gin.H{
				"result":      "task_running",
				"status_code": http.StatusConflict,
				"status_msg":  "Another task is already running",
			})
			return
		}

		// 创建测试请求
		testRequest := &service.TestProxyRequest{
			ReloadSubscribeConfig: req.ReloadSubscribeConfig,
			TestAll:               req.TestAll,
			TestNew:               req.TestNew,
			TestFailed:            req.TestFailed,
			TestSpeed:             req.TestSpeed,
			Concurrent:            req.Concurrent,
		}

		// 执行测试
		if err := proxyTester.TestProxies(testRequest); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"result":      "fail",
				"status_code": http.StatusInternalServerError,
				"status_msg":  "Failed to start test: " + err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"result":      "task_submit",
			"status_code": http.StatusOK,
			"status_msg":  "Task submitted successfully",
		})
	}
}
