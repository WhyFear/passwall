package repository

import (
	"passwall/internal/model"

	"gorm.io/gorm"
)

type ShareConfigRepository interface {
	Create(config *model.ShareConfig) error
	FindByID(id uint) (*model.ShareConfig, error)
	FindBySlug(slug string) (*model.ShareConfig, error)
	FindAll() ([]*model.ShareConfig, error)
	Update(config *model.ShareConfig) error
	Disable(id uint) error
	SoftDelete(id uint) error
	SlugExists(slug string) (bool, error)
}

type shareConfigRepository struct {
	db *gorm.DB
}

func NewShareConfigRepository(db *gorm.DB) ShareConfigRepository {
	return &shareConfigRepository{db: db}
}

func (r *shareConfigRepository) Create(config *model.ShareConfig) error {
	return r.db.Create(config).Error
}

func (r *shareConfigRepository) FindByID(id uint) (*model.ShareConfig, error) {
	var config model.ShareConfig
	if err := r.db.Where("id = ? AND deleted = ?", id, false).First(&config).Error; err != nil {
		return nil, err
	}
	return &config, nil
}

func (r *shareConfigRepository) FindBySlug(slug string) (*model.ShareConfig, error) {
	var config model.ShareConfig
	if err := r.db.Where("slug = ? AND deleted = ?", slug, false).First(&config).Error; err != nil {
		return nil, err
	}
	return &config, nil
}

func (r *shareConfigRepository) FindAll() ([]*model.ShareConfig, error) {
	var configs []*model.ShareConfig
	if err := r.db.Where("deleted = ?", false).Order("updated_at desc").Find(&configs).Error; err != nil {
		return nil, err
	}
	return configs, nil
}

func (r *shareConfigRepository) Update(config *model.ShareConfig) error {
	return r.db.Save(config).Error
}

func (r *shareConfigRepository) Disable(id uint) error {
	return r.db.Model(&model.ShareConfig{}).
		Where("id = ? AND deleted = ?", id, false).
		Update("enabled", false).Error
}

func (r *shareConfigRepository) SoftDelete(id uint) error {
	return r.db.Model(&model.ShareConfig{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"enabled": false,
			"deleted": true,
		}).Error
}

func (r *shareConfigRepository) SlugExists(slug string) (bool, error) {
	var count int64
	if err := r.db.Model(&model.ShareConfig{}).Where("slug = ?", slug).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
