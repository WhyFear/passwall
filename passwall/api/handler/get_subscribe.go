package handler

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	"passwall/internal/adapter/generator"
	"passwall/internal/model"
	"passwall/internal/repository"

	"github.com/gin-gonic/gin"
	"github.com/metacubex/mihomo/log"
	"gorm.io/gorm"
)

type SubscribeReq struct {
	Token     string `form:"token" required:"true"`
	Type      string `form:"type" required:"true"`
	StatusStr string `form:"status"`
	Sort      string `form:"sort"`
	Limit     int    `form:"limit"`
	ID        int    `form:"id"`
}

// GetSubscribe 获取订阅处理器
func GetSubscribe(db *gorm.DB, configToken string, generatorFactory generator.GeneratorFactory) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 解析请求参数
		var req SubscribeReq
		if err := c.ShouldBindQuery(&req); err != nil {
			log.Errorln("解析请求参数失败:", err)
			c.JSON(http.StatusBadRequest, gin.H{
				"result":      "fail",
				"status_code": http.StatusBadRequest,
				"status_msg":  "Invalid request parameters: " + err.Error(),
			})
		}

		// 验证token
		token := req.Token
		if token == "" || token != configToken {
			c.JSON(http.StatusUnauthorized, gin.H{
				"result":      "fail",
				"status_code": http.StatusUnauthorized,
				"status_msg":  "Unauthorized",
			})
			return
		}

		// 获取请求参数
		subType := req.Type
		statusStr := req.StatusStr
		sortBy := req.Sort
		limit := req.Limit
		id := req.ID

		// 查询代理
		proxyRepo := repository.NewProxyRepository(db)
		var proxies []*model.Proxy

		if id != 0 {
			// 根据ID查询订阅
			proxy, err := proxyRepo.FindByID(uint(id))
			if err != nil {
				log.Infoln("没有找到符合条件的代理服务器")
				c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(""))
			}
			proxies = append(proxies, proxy)

		} else {
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

			// 根据状态过滤
			for _, status := range statusList {
				statusProxies, err := proxyRepo.FindByStatus(status)
				if err != nil {
					log.Errorln("查询代理服务器失败:", err)
					c.JSON(http.StatusInternalServerError, gin.H{
						"result":      "fail",
						"status_code": http.StatusInternalServerError,
						"status_msg":  "Failed to query proxies: " + err.Error(),
					})
					return
				}
				proxies = append(proxies, statusProxies...)
			}
			// 如果没有代理，返回空订阅
			if len(proxies) == 0 {
				log.Warnln("没有找到符合条件的代理服务器")
				c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(""))
				return
			}

			// 根据排序参数对proxies进行排序
			switch sortBy {
			case "speed", "download_speed":
				// 按下载速度排序（默认）
				sort.Slice(proxies, func(i, j int) bool {
					return proxies[i].DownloadSpeed > proxies[j].DownloadSpeed
				})
			case "ping", "latency":
				// 按延迟排序
				sort.Slice(proxies, func(i, j int) bool {
					// 延迟为0的放在最后（可能是未测试）
					if proxies[i].Ping == 0 {
						return false
					}
					if proxies[j].Ping == 0 {
						return true
					}
					return proxies[i].Ping < proxies[j].Ping
				})
			case "name":
				// 按名称排序
				sort.Slice(proxies, func(i, j int) bool {
					return proxies[i].Name < proxies[j].Name
				})
			case "time", "latest":
				// 按最近测试时间排序
				sort.Slice(proxies, func(i, j int) bool {
					if proxies[i].LatestTestTime == nil {
						return false
					}
					if proxies[j].LatestTestTime == nil {
						return true
					}
					return proxies[i].LatestTestTime.After(*proxies[j].LatestTestTime)
				})
			default:
				// 默认按下载速度排序
				sort.Slice(proxies, func(i, j int) bool {
					return proxies[i].DownloadSpeed > proxies[j].DownloadSpeed
				})
			}

			// 限制返回的代理数量
			if limit > 0 && len(proxies) > limit {
				proxies = proxies[:limit]
			}
		}

		// 获取订阅生成器
		subscribeGenerator, err := generatorFactory.GetGenerator(subType)
		if err != nil {
			log.Errorln("不支持的订阅类型:", subType)
			c.JSON(http.StatusBadRequest, gin.H{
				"result":      "fail",
				"status_code": http.StatusBadRequest,
				"status_msg":  "Unsupported subscription type: " + subType,
			})
			return
		}

		// 生成订阅内容
		content, err := subscribeGenerator.Generate(proxies)
		if err != nil {
			log.Errorln("生成订阅内容失败:", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"result":      "fail",
				"status_code": http.StatusInternalServerError,
				"status_msg":  "Failed to generate subscription: " + err.Error(),
			})
			return
		}

		log.Infoln("成功生成订阅，类型: %s，代理数量: %d", subType, len(proxies))

		// 直接返回内容
		c.Data(http.StatusOK, "text/plain; charset=utf-8", content)
	}
}
