package handler

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"passwall/internal/service/task"
)

type GetTaskStatusReq struct {
	task.TaskType `form:"task_type" json:"task_type" required:"true"`
}

func GetTaskStatus(manager task.TaskManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req GetTaskStatusReq
		if err := c.ShouldBindQuery(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "task_type is required"})
			return
		}
		status := manager.GetStatus(req.TaskType)
		c.JSON(http.StatusOK, status)
	}
}
