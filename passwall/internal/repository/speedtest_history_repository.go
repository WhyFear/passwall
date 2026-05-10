package repository

import (
	"fmt"
	"passwall/internal/model"
	"strings"
	"time"

	"gorm.io/gorm"
)

type SpeedTestHistoryPageResult struct {
	Total int64
	Items []*model.SpeedTestHistory
}

type SpeedTestHistorySummary struct {
	ProxyID       uint
	DownloadSpeed int
	CreatedAt     time.Time
}

// SpeedTestHistoryRepository 测速历史记录仓库接口
type SpeedTestHistoryRepository interface {
	FindByID(id uint) (*model.SpeedTestHistory, error)
	FindByProxyID(proxyID uint, page PageQuery) (SpeedTestHistoryPageResult, error)
	BatchFindByProxyIDList(proxyIDList []uint) (map[uint][]model.SpeedTestHistory, error)
	BatchFindLatestSummariesByProxyIDList(proxyIDList []uint, limit int) (map[uint][]SpeedTestHistorySummary, error)
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

func (r *GormSpeedTestHistoryRepository) BatchFindLatestSummariesByProxyIDList(proxyIDList []uint, limit int) (map[uint][]SpeedTestHistorySummary, error) {
	result := make(map[uint][]SpeedTestHistorySummary)
	if len(proxyIDList) == 0 {
		return result, nil
	}
	if limit <= 0 {
		limit = 5
	}

	if r.db.Dialector != nil && r.db.Dialector.Name() == "postgres" {
		return r.batchFindLatestSummariesPostgres(proxyIDList, limit)
	}
	return r.batchFindLatestSummariesFallback(proxyIDList, limit)
}

func (r *GormSpeedTestHistoryRepository) batchFindLatestSummariesPostgres(proxyIDList []uint, limit int) (map[uint][]SpeedTestHistorySummary, error) {
	values := make([]string, 0, len(proxyIDList))
	for _, proxyID := range proxyIDList {
		values = append(values, fmt.Sprintf("(%d)", proxyID))
	}

	var summaries []SpeedTestHistorySummary
	err := r.db.Raw(`
		SELECT h.proxy_id, h.download_speed, h.created_at
		FROM (VALUES `+strings.Join(values, ",")+`) AS ids(proxy_id)
		JOIN LATERAL (
			SELECT proxy_id, download_speed, created_at
			FROM speed_test_histories
			WHERE proxy_id = ids.proxy_id
			ORDER BY created_at DESC
			LIMIT ?
		) h ON true
		ORDER BY h.proxy_id, h.created_at DESC
	`, limit).Scan(&summaries).Error
	if err != nil {
		return nil, err
	}

	result := make(map[uint][]SpeedTestHistorySummary)
	for _, summary := range summaries {
		result[summary.ProxyID] = append(result[summary.ProxyID], summary)
	}
	return result, nil
}

func (r *GormSpeedTestHistoryRepository) batchFindLatestSummariesFallback(proxyIDList []uint, limit int) (map[uint][]SpeedTestHistorySummary, error) {
	result := make(map[uint][]SpeedTestHistorySummary)
	for _, proxyID := range proxyIDList {
		var summaries []SpeedTestHistorySummary
		err := r.db.Model(&model.SpeedTestHistory{}).
			Select("proxy_id", "download_speed", "created_at").
			Where("proxy_id = ?", proxyID).
			Order("created_at DESC").
			Limit(limit).
			Find(&summaries).Error
		if err != nil {
			return nil, err
		}
		if len(summaries) > 0 {
			result[proxyID] = summaries
		}
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
