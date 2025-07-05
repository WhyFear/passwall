package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"passwall/internal/service/proxy"
	"strconv"
	"strings"

	"passwall/internal/adapter/generator"
	"passwall/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/metacubex/mihomo/log"
)

const (
	SubscribeTypeClash = "clash"

	ErrNoProxiesFound = "没有找到符合条件的代理服务器"
	ErrConfigUpdate   = "更新代理配置失败"
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
func GetSubscribe(proxyService proxy.ProxyService, generatorFactory generator.GeneratorFactory) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 解析请求参数
		var req SubscribeReq
		if err := c.ShouldBindQuery(&req); err != nil {
			log.Errorln("解析请求参数失败: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{
				"result":      "fail",
				"status_code": http.StatusBadRequest,
				"status_msg":  "Invalid request parameters: " + err.Error(),
			})
			return
		}

		// 获取请求参数
		subType := req.Type
		limit := req.Limit
		id := req.ID

		// 查询代理
		var proxies []*model.Proxy

		if id > 0 {
			// 根据ID查询订阅
			singleProxy, err := proxyService.GetProxyByID(uint(id))
			if err != nil {
				log.Infoln(ErrNoProxiesFound)
				c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(""))
				return
			}
			proxies = append(proxies, singleProxy)
		} else {
			// 构建过滤条件
			filters := make(map[string]interface{})

			// 处理状态过滤
			if req.StatusStr != "" {
				statusList := strings.Split(req.StatusStr, ",")
				filters["status"] = statusList
			}
			// 和status一样，处理proxy_type
			if req.ProxyType != "" {
				proxyTypeList := strings.Split(req.ProxyType, ",")
				filters["type"] = proxyTypeList
			}

			// 获取所有符合条件的代理
			var err error
			proxies, _, err = proxyService.GetProxiesByFilters(filters, req.Sort, req.SortOrder, 1, limit)
			if err != nil {
				log.Errorln("查询代理服务器失败: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{
					"result":      "fail",
					"status_code": http.StatusInternalServerError,
					"status_msg":  "Failed to query proxies: " + err.Error(),
				})
				return
			}

			// 如果没有代理，返回空订阅
			if len(proxies) == 0 {
				log.Infoln(ErrNoProxiesFound)
				c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(""))
				return
			}
		}

		if req.WithIndex {
			for i, singleProxy := range proxies {
				singleProxy.Name = "[" + strconv.Itoa(i+1) + "]-" + singleProxy.Name

				if subType == SubscribeTypeClash {
					if err := updateProxyConfigName(singleProxy); err != nil {
						log.Errorln("%s: %v，id：%v", ErrConfigUpdate, err, singleProxy.ID)
						continue
					}
				}
			}
		}

		// 获取订阅生成器
		subscribeGenerator, err := generatorFactory.GetGenerator(subType)
		if err != nil {
			log.Errorln("不支持的订阅类型: %v", subType)
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
			log.Errorln("生成订阅内容失败: %v", err.Error())
			if err.Error() == "没有可生成分享链接的代理" {
				c.JSON(http.StatusOK, gin.H{
					"result":      "fail",
					"status_code": http.StatusNotImplemented,
					"status_msg":  "暂不支持该订阅类型生成分享链接",
				})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{
					"result":      "fail",
					"status_code": http.StatusInternalServerError,
					"status_msg":  "Failed to generate subscription: " + err.Error(),
				})
			}
			return
		}

		log.Infoln("成功生成订阅，类型: %s，代理数量: %d", subType, len(proxies))

		// 直接返回内容
		c.Data(http.StatusOK, "text/plain; charset=utf-8", content)
	}
}

// updateProxyConfigName 更新代理配置中的name字段
func updateProxyConfigName(proxy *model.Proxy) error {
	if proxy.Config == "" {
		return fmt.Errorf("代理配置为空")
	}

	var config map[string]interface{}

	if err := json.Unmarshal([]byte(proxy.Config), &config); err != nil {
		return fmt.Errorf("反序列化代理配置失败: %w", err)
	}

	if config == nil {
		return fmt.Errorf("代理配置无效")
	}

	config["name"] = proxy.Name

	jsonConfig, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("序列化代理配置失败: %w", err)
	}

	proxy.Config = string(jsonConfig)
	return nil
}
