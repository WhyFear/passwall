package handler

import (
	"net/http"
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
	ProxyType string `form:"proxy_type"`
	Sort      string `form:"sort"`
	SortOrder string `form:"sortOrder"`
	Limit     int    `form:"limit"`
	ID        int    `form:"id"`
	WithIndex bool   `form:"with_index"`
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
			return
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
		limit := req.Limit
		id := req.ID

		// 查询代理
		proxyRepo := repository.NewProxyRepository(db)
		var proxies []*model.Proxy

		if id > 0 {
			// 根据ID查询订阅
			proxy, err := proxyRepo.FindByID(uint(id))
			if err != nil {
				log.Infoln("没有找到符合条件的代理服务器")
				c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(""))
				return
			}
			proxies = append(proxies, proxy)
		} else {
			// 构建过滤条件
			filters := make(map[string]interface{})

			// 处理状态过滤
			if req.StatusStr != "" {
				statusList := strings.Split(req.StatusStr, ",")
				filters["status"] = statusList
			} else {
				// 如果没有指定状态，默认使用正常状态
				filters["status"] = []model.ProxyStatus{model.ProxyStatusOK}
			}
			// 和status一样，处理proxy_type
			if req.ProxyType != "" {
				proxyTypeList := strings.Split(req.ProxyType, ",")
				filters["type"] = proxyTypeList
			}

			// 构建查询参数
			pageQuery := repository.PageQuery{
				Filters: filters,
			}

			// 设置排序
			sortField := req.Sort
			if sortField != "" {
				if req.SortOrder == "ascend" || req.SortOrder == "asc" {
					pageQuery.OrderBy = sortField + " ASC"
				} else {
					pageQuery.OrderBy = sortField + " DESC"
				}
			} else {
				// 默认按下载速度降序排序
				pageQuery.OrderBy = "download_speed DESC"
			}

			// 限制返回的代理数量
			if limit > 0 {
				pageQuery.PageSize = limit
			} else {
				pageQuery.PageSize = 10000
			}

			// 执行查询
			queryResult, err := proxyRepo.FindPage(pageQuery)
			if err != nil {
				log.Errorln("查询代理服务器失败:", err)
				c.JSON(http.StatusInternalServerError, gin.H{
					"result":      "fail",
					"status_code": http.StatusInternalServerError,
					"status_msg":  "Failed to query proxies: " + err.Error(),
				})
				return
			}

			proxies = queryResult.Items

			// 如果没有代理，返回空订阅
			if len(proxies) == 0 {
				log.Infoln("没有找到符合条件的代理服务器")
				c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(""))
				return
			}
		}

		if req.WithIndex {
			for i, proxy := range proxies {
				proxy.Name = "[" + strconv.Itoa(i+1) + "]-" + proxy.Name
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
