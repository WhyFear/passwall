package handler

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"passwall/internal/adapter/generator"
	"passwall/internal/model"
	"passwall/internal/repository"
)

// GetSubscribe 获取订阅处理器
func GetSubscribe(db *gorm.DB, configToken string, generatorFactory generator.GeneratorFactory) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 验证token
		token := c.Query("token")
		if token == "" || token != configToken {
			c.JSON(http.StatusUnauthorized, gin.H{
				"result":      "fail",
				"status_code": http.StatusUnauthorized,
				"status_msg":  "Unauthorized",
			})
			return
		}

		// 获取请求参数
		subType := c.DefaultQuery("type", "share_url")
		statusStr := c.DefaultQuery("status", "1")

		// 解析状态
		var statusList []model.ProxyStatus
		if statusStr != "" {
			for _, s := range strings.Split(statusStr, ",") {
				if status, err := strconv.Atoi(s); err == nil {
					statusList = append(statusList, model.ProxyStatus(status))
				}
			}
		}

		// 如果没有指定状态，默认使用正常状态
		if len(statusList) == 0 {
			statusList = append(statusList, model.ProxyStatusOK)
		}

		// 查询代理
		proxyRepo := repository.NewProxyRepository(db)
		var proxies []*model.Proxy

		// 根据状态过滤
		for _, status := range statusList {
			statusProxies, err := proxyRepo.FindByStatus(status)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"result":      "fail",
					"status_code": http.StatusInternalServerError,
					"status_msg":  "Failed to query proxies",
				})
				return
			}
			proxies = append(proxies, statusProxies...)
		}
		// 如果没有代理，返回空订阅
		if len(proxies) == 0 {
			c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(""))
			return
		}
		// 对proxies按下载速度排序
		sort.Slice(proxies, func(i, j int) bool {
			return proxies[i].DownloadSpeed > proxies[j].DownloadSpeed
		})

		subscribeGenerator, err := generatorFactory.GetGenerator(subType)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"result":      "fail",
				"status_code": http.StatusBadRequest,
				"status_msg":  "Unsupported subscription type",
			})
			return
		}
		content, err := subscribeGenerator.Generate(proxies)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"result":      "fail",
				"status_code": http.StatusInternalServerError,
				"status_msg":  err.Error(),
			})
		}
		// 直接返回内容
		c.Data(http.StatusOK, "text/plain; charset=utf-8", content)
	}
}
