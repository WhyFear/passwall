package repository

import (
	"passwall/internal/model"
	"time"

	"gorm.io/gorm"
)

type SpeedTestHistoryPageResult struct {
	Total int64
	Items []*model.SpeedTestHistory
}

// SpeedTestHistoryRepository 测速历史记录仓库接口
type SpeedTestHistoryRepository interface {
	FindByID(id uint) (*model.SpeedTestHistory, error)
	FindByProxyID(proxyID uint, page PageQuery) (SpeedTestHistoryPageResult, error)
	FindByTimeRange(proxyID uint, startTime, endTime time.Time) ([]*model.SpeedTestHistory, error)
	Create(history *model.SpeedTestHistory) error
	Delete(id uint) error
	DeleteByProxyID(proxyID uint) error
}

// GormSpeedTestHistoryRepository 基于GORM的测速历史记录仓库实现
type GormSpeedTestHistoryRepository struct {
	db *gorm.DB
}

// NewSpeedTestHistoryRepository 创建测速历史记录仓库
func NewSpeedTestHistoryRepository(db *gorm.DB) SpeedTestHistoryRepository {
	return &GormSpeedTestHistoryRepository{db: db}
}

// FindByID 根据ID查找测速历史记录
func (r *GormSpeedTestHistoryRepository) FindByID(id uint) (*model.SpeedTestHistory, error) {
	var history model.SpeedTestHistory
	result := r.db.First(&history, id)
	if result.Error != nil {
		return nil, result.Error
	}
	return &history, nil
}

// FindByProxyID 根据代理ID查找测速历史记录
func (r *GormSpeedTestHistoryRepository) FindByProxyID(proxyID uint, page PageQuery) (SpeedTestHistoryPageResult, error) {
	var histories []*model.SpeedTestHistory
	var total int64
	query := r.db.Model(&model.SpeedTestHistory{}).Where("proxy_id = ?", proxyID).Order("created_at DESC")

	query.Count(&total)
	// 设置默认值
	if page.Page <= 0 {
		page.Page = 1
	}

	if page.PageSize <= 0 {
		page.PageSize = 10
	}
	// 执行分页查询
	if err := query.Offset((page.Page - 1) * page.PageSize).
		Limit(page.PageSize).
		Find(&histories).Error; err != nil {
		return SpeedTestHistoryPageResult{}, err
	}
	return SpeedTestHistoryPageResult{
		Total: total,
		Items: histories,
	}, nil
}

// FindByTimeRange 根据时间范围查找测速历史记录
func (r *GormSpeedTestHistoryRepository) FindByTimeRange(proxyID uint, startTime, endTime time.Time) ([]*model.SpeedTestHistory, error) {
	var histories []*model.SpeedTestHistory
	result := r.db.Where("proxy_id = ? AND created_at BETWEEN ? AND ?", proxyID, startTime, endTime).
		Order("created_at DESC").Find(&histories)
	if result.Error != nil {
		return nil, result.Error
	}
	return histories, nil
}

// Create 创建测速历史记录
func (r *GormSpeedTestHistoryRepository) Create(history *model.SpeedTestHistory) error {
	return r.db.Create(history).Error
}

// Delete 删除测速历史记录
func (r *GormSpeedTestHistoryRepository) Delete(id uint) error {
	return r.db.Delete(&model.SpeedTestHistory{}, id).Error
}

// DeleteByProxyID 删除指定代理的所有测速历史记录
func (r *GormSpeedTestHistoryRepository) DeleteByProxyID(proxyID uint) error {
	return r.db.Where("proxy_id = ?", proxyID).Delete(&model.SpeedTestHistory{}).Error
}
