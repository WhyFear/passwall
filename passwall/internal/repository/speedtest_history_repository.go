package repository

import (
	"fmt"
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
	BatchFindByProxyIDList(proxyIDList []uint) (map[uint][]model.SpeedTestHistory, error)
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
	err := query.Offset((page.Page - 1) * page.PageSize).
		Limit(page.PageSize).
		Find(&histories).Error

	if err != nil {
		return SpeedTestHistoryPageResult{}, err
	}
	return SpeedTestHistoryPageResult{
		Total: total,
		Items: histories,
	}, nil
}

func (r *GormSpeedTestHistoryRepository) BatchFindByProxyIDList(proxyIDList []uint) (map[uint][]model.SpeedTestHistory, error) {
	if len(proxyIDList) == 0 {
		return nil, fmt.Errorf("proxyIDList is empty")
	}

	var histories []model.SpeedTestHistory

	err := r.db.Raw(`
		WITH filtered_histories AS (
			SELECT *
			FROM speed_test_histories
			WHERE proxy_id IN ?
		),
		ranked_histories AS (
			SELECT 
				*,
				ROW_NUMBER() OVER (PARTITION BY proxy_id ORDER BY created_at DESC) as rn
			FROM filtered_histories
		)
		SELECT *
		FROM ranked_histories
		WHERE rn <= 5
		ORDER BY proxy_id, created_at DESC
	`, proxyIDList).Scan(&histories).Error

	if err != nil {
		return nil, err
	}

	result := make(map[uint][]model.SpeedTestHistory)
	for _, history := range histories {
		result[history.ProxyID] = append(result[history.ProxyID], history)
	}
	return result, nil
}

// FindByTimeRange 根据时间范围查找测速历史记录
func (r *GormSpeedTestHistoryRepository) FindByTimeRange(proxyID uint, startTime, endTime time.Time) ([]*model.SpeedTestHistory, error) {
	var histories []*model.SpeedTestHistory

	err := r.db.Where("proxy_id = ? AND created_at BETWEEN ? AND ?", proxyID, startTime, endTime).
		Order("created_at DESC").Find(&histories).Error

	if err != nil {
		return nil, err
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
