package repository

import (
	"passwall/internal/model"

	"gorm.io/gorm"
)

type SystemConfigRepository interface {
	Get(key string) (*model.SystemConfig, error)
	Set(key string, value string) error
	GetAll() (map[string]string, error)
}

type systemConfigRepository struct {
	db *gorm.DB
}

func NewSystemConfigRepository(db *gorm.DB) SystemConfigRepository {
	return &systemConfigRepository{db: db}
}

func (r *systemConfigRepository) Get(key string) (*model.SystemConfig, error) {
	var config model.SystemConfig
	if err := r.db.Where("key = ?", key).First(&config).Error; err != nil {
		return nil, err
	}
	return &config, nil
}

func (r *systemConfigRepository) Set(key string, value string) error {
	config := model.SystemConfig{
		Key:   key,
		Value: value,
	}
	// Upsert: On conflict update value
	return r.db.Save(&config).Error
}

func (r *systemConfigRepository) GetAll() (map[string]string, error) {
	var configs []model.SystemConfig
	if err := r.db.Find(&configs).Error; err != nil {
		return nil, err
	}
	result := make(map[string]string)
	for _, c := range configs {
		result[c.Key] = c.Value
	}
	return result, nil
}
