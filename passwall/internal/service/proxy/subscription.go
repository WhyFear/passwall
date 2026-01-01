package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"passwall/internal/adapter/parser"
	"passwall/internal/model"
	"passwall/internal/repository"
	"passwall/internal/service/task"
	"passwall/internal/util"

	"github.com/google/go-cmp/cmp"
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
	RefreshSubscriptionAsync(ctx context.Context, subID uint, options *util.DownloadOptions) error
	RefreshAllSubscriptions(ctx context.Context, async bool, options *util.DownloadOptions) error

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
	return s.subscriptionRepo.Create(subscription)
}

// UpdateSubscriptionStatus 更新订阅
func (s *subscriptionManagerImpl) UpdateSubscriptionStatus(subscription *model.Subscription) error {
	return s.subscriptionRepo.UpdateStatus(subscription)
}

// DeleteSubscription 删除订阅
func (s *subscriptionManagerImpl) DeleteSubscription(id uint) error {
	return s.subscriptionRepo.Delete(id)
}

// RefreshSubscriptionAsync 刷新单个订阅
func (s *subscriptionManagerImpl) RefreshSubscriptionAsync(ctx context.Context, subID uint, options *util.DownloadOptions) error {
	// 获取订阅信息
	subscription, err := s.subscriptionRepo.FindByID(subID)
	if err != nil {
		return fmt.Errorf("获取订阅信息失败: %w", err)
	}

	if subscription == nil {
		return fmt.Errorf("订阅不存在")
	}

	// 尝试启动任务（仅用于 UI 进度展示，失败不影响后台刷新）
	taskType := task.TaskTypeReloadSubs
	taskCtx, started := s.taskManager.StartTask(ctx, taskType, 1)
	if !started {
		taskCtx = ctx
	}

	// 异步刷新订阅
	go func() {
		if started {
			defer s.taskManager.FinishTask(taskType, "")
		}

		err := s.refreshSubscription(taskCtx, subscription, options)
		if started {
			if err != nil {
				log.Errorln("刷新订阅失败: %v", err)
				s.taskManager.UpdateProgress(taskType, 1, err.Error())
			} else {
				s.taskManager.UpdateProgress(taskType, 1, "")
			}
		}
	}()

	return nil
}

// RefreshAllSubscriptions 异步刷新所有订阅
func (s *subscriptionManagerImpl) RefreshAllSubscriptions(ctx context.Context, async bool, options *util.DownloadOptions) error {
	// 获取所有订阅
	subscriptions, err := s.subscriptionRepo.FindAll()
	if err != nil {
		return fmt.Errorf("获取订阅列表失败: %w", err)
	}

	if len(subscriptions) == 0 {
		log.Infoln("没有找到订阅配置")
		return nil
	}

	// 尝试启动任务（仅用于 UI 进度展示，失败不影响后台刷新）
	taskType := task.TaskTypeReloadSubs
	taskCtx, started := s.taskManager.StartTask(ctx, taskType, len(subscriptions))
	if !started {
		taskCtx = ctx
	}

	if async {
		go s.refreshAllSubscriptions(taskCtx, taskType, subscriptions, options, started)
	} else {
		s.refreshAllSubscriptions(taskCtx, taskType, subscriptions, options, started)
	}

	return nil
}

// refreshAllSubscriptions 刷新所有订阅
func (s *subscriptionManagerImpl) refreshAllSubscriptions(ctx context.Context, taskType task.TaskType, subscriptions []*model.Subscription, options *util.DownloadOptions, updateTask bool) {
	// 用于跟踪任务是否已经完成的标志
	var finished bool
	var finishMessage string
	var finishMutex sync.Mutex

	// 确保任务最终会被标记为完成，并处理可能的panic
	defer func() {
		if !updateTask {
			return
		}
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
			return // 退出循环
		default:
			// 继续执行
		}

		err := s.refreshSubscription(ctx, subscription, options)
		if err != nil {
			log.Errorln("刷新订阅[%s]失败: %v", subscription.URL, err)
			lastError = err
		}

		doneMutex.Lock()
		jobsDone++
		completed++
		doneMutex.Unlock()

		if updateTask {
			s.taskManager.UpdateProgress(taskType, completed, "")
		}
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
	if updateTask {
		finishMutex.Lock()
		finished = true
		s.taskManager.FinishTask(taskType, finishMessage)
		finishMutex.Unlock()
	}

	log.Infoln("所有订阅刷新完成, 共处理 %d 个订阅, 完成 %d 个, 错误: %v", jobsTotal, jobsDone, lastError)
}

