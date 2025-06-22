package proxy

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"passwall/config"
	"passwall/internal/adapter/parser"
	"passwall/internal/model"
	"passwall/internal/repository"
	"passwall/internal/service/task"
	"passwall/internal/util"

	"github.com/metacubex/mihomo/log"
)

type SubsPage struct {
	Page     int
	PageSize int
}

// SubscriptionManager 订阅管理服务
type SubscriptionManager interface {
	// 基本CRUD操作
	GetSubscriptionByID(id uint) (*model.Subscription, error)
	GetSubscriptionsPage(page SubsPage) ([]*model.Subscription, int64, error)
	GetSubscriptionByURL(url string) (*model.Subscription, error)
	CreateSubscription(subscription *model.Subscription) error
	UpdateSubscriptionStatus(subscription *model.Subscription) error
	DeleteSubscription(id uint) error

	// 刷新操作
	RefreshSubscriptionAsync(ctx context.Context, subID uint) error
	RefreshAllSubscriptions(ctx context.Context, async bool) error

	// 解析和保存
	ParseAndSaveProxies(ctx context.Context, subscription *model.Subscription, content []byte) error
}

// subscriptionManagerImpl 订阅管理服务实现
type subscriptionManagerImpl struct {
	subscriptionRepo repository.SubscriptionRepository
	proxyRepo        repository.ProxyRepository
	parserFactory    parser.ParserFactory
	taskManager      task.TaskManager
}

// NewSubscriptionManager 创建订阅管理服务
func NewSubscriptionManager(
	subscriptionRepo repository.SubscriptionRepository,
	proxyRepo repository.ProxyRepository,
	parserFactory parser.ParserFactory,
	taskManager task.TaskManager,
) SubscriptionManager {
	return &subscriptionManagerImpl{
		subscriptionRepo: subscriptionRepo,
		proxyRepo:        proxyRepo,
		parserFactory:    parserFactory,
		taskManager:      taskManager,
	}
}

// GetSubscriptionByID 根据ID获取订阅
func (s *subscriptionManagerImpl) GetSubscriptionByID(id uint) (*model.Subscription, error) {
	return s.subscriptionRepo.FindByID(id)
}

// GetAllSubscriptions 获取所有订阅
func (s *subscriptionManagerImpl) GetSubscriptionsPage(page SubsPage) ([]*model.Subscription, int64, error) {
	req := repository.SubsPage{
		Page:     page.Page,
		PageSize: page.PageSize,
	}
	return s.subscriptionRepo.FindPage(req)
}

// GetSubscriptionByURL 根据URL获取订阅
func (s *subscriptionManagerImpl) GetSubscriptionByURL(url string) (*model.Subscription, error) {
	return s.subscriptionRepo.FindByURL(url)
}

// CreateSubscription 创建订阅
func (s *subscriptionManagerImpl) CreateSubscription(subscription *model.Subscription) error {
	return SafeDBOperation(func() error {
		return s.subscriptionRepo.Create(subscription)
	})
}

// UpdateSubscriptionStatus 更新订阅
func (s *subscriptionManagerImpl) UpdateSubscriptionStatus(subscription *model.Subscription) error {
	return SafeDBOperation(func() error {
		return s.subscriptionRepo.UpdateStatus(subscription)
	})
}

// DeleteSubscription 删除订阅
func (s *subscriptionManagerImpl) DeleteSubscription(id uint) error {
	// TODO: 考虑是否需要删除关联的代理服务器
	return SafeDBOperation(func() error {
		return s.subscriptionRepo.Delete(id)
	})
}

// RefreshSubscriptionAsync 刷新单个订阅
func (s *subscriptionManagerImpl) RefreshSubscriptionAsync(ctx context.Context, subID uint) error {
	// 获取订阅信息
	subscription, err := s.subscriptionRepo.FindByID(subID)
	if err != nil {
		return fmt.Errorf("获取订阅信息失败: %w", err)
	}

	if subscription == nil {
		return fmt.Errorf("订阅不存在")
	}

	// 启动任务
	taskType := task.TaskTypeReloadSubs
	ctx, started := s.taskManager.StartTask(ctx, taskType, 1)
	if !started {
		return fmt.Errorf("已有任务正在运行")
	}

	// 异步刷新订阅
	go func() {
		defer s.taskManager.FinishTask(taskType, "")

		err := s.refreshSubscription(ctx, subscription)
		if err != nil {
			log.Errorln("刷新订阅失败: %v", err)
			s.taskManager.UpdateProgress(taskType, 1, err.Error())
		} else {
			s.taskManager.UpdateProgress(taskType, 1, "")
		}
	}()

	return nil
}

