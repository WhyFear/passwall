package handler

import (
	"net/http"
	"passwall/internal/model"
	"passwall/internal/service/proxy"
	"strconv"

	"github.com/gin-gonic/gin"
)

// GetSubscriptionConfig 获取订阅配置
func GetSubscriptionConfig(subsManager proxy.SubscriptionManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.ParseUint(idStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的订阅ID"})
			return
		}

		config, err := subsManager.GetSubscriptionConfig(uint(id))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// 如果没有自定义配置，返回一个标识
		if config == nil {
			c.JSON(http.StatusOK, gin.H{
				"subscription_id": id,
				"is_custom":       false,
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"subscription_id": config.SubscriptionID,
			"auto_update":     config.AutoUpdate,
			"update_interval": config.UpdateInterval,
			"use_proxy":       config.UseProxy,
			"is_custom":       true,
		})
	}
}

// SaveSubscriptionConfig 保存订阅配置
func SaveSubscriptionConfig(subsManager proxy.SubscriptionManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.ParseUint(idStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的订阅ID"})
			return
		}

		var req struct {
			AutoUpdate     bool   `json:"auto_update"`
			UpdateInterval string `json:"update_interval"`
			UseProxy       bool   `json:"use_proxy"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
			return
		}

		subConfig := &model.SubscriptionConfig{
			SubscriptionID: uint(id),
			AutoUpdate:     req.AutoUpdate,
			UpdateInterval: req.UpdateInterval,
			UseProxy:       req.UseProxy,
		}

		if err := subsManager.SaveSubscriptionConfig(subConfig); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "保存成功"})
	}
}
