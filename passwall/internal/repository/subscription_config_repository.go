package repository

import (
	"passwall/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SubscriptionConfigRepository 订阅配置仓库接口
type SubscriptionConfigRepository interface {
	FindByID(subID uint) (*model.SubscriptionConfig, error)
	FindAll() ([]*model.SubscriptionConfig, error)
	Save(config *model.SubscriptionConfig) error
	Delete(subID uint) error
}

// GormSubscriptionConfigRepository 基于GORM的实现
type GormSubscriptionConfigRepository struct {
	db *gorm.DB
}

// NewSubscriptionConfigRepository 创建订阅配置仓库
func NewSubscriptionConfigRepository(db *gorm.DB) SubscriptionConfigRepository {
	return &GormSubscriptionConfigRepository{db: db}
}

func (r *GormSubscriptionConfigRepository) FindByID(subID uint) (*model.SubscriptionConfig, error) {
	var config model.SubscriptionConfig
	err := r.db.First(&config, subID).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &config, nil
}

func (r *GormSubscriptionConfigRepository) FindAll() ([]*model.SubscriptionConfig, error) {
	var configs []*model.SubscriptionConfig
	err := r.db.Find(&configs).Error
	return configs, err
}

func (r *GormSubscriptionConfigRepository) Save(config *model.SubscriptionConfig) error {
	return r.db.Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(config).Error
}

func (r *GormSubscriptionConfigRepository) Delete(subID uint) error {
	return r.db.Delete(&model.SubscriptionConfig{}, subID).Error
}
