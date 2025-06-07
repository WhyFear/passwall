package proxy

import (
	"context"
	"fmt"
	"sync"
	"time"

	"passwall/internal/adapter/speedtester"
	"passwall/internal/model"
	"passwall/internal/repository"
	"passwall/internal/service/task"

	"github.com/metacubex/mihomo/log"
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
	TestProxy(ctx context.Context, proxy *model.Proxy) (*model.SpeedTestResult, error)

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
func (t *testerImpl) TestProxy(ctx context.Context, proxy *model.Proxy) (*model.SpeedTestResult, error) {
	if proxy == nil {
		return nil, fmt.Errorf("代理对象不能为空")
	}

	// 获取速度测试器
	tester, err := t.speedTesterFactory.GetSpeedTester(proxy.Type)
	if err != nil {
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
		proxies, err = t.proxyRepo.FindAll(nil)
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

// runTests 异步执行测试
func (t *testerImpl) runTests(ctx context.Context, taskType task.TaskType, proxies []*model.Proxy, concurrent int) {
	// 用于跟踪任务是否已经完成的标志
	var finished bool
	var finishMessage string
	var finishMutex sync.Mutex

	// 确保任务最终会被标记为完成，并处理可能的panic
	defer func() {
		finishMutex.Lock()
		defer finishMutex.Unlock()

		// 检查是否有panic发生
		if r := recover(); r != nil {
			errMsg := fmt.Sprintf("测试代理任务发生panic: %v", r)
			log.Errorln(errMsg)
			finishMessage = errMsg
		}

		// 如果任务尚未标记为完成，则标记它
		if !finished {
			t.taskManager.FinishTask(taskType, finishMessage)
			log.Infoln("测试任务执行完毕（通过defer）")
		}
	}()

	// 创建工作池
	wg := &sync.WaitGroup{}
	semaphore := make(chan struct{}, concurrent)
	completed := 0
	var mu sync.Mutex

	// 创建一个标志，用于标记是否已经处理了取消情况
	cancelled := false
	var cancelledMu sync.Mutex

	for _, proxy := range proxies {
		// 检查上下文是否已取消
		select {
		case <-ctx.Done():
			cancelledMu.Lock()
			if !cancelled {
				cancelled = true
				log.Infoln("测试任务已被取消，停止处理剩余代理")
			}
			cancelledMu.Unlock()

			// 不再添加新的goroutine
			continue
		default:
			// 继续执行
		}

		wg.Add(1)

		// 在添加到信号量通道前检查一次上下文是否被取消
		select {
		case <-ctx.Done():
			wg.Done() // 立即减少计数，避免等待
			continue
		case semaphore <- struct{}{}:
			// 成功获取信号量槽位，继续执行
		}

		go func(p *model.Proxy) {
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

				<-semaphore
				wg.Done()

				mu.Lock()
				completed++
				t.taskManager.UpdateProgress(taskType, completed, "")
				mu.Unlock()
			}()

			// 测试代理
			result, err := t.TestProxy(ctx, p)
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
			now := time.Now()
			p.LatestTestTime = &now

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
					TestTime:      now,
					CreatedAt:     now,
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
		}(proxy)
	}

	// 等待所有测试完成或任务被取消
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		// 任务被取消
		if ctx.Err() == context.Canceled {
			finishMessage = "任务被取消"
		} else {
			finishMessage = "任务超时或其他原因终止"
		}
		log.Infoln("任务已被取消，等待正在进行的测试完成")

		// 设置超时，防止等待时间过长
		select {
		case <-done: // 等待所有goroutine结束
			log.Infoln("所有测试已停止")
		case <-time.After(20 * time.Second):
			log.Warnln("等待测试完成超时，强制结束任务")
			finishMessage = "等待测试完成超时，强制结束任务"
		}
	case <-done:
		// 所有测试正常完成
		log.Infoln("所有测试已完成")
	}

	// 标记任务完成（正常流程）
	finishMutex.Lock()
	finished = true
	t.taskManager.FinishTask(taskType, finishMessage)
	finishMutex.Unlock()

	log.Infoln("测试任务执行完毕")
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
