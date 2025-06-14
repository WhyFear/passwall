package service

import (
	"context"
	"errors"
	"fmt"

	"passwall/config"
	"passwall/internal/adapter/parser"
	"passwall/internal/adapter/speedtester"
	"passwall/internal/model"
	"passwall/internal/repository"
	"passwall/internal/service/proxy"
	"passwall/internal/service/task"

	"github.com/metacubex/mihomo/log"
)

// ProxyTesterAdapter 代理测试适配器
// 兼容旧版 ProxyTester 接口，内部使用新的服务实现
type ProxyTesterAdapter struct {
	proxyTester         proxy.Tester
	subscriptionManager proxy.SubscriptionManager
	taskManager         task.TaskManager
}

// NewProxyTesterAdapter 创建代理测试服务适配器
func NewProxyTesterAdapter(
	proxyRepo repository.ProxyRepository,
	subscriptionRepo repository.SubscriptionRepository,
	speedTestHistoryRepo repository.SpeedTestHistoryRepository,
	speedTesterFactory speedtester.SpeedTesterFactory,
	parserFactory parser.ParserFactory,
	taskManager task.TaskManager,
) ProxyTester {
	// 创建代理测试服务
	proxyTester := proxy.NewTester(
		proxyRepo,
		speedTestHistoryRepo,
		speedTesterFactory,
		taskManager,
	)

	// 创建订阅管理服务
	subscriptionManager := proxy.NewSubscriptionManager(
		subscriptionRepo,
		proxyRepo,
		parserFactory,
		taskManager,
	)

	// 创建适配器
	return &ProxyTesterAdapter{
		proxyTester:         proxyTester,
		subscriptionManager: subscriptionManager,
		taskManager:         taskManager,
	}
}

// TestProxies 测试代理
// 兼容旧版接口，转发到新的服务实现
func (a *ProxyTesterAdapter) TestProxies(request *TestProxyRequest) error {
	if request == nil {
		return errors.New("request cannot be nil")
	}

	ctx := context.Background()

	// 处理订阅刷新
	if request.ReloadSubscribeConfig {
		if err := a.subscriptionManager.RefreshAllSubscriptions(ctx); err != nil {
			log.Errorln("刷新订阅失败: %v", err)
			return fmt.Errorf("刷新订阅失败: %w", err)
		}
	}

	// 设置并发数
	concurrent := request.Concurrent
	if concurrent <= 0 {
		// 从配置文件中获取默认并发数
		cfg, err := config.LoadConfig()
		if err == nil && cfg != nil {
			concurrent = cfg.Concurrent
		}
		if concurrent <= 0 {
			concurrent = 5 // 如果配置中没有设置，使用默认值
		}
	}

	// 创建测试请求
	testRequest := &proxy.TestRequest{
		Concurrent: concurrent,
	}

	// 根据不同的测试类型设置过滤条件
	if request.TestAll {
		// 测试所有代理
		testRequest.Filters = nil
	} else if request.TestNew {
		// 测试新代理
		testRequest.Filters = &proxy.ProxyFilter{
			Status: []model.ProxyStatus{model.ProxyStatusPending},
		}
	} else if request.TestFailed {
		// 测试失败的代理
		testRequest.Filters = &proxy.ProxyFilter{
			Status: []model.ProxyStatus{model.ProxyStatusFailed},
		}
	} else if request.TestSpeed {
		// 测试正常的代理
		testRequest.Filters = &proxy.ProxyFilter{
			Status: []model.ProxyStatus{model.ProxyStatusOK},
		}
		testRequest.Concurrent = 1 // 速度测试使用单线程
	} else {
		log.Infoln("nothing to do")
		return nil
	}

	// 执行测试
	return a.proxyTester.TestProxies(ctx, testRequest)
}
