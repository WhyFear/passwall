package proxy

import (
	"context"
	"fmt"
	"log"

	"passwall/internal/adapter/parser"
	"passwall/internal/adapter/speedtester"
	"passwall/internal/model"
	"passwall/internal/repository"
	"passwall/internal/service/task"
)

// 这是一个使用示例，用于演示如何使用新的服务架构

// TestSingleProxy 测试单个代理
func TestSingleProxy(proxyID uint, repos *repository.Repositories) error {
	// 创建上下文
	ctx := context.Background()

	// 实例化测试服务
	tester := createTester(repos)

	// 获取代理
	proxy, err := repos.Proxy.FindByID(proxyID)
	if err != nil {
		return fmt.Errorf("查找代理失败: %w", err)
	}

	// 测试代理
	log.Printf("开始测试代理: %s", proxy.Name)
	result, err := tester.TestProxy(ctx, proxy)
	if err != nil {
		return fmt.Errorf("测试代理失败: %w", err)
	}

	// 打印结果
	log.Printf("代理[%s]测试结果: Ping=%dms, 下载=%v, 上传=%v",
		proxy.Name, result.Ping, result.DownloadSpeed, result.UploadSpeed)

	return nil
}

// TestAllProxies 测试所有代理
func TestAllProxies(repos *repository.Repositories, concurrent int) error {
	// 创建上下文
	ctx := context.Background()

	// 实例化测试服务
	tester := createTester(repos)

	// 创建测试请求
	request := &TestRequest{
		Concurrent: concurrent,
	}

	// 执行测试
	log.Printf("开始测试所有代理，并发数: %d", concurrent)
	if err := tester.TestProxies(ctx, request); err != nil {
		return fmt.Errorf("测试代理失败: %w", err)
	}

	log.Printf("测试任务已启动")
	return nil
}

// TestPendingProxies 测试待处理的代理
func TestPendingProxies(repos *repository.Repositories, concurrent int) error {
	// 创建上下文
	ctx := context.Background()

	// 实例化测试服务
	tester := createTester(repos)

	// 创建测试请求
	request := &TestRequest{
		Filters: &ProxyFilter{
			Status: []model.ProxyStatus{model.ProxyStatusPending},
		},
		Concurrent: concurrent,
	}

	// 执行测试
	log.Printf("开始测试待处理代理，并发数: %d", concurrent)
	if err := tester.TestProxies(ctx, request); err != nil {
		return fmt.Errorf("测试代理失败: %w", err)
	}

	log.Printf("测试任务已启动")
	return nil
}

// RefreshSubscription 刷新订阅
func RefreshSubscription(repos *repository.Repositories, subID uint) error {
	// 创建上下文
	ctx := context.Background()

	// 实例化订阅管理服务
	manager := createSubscriptionManager(repos)

	// 刷新订阅
	log.Printf("开始刷新订阅 ID: %d", subID)
	if err := manager.RefreshSubscription(ctx, subID); err != nil {
		return fmt.Errorf("刷新订阅失败: %w", err)
	}

	log.Printf("订阅刷新任务已启动")
	return nil
}

// 辅助函数，用于创建服务实例
// 注意：实际应用中应该使用依赖注入来管理这些服务实例
// 这里仅作为示例

// createTester 创建测试器实例
func createTester(repos *repository.Repositories) Tester {
	// 这里需要替换为实际的 SpeedTesterFactory 实例
	speedTesterFactory := getSpeedTesterFactory()
	taskManager := getTaskManager()

	return NewTester(
		repos.Proxy,
		repos.SpeedTestHistory,
		speedTesterFactory,
		taskManager,
	)
}

// createSubscriptionManager 创建订阅管理器实例
func createSubscriptionManager(repos *repository.Repositories) SubscriptionManager {
	// 这里需要替换为实际的 ParserFactory 实例
	parserFactory := getParserFactory()
	taskManager := getTaskManager()

	return NewSubscriptionManager(
		repos.Subscription,
		repos.Proxy,
		parserFactory,
		taskManager,
	)
}

// 辅助函数，获取外部依赖
// 在实际应用中，这些应该通过依赖注入获取
func getSpeedTesterFactory() speedtester.SpeedTesterFactory {
	return nil // 需要替换为实际实例
}

func getParserFactory() parser.ParserFactory {
	return nil // 需要替换为实际实例
}

func getTaskManager() task.TaskManager {
	return task.NewTaskManager()
}
