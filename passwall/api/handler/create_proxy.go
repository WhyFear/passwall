package handler

import (
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/metacubex/mihomo/log"
	"gorm.io/gorm"

	"passwall/config"
	"passwall/internal/adapter/parser"
	"passwall/internal/model"
	"passwall/internal/repository"
	"passwall/internal/service"
	"passwall/internal/util"
)

// CreateProxyRequest 创建代理请求
type CreateProxyRequest struct {
	URL  string `form:"url"`
	Type string `form:"type" binding:"required"`
}

// CreateProxy 创建代理处理器
func CreateProxy(db *gorm.DB, parserFactory parser.ParserFactory, proxyTester service.ProxyTester) gin.HandlerFunc {
	// 加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Errorln("Failed to load config:", err)
	}

	return func(c *gin.Context) {
		var req CreateProxyRequest

		// 绑定请求参数
		if err := c.ShouldBind(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"result":      "fail",
				"status_code": http.StatusBadRequest,
				"status_msg":  "Invalid request parameters",
			})
			return
		}

		// 处理文件上传或URL
		var content []byte
		var subscriptionURL string
		var err error

		// 检查URL是否为空字符串
		if req.URL != "" {
			// 处理URL (去除两端空白)
			req.URL = strings.TrimSpace(req.URL)
			if req.URL == "" {
				c.JSON(http.StatusBadRequest, gin.H{
					"result":      "fail",
					"status_code": http.StatusBadRequest,
					"status_msg":  "URL cannot be empty",
				})
				return
			}

			subscriptionURL = req.URL

			// 判断是否为代理协议URL（如vmess://、ss://等）或普通HTTP/HTTPS链接
			if strings.Contains(req.URL, "://") && !strings.HasPrefix(req.URL, "http") {
				// 对于代理协议URL，直接使用URL作为内容
				content = []byte(req.URL)
			} else {
				// 设置下载选项，包括代理
				downloadOptions := &util.DownloadOptions{
					Timeout:     util.DefaultDownloadOptions.Timeout,
					MaxFileSize: util.DefaultDownloadOptions.MaxFileSize,
				}

				// 如果配置了代理并启用，则使用配置的代理
				if cfg != nil && cfg.Proxy.Enabled && cfg.Proxy.URL != "" {
					downloadOptions.ProxyURL = cfg.Proxy.URL
					log.Infoln("Using proxy for download:", cfg.Proxy.URL)
				}

				// 对于HTTP/HTTPS链接，下载内容
				content, err = util.DownloadFromURL(req.URL, downloadOptions)
				if err != nil {
					log.Errorln("下载订阅内容失败:", err)
					c.JSON(http.StatusBadRequest, gin.H{
						"result":      "fail",
						"status_code": http.StatusBadRequest,
						"status_msg":  "Failed to download from URL: " + err.Error(),
					})
					return
				}
			}
		} else {
			// 处理文件上传
			file, fileHeader, err := c.Request.FormFile("file")
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"result":      "fail",
					"status_code": http.StatusBadRequest,
					"status_msg":  "Missing URL or file",
				})
				return
			}
			defer file.Close()

			// 检查文件大小
			if fileHeader.Size == 0 {
				c.JSON(http.StatusBadRequest, gin.H{
					"result":      "fail",
					"status_code": http.StatusBadRequest,
					"status_msg":  "File cannot be empty",
				})
				return
			}

			// 限制文件大小
			if fileHeader.Size > 10*1024*1024 { // 10MB
				c.JSON(http.StatusBadRequest, gin.H{
					"result":      "fail",
					"status_code": http.StatusBadRequest,
					"status_msg":  "File too large, max 10MB",
				})
				return
			}

			content, err = io.ReadAll(io.LimitReader(file, 10*1024*1024)) // 限制读取大小
			if err != nil {
				log.Errorln("读取上传文件失败:", err)
				c.JSON(http.StatusInternalServerError, gin.H{
					"result":      "fail",
					"status_code": http.StatusInternalServerError,
					"status_msg":  "Failed to read file: " + err.Error(),
				})
				return
			}

			// 检查读取的内容是否为空
			if len(content) == 0 {
				c.JSON(http.StatusBadRequest, gin.H{
					"result":      "fail",
					"status_code": http.StatusBadRequest,
					"status_msg":  "File content cannot be empty",
				})
				return
			}

			// 对于文件上传，使用截取的md5作为订阅URL
			subscriptionURL = util.MD5(string(content))[:20]
		}

		// 获取解析器
		p, err := parserFactory.GetParser(req.Type)
		if err != nil {
			log.Errorln("不支持的代理类型:", req.Type)
			c.JSON(http.StatusBadRequest, gin.H{
				"result":      "fail",
				"status_code": http.StatusBadRequest,
				"status_msg":  "Unsupported proxy type: " + req.Type,
			})
			return
		}

		// 先创建一个订阅记录
		subscriptionRepo := repository.NewSubscriptionRepository(db)

		// 检查URL是否已存在
		existingSub, err := subscriptionRepo.FindByURL(subscriptionURL)
		if err == nil && existingSub != nil {
			// URL已存在，返回现有订阅ID
			log.Infoln("订阅配置已存在:", subscriptionURL)
			c.JSON(http.StatusOK, gin.H{
				"result":          "fail",
				"status_code":     http.StatusOK,
				"status_msg":      "订阅配置已存在",
				"subscription_id": existingSub.ID,
			})
			return
		}

		subscription := &model.Subscription{
			URL:     subscriptionURL,
			Content: string(content), // 保存原始内容
			Type:    model.SubscriptionType(req.Type),
			Status:  model.SubscriptionStatusPending,
		}

		// 先落库
		if err := subscriptionRepo.Create(subscription); err != nil {
			// 检查是否是唯一键冲突错误
			if strings.Contains(err.Error(), "UNIQUE constraint failed") {
				// 尝试查找已存在的记录
				existingSub, findErr := subscriptionRepo.FindByURL(subscriptionURL)
				if findErr == nil && existingSub != nil {
					log.Infoln("订阅配置已存在(创建时检测):", subscriptionURL)
					c.JSON(http.StatusOK, gin.H{
						"result":          "fail",
						"status_code":     http.StatusOK,
						"status_msg":      "订阅配置已存在",
						"subscription_id": existingSub.ID,
					})
					return
				}
			}

			// 其他错误
			log.Errorln("保存订阅配置失败:", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"result":      "fail",
				"status_code": http.StatusInternalServerError,
				"status_msg":  "Failed to save subscription: " + err.Error(),
			})
			return
		}

		// 解析配置
		proxies, err := p.Parse(content)
		if err != nil {
			log.Errorln("解析订阅配置失败:", err)
			// 更新订阅状态为无法处理
			subscription.Status = model.SubscriptionStatusInvalid
			subscriptionRepo.Update(subscription)
			c.JSON(http.StatusBadRequest, gin.H{
				"result":      "fail",
				"status_code": http.StatusBadRequest,
				"status_msg":  "Failed to parse subscription: " + err.Error(),
			})
			return
		}

		// 检查解析结果是否为空
		if len(proxies) == 0 {
			log.Errorln("订阅配置中未找到代理服务器")
			// 更新订阅状态为无法处理
			subscription.Status = model.SubscriptionStatusInvalid
			subscriptionRepo.Update(subscription)
			c.JSON(http.StatusBadRequest, gin.H{
				"result":      "fail",
				"status_code": http.StatusBadRequest,
				"status_msg":  "No proxies found in subscription",
			})
			return
		}

		// 保存解析出的代理服务器
		proxyRepo := repository.NewProxyRepository(db)
		savedCount := 0
		for _, proxy := range proxies {
			// 设置订阅ID
			proxy.SubscriptionID = &subscription.ID

			// 保存代理服务器
			if err := proxyRepo.Create(proxy); err != nil {
				log.Errorln("保存代理服务器失败:", err)
				continue
			}
			savedCount++
		}

		// 更新订阅状态为正常
		subscription.Status = model.SubscriptionStatusOK
		subscriptionRepo.Update(subscription)

		log.Infoln("成功保存 %d/%d 个代理服务器", savedCount, len(proxies))

		// 异步处理解析
		go func() {
			log.Infoln("开始测试代理...")
			if err := proxyTester.TestProxies(&service.TestProxyRequest{
				TestNew:    true,
				Concurrent: cfg.Concurrent,
			}); err != nil {
				log.Errorln("测试代理失败:", err)
			}
		}()

		c.JSON(http.StatusOK, gin.H{
			"result":          "success",
			"status_code":     http.StatusOK,
			"status_msg":      "订阅配置已接收，正在异步处理",
			"subscription_id": subscription.ID,
			"proxy_count":     len(proxies),
			"saved_count":     savedCount,
		})
	}
}
