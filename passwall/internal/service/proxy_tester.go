package service

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"passwall/config"
	"passwall/internal/adapter/parser"
	"passwall/internal/adapter/speedtester"
	"passwall/internal/model"
	"passwall/internal/repository"
	"passwall/internal/util"

	"github.com/metacubex/mihomo/log"
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
	taskManager          TaskManager
}

// NewProxyTester 创建代理测试服务
func NewProxyTester(
	proxyRepo repository.ProxyRepository,
	subscriptionRepo repository.SubscriptionRepository,
	speedTestHistoryRepo repository.SpeedTestHistoryRepository,
	speedTesterFactory speedtester.SpeedTesterFactory,
	parserFactory parser.ParserFactory,
	taskManager TaskManager,
) ProxyTester {
	return &proxyTesterImpl{
		proxyRepo:            proxyRepo,
		subscriptionRepo:     subscriptionRepo,
		speedTestHistoryRepo: speedTestHistoryRepo,
		speedTesterFactory:   speedTesterFactory,
		parserFactory:        parserFactory,
		taskManager:          taskManager,
	}
}

// TestProxies 测试代理
func (s *proxyTesterImpl) TestProxies(request *TestProxyRequest) error {
	if request == nil {
		return errors.New("request cannot be nil")
	}

	// 如果已有任务在运行，返回错误
	if s.taskManager.IsAnyTaskRunning() {
		return errors.New("another task is already running")
	}

	// 加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		return errors.New("load config failed: " + err.Error())
	}

	// 重新加载订阅配置
	if request.ReloadSubscribeConfig {

		// 加载配置
		cfg, err := config.LoadConfig()
		if err != nil {
			log.Errorln("加载配置失败: %v", err)
			return err
		}

		// 获取所有订阅
		subscriptions, err := s.subscriptionRepo.FindAll()
		if err != nil {
			log.Errorln("获取订阅列表失败: %v", err)
			return err
		}

		if len(subscriptions) == 0 {
			log.Infoln("没有找到订阅配置")
			return nil
		}

		// 过滤掉URL为空的订阅（文件上传的不需要重新获取）
		var validSubscriptions []*model.Subscription
		for _, sub := range subscriptions {
			// 只处理URL不为空的订阅（文件上传的订阅URL通常为MD5值，长度为20）
			if strings.Contains(sub.URL, "://") || strings.HasPrefix(sub.URL, "http") {
				validSubscriptions = append(validSubscriptions, sub)
			}
		}

		if len(validSubscriptions) == 0 {
			log.Infoln("没有找到有效的URL订阅配置")
			return nil
		}

		// 更新任务总数
		taskType := TaskTypeReloadSubs
		s.taskManager.StartTask(taskType, len(validSubscriptions))

		// 处理每个订阅
		var lastError error
		for i, subscription := range validSubscriptions {
			log.Infoln("正在重新加载订阅 [%d/%d]: %s", i+1, len(validSubscriptions), subscription.URL)

			// 设置下载选项，包括代理
			downloadOptions := &util.DownloadOptions{
				Timeout:     util.DefaultDownloadOptions.Timeout,
				MaxFileSize: util.DefaultDownloadOptions.MaxFileSize,
			}

			// 如果配置了代理并启用，则使用配置的代理
			if cfg != nil && cfg.Proxy.Enabled && cfg.Proxy.URL != "" {
				downloadOptions.ProxyURL = cfg.Proxy.URL
				log.Infoln("使用代理下载: %s", cfg.Proxy.URL)
			}

			var content []byte

			// 下载订阅内容
			content, err = util.DownloadFromURL(subscription.URL, downloadOptions)
			if err != nil {
				log.Infoln("下载订阅内容失败: %v", err)
				subscription.Status = model.SubscriptionStatusExpired
				s.subscriptionRepo.Update(subscription)
				lastError = err
				continue
			}

			// 解析订阅内容
			subParser, err := s.parserFactory.GetParser(string(subscription.Type))
			if err != nil {
				log.Infoln("获取解析器失败: %v", err)
				subscription.Status = model.SubscriptionStatusInvalid
				s.subscriptionRepo.Update(subscription)
				lastError = err
				continue
			}

			newProxies, err := subParser.Parse(content)
			if err != nil {
				log.Infoln("解析订阅内容失败: %v", err)
				subscription.Status = model.SubscriptionStatusInvalid
				s.subscriptionRepo.Update(subscription)
				lastError = err
				continue
			}

			if len(newProxies) == 0 {
				log.Infoln("未从订阅中解析出任何代理")
				subscription.Status = model.SubscriptionStatusInvalid
				s.subscriptionRepo.Update(subscription)
				lastError = errors.New("未从订阅中解析出任何代理")
				continue
			}

			// 保存解析的代理
			oldProxies, err := s.proxyRepo.FindAll(map[string]interface{}{
				"subscription_id": subscription.ID,
			})
			if err != nil {
				log.Errorln("查找旧代理失败: %v", err)
			}

			// 将旧代理转换为映射以便快速查找
			oldProxyMap := make(map[string]*model.Proxy)
			if len(oldProxies) > 0 {
				for _, oldProxy := range oldProxies {
					// 使用代理的唯一标识(如域名+端口)作为键
					key := oldProxy.Domain + ":" + strconv.Itoa(oldProxy.Port)
					oldProxyMap[key] = oldProxy
				}
			}

			// 记录已处理的代理ID，用于后续处理未匹配的旧代理
			processedIDs := make(map[uint]bool)

			// 处理新的代理列表
			for _, newProxy := range newProxies {
				newProxy.SubscriptionID = &subscription.ID

				// 生成唯一标识
				key := newProxy.Domain + ":" + strconv.Itoa(newProxy.Port)

				// 检查是否存在匹配的旧代理，旧代理的状态不更新
				if oldProxy, exists := oldProxyMap[key]; exists {
					// 更新现有代理
					oldProxy.Name = newProxy.Name
					oldProxy.Type = newProxy.Type
					oldProxy.Config = newProxy.Config

					if err := s.proxyRepo.Update(oldProxy); err != nil {
						log.Errorln("更新代理失败: %v", err)
						continue
					}

					// 标记为已处理
					processedIDs[oldProxy.ID] = true
				} else {
					// 添加新代理
					newProxy.Status = model.ProxyStatusPending // 设置为待处理状态，等待后续测试
					if err := s.proxyRepo.Create(newProxy); err != nil {
						log.Errorln("保存代理失败: %v", err)
						continue
					}
				}
			}

			// 不处理未匹配的旧代理（可选：删除或标记为过期）

			// 更新订阅状态
			subscription.Status = model.SubscriptionStatusOK
			subscription.Content = string(content)
			if err := s.subscriptionRepo.Update(subscription); err != nil {
				log.Errorln("更新订阅状态失败: %v", err)
				lastError = err
			}

			// 更新任务进度
			s.taskManager.UpdateTaskProgress(taskType, i+1, "")
		}

		// 任务完成
		if lastError != nil {
			s.taskManager.FinishTask(taskType, lastError.Error())
		} else {
			s.taskManager.FinishTask(taskType, "")
		}

		log.Infoln("重新加载订阅完成")
	}

	// 测试代理
	var proxies []*model.Proxy
	var taskType TaskType

	// 设置并发数
	concurrent := request.Concurrent
	if concurrent <= 0 {
		concurrent = cfg.Concurrent
	}

	if request.TestAll {
		taskType = TaskTypeSpeedTest
		proxies, err = s.proxyRepo.FindAll(nil)
		if err != nil {
			return errors.New("failed to find all proxies: " + err.Error())
		}
	} else if request.TestNew {
		taskType = TaskTypeSpeedTest
		proxies, err = s.proxyRepo.FindByStatus(model.ProxyStatusPending)
		if err != nil {
			return errors.New("failed to find pending proxies: " + err.Error())
		}
	} else if request.TestFailed {
		taskType = TaskTypeSpeedTest
		proxies, err = s.proxyRepo.FindByStatus(model.ProxyStatusFailed)
		if err != nil {
			return errors.New("failed to find failed proxies: " + err.Error())
		}
	} else if request.TestSpeed {
		concurrent = 1
		taskType = TaskTypeSpeedTest
		proxies, err = s.proxyRepo.FindByStatus(model.ProxyStatusOK)
		if err != nil {
			return errors.New("failed to find working proxies: " + err.Error())
		}
	} else {
		return errors.New("invalid request: no test type specified")
	}

	// 如果没有代理需要测试，直接返回
	if len(proxies) == 0 {
		return errors.New("no proxies to test")
	}

	// 开始任务
	if !s.taskManager.StartTask(taskType, len(proxies)) {
		return errors.New("failed to start task")
	}

	// 异步执行测试
	go func() {
		s.testProxiesAsync(proxies, concurrent, taskType)
	}()

	return nil
}