// RefreshAllSubscriptions 异步刷新所有订阅
func (s *subscriptionManagerImpl) RefreshAllSubscriptions(ctx context.Context, async bool) error {
	// 如果已有任务在运行，返回错误
	if s.taskManager.IsRunning(task.TaskTypeReloadSubs) {
		return fmt.Errorf("已有其他任务正在运行")
	}

	// 获取所有订阅
	subscriptions, err := s.subscriptionRepo.FindAll()
	if err != nil {
		return fmt.Errorf("获取订阅列表失败: %w", err)
	}

	if len(subscriptions) == 0 {
		log.Infoln("没有找到订阅配置")
		return nil
	}

	// 启动任务
	taskType := task.TaskTypeReloadSubs
	ctx, started := s.taskManager.StartTask(ctx, taskType, len(subscriptions))
	if !started {
		return fmt.Errorf("启动任务失败")
	}

	if async {
		go s.refreshAllSubscriptions(ctx, taskType, subscriptions)
	} else {
		s.refreshAllSubscriptions(ctx, taskType, subscriptions)
	}

	return nil
}

// refreshAllSubscriptions 刷新所有订阅
func (s *subscriptionManagerImpl) refreshAllSubscriptions(ctx context.Context, taskType task.TaskType, subscriptions []*model.Subscription) {
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
			errMsg := fmt.Sprintf("刷新订阅任务发生panic: %v", r)
			log.Errorln(errMsg)
			finishMessage = errMsg
		}

		// 如果任务尚未标记为完成，则标记它
		if !finished {
			s.taskManager.FinishTask(taskType, finishMessage)
			log.Infoln("订阅刷新任务执行完毕（通过defer）")
		}
	}()

	var lastError error
	completed := 0
	cancelled := false

	// 创建一个通道，用于跟踪所有订阅处理的完成
	jobsTotal := len(subscriptions)
	jobsDone := 0
	var doneMutex sync.Mutex

	for _, subscription := range subscriptions {
		// 检查任务是否已被取消
		select {
		case <-ctx.Done():
			if !cancelled {
				cancelled = true
				log.Infoln("任务已被取消，停止处理剩余订阅")
			}
			break
		default:
			// 继续执行
		}

		err := s.refreshSubscription(ctx, subscription)
		if err != nil {
			log.Errorln("刷新订阅[%s]失败: %v", subscription.URL, err)
			lastError = err
		}

		doneMutex.Lock()
		jobsDone++
		completed++
		doneMutex.Unlock()

		s.taskManager.UpdateProgress(taskType, completed, "")
	}

	// 设置完成消息
	if cancelled {
		if errors.Is(ctx.Err(), context.Canceled) {
			finishMessage = "任务被取消"
		} else {
			finishMessage = "任务超时或其他原因终止"
		}
	} else if lastError != nil {
		finishMessage = lastError.Error()
	}

	// 标记任务完成（正常流程）
	finishMutex.Lock()
	finished = true
	s.taskManager.FinishTask(taskType, finishMessage)
	finishMutex.Unlock()

	log.Infoln("所有订阅刷新完成, 共处理 %d 个订阅, 完成 %d 个, 错误: %v", jobsTotal, jobsDone, lastError)
}

