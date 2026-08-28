package handler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"strings"

	"passwall/config"
	"passwall/internal/adapter/parser"
	"passwall/internal/model"
	"passwall/internal/service"
	"passwall/internal/service/proxy"
	"passwall/internal/util"

	"github.com/gin-gonic/gin"
	"github.com/metacubex/mihomo/log"
	"gorm.io/gorm"
)

const (
	maxCreateProxyRequestBytes = 12 * 1024 * 1024
	maxSubscriptionFileBytes   = 10 * 1024 * 1024
)

var (
	errSubscriptionExists        = errors.New("subscription already exists")
	errInvalidSubscriptionSource = errors.New("invalid subscription source")
	errInvalidSubscription       = errors.New("invalid subscription")
	errSubscriptionFileTooLarge  = errors.New("subscription file is too large")
)

// CreateProxyRequest 创建代理请求
type CreateProxyRequest struct {
	URL     string   `form:"url" json:"url"`
	URLList []string `form:"url_list" json:"url_list"`
	Type    string   `form:"type" json:"type" binding:"required"`
}

// subProcessor 封装订阅处理的核心上下文
type subProcessor struct {
	proxyService        proxy.ProxyService
	subscriptionManager proxy.SubscriptionManager
	parserFactory       parser.ParserFactory
	proxyTester         service.ProxyTester
	ipDetectorService   service.IPDetectorService
	cfg                 *config.Config
}

// run 核心流水线：解析器 -> 查重 -> 创建订阅 -> 解析节点 -> 节点入库 -> 触发后续
func (p *subProcessor) run(url, reqType string, content []byte) (*model.Subscription, int, error) {
	// 1. 获取解析器
	psr, err := p.parserFactory.GetParser(reqType, content)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: 不支持的解析类型: %v", errInvalidSubscription, err)
	}

	// 2. 查重处理
	existing, err := p.subscriptionManager.GetSubscriptionByURL(url)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, 0, fmt.Errorf("查询订阅失败: %w", err)
	}
	if existing != nil {
		return existing, 0, fmt.Errorf("%w (ID:%d)", errSubscriptionExists, existing.ID)
	}

	// 3. 订阅源初始化入库
	sub := &model.Subscription{
		URL:     url,
		Content: string(content),
		Type:    psr.GetType(),
		Status:  model.SubscriptionStatusPending,
	}
	if err := p.subscriptionManager.CreateSubscription(sub); err != nil {
		return nil, 0, fmt.Errorf("保存订阅失败: %w", err)
	}

	// 4. 解析代理节点
	proxies, err := psr.Parse(content)
	if err != nil {
		return p.failSubscription(sub, fmt.Errorf("%w: 解析节点失败: %v", errInvalidSubscription, err))
	}
	if len(proxies) == 0 {
		return p.failSubscription(sub, fmt.Errorf("%w: 未解析出有效节点", errInvalidSubscription))
	}

	// 5. 节点批量入库
	for _, node := range proxies {
		node.SubscriptionID = &sub.ID
		node.Status = model.ProxyStatusPending
	}
	if err := p.proxyService.BatchCreateProxies(proxies); err != nil {
		return p.failSubscription(sub, fmt.Errorf("节点入库失败: %w", err))
	}

	// 6. 更新订阅状态为完成
	sub.Status = model.SubscriptionStatusOK
	if err := p.subscriptionManager.UpdateSubscriptionStatus(sub); err != nil {
		return sub, 0, fmt.Errorf("更新订阅状态失败: %w", err)
	}

	// 7. 触发自动化后续任务（测试、IP检测）
	p.dispatchTasks(sub.ID, proxies)

	return sub, len(proxies), nil
}

func (p *subProcessor) failSubscription(sub *model.Subscription, cause error) (*model.Subscription, int, error) {
	sub.Status = model.SubscriptionStatusInvalid
	if err := p.subscriptionManager.UpdateSubscriptionStatus(sub); err != nil {
		return sub, 0, fmt.Errorf("更新订阅状态失败: %w", err)
	}
	return sub, 0, cause
}

