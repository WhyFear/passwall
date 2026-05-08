package proxy

import (
	"context"
	"fmt"

	"passwall/internal/adapter/parser"
	"passwall/internal/model"
	"passwall/internal/repository"
	"passwall/internal/service/task"
	"passwall/internal/util"

	"passwall/config"
)

type SubsPage struct {
	Page     int
	PageSize int
}

// SystemConfigProvider 定义获取系统配置的接口，用于打破循环依赖
type SystemConfigProvider interface {
	GetConfig() (*config.Config, error)
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

	// 配置操作
	GetSubscriptionConfig(id uint) (*model.SubscriptionConfig, error)
	GetAllSubscriptionConfigs() ([]*model.SubscriptionConfig, error)
	SaveSubscriptionConfig(config *model.SubscriptionConfig) error

	// 刷新操作
	RefreshSubscriptionAsync(ctx context.Context, subID uint, options *util.DownloadOptions) error
	RefreshAllSubscriptions(ctx context.Context, async bool, options *util.DownloadOptions) error

	// 解析和保存
	ParseAndSaveProxies(ctx context.Context, subscription *model.Subscription, content []byte) error
}

// subscriptionManagerImpl 订阅管理服务实现
type subscriptionManagerImpl struct {
	subscriptionRepo       repository.SubscriptionRepository
	subscriptionConfigRepo repository.SubscriptionConfigRepository
	configProvider         SystemConfigProvider
	refresher              *subscriptionRefresher
	proxySyncer            *proxySyncer
}

// NewSubscriptionManager 创建订阅管理服务
func NewSubscriptionManager(
	subscriptionRepo repository.SubscriptionRepository,
	subscriptionConfigRepo repository.SubscriptionConfigRepository,
	proxyRepo repository.ProxyRepository,
	parserFactory parser.ParserFactory,
	taskManager task.TaskManager,
	configProvider SystemConfigProvider,
	proxyTester Tester,
) SubscriptionManager {
	proxySyncer := newProxySyncer(parserFactory, proxyRepo)
	return &subscriptionManagerImpl{
		subscriptionRepo:       subscriptionRepo,
		subscriptionConfigRepo: subscriptionConfigRepo,
		configProvider:         configProvider,
		proxySyncer:            proxySyncer,
		refresher: newSubscriptionRefresher(
			subscriptionRepo,
			taskManager,
			configProvider,
			proxyTester,
			proxySyncer,
			util.DownloadFromURL,
		),
	}
}

// GetSubscriptionConfig 获取订阅自定义配置
func (s *subscriptionManagerImpl) GetSubscriptionConfig(id uint) (*model.SubscriptionConfig, error) {
	return s.subscriptionConfigRepo.FindByID(id)
}

// GetAllSubscriptionConfigs 获取所有自定义配置
func (s *subscriptionManagerImpl) GetAllSubscriptionConfigs() ([]*model.SubscriptionConfig, error) {
	return s.subscriptionConfigRepo.FindAll()
}

// SaveSubscriptionConfig 保存订阅自定义配置
func (s *subscriptionManagerImpl) SaveSubscriptionConfig(subConfig *model.SubscriptionConfig) error {
	// 获取系统默认配置
	sysCfg, err := s.configProvider.GetConfig()
	if err != nil {
		return fmt.Errorf("获取系统配置失败: %w", err)
	}

	// 比较是否与默认配置一致
	isSameAsDefault := subConfig.AutoUpdate == sysCfg.DefaultSub.AutoUpdate &&
		subConfig.UpdateInterval == sysCfg.DefaultSub.Interval &&
		subConfig.UseProxy == sysCfg.DefaultSub.UseProxy

	if isSameAsDefault {
		// 如果一致，删除自定义配置
		return s.subscriptionConfigRepo.Delete(subConfig.SubscriptionID)
	}

	// 如果不一致，保存自定义配置
	return s.subscriptionConfigRepo.Save(subConfig)
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
	subscription, err := s.subscriptionRepo.FindByID(subID)
	if err != nil {
		return fmt.Errorf("获取订阅信息失败: %w", err)
	}

	if subscription == nil {
		return fmt.Errorf("订阅不存在")
	}
	s.refresher.RefreshAsync(ctx, subscription, options)
	return nil
}

// RefreshAllSubscriptions 异步刷新所有订阅
func (s *subscriptionManagerImpl) RefreshAllSubscriptions(ctx context.Context, async bool, options *util.DownloadOptions) error {
	subscriptions, err := s.subscriptionRepo.FindAll()
	if err != nil {
		return fmt.Errorf("获取订阅列表失败: %w", err)
	}
	s.refresher.RefreshMany(ctx, subscriptions, options, async)
	return nil
}

// ParseAndSaveProxies 解析并保存代理
func (s *subscriptionManagerImpl) ParseAndSaveProxies(ctx context.Context, subscription *model.Subscription, content []byte) error {
	result, err := s.proxySyncer.Sync(ctx, subscription, content)
	if err != nil {
		_ = markSubscriptionInvalid(s.subscriptionRepo, subscription)
		return err
	}
	if err := markSubscriptionOK(s.subscriptionRepo, subscription, content); err != nil {
		return err
	}
	logProxySyncResult(subscription, result)
	return nil
}
