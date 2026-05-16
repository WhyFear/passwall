package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"passwall/internal/model"
	"passwall/internal/service/proxy"
	"strconv"
	"strings"

	"passwall/internal/adapter/generator"

	"github.com/gin-gonic/gin"
	"github.com/metacubex/mihomo/log"
)

const (
	SubscribeTypeClash = "clash"

	ErrNoProxiesFound = "没有找到符合条件的代理服务器"
	ErrConfigUpdate   = "更新代理配置失败"
)

type SubscribeReq struct {
	Token       string `form:"token" required:"true"`
	Type        string `form:"type" required:"true"`
	StatusStr   string `form:"status"`
	ProxyType   string `form:"proxy_type"`
	CountryCode string `form:"country_code"`
	RiskLevel   string `form:"risk_level"`
	AppUnlock   string `form:"app_unlock"`
	Sort        string `form:"sort"`
	SortOrder   string `form:"sortOrder"`
	Limit       int    `form:"limit"`
	ID          int    `form:"id"`
	WithIndex   bool   `form:"with_index"`
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

		content, err := GenerateSubscribeContent(req, proxyService, generatorFactory)
		if err != nil {
			writeSubscribeError(c, err)
			return
		}

		// 直接返回内容
		c.Data(http.StatusOK, "text/plain; charset=utf-8", content)
	}
}

func GenerateSubscribeContent(req SubscribeReq, proxyService proxy.ProxyService, generatorFactory generator.GeneratorFactory) ([]byte, error) {
	subType := req.Type
	limit := req.Limit
	id := req.ID

	var proxies []*model.Proxy

	if id > 0 {
		singleProxy, err := proxyService.GetProxyByID(uint(id))
		if err != nil {
			log.Infoln(ErrNoProxiesFound)
			return []byte(""), nil
		}
		proxies = append(proxies, singleProxy)
	} else {
		filters, err := parseNodeFilter(req.StatusStr, req.ProxyType, req.CountryCode, req.RiskLevel, req.AppUnlock)
		if err != nil {
			return nil, err
		}

		proxies, _, err = proxyService.GetProxiesByFilters(filters, req.Sort, req.SortOrder, 1, limit)
		if err != nil {
			log.Errorln("查询代理服务器失败: %v", err)
			return nil, fmt.Errorf("Failed to query proxies: %w", err)
		}

		if len(proxies) == 0 {
			log.Infoln(ErrNoProxiesFound)
			return []byte(""), nil
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

	subscribeGenerator, err := generatorFactory.GetGenerator(subType)
	if err != nil {
		log.Errorln("不支持的订阅类型: %v", subType)
		return nil, fmt.Errorf("Unsupported subscription type: %s", subType)
	}

	content, err := subscribeGenerator.Generate(proxies)
	if err != nil {
		log.Errorln("生成订阅内容失败: %v", err.Error())
		return nil, err
	}

	log.Infoln("成功生成订阅，类型: %s，代理数量: %d", subType, len(proxies))
	return content, nil
}

func writeSubscribeError(c *gin.Context, err error) {
	msg := err.Error()
	if errors.Is(err, errInvalidNodeFilter) {
		c.JSON(http.StatusBadRequest, gin.H{
			"result":      "fail",
			"status_code": http.StatusBadRequest,
			"status_msg":  "Invalid filter parameters: " + msg,
		})
		return
	}
	if msg == "没有可生成分享链接的代理" {
		c.JSON(http.StatusOK, gin.H{
			"result":      "fail",
			"status_code": http.StatusNotImplemented,
			"status_msg":  "暂不支持该订阅类型生成分享链接",
		})
		return
	}
	if strings.HasPrefix(msg, "Unsupported subscription type:") {
		c.JSON(http.StatusBadRequest, gin.H{
			"result":      "fail",
			"status_code": http.StatusBadRequest,
			"status_msg":  msg,
		})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{
		"result":      "fail",
		"status_code": http.StatusInternalServerError,
		"status_msg":  "Failed to generate subscription: " + msg,
	})
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
