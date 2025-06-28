package repository

import (
	"passwall/internal/model"
	"strings"

	"gorm.io/gorm"
)

type SubsPage struct {
	Page     int
	PageSize int
}

// SubscriptionRepository 订阅仓库接口
type SubscriptionRepository interface {
	FindByID(id uint) (*model.Subscription, error)
	FindAll() ([]*model.Subscription, error)
	FindByStatus(status model.SubscriptionStatus) ([]*model.Subscription, error)
	FindByURL(url string) (*model.Subscription, error)
	FindPage(page SubsPage) ([]*model.Subscription, int64, error)
	Create(subscription *model.Subscription) error
	Update(subscription *model.Subscription) error
	UpdateStatus(subscription *model.Subscription) error
	UpdateStatusAndContent(subscription *model.Subscription) error
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

func (r *GormSubscriptionRepository) FindPage(page SubsPage) ([]*model.Subscription, int64, error) {
	if page.Page <= 0 {
		page.Page = 1
	}
	if page.PageSize <= 0 {
		page.PageSize = 10
	}

	var subscriptions []*model.Subscription
	query := r.db.Model(&model.Subscription{})
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	query = query.Offset((page.Page - 1) * page.PageSize).Limit(page.PageSize).Order("created_at desc")
	if err := query.Find(&subscriptions).Error; err != nil {
		return nil, 0, err
	}
	return subscriptions, total, nil
}

// sanitizeContent 处理内容，移除或替换空字节
func sanitizeContent(content string) string {
	// 替换所有空字节为空字符串
	return strings.ReplaceAll(content, string([]byte{0x00}), "")
}

// Create 创建订阅
func (r *GormSubscriptionRepository) Create(subscription *model.Subscription) error {
	// 在保存前处理content内容
	subscription.Content = sanitizeContent(subscription.Content)
	return r.db.Create(subscription).Error
}

// Update 更新订阅
func (r *GormSubscriptionRepository) Update(subscription *model.Subscription) error {
	// 在保存前处理content内容
	subscription.Content = sanitizeContent(subscription.Content)
	return r.db.Save(subscription).Error
}

// UpdateStatus 更新订阅状态
func (r *GormSubscriptionRepository) UpdateStatus(subscription *model.Subscription) error {
	return r.db.Model(subscription).Select("status").Updates(map[string]interface{}{"status": subscription.Status}).Error
}

// UpdateStatusAndContent 更新订阅状态和内容
func (r *GormSubscriptionRepository) UpdateStatusAndContent(subscription *model.Subscription) error {
	// 在保存前处理content内容
	subscription.Content = sanitizeContent(subscription.Content)
	return r.db.Model(subscription).Select("status", "content").Updates(map[string]interface{}{
		"status":  subscription.Status,
		"content": subscription.Content,
	}).Error
}

// Delete 删除订阅
func (r *GormSubscriptionRepository) Delete(id uint) error {
	return r.db.Delete(&model.Subscription{}, id).Error
}
