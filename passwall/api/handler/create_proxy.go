package handler

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"passwall/config"
	"passwall/internal/adapter/parser"
	"passwall/internal/model"
	"passwall/internal/service"
	"passwall/internal/service/proxy"
	"passwall/internal/util"

	"github.com/gin-gonic/gin"
	"github.com/metacubex/mihomo/log"
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
		return nil, 0, fmt.Errorf("不支持的解析类型: %w", err)
	}

	// 2. 查重处理
	if existing, err := p.subscriptionManager.GetSubscriptionByURL(url); err == nil && existing != nil {
		return existing, 0, fmt.Errorf("订阅已存在(ID:%d)", existing.ID)
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
		sub.Status = model.SubscriptionStatusInvalid
		_ = p.subscriptionManager.UpdateSubscriptionStatus(sub)
		return sub, 0, fmt.Errorf("解析节点失败: %w", err)
	}

	// 5. 节点批量入库
	if len(proxies) > 0 {
		for _, node := range proxies {
			node.SubscriptionID = &sub.ID
			node.Status = model.ProxyStatusPending
		}
		if err := p.proxyService.BatchCreateProxies(proxies); err != nil {
			log.Errorln("[%s] 节点入库异常: %v", url, err)
		}
	}

	// 6. 更新订阅状态为完成
	sub.Status = model.SubscriptionStatusOK
	_ = p.subscriptionManager.UpdateSubscriptionStatus(sub)

	// 7. 触发自动化后续任务（测试、IP检测）
	p.dispatchTasks(sub.ID, proxies)

	return sub, len(proxies), nil
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
		_ = p.ipDetectorService.BatchDetect(&service.BatchIPDetectorReq{
			ProxyIDList:     ids,
			Enabled:         true,
			IPInfoEnable:    p.cfg.IPCheck.IPInfo.Enable,
			APPUnlockEnable: p.cfg.IPCheck.IPInfo.Enable,
			Concurrent:      p.cfg.IPCheck.Concurrent,
		})
	}()
}

// download 处理网络资源下载
func (p *subProcessor) download(u string) ([]byte, error) {
	if strings.Contains(u, "://") && !strings.HasPrefix(u, "http") {
		return []byte(u), nil
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
func CreateProxy(proxyService proxy.ProxyService, subscriptionManager proxy.SubscriptionManager, parserFactory parser.ParserFactory, proxyTester service.ProxyTester, ipDetectorService service.IPDetectorService) gin.HandlerFunc {
	cfg, _ := config.LoadConfig()
	proc := &subProcessor{
		proxyService:        proxyService,
		subscriptionManager: subscriptionManager,
		parserFactory:       parserFactory,
		proxyTester:         proxyTester,
		ipDetectorService:   ipDetectorService,
		cfg:                 cfg,
	}

	return func(c *gin.Context) {
		var req CreateProxyRequest
		if err := c.ShouldBind(&req); err != nil {
			c.JSON(http.StatusOK, gin.H{"result": "fail", "status_code": http.StatusBadRequest, "status_msg": "请求参数无效"})
			return
		}

		// 分支 1: URLList 批量导入 (后台异步)
		if len(req.URLList) > 0 {
			if len(req.URLList) > 50 {
				c.JSON(http.StatusOK, gin.H{"result": "fail", "status_code": http.StatusBadRequest, "status_msg": "单次最多支持 50 个订阅链接"})
				return
			}
			go func() {
				for _, u := range req.URLList {
					if content, err := proc.download(u); err == nil {
						_, _, _ = proc.run(u, req.Type, content)
					} else {
						log.Errorln("批量下载失败 [%s]: %v", u, err)
					}
				}
			}()
			c.JSON(http.StatusOK, gin.H{"result": "success", "status_code": http.StatusOK, "status_msg": "批量任务已提交后台处理"})
			return
		} else if req.URL != "" { // 分支 2: 单个 URL 导入 (同步)
			content, err := proc.download(req.URL)
			if err != nil {
				c.JSON(http.StatusOK, gin.H{"result": "fail", "status_code": http.StatusBadRequest, "status_msg": "订阅下载失败: " + err.Error()})
				return
			}
			sub, count, err := proc.run(req.URL, req.Type, content)
			if err != nil && sub == nil {
				c.JSON(http.StatusOK, gin.H{"result": "fail", "status_code": http.StatusOK, "status_msg": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"result": "success", "status_code": http.StatusOK, "subscription_id": sub.ID, "proxy_count": count})
			return
		} else if file, _, err := c.Request.FormFile("file"); err == nil { // 分支 3: 文件上传导入 (同步)
			defer file.Close()
			content, _ := io.ReadAll(io.LimitReader(file, 10*1024*1024))
			pseudoURL := util.MD5(string(content))[:20]
			sub, count, err := proc.run(pseudoURL, req.Type, content)
			if err != nil && sub == nil {
				c.JSON(http.StatusOK, gin.H{"result": "fail", "status_code": http.StatusOK, "status_msg": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"result": "success", "status_code": http.StatusOK, "subscription_id": sub.ID, "proxy_count": count})
			return
		}

		c.JSON(http.StatusOK, gin.H{"result": "fail", "status_code": http.StatusBadRequest, "status_msg": "未识别到有效的订阅来源"})
	}
}
