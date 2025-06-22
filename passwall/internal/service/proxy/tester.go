package proxy

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"passwall/internal/adapter/speedtester"
	"passwall/internal/model"
	"passwall/internal/repository"
	"passwall/internal/service/task"

	"github.com/metacubex/mihomo/log"
	"golang.org/x/sync/errgroup"
)

// TestRequest 测试代理请求
type TestRequest struct {
	ProxyIDs   []int64      // 指定要测试的代理ID列表
	Filters    *ProxyFilter // 筛选条件
	Concurrent int          // 并发数
}

// ProxyFilter 代理筛选条件
type ProxyFilter struct {
	Status []model.ProxyStatus // 状态过滤
	Types  []model.ProxyType   // 类型过滤
}

// Tester 代理测试服务接口
type Tester interface {
	// TestProxy 测试单个代理
	TestProxy(proxy *model.Proxy) (*model.SpeedTestResult, error)

	// TestProxies 批量测试代理
	TestProxies(ctx context.Context, request *TestRequest) error
}

// testerImpl 代理测试服务实现
type testerImpl struct {
	proxyRepo            repository.ProxyRepository
	speedTestHistoryRepo repository.SpeedTestHistoryRepository
	speedTesterFactory   speedtester.SpeedTesterFactory
	taskManager          task.TaskManager
}

// NewTester 创建代理测试服务
func NewTester(
	proxyRepo repository.ProxyRepository,
	speedTestHistoryRepo repository.SpeedTestHistoryRepository,
	speedTesterFactory speedtester.SpeedTesterFactory,
	taskManager task.TaskManager,
) Tester {
	return &testerImpl{
		proxyRepo:            proxyRepo,
		speedTestHistoryRepo: speedTestHistoryRepo,
		speedTesterFactory:   speedTesterFactory,
		taskManager:          taskManager,
	}
}

// TestProxy 测试单个代理
func (t *testerImpl) TestProxy(proxy *model.Proxy) (*model.SpeedTestResult, error) {
	if proxy == nil {
		return nil, fmt.Errorf("代理对象不能为空")
	}

	// 获取速度测试器
	tester, err := t.speedTesterFactory.GetSpeedTester(proxy.Type)
	if err != nil || tester == nil {
		return nil, fmt.Errorf("获取测速器失败: %w", err)
	}

	// 测试代理
	result, err := tester.Test(proxy)
	if err != nil {
		log.Errorln("测试代理失败[代理ID:%d]: %v", proxy.ID, err)
		return nil, err
	}

	// 检查结果
	if result == nil {
		return nil, fmt.Errorf("测试结果为空")
	}

	return result, nil
}

// TestProxies 批量测试代理
func (t *testerImpl) TestProxies(ctx context.Context, request *TestRequest) error {
	if request == nil {
		return fmt.Errorf("请求参数不能为空")
	}

	// 如果已有任务在运行，返回错误
	if t.taskManager.IsRunning(task.TaskTypeSpeedTest) {
		log.Infoln("已有其他任务正在运行")
		return fmt.Errorf("已有其他任务正在运行")
	}

	// 根据请求参数获取需要测试的代理
	var proxies []*model.Proxy
	var err error

	if len(request.ProxyIDs) > 0 {
		// 如果指定了代理ID列表，则测试这些代理
		proxies = make([]*model.Proxy, 0, len(request.ProxyIDs))
		for _, id := range request.ProxyIDs {
			proxy, err := t.proxyRepo.FindByID(uint(id))
			if err != nil {
				log.Warnln("获取代理失败[ID:%d]: %v", id, err)
				continue
			}
			if proxy != nil {
				proxies = append(proxies, proxy)
			}
		}
	} else if request.Filters != nil {
		// 如果指定了筛选条件，则根据条件筛选代理
		filter := &repository.ProxyFilter{
			Status: request.Filters.Status,
			Types:  request.Filters.Types,
		}
		proxies, err = t.proxyRepo.FindByFilter(filter)
		if err != nil {
			return fmt.Errorf("筛选代理失败: %w", err)
		}
	} else {
		// 如果既没有指定ID也没有指定筛选条件，则获取所有代理
		proxies, err = t.proxyRepo.FindAll()
		if err != nil {
			return fmt.Errorf("获取所有代理失败: %w", err)
		}
	}

	// 如果没有代理需要测试，直接返回
	if len(proxies) == 0 {
		log.Infoln("没有找到需要测试的代理")
		return nil
	}

	// 开始任务
	taskType := task.TaskTypeSpeedTest
	ctx, started := t.taskManager.StartTask(ctx, taskType, len(proxies))
	if !started {
		return fmt.Errorf("启动任务失败")
	}

	// 设置并发数
	concurrent := request.Concurrent
	if concurrent <= 0 {
		concurrent = 5 // 默认并发数
	}

	// 异步执行测试
	go t.runTests(ctx, taskType, proxies, concurrent)

	return nil
}

