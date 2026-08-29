package handler

import (
	"errors"
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

		// 如果有自定义配置
		if config != nil {
			c.JSON(http.StatusOK, gin.H{
				"subscription_id": config.SubscriptionID,
				"auto_update":     config.AutoUpdate,
				"update_interval": config.UpdateInterval,
				"use_proxy":       config.UseProxy,
				"is_custom":       true,
			})
			return
		}

		// 如果没有自定义配置，尝试获取系统默认配置
		// 注意：这里我们通过 subsManager 获取，如果 subsManager 没有直接暴露，我们可能需要通过它转发
		// 实际上，我们可以在这里直接返回 is_custom: false，但为了用户体验，我们最好直接给数据
		c.JSON(http.StatusOK, gin.H{
			"subscription_id": id,
			"is_custom":       false,
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
			status := http.StatusInternalServerError
			if errors.Is(err, proxy.ErrSubscriptionNotFound) {
				status = http.StatusNotFound
			} else if errors.Is(err, proxy.ErrInvalidSubscriptionConfig) {
				status = http.StatusUnprocessableEntity
			}
			c.JSON(status, gin.H{"error": "保存订阅配置失败"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "保存成功"})
	}
}
