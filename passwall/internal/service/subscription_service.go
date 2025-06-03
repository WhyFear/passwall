package service

import (
	"errors"
	"io/ioutil"
	"net/http"
	"passwall/internal/repository"

	"passwall/internal/adapter/parser"
	"passwall/internal/model"
)

// SubscriptionService 订阅服务接口
type SubscriptionService interface {
	GetSubscriptionByID(id uint) (*model.Subscription, error)
	GetAllSubscriptions() ([]*model.Subscription, error)
	GetSubscriptionByURL(url string) (*model.Subscription, error)
	CreateSubscription(subscription *model.Subscription) error
	UpdateSubscription(subscription *model.Subscription) error
	DeleteSubscription(id uint) error
	ReloadSubscription(id uint) error
	ReloadAllSubscriptions() error
}

// DefaultSubscriptionService 默认订阅服务实现
type DefaultSubscriptionService struct {
	subscriptionRepo repository.SubscriptionRepository
	proxyRepo        repository.ProxyRepository
	parserFactory    parser.ParserFactory
}

// NewSubscriptionService 创建订阅服务
func NewSubscriptionService(
	subscriptionRepo repository.SubscriptionRepository,
	proxyRepo repository.ProxyRepository,
	parserFactory parser.ParserFactory,
) SubscriptionService {
	return &DefaultSubscriptionService{
		subscriptionRepo: subscriptionRepo,
		proxyRepo:        proxyRepo,
		parserFactory:    parserFactory,
	}
}

// GetSubscriptionByID 根据ID获取订阅
func (s *DefaultSubscriptionService) GetSubscriptionByID(id uint) (*model.Subscription, error) {
	return s.subscriptionRepo.FindByID(id)
}

// GetAllSubscriptions 获取所有订阅
func (s *DefaultSubscriptionService) GetAllSubscriptions() ([]*model.Subscription, error) {
	return s.subscriptionRepo.FindAll()
}

// GetSubscriptionByURL 根据URL获取订阅
func (s *DefaultSubscriptionService) GetSubscriptionByURL(url string) (*model.Subscription, error) {
	return s.subscriptionRepo.FindByURL(url)
}

// CreateSubscription 创建订阅
func (s *DefaultSubscriptionService) CreateSubscription(subscription *model.Subscription) error {
	if err := s.subscriptionRepo.Create(subscription); err != nil {
		return err
	}
	return nil
}

// UpdateSubscription 更新订阅
func (s *DefaultSubscriptionService) UpdateSubscription(subscription *model.Subscription) error {
	return s.subscriptionRepo.Update(subscription)
}

// DeleteSubscription 删除订阅
func (s *DefaultSubscriptionService) DeleteSubscription(id uint) error {
	// TODO: 考虑是否需要删除关联的代理服务器
	return s.subscriptionRepo.Delete(id)
}

// ReloadSubscription 重新加载订阅
func (s *DefaultSubscriptionService) ReloadSubscription(id uint) error {
	// 1. 获取订阅信息
	subscription, err := s.subscriptionRepo.FindByID(id)
	if err != nil {
		return err
	}

	// 2. 下载订阅内容
	resp, err := http.Get(subscription.URL)
	if err != nil {
		subscription.Status = model.SubscriptionStatusExpired
		_ = s.subscriptionRepo.Update(subscription)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		subscription.Status = model.SubscriptionStatusExpired
		_ = s.subscriptionRepo.Update(subscription)
		return errors.New("failed to download subscription content")
	}

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// 3. 解析订阅内容
	subParser, err := s.parserFactory.GetParser(string(subscription.Type))
	if err != nil {
		return err
	}

	_, err = subParser.Parse(content)
	if err != nil {
		subscription.Status = model.SubscriptionStatusInvalid
		_ = s.subscriptionRepo.Update(subscription)
		return err
	}

	// 4. 保存解析的代理服务器
	// TODO: 实现保存逻辑，处理已存在的代理

	// 5. 更新订阅状态
	subscription.Status = model.SubscriptionStatusOK
	return s.subscriptionRepo.Update(subscription)
}

// ReloadAllSubscriptions 重新加载所有订阅
func (s *DefaultSubscriptionService) ReloadAllSubscriptions() error {
	// 获取所有状态为正常或过期的订阅
	subscriptions, err := s.subscriptionRepo.FindAll()
	if err != nil {
		return err
	}

	var lastError error
	for _, subscription := range subscriptions {
		if subscription.Status == model.SubscriptionStatusOK || subscription.Status == model.SubscriptionStatusExpired {
			if err := s.ReloadSubscription(subscription.ID); err != nil {
				lastError = err
			}
		}
	}

	return lastError
}
