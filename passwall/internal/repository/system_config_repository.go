package repository

import (
	"passwall/internal/model"

	"gorm.io/gorm"
)

type SystemConfigRepository interface {
	Get(key string) (*model.SystemConfig, error)
	SetMany(values map[string]string) error
	RestoreAll(values map[string]string) error
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

func (r *systemConfigRepository) SetMany(values map[string]string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		for key, value := range values {
			config := model.SystemConfig{Key: key, Value: value}
			if err := tx.Save(&config).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *systemConfigRepository) RestoreAll(values map[string]string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("1 = 1").Delete(&model.SystemConfig{}).Error; err != nil {
			return err
		}
		for key, value := range values {
			if err := tx.Create(&model.SystemConfig{Key: key, Value: value}).Error; err != nil {
				return err
			}
		}
		return nil
	})
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