// dispatchTasks 统一分发节点测试和 IP 归属地检测任务
func (p *subProcessor) dispatchTasks(subID uint, proxies []*model.Proxy) {
	if len(proxies) == 0 {
		return
	}

	// 提取 ID 列表，避免并发引用问题
	ids := make([]uint, len(proxies))
	for i, n := range proxies {
		ids[i] = n.ID
	}

	// 异步延迟测试
	go func() {
		concurrent := 1
		if p.cfg != nil {
			concurrent = p.cfg.Concurrent
		}
		log.Infoln("开始对订阅[ID:%d]进行延迟测试...", subID)
		_ = p.proxyTester.TestProxies(&service.TestProxyRequest{TestNew: true, Concurrent: concurrent}, true)
	}()

	// 异步 IP 详细检测
	go func() {
		if p.cfg == nil || !p.cfg.IPCheck.Enable {
			return
		}
		log.Infoln("开始对订阅[ID:%d]进行 IP 归属地及流媒体检测...", subID)
		_ = p.ipDetectorService.BatchDetect(context.Background(), &service.BatchIPDetectorReq{
			ProxyIDList:     ids,
			Enabled:         true,
			IPInfoEnable:    p.cfg.IPCheck.IPInfo.Enable,
			APPUnlockEnable: p.cfg.IPCheck.AppUnlock.Enable,
			Concurrent:      p.cfg.IPCheck.Concurrent,
		})
	}()
}

// download 处理网络资源下载
func (p *subProcessor) download(u string) ([]byte, error) {
	parsed, err := url.Parse(u)
	if err != nil || parsed.Scheme == "" {
		return nil, errInvalidSubscriptionSource
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return []byte(u), nil
	}
	if parsed.Hostname() == "" || parsed.User != nil {
		return nil, errInvalidSubscriptionSource
	}
	opts := &util.DownloadOptions{
		Timeout:     util.DefaultDownloadOptions.Timeout,
		MaxFileSize: util.DefaultDownloadOptions.MaxFileSize,
	}
	if p.cfg != nil && p.cfg.Proxy.Enabled {
		opts.ProxyURL = p.cfg.Proxy.URL
	}
	return util.DownloadFromURL(u, opts)
}

