package handler

import (
	"context"
	"net/http"
	"passwall/internal/scheduler"
	"passwall/internal/service/proxy"

	"github.com/gin-gonic/gin"
)

type DeleteSubscriptionReq struct {
	ID uint `form:"id"`
}

func DeleteSubscription(ctx context.Context, service proxy.SubscriptionManager, scheduler *scheduler.Scheduler) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req DeleteSubscriptionReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"result":      err.Error(),
				"status_code": http.StatusBadRequest,
				"status_msg":  "Invalid request parameters",
			})
			return
		}

		if req.ID <= 0 {
			c.JSON(http.StatusOK, gin.H{
				"result":      "ID is required",
				"status_code": http.StatusBadRequest,
				"status_msg":  "Invalid request parameters",
			})
			return
		}

		if err := service.DeleteSubscription(req.ID); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"result":      err.Error(),
				"status_code": http.StatusInternalServerError,
				"status_msg":  "Failed to delete subscription",
			})
			return
		}

		// 通知调度器移除任务
		_ = scheduler.UpdateSubscriptionJob(req.ID)

		c.JSON(http.StatusOK, gin.H{
			"result":      "Subscription deleted successfully",
			"status_code": http.StatusOK,
			"status_msg":  "Subscription deleted successfully",
		})
	}
}
