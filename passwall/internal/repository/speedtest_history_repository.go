package repository

import (
	"passwall/internal/model"
	"time"

	"gorm.io/gorm"
)

// SpeedTestHistoryRepository 测速历史记录仓库接口
type SpeedTestHistoryRepository interface {
	FindByID(id uint) (*model.SpeedTestHistory, error)
	FindByProxyID(proxyID uint, limit int) ([]*model.SpeedTestHistory, error)
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
func (r *GormSpeedTestHistoryRepository) FindByProxyID(proxyID uint, limit int) ([]*model.SpeedTestHistory, error) {
	var histories []*model.SpeedTestHistory
	query := r.db.Where("proxy_id = ?", proxyID).Order("created_at DESC")

	// 限制返回数量
	if limit > 0 {
		query = query.Limit(limit)
	}

	result := query.Find(&histories)
	if result.Error != nil {
		return nil, result.Error
	}
	return histories, nil
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