// CreateProxy 创建代理处理器
func CreateProxy(proxyService proxy.ProxyService, subscriptionManager proxy.SubscriptionManager, parserFactory parser.ParserFactory, proxyTester service.ProxyTester, ipDetectorService service.IPDetectorService, configService service.ConfigService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.HasPrefix(strings.ToLower(c.GetHeader("Content-Type")), "multipart/form-data") {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxCreateProxyRequestBytes)
		}

		// 每次请求都重新获取配置并创建独立的处理器，避免并发竞态
		cfg, err := configService.GetConfig()
		if err != nil {
			writeCreateProxyFailure(c, http.StatusInternalServerError, "读取配置失败")
			return
		}
		proc := &subProcessor{
			proxyService:        proxyService,
			subscriptionManager: subscriptionManager,
			parserFactory:       parserFactory,
			proxyTester:         proxyTester,
			ipDetectorService:   ipDetectorService,
			cfg:                 cfg,
		}

		var req CreateProxyRequest
		bindErr := c.ShouldBind(&req)
		if c.Request.MultipartForm != nil {
			defer c.Request.MultipartForm.RemoveAll()
		}
		if bindErr != nil {
			status := http.StatusBadRequest
			if isRequestTooLarge(bindErr) {
				status = http.StatusRequestEntityTooLarge
			}
			writeCreateProxyFailure(c, status, "请求参数无效")
			return
		}

		// 分支 1: URLList 批量导入 (后台异步)
		if len(req.URLList) > 0 {
			if len(req.URLList) > 50 {
				writeCreateProxyFailure(c, http.StatusBadRequest, "单次最多支持 50 个订阅链接")
				return
			}
			go func() {
				for batchIndex, u := range req.URLList {
					if content, err := proc.download(u); err == nil {
						subscription, count, err := proc.run(u, req.Type, content)
						if err != nil {
							log.Errorln("批量订阅[%d]处理失败，error type: %T", batchIndex, err)
						} else {
							log.Infoln("批量订阅[%d][ID:%d]处理成功，共 %d 个节点", batchIndex, subscription.ID, count)
						}
					} else {
						log.Errorln("批量订阅[%d]下载失败，error type: %T", batchIndex, err)
					}
				}
			}()
			c.JSON(http.StatusAccepted, gin.H{"result": "success", "status_code": http.StatusAccepted, "status_msg": "批量任务已提交后台处理"})
			return
		} else if req.URL != "" { // 分支 2: 单个 URL 导入 (同步)
			content, err := proc.download(req.URL)
			if err != nil {
				log.Errorln("订阅下载失败，error type: %T", err)
				writeCreateProxyFailure(c, createProxyDownloadStatus(err), "订阅下载失败")
				return
			}
			sub, count, err := proc.run(req.URL, req.Type, content)
			if err != nil {
				log.Errorln("订阅处理失败，error type: %T", err)
				writeCreateProxyFailure(c, createProxyProcessingStatus(err), "订阅处理失败")
				return
			}
			c.JSON(http.StatusOK, gin.H{"result": "success", "status_code": http.StatusOK, "subscription_id": sub.ID, "proxy_count": count})
			return
		} else if file, _, err := c.Request.FormFile("file"); err == nil { // 分支 3: 文件上传导入 (同步)
			defer func(file multipart.File) {
				_ = file.Close()
			}(file)
			content, err := readSubscriptionFile(file)
			if err != nil {
				if errors.Is(err, errSubscriptionFileTooLarge) {
					writeCreateProxyFailure(c, http.StatusRequestEntityTooLarge, "订阅文件超过 10 MiB")
				} else {
					writeCreateProxyFailure(c, http.StatusInternalServerError, "读取订阅文件失败")
				}
				return
			}
			pseudoURL := util.MD5(string(content))[:20]
			sub, count, err := proc.run(pseudoURL, req.Type, content)
			if err != nil {
				log.Errorln("本地订阅处理失败，error type: %T", err)
				writeCreateProxyFailure(c, createProxyProcessingStatus(err), "订阅处理失败")
				return
			}
			c.JSON(http.StatusOK, gin.H{"result": "success", "status_code": http.StatusOK, "subscription_id": sub.ID, "proxy_count": count})
			return
		} else if !errors.Is(err, http.ErrMissingFile) {
			status := http.StatusBadRequest
			if isRequestTooLarge(err) {
				status = http.StatusRequestEntityTooLarge
			}
			writeCreateProxyFailure(c, status, "读取上传文件失败")
			return
		}

		writeCreateProxyFailure(c, http.StatusBadRequest, "未识别到有效的订阅来源")
	}
}

func readSubscriptionFile(file io.Reader) ([]byte, error) {
	content, err := io.ReadAll(io.LimitReader(file, maxSubscriptionFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(content) > maxSubscriptionFileBytes {
		return nil, errSubscriptionFileTooLarge
	}
	return content, nil
}

func createProxyProcessingStatus(err error) int {
	switch {
	case errors.Is(err, errSubscriptionExists):
		return http.StatusConflict
	case errors.Is(err, errInvalidSubscription):
		return http.StatusUnprocessableEntity
	default:
		return http.StatusInternalServerError
	}
}

func createProxyDownloadStatus(err error) int {
	switch {
	case errors.Is(err, errInvalidSubscriptionSource):
		return http.StatusBadRequest
	case isTimeoutError(err):
		return http.StatusGatewayTimeout
	default:
		return http.StatusBadGateway
	}
}

func isTimeoutError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func isRequestTooLarge(err error) bool {
	var maxBytesErr *http.MaxBytesError
	return errors.As(err, &maxBytesErr)
}

func writeCreateProxyFailure(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"result": "fail", "status_code": status, "status_msg": message})
}