// refreshSubscription 刷新单个订阅
func (s *subscriptionManagerImpl) refreshSubscription(ctx context.Context, subscription *model.Subscription) error {
	log.Infoln("开始刷新订阅: %s", subscription.URL)
	if subscription.URL == "" {
		return fmt.Errorf("订阅为空")
	}
	if !strings.HasPrefix(subscription.URL, "http") {
		log.Infoln("非下载链接，无需刷新")
		return nil
	}

	// 设置下载选项，包括代理
	downloadOptions := &util.DownloadOptions{
		Timeout:     util.DefaultDownloadOptions.Timeout,
		MaxFileSize: util.DefaultDownloadOptions.MaxFileSize,
	}

	// 加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Errorln("加载配置失败: %v", err)
	} else if cfg != nil && cfg.Proxy.Enabled && cfg.Proxy.URL != "" {
		downloadOptions.ProxyURL = cfg.Proxy.URL
		log.Infoln("使用代理下载: %s", cfg.Proxy.URL)
	}

	// 下载订阅内容
	content, err := util.DownloadFromURL(subscription.URL, downloadOptions)
	if err != nil {
		log.Errorln("下载订阅内容失败: %v", err)

		// 使用安全的数据库操作函数
		SafeDBOperation(func() error {
			subscription.Status = model.SubscriptionStatusExpired
			return s.subscriptionRepo.UpdateStatus(subscription)
		})

		return fmt.Errorf("下载订阅内容失败: %w", err)
	}

	// 解析并保存代理
	if err := s.ParseAndSaveProxies(ctx, subscription, content); err != nil {
		log.Errorln("解析订阅内容失败: %v", err)

		// 使用安全的数据库操作函数
		SafeDBOperation(func() error {
			subscription.Status = model.SubscriptionStatusInvalid
			return s.subscriptionRepo.UpdateStatus(subscription)
		})

		return err
	}

	return nil
}

// ParseAndSaveProxies 解析并保存代理
func (s *subscriptionManagerImpl) ParseAndSaveProxies(ctx context.Context, subscription *model.Subscription, content []byte) error {
	// 解析订阅内容
	subParser, err := s.parserFactory.GetParser(string(subscription.Type))
	if err != nil {
		log.Errorln("获取解析器失败: %v", err)

		// 使用安全的数据库操作函数
		SafeDBOperation(func() error {
			subscription.Status = model.SubscriptionStatusInvalid
			return s.subscriptionRepo.UpdateStatus(subscription)
		})

		return fmt.Errorf("获取解析器失败: %w", err)
	}

	newProxies, err := subParser.Parse(content)
	if err != nil {
		log.Errorln("解析订阅内容失败: %v", err)

		// 使用安全的数据库操作函数
		SafeDBOperation(func() error {
			subscription.Status = model.SubscriptionStatusInvalid
			return s.subscriptionRepo.UpdateStatus(subscription)
		})

		return fmt.Errorf("解析订阅内容失败: %w", err)
	}

	if len(newProxies) == 0 {
		log.Errorln("未从订阅中解析出任何代理")

		// 使用安全的数据库操作函数
		SafeDBOperation(func() error {
			subscription.Status = model.SubscriptionStatusInvalid
			return s.subscriptionRepo.UpdateStatus(subscription)
		})

		return fmt.Errorf("未从订阅中解析出任何代理")
	}

	// 保存代理
	for _, newProxy := range newProxies {
		// 检查上下文是否已取消
		select {
		case <-ctx.Done():
			return fmt.Errorf("上下文已取消")
		default:
			// 继续执行
		}

		// 检查是否已存在相同的代理
		oldProxy, err := s.proxyRepo.FindByDomainAndPort(newProxy.Domain, newProxy.Port)
		if err != nil {
			log.Errorln("查找旧代理失败: %v", err)
			continue
		}

		// 如果旧代理存在，则更新旧代理
		if oldProxy != nil {
			// 判断是否一致，不一致则更新
			if oldProxy.Type == newProxy.Type && oldProxy.Config == newProxy.Config {
				continue
			}
			SafeDBOperation(func() error {
				if err := s.proxyRepo.UpdateProxyConfig(oldProxy); err != nil {
					log.Errorln("更新代理配置失败[%s]: %v", oldProxy.Name, err)
					return err
				}
				return nil
			})
		} else {
			// 如果旧代理不存在，则创建新代理
			newProxy.SubscriptionID = &subscription.ID
			newProxy.Status = model.ProxyStatusPending // 设置为待处理状态，等待后续测试
			SafeDBOperation(func() error {
				if err := s.proxyRepo.Create(newProxy); err != nil {
					log.Errorln("保存代理失败[%s]: %v", newProxy.Name, err)
					return err
				}
				return nil
			})
		}
	}

	// 更新订阅状态
	subscription.Status = model.SubscriptionStatusOK
	subscription.Content = string(content)
	SafeDBOperation(func() error {
		return s.subscriptionRepo.UpdateStatusAndContent(subscription)
	})

	log.Infoln("订阅[%s]刷新成功，解析出%d个代理", subscription.URL, len(newProxies))
	return nil
}
