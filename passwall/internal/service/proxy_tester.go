package service

import (
	"context"
	"errors"
	"github.com/metacubex/mihomo/log"

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
	TestAll    bool // 是否测试所有代理
	TestNew    bool // 是否测试新代理
	TestFailed bool // 是否测试失败的代理
	TestSpeed  bool // 是否单线程测试速度
	Concurrent int  // 并发数
}

// ProxyTester 代理测试服务接口
type ProxyTester interface {
	// TestProxies 测试代理
	TestProxies(request *TestProxyRequest, async bool) error
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

// TestProxies 测试代理
func (s *proxyTesterImpl) TestProxies(request *TestProxyRequest, async bool) error {
	if request == nil {
		return errors.New("request cannot be nil")
	}

	// 创建上下文
	ctx := context.Background()

	// 设置并发数
	concurrent := request.Concurrent
	if concurrent <= 0 {
		// 加载配置
		cfg, err := config.LoadConfig()
		if err != nil {
			return errors.New("load config failed: " + err.Error())
		}
		concurrent = cfg.Concurrent
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

	return s.proxyTester.TestProxies(ctx, testRequest, async)
}