// refreshSubscription 刷新单个订阅
func (s *subscriptionManagerImpl) refreshSubscription(ctx context.Context, subscription *model.Subscription, options *util.DownloadOptions) error {
	taskType := task.TaskTypeReloadSubs
	// 尝试在 TaskManager 中锁定该资源
	taskCtx, started := s.taskManager.StartResourceTask(ctx, taskType, subscription.ID, 1)
	if !started {
		log.Infoln("订阅[ID:%d]正在刷新中，本次跳过", subscription.ID)
		return nil
	}

	var err error
	defer func() {
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		}
		s.taskManager.FinishResourceTask(taskType, subscription.ID, errMsg)
	}()

	log.Infoln("开始刷新订阅: %s", subscription.URL)
	if subscription.URL == "" {
		err = fmt.Errorf("订阅为空")
		return err
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
	if options != nil {
		downloadOptions.ProxyURL = options.ProxyURL
		if options.Timeout > 0 {
			downloadOptions.Timeout = options.Timeout
		}
	}

	if downloadOptions.ProxyURL != "" {
		log.Infoln("使用代理下载: %s", downloadOptions.ProxyURL)
	}

	// 下载订阅内容
	var content []byte
	content, err = util.DownloadFromURL(subscription.URL, downloadOptions)
	if err != nil {
		log.Errorln("下载订阅内容失败: %v", err)

		subscription.Status = model.SubscriptionStatusInvalid
		if updateErr := s.subscriptionRepo.UpdateStatus(subscription); updateErr != nil {
			log.Errorln("更新订阅状态失败: %v", updateErr)
		}
		return fmt.Errorf("下载订阅内容失败: %w", err)
	}

	// 解析并保存代理
	if err = s.ParseAndSaveProxies(taskCtx, subscription, content); err != nil {
		log.Errorln("解析订阅内容失败: %v", err)

		subscription.Status = model.SubscriptionStatusInvalid
		if updateErr := s.subscriptionRepo.UpdateStatus(subscription); updateErr != nil {
			log.Errorln("更新订阅状态失败: %v", updateErr)
		}
		return err
	}

	s.taskManager.UpdateResourceProgress(taskType, subscription.ID, 1, "")
	return nil
}

