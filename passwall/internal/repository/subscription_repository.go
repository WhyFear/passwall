package repository

import (
	"passwall/internal/model"

	"gorm.io/gorm"
)

// SubscriptionRepository 订阅仓库接口
type SubscriptionRepository interface {
	FindByID(id uint) (*model.Subscription, error)
	FindAll() ([]*model.Subscription, error)
	FindByStatus(status model.SubscriptionStatus) ([]*model.Subscription, error)
	FindByURL(url string) (*model.Subscription, error)
	Create(subscription *model.Subscription) error
	Update(subscription *model.Subscription) error
	Delete(id uint) error
}

// GormSubscriptionRepository 基于GORM的订阅仓库实现
type GormSubscriptionRepository struct {
	db *gorm.DB
}

// NewSubscriptionRepository 创建订阅仓库
func NewSubscriptionRepository(db *gorm.DB) SubscriptionRepository {
	return &GormSubscriptionRepository{db: db}
}

// FindByID 根据ID查找订阅
func (r *GormSubscriptionRepository) FindByID(id uint) (*model.Subscription, error) {
	var subscription model.Subscription
	result := r.db.First(&subscription, id)
	if result.Error != nil {
		return nil, result.Error
	}
	return &subscription, nil
}

// FindAll 查找所有订阅
func (r *GormSubscriptionRepository) FindAll() ([]*model.Subscription, error) {
	var subscriptions []*model.Subscription
	result := r.db.Order("updated_at desc").Find(&subscriptions)
	if result.Error != nil {
		return nil, result.Error
	}
	return subscriptions, nil
}

// FindByStatus 根据状态查找订阅
func (r *GormSubscriptionRepository) FindByStatus(status model.SubscriptionStatus) ([]*model.Subscription, error) {
	var subscriptions []*model.Subscription
	result := r.db.Where("status = ?", status).Find(&subscriptions)
	if result.Error != nil {
		return nil, result.Error
	}
	return subscriptions, nil
}

// FindByURL 根据URL查找订阅
func (r *GormSubscriptionRepository) FindByURL(url string) (*model.Subscription, error) {
	var subscription model.Subscription
	result := r.db.Where("url = ?", url).First(&subscription)
	if result.Error != nil {
		return nil, result.Error
	}
	return &subscription, nil
}

// Create 创建订阅
func (r *GormSubscriptionRepository) Create(subscription *model.Subscription) error {
	return r.db.Create(subscription).Error
}

// Update 更新订阅
func (r *GormSubscriptionRepository) Update(subscription *model.Subscription) error {
	return r.db.Save(subscription).Error
}

// Delete 删除订阅
func (r *GormSubscriptionRepository) Delete(id uint) error {
	return r.db.Delete(&model.Subscription{}, id).Error
}
