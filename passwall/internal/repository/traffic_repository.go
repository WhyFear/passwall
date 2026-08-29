package repository

import (
	"errors"
	"fmt"
	"passwall/internal/model"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TrafficRepository 流量统计仓库接口
type TrafficRepository interface {
	FindByID(id uint) (*model.TrafficStatistics, error)
	FindByProxyID(proxyID uint) (*model.TrafficStatistics, error)
	FindByProxyIDList(proxyIDList []uint) (map[uint]*model.TrafficStatistics, error)
	FindAll() ([]*model.TrafficStatistics, error)
	IncrementTraffic(batch []model.TrafficStatistics) error
	ValidateProxyIDUnique() error
}

// GormTrafficRepository 基于GORM的流量统计仓库实现
type GormTrafficRepository struct {
	db *gorm.DB
}

func (r *GormTrafficRepository) FindByProxyIDList(proxyIDList []uint) (map[uint]*model.TrafficStatistics, error) {
	if proxyIDList == nil || len(proxyIDList) == 0 {
		return nil, fmt.Errorf("proxyIDList is empty")
	}

	var traffics []*model.TrafficStatistics
	result := r.db.Where("proxy_id IN ?", proxyIDList).Find(&traffics)
	if result.Error != nil {
		return nil, result.Error
	}

	trafficMap := make(map[uint]*model.TrafficStatistics)
	for _, traffic := range traffics {
		trafficMap[traffic.ProxyID] = traffic
	}

	return trafficMap, nil
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
	err := r.db.Find(&traffics).Error
	if err != nil {
		return nil, err
	}
	return traffics, nil
}

// IncrementTraffic atomically adds a whole flush batch in one UPSERT.
func (r *GormTrafficRepository) IncrementTraffic(batch []model.TrafficStatistics) error {
	if len(batch) == 0 {
		return nil
	}
	// ponytail: one statement is atomic; chunk transactionally if a flush reaches PostgreSQL's bind limit.
	return r.incrementTrafficStatement(batch).Error
}

func (r *GormTrafficRepository) incrementTrafficStatement(batch []model.TrafficStatistics) *gorm.DB {
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "proxy_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"download_total": gorm.Expr("traffic_statistics.download_total + excluded.download_total"),
			"upload_total":   gorm.Expr("traffic_statistics.upload_total + excluded.upload_total"),
			"updated_at":     time.Now(),
		}),
	}).Create(&batch)
}

func (r *GormTrafficRepository) ValidateProxyIDUnique() error {
	if r.db.Dialector.Name() == "postgres" {
		var ready bool
		err := r.db.Raw(`
			SELECT EXISTS (
				SELECT 1
				FROM pg_index AS i
				JOIN pg_class AS t ON t.oid = i.indrelid
				JOIN pg_attribute AS a ON a.attrelid = t.oid AND a.attname = 'proxy_id'
				WHERE t.oid = to_regclass('traffic_statistics')
				  AND i.indisunique AND i.indisvalid AND i.indisready
				  AND i.indnkeyatts = 1 AND i.indkey[0] = a.attnum
				  AND i.indpred IS NULL AND i.indexprs IS NULL
			)`).Scan(&ready).Error
		if err != nil {
			return fmt.Errorf("inspect traffic_statistics indexes: %w", err)
		}
		if ready {
			return nil
		}
		return fmt.Errorf("traffic_statistics(proxy_id) requires a unique index; apply the PostgreSQL traffic migration")
	}

	indexes, err := r.db.Migrator().GetIndexes(&model.TrafficStatistics{})
	if err != nil {
		return fmt.Errorf("inspect traffic_statistics indexes: %w", err)
	}
	for _, index := range indexes {
		unique, ok := index.Unique()
		columns := index.Columns()
		if ok && unique && len(columns) == 1 && columns[0] == "proxy_id" {
			return nil
		}
	}
	return fmt.Errorf("traffic_statistics(proxy_id) requires a unique index; apply the PostgreSQL traffic migration")
}