// ParseAndSaveProxies 解析并保存代理
func (s *subscriptionManagerImpl) ParseAndSaveProxies(ctx context.Context, subscription *model.Subscription, content []byte) error {
	// 解析订阅内容
	subParser, err := s.parserFactory.GetParser(string(subscription.Type), nil)
	if err != nil {
		log.Errorln("获取解析器失败: %v", err)

		subscription.Status = model.SubscriptionStatusInvalid
		if err := s.subscriptionRepo.UpdateStatus(subscription); err != nil {
			log.Errorln("更新订阅状态失败: %v", err)
		}
		return fmt.Errorf("获取解析器失败: %w", err)
	}

	newProxies, err := subParser.Parse(content)
	if err != nil {
		log.Errorln("解析订阅内容失败: %v", err)

		subscription.Status = model.SubscriptionStatusInvalid
		if err := s.subscriptionRepo.UpdateStatus(subscription); err != nil {
			log.Errorln("更新订阅状态失败: %v", err)
		}
		return fmt.Errorf("解析订阅内容失败: %w", err)
	}

	if len(newProxies) == 0 {
		log.Errorln("未从订阅中解析出任何代理")

		// 更新订阅状态
		subscription.Status = model.SubscriptionStatusInvalid
		if err := s.subscriptionRepo.UpdateStatus(subscription); err != nil {
			log.Errorln("更新订阅状态失败: %v", err)
		}
		return fmt.Errorf("未从订阅中解析出任何代理")
	}

	// 去重
	uniqueProxies := make([]*model.Proxy, 0)
	exist := make(map[string]bool)

	for _, proxy := range newProxies {
		key := proxy.Domain + ":" + strconv.Itoa(proxy.Port) + ":" + proxy.Password
		if !exist[key] {
			exist[key] = true
			uniqueProxies = append(uniqueProxies, proxy)
		} else {
			log.Infoln(fmt.Sprintf("跳过重复的代理服务器：%s:%d:%s", proxy.Domain, proxy.Port, proxy.Password))
		}
	}

	// 保存代理
	var toUpdate []*model.Proxy
	var toCreate []*model.Proxy

	// 分类处理代理
	for _, newProxy := range uniqueProxies {
		// 检查上下文是否已取消
		select {
		case <-ctx.Done():
			return fmt.Errorf("上下文已取消")
		default:
			// 继续执行
		}

		// 检查是否已存在相同的代理
		oldProxy, err := s.proxyRepo.FindByDomainPortPassword(newProxy.Domain, newProxy.Port, newProxy.Password)
		if err != nil {
			log.Errorln("查找旧代理失败: %v", err)
			continue
		}

		// 如果旧代理存在，则更新旧代理
		if oldProxy != nil {
			// 判断是否一致，不一致则更新
			if s.IsProxyConfigSame(oldProxy, newProxy) {
				continue
			}
			// 更新旧代理的配置
			oldProxy.Name = newProxy.Name
			oldProxy.Type = newProxy.Type
			oldProxy.Config = newProxy.Config
			oldProxy.SubscriptionID = &subscription.ID
			oldProxy.Status = model.ProxyStatusPending
			toUpdate = append(toUpdate, oldProxy)
		} else {
			// 如果旧代理不存在，则准备创建新代理
			newProxy.SubscriptionID = &subscription.ID
			newProxy.Status = model.ProxyStatusPending // 设置为待处理状态，等待后续测试
			toCreate = append(toCreate, newProxy)
		}
	}
	// 批量创建新代理
	if len(toCreate) > 0 {
		if err := s.proxyRepo.BatchCreate(toCreate); err != nil {
			log.Errorln("批量创建代理失败: %v", err)
			return err
		}
		log.Infoln("批量创建了 %d 个新代理", len(toCreate))
	}

	// 批量更新已存在的代理
	if len(toUpdate) > 0 {
		if err := s.proxyRepo.BatchUpdateProxyConfig(toUpdate); err != nil {
			log.Errorln("批量更新代理配置失败: %v", err)
			return err
		}
		log.Infoln("批量更新了 %d 个代理", len(toUpdate))
	}

	// 更新订阅状态
	subscription.Status = model.SubscriptionStatusOK
	subscription.Content = string(content)
	err = s.subscriptionRepo.UpdateStatusAndContent(subscription)
	if err != nil {
		log.Errorln("更新订阅状态失败: %v", err)
		return err
	}

	log.Infoln("订阅[%s]刷新成功，解析出%d个代理，去重后%d个", subscription.URL, len(newProxies), len(uniqueProxies))
	return nil
}

// IsProxyConfigSame 比较两个代理的Type和Config是否一致，排除name字段
func (s *subscriptionManagerImpl) IsProxyConfigSame(oldProxy, newProxy *model.Proxy) bool {
	if oldProxy.Type != newProxy.Type {
		return false
	}

	if oldProxy.Config == newProxy.Config {
		return true
	}

	var oldConfig map[string]interface{}
	if err := json.Unmarshal([]byte(oldProxy.Config), &oldConfig); err != nil {
		return false
	}

	var newConfig map[string]interface{}
	if err := json.Unmarshal([]byte(newProxy.Config), &newConfig); err != nil {
		return false
	}

	delete(oldConfig, "name")
	delete(newConfig, "name")

	return cmp.Equal(oldConfig, newConfig)
}