// runTests 多线程执行测试
func (t *testerImpl) runTests(ctx context.Context, taskType task.TaskType, proxies []*model.Proxy, concurrent int) {
	var finishMessage string
	defer func() {
		if r := recover(); r != nil {
			finishMessage = fmt.Sprintf("测试代理任务发生panic: %v", r)
			log.Errorln(finishMessage)
		}
		t.taskManager.FinishTask(taskType, finishMessage)
		log.Infoln("测试任务执行完毕")
	}()

	// 使用限制并发的context
	eg, ctx := errgroup.WithContext(ctx)
	eg.SetLimit(concurrent)
	var completedCount int32

	for _, proxy := range proxies {
		if ctx.Err() != nil {
			log.Infoln("测试任务已被取消，停止处理剩余代理")
			break
		}

		p := proxy // 创建局部变量避免闭包问题
		eg.Go(func() error {
			defer func() {
				completed := atomic.AddInt32(&completedCount, 1)
				t.taskManager.UpdateProgress(taskType, int(completed), "")
			}()

			if ctx.Err() != nil {
				return ctx.Err()
			}

			t.testProxyAndUpdateDB(p)
			return nil
		})
	}

	// 等待所有任务完成或被取消
	waitCh := make(chan struct{})
	go func() {
		_ = eg.Wait()
		close(waitCh)
	}()

	// 处理完成或取消情况
	select {
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.Canceled) {
			finishMessage = "任务被取消"
		} else {
			finishMessage = "任务超时或其他原因终止"
		}
		log.Infoln("任务已被取消，等待正在进行的测试完成")

		select {
		case <-waitCh:
			log.Infoln("所有测试已停止")
		case <-time.After(20 * time.Second):
			log.Warnln("等待测试完成超时，强制结束任务")
			finishMessage = "等待测试完成超时，强制结束任务"
		}
	case <-waitCh:
		log.Infoln("所有测试已完成")
	}
}

// testProxyAndUpdateDB 测试单个代理并更新数据库
func (t *testerImpl) testProxyAndUpdateDB(p *model.Proxy) {
	defer func() {
		if r := recover(); r != nil {
			log.Errorln("测试代理过程中发生panic[代理ID:%d]: %v", p.ID, r)
			p.Status = model.ProxyStatusUnknowError

			// 使用安全的数据库操作函数
			if err := SafeDBOperation(func() error {
				return t.proxyRepo.UpdateSpeedTestInfo(p)
			}); err != nil {
				log.Errorln("更新代理状态失败[代理ID:%d]: %v", p.ID, err)
			}
		}
	}()

	// 测试代理
	testTime := time.Now()
	result, err := t.TestProxy(p)
	if err != nil {
		log.Errorln("测试代理失败[代理ID:%d]: %v", p.ID, err)
		p.Status = model.ProxyStatusFailed

		// 使用安全的数据库操作函数
		if err := SafeDBOperation(func() error {
			return t.proxyRepo.UpdateSpeedTestInfo(p)
		}); err != nil {
			log.Errorln("更新代理状态失败[代理ID:%d]: %v", p.ID, err)
		}
		return
	}

	log.Infoln("测试代理[代理ID:%d 名称:%v]结果: Ping=%dms, 下载=%v, 上传=%v",
		p.ID, p.Name, result.Ping, formatSpeed(result.DownloadSpeed), formatSpeed(result.UploadSpeed))

	// 更新代理状态
	p.Ping = result.Ping
	p.DownloadSpeed = result.DownloadSpeed
	p.UploadSpeed = result.UploadSpeed
	p.Status = model.ProxyStatusOK
	p.LatestTestTime = &testTime

	if result.DownloadSpeed == 0 {
		log.Debugln("测试代理[代理ID:%d 名称:%v]无速度", p.ID, p.Name)
		p.Status = model.ProxyStatusFailed
	}

	// 使用安全的数据库操作函数进行批量操作
	if err := SafeDBOperation(func() error {
		// 保存测速历史记录
		speedTestHistory := &model.SpeedTestHistory{
			ProxyID:       p.ID,
			Ping:          result.Ping,
			DownloadSpeed: result.DownloadSpeed,
			UploadSpeed:   result.UploadSpeed,
			TestTime:      testTime,
			CreatedAt:     time.Now(),
		}
		if err := t.speedTestHistoryRepo.Create(speedTestHistory); err != nil {
			log.Errorln("保存测速历史记录失败: %v", err)
			return err
		}

		// 保存代理状态
		return t.proxyRepo.UpdateSpeedTestInfo(p)
	}); err != nil {
		log.Errorln("更新代理数据失败[代理ID:%d]: %v", p.ID, err)
	}
}

// formatSpeed 格式化速度
func formatSpeed(bytesPerSecond int) string {
	units := []string{"B/s", "KB/s", "MB/s", "GB/s", "TB/s"}
	unit := 0
	speed := float64(bytesPerSecond)
	for speed >= 1024 && unit < len(units)-1 {
		speed /= 1024
		unit++
	}
	return fmt.Sprintf("%.2f%s", speed, units[unit])
}
