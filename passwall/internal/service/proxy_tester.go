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
)

// TestProxyRequest 测试代理请求
type TestProxyRequest struct {
	ReloadSubscribeConfig bool // 是否重新加载订阅配置
	TestAll               bool // 是否测试所有代理
	TestNew               bool // 是否测试新代理
	TestFailed            bool // 是否测试失败的代理
	TestSpeed             bool // 是否单线程测试速度
	Concurrent            int  // 并发数
}

// ProxyTester 代理测试服务接口
type ProxyTester interface {
	// TestProxies 测试代理
	TestProxies(request *TestProxyRequest) error
}

// proxyTesterImpl 代理测试服务实现
type proxyTesterImpl struct {
	proxyRepo            repository.ProxyRepository
	subscriptionRepo     repository.SubscriptionRepository
	speedTestHistoryRepo repository.SpeedTestHistoryRepository
	speedTesterFactory   speedtester.SpeedTesterFactory
	parserFactory        parser.ParserFactory
	taskManager          task.TaskManager

	// 新增的代理相关服务
	proxyTester         proxy.Tester
	subscriptionManager proxy.SubscriptionManager
}

// NewProxyTester 创建代理测试服务
func NewProxyTester(
	proxyRepo repository.ProxyRepository,
	subscriptionRepo repository.SubscriptionRepository,
	speedTestHistoryRepo repository.SpeedTestHistoryRepository,
	speedTesterFactory speedtester.SpeedTesterFactory,
	parserFactory parser.ParserFactory,
	taskManager task.TaskManager,
) ProxyTester {
	// 创建代理测试器
	proxyTester := proxy.NewTester(
		proxyRepo,
		speedTestHistoryRepo,
		speedTesterFactory,
		taskManager,
	)

	// 创建订阅管理器
	subscriptionManager := proxy.NewSubscriptionManager(
		subscriptionRepo,
		proxyRepo,
		parserFactory,
		taskManager,
	)

	return &proxyTesterImpl{
		proxyRepo:            proxyRepo,
		subscriptionRepo:     subscriptionRepo,
		speedTestHistoryRepo: speedTestHistoryRepo,
		speedTesterFactory:   speedTesterFactory,
		parserFactory:        parserFactory,
		taskManager:          taskManager,
		proxyTester:          proxyTester,
		subscriptionManager:  subscriptionManager,
	}
}

// TestProxies 测试代理  已经没用了
func (s *proxyTesterImpl) TestProxies(request *TestProxyRequest) error {
	if request == nil {
		return errors.New("request cannot be nil")
	}

	// 创建上下文
	ctx := context.Background()

	// 加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		return errors.New("load config failed: " + err.Error())
	}

	// 重新加载订阅配置
	if request.ReloadSubscribeConfig {
		// 调用订阅管理器刷新所有订阅
		if err := s.subscriptionManager.RefreshAllSubscriptions(ctx); err != nil {
			return fmt.Errorf("刷新订阅失败: %w", err)
		}
	}

	// 测试代理
	var proxiesFilter *proxy.ProxyFilter

	// 设置并发数
	concurrent := request.Concurrent
	if concurrent <= 0 {
		concurrent = cfg.Concurrent
	}

	// 根据请求类型设置过滤条件
	if request.TestAll {
		// 测试所有代理，不需要过滤
	} else if request.TestNew {
		proxiesFilter = &proxy.ProxyFilter{
			Status: []model.ProxyStatus{model.ProxyStatusPending},
		}
	} else if request.TestFailed {
		proxiesFilter = &proxy.ProxyFilter{
			Status: []model.ProxyStatus{model.ProxyStatusFailed},
		}
	} else if request.TestSpeed {
		concurrent = 1
		proxiesFilter = &proxy.ProxyFilter{
			Status: []model.ProxyStatus{model.ProxyStatusOK},
		}
	} else {
		return errors.New("invalid request: no test type specified")
	}

	// 调用代理测试器测试代理
	testRequest := &proxy.TestRequest{
		Filters:    proxiesFilter,
		Concurrent: concurrent,
	}

	return s.proxyTester.TestProxies(ctx, testRequest)
}