// testProxiesAsync 异步测试代理
func (s *proxyTesterImpl) testProxiesAsync(proxies []*model.Proxy, concurrent int, taskType TaskType) {
	// 创建工作池
	wg := &sync.WaitGroup{}
	semaphore := make(chan struct{}, concurrent)
	completed := 0
	var mu sync.Mutex

	for _, proxy := range proxies {
		wg.Add(1)
		semaphore <- struct{}{}

		go func(p *model.Proxy) {
			// 添加panic恢复
			defer func() {
				if r := recover(); r != nil {
					log.Errorln("测试代理时发生panic[代理ID:%d]: %v", p.ID, r)
					// 更新代理状态为失败
					p.Status = model.ProxyStatusUnknowError
					if err := s.proxyRepo.Update(p); err != nil {
						log.Errorln("更新代理状态失败[代理ID:%d]: %v", p.ID, err)
					}
				}

				<-semaphore
				wg.Done()

				mu.Lock()
				completed++
				s.taskManager.UpdateTaskProgress(taskType, completed, "")
				mu.Unlock()
			}()

			var result *model.SpeedTestResult
			var err error

			// 根据代理类型获取测速器
			tester, err := s.speedTesterFactory.GetSpeedTester(p.Type)
			if err != nil {
				log.Errorln("获取测速器失败[代理ID:%d]: %v", p.ID, err)
				return
			}

			// 测试代理
			result, err = tester.Test(p)
			if err != nil {
				log.Errorln("测试代理失败[代理ID:%d]: %v", p.ID, err)
				p.Status = model.ProxyStatusUnknowError
				if err := s.proxyRepo.Update(p); err != nil {
					log.Errorln("更新代理状态失败[代理ID:%d]: %v", p.ID, err)
				}
				return
			}

			// 检查结果是否为nil
			if result == nil {
				log.Errorln("测试代理返回空结果[代理ID:%d]", p.ID)
				p.Status = model.ProxyStatusFailed
				if err := s.proxyRepo.Update(p); err != nil {
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
				log.Warnln("测试代理[代理ID:%d 名称:%v]无速度", p.ID, p.Name)
				p.Status = model.ProxyStatusFailed
			}

			// 保存测速历史记录
			speedTestHistory := &model.SpeedTestHistory{
				ProxyID:       p.ID,
				Ping:          result.Ping,
				DownloadSpeed: result.DownloadSpeed,
				UploadSpeed:   result.UploadSpeed,
				TestTime:      now,
				CreatedAt:     now,
			}
			if err := s.speedTestHistoryRepo.Create(speedTestHistory); err != nil {
				log.Errorln("保存测速历史记录失败: %v", err)
			}

			// 保存代理
			if err := s.proxyRepo.Update(p); err != nil {
				log.Errorln("更新代理失败[代理ID:%d]: %v", p.ID, err)
			}
		}(proxy)
	}

	// 等待所有测试完成
	wg.Wait()
	close(semaphore)

	// 完成任务
	s.taskManager.FinishTask(taskType, "")
	log.Infoln("本次测试完成")
}

func formatSpeed(bytesPerSecond int64) string {
	units := []string{"B/s", "KB/s", "MB/s", "GB/s", "TB/s"}
	unit := 0
	speed := bytesPerSecond
	for speed >= 1024 && unit < len(units)-1 {
		speed /= 1024
		unit++
	}
	return fmt.Sprintf("%.2f%s", speed, units[unit])
}
