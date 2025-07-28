package repository

import (
	"errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"passwall/internal/model"
	"time"
)

// TrafficRepository 流量统计仓库接口
type TrafficRepository interface {
	FindByID(id uint) (*model.TrafficStatistics, error)
	FindByProxyID(proxyID uint) (*model.TrafficStatistics, error)
	FindAll() ([]*model.TrafficStatistics, error)
	Create(traffic *model.TrafficStatistics) error
	CreateOrUpdate(traffic *model.TrafficStatistics) error
	UpdateTrafficByProxyID(traffic *model.TrafficStatistics) error
}

// GormTrafficRepository 基于GORM的流量统计仓库实现
type GormTrafficRepository struct {
	db *gorm.DB
}

// NewTrafficRepository 创建流量统计仓库
func NewTrafficRepository(db *gorm.DB) TrafficRepository {
	return &GormTrafficRepository{db: db}
}

// FindByID 根据ID查找流量统计记录
func (r *GormTrafficRepository) FindByID(id uint) (*model.TrafficStatistics, error) {
	var traffic model.TrafficStatistics
	result := r.db.First(&traffic, id)
	if result.Error != nil {
		return nil, result.Error
	}
	return &traffic, nil
}

// FindByProxyID 根据代理ID查找流量统计记录
func (r *GormTrafficRepository) FindByProxyID(proxyID uint) (*model.TrafficStatistics, error) {
	var traffic model.TrafficStatistics
	result := r.db.Where("proxy_id = ?", proxyID).First(&traffic)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, result.Error
	}
	return &traffic, nil
}

// FindLatestByProxyID 根据代理ID查找最新的流量统计记录
func (r *GormTrafficRepository) FindLatestByProxyID(proxyID uint) (*model.TrafficStatistics, error) {
	var traffic model.TrafficStatistics
	result := r.db.Where("proxy_id = ?", proxyID).Order("created_at DESC").First(&traffic)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, result.Error
	}
	return &traffic, nil
}

// FindAll 查找所有流量统计记录
func (r *GormTrafficRepository) FindAll() ([]*model.TrafficStatistics, error) {
	var traffics []*model.TrafficStatistics
	result := r.db.Find(&traffics)
	if result.Error != nil {
		return nil, result.Error
	}
	return traffics, nil
}

// Create 创建流量统计记录
func (r *GormTrafficRepository) Create(traffic *model.TrafficStatistics) error {
	return r.db.Create(traffic).Error
}

// CreateOrUpdate 创建或更新流量统计记录（根据proxy_id判断）
func (r *GormTrafficRepository) CreateOrUpdate(traffic *model.TrafficStatistics) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "proxy_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"download_total", "upload_total", "updated_at"}),
	}).Create(traffic).Error
}

// UpdateTrafficByProxyID 根据代理ID更新流量数据
func (r *GormTrafficRepository) UpdateTrafficByProxyID(traffic *model.TrafficStatistics) error {
	return r.db.Model(&model.TrafficStatistics{}).
		Where("proxy_id = ?", traffic.ProxyID).
		Updates(map[string]interface{}{
			"download_total": traffic.DownloadTotal,
			"upload_total":   traffic.UploadTotal,
			"updated_at":     time.Now(),
		}).Error
}
