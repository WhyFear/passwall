package handler

import (
	"net/http"
	"passwall/internal/service/task"

	"github.com/gin-gonic/gin"
)

// StopTaskRequest 停止任务请求
type StopTaskRequest struct {
	TaskType string `form:"task_type" json:"task_type" binding:"required"` // 任务类型，必填
	Wait     *bool  `form:"wait" json:"wait"`                              // 是否等待任务清理完成
}

// StopTask 停止任务处理器
func StopTask(taskManager task.TaskManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req StopTaskRequest

		// 绑定请求参数
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"result":      "fail",
				"status_code": http.StatusBadRequest,
				"status_msg":  "无效的请求参数，必须指定task_type",
			})
			return
		}

		// 将字符串转换为TaskType
		taskType := task.TaskType(req.TaskType)

		// 检查任务是否存在且正在运行
		if !taskManager.IsRunning(taskType) {
			c.JSON(http.StatusNotFound, gin.H{
				"result":      "fail",
				"status_code": http.StatusNotFound,
				"status_msg":  "指定的任务不存在或未在运行",
			})
			return
		}

		// 默认情况下等待任务清理完成，但不超过10秒
		// 客户端可以通过设置wait=false来立即返回
		wait := true
		if req.Wait != nil {
			wait = *req.Wait
		}

		// 取消任务
		cancelled, timedOut := taskManager.CancelTask(taskType, wait)
		if !cancelled {
			c.JSON(http.StatusInternalServerError, gin.H{
				"result":      "fail",
				"status_code": http.StatusInternalServerError,
				"status_msg":  "停止任务失败",
			})
			return
		}

		// 返回结果
		status := http.StatusOK
		result := "success"
		msg := "任务已成功停止"

		if timedOut {
			msg = "任务已停止，但等待清理超时"
		}

		c.JSON(status, gin.H{
			"result":      result,
			"status_code": status,
			"status_msg":  msg,
			"timed_out":   timedOut,
		})
	}
}
