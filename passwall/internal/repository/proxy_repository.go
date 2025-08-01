package repository

import (
	"errors"
	"fmt"
	"github.com/metacubex/mihomo/log"
	"passwall/internal/model"
	"strconv"
	"time"

	"gorm.io/gorm/clause"

	"gorm.io/gorm"
)

// PageQuery 分页查询参数
type PageQuery struct {
	Page     int
	PageSize int
	OrderBy  string
	Filters  map[string]interface{}
}

// PageResult 分页查询结果
type PageResult struct {
	Total int64
	Items []*model.Proxy
}

// ProxyFilter 代理过滤条件
type ProxyFilter struct {
	Status []model.ProxyStatus
	Types  []model.ProxyType
}

// ProxyRepository 代理服务器仓库接口
type ProxyRepository interface {
	FindByID(id uint) (*model.Proxy, error)
	FindAll() ([]*model.Proxy, error)
	FindByStatus(status model.ProxyStatus) ([]*model.Proxy, error)
	FindByFilter(filter *ProxyFilter) ([]*model.Proxy, error)
	//FindBySubscriptionID(subscriptionID uint) ([]*model.Proxy, error)  // 暂时用不上
	FindByDomainAndPort(domain string, port int) (*model.Proxy, error)
	FindPage(query PageQuery) (*PageResult, error)
	FindByName(name string) (*model.Proxy, error)
	Create(proxy *model.Proxy) error
	BatchCreate(proxies []*model.Proxy) error
	Update(proxy *model.Proxy) error
	UpdateSpeedTestInfo(proxy *model.Proxy) error
	UpdateProxyConfig(proxy *model.Proxy) error
	UpdateProxyStatus(proxy *model.Proxy) error
	PinProxy(id uint, pin bool) error
	Delete(id uint) error
	GetTypes(types *[]string) error
	CountValidBySubscriptionID(subscriptionID uint) (int64, error)
	CountBySubscriptionID(subscriptionID uint) (int64, error)
}

// GormProxyRepository 基于GORM的代理服务器仓库实现
type GormProxyRepository struct {
	db *gorm.DB
}

// NewProxyRepository 创建代理服务器仓库
func NewProxyRepository(db *gorm.DB) ProxyRepository {
	return &GormProxyRepository{db: db}
}

// FindByID 根据ID查找代理服务器
func (r *GormProxyRepository) FindByID(id uint) (*model.Proxy, error) {
	var proxy model.Proxy
	result := r.db.First(&proxy, id)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return &proxy, nil
}

// FindAll 查找所有代理服务器
func (r *GormProxyRepository) FindAll() ([]*model.Proxy, error) {
	var proxies []*model.Proxy
	err := r.db.Where("status != ?", model.ProxyStatusBanned).Find(&proxies).Error
	if err != nil {
		return nil, err
	}
	return proxies, nil
}

// FindByFilter 根据过滤条件查找代理服务器
func (r *GormProxyRepository) FindByFilter(filter *ProxyFilter) ([]*model.Proxy, error) {
	var proxies []*model.Proxy
	query := r.db
	if filter != nil {
		if len(filter.Status) > 0 {
			query = query.Where("status IN ?", filter.Status)
		} else {
			query = query.Where("status != ?", model.ProxyStatusBanned)
		}
		if len(filter.Types) > 0 {
			query = query.Where("type IN ?", filter.Types)
		}
	}
	err := query.Find(&proxies).Error
	if err != nil {
		return nil, err
	}
	return proxies, nil
}

// FindByStatus 根据状态查找代理服务器
func (r *GormProxyRepository) FindByStatus(status model.ProxyStatus) ([]*model.Proxy, error) {
	var proxies []*model.Proxy
	err := r.db.Where("status = ?", status).Find(&proxies).Error
	if err != nil {
		return nil, err
	}
	return proxies, nil
}

func (r *GormProxyRepository) FindByDomainAndPort(domain string, port int) (*model.Proxy, error) {
	var proxy model.Proxy
	result := r.db.Where("domain = ? AND port = ?", domain, port).First(&proxy)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return &proxy, nil
}

// FindPage 分页查询代理服务器
func (r *GormProxyRepository) FindPage(query PageQuery) (*PageResult, error) {
	var result PageResult
	var proxies []*model.Proxy
	var total int64
	db := r.db.Model(&model.Proxy{})

	if query.Filters != nil {
		for key, value := range query.Filters {
			if key == "status" {
				if statusArray, ok := value.([]string); ok && len(statusArray) > 0 {
					db = db.Where("status IN ?", statusArray)
					continue
				} else {
					db = db.Where("status != ?", model.ProxyStatusBanned)
					continue
				}
			}
			if key == "type" {
				if typeArray, ok := value.([]string); ok && len(typeArray) > 0 {
					db = db.Where("type IN ?", typeArray)
					continue
				}
			}
			db = db.Where(key, value)
		}
	}
	db = db.Where("status != ?", model.ProxyStatusBanned)

	if err := db.Count(&total).Error; err != nil {
		return nil, err
	}
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 10
	}
	if query.OrderBy != "" {
		db = db.Order(query.OrderBy)
	} else {
		db = db.Order("updated_at DESC")
	}
	if err := db.Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize).Find(&proxies).Error; err != nil {
		return nil, err
	}
	result = PageResult{Total: total, Items: proxies}
	return &result, nil
}

// FindByName 根据名称查询代理服务器
func (r *GormProxyRepository) FindByName(name string) (*model.Proxy, error) {
	var proxy model.Proxy
	result := r.db.Where("name = ?", name).Order("download_speed DESC").First(&proxy)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return &proxy, nil
}

// Create 创建代理服务器
func (r *GormProxyRepository) Create(proxy *model.Proxy) error {
	return r.db.Create(proxy).Error
}

// BatchCreate 批量创建代理服务器
func (r *GormProxyRepository) BatchCreate(proxies []*model.Proxy) error {
	uniqueProxies := make([]*model.Proxy, 0)
	exist := make(map[string]bool)
	for _, proxy := range proxies {
		key := proxy.Domain + ":" + strconv.Itoa(proxy.Port)
		if !exist[key] {
			exist[key] = true
			uniqueProxies = append(uniqueProxies, proxy)
		} else {
			log.Infoln(fmt.Sprintf("跳过重复的代理服务器：%s:%d", proxy.Domain, proxy.Port))
		}
	}
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "domain"}, {Name: "port"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "type", "config", "subscription_id", "status", "updated_at"}),
	}).Create(uniqueProxies).Error
}

// Update 更新代理服务器
func (r *GormProxyRepository) Update(proxy *model.Proxy) error {
	return r.db.Save(proxy).Error
}

// UpdateSpeedTestInfo 只更新代理服务器的测速信息
func (r *GormProxyRepository) UpdateSpeedTestInfo(proxy *model.Proxy) error {
	return r.db.Model(proxy).
		Select("ping", "download_speed", "upload_speed", "latest_test_time", "status", "updated_at").
		Updates(map[string]interface{}{
			"ping":             proxy.Ping,
			"download_speed":   proxy.DownloadSpeed,
			"upload_speed":     proxy.UploadSpeed,
			"latest_test_time": proxy.LatestTestTime,
			"status":           proxy.Status,
			"updated_at":       time.Now(),
		}).Error
}

// UpdateProxyConfig 只更新代理服务器的基本配置信息
func (r *GormProxyRepository) UpdateProxyConfig(proxy *model.Proxy) error {
	return r.db.Model(proxy).
		Select("name", "type", "config", "updated_at").
		Updates(map[string]interface{}{
			"name":            proxy.Name,
			"type":            proxy.Type,
			"config":          proxy.Config,
			"subscription_id": proxy.SubscriptionID,
			"status":          proxy.Status,
			"updated_at":      time.Now(),
		}).Error
}

// UpdateProxyStatus 只更新代理服务器的状态
func (r *GormProxyRepository) UpdateProxyStatus(proxy *model.Proxy) error {
	return r.db.Model(proxy).
		Select("status", "updated_at").
		Updates(map[string]interface{}{
			"status":     proxy.Status,
			"updated_at": time.Now(),
		}).Error
}

// Delete 删除代理服务器
func (r *GormProxyRepository) Delete(id uint) error {
	return r.db.Delete(&model.Proxy{}, id).Error
}

// SaveSpeedTestResult 保存测速结果
func (r *GormProxyRepository) SaveSpeedTestResult(proxyID uint, result *model.SpeedTestResult) error {
	// 1. 更新代理服务器的测速信息
	proxy, err := r.FindByID(proxyID)
	if err != nil {
		return err
	}

	// 更新测速结果
	proxy.Ping = result.Ping
	proxy.DownloadSpeed = result.DownloadSpeed
	proxy.UploadSpeed = result.UploadSpeed

	// 根据测速结果更新状态
	if result.Error != nil {
		proxy.Status = model.ProxyStatusFailed
	} else if result.DownloadSpeed < 100 { // 假设下载速度低于100KB/s为慢
		proxy.Status = model.ProxyStatusUnknowError
	} else {
		proxy.Status = model.ProxyStatusOK
	}

	// 更新测试时间
	now := time.Now()
	proxy.LatestTestTime = &now

	// 2. 保存历史记录
	history := &model.SpeedTestHistory{
		ProxyID:       proxyID,
		Ping:          result.Ping,
		DownloadSpeed: result.DownloadSpeed,
		UploadSpeed:   result.UploadSpeed,
		TestTime:      time.Now(),
	}

	// 使用事务保存
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := r.UpdateSpeedTestInfo(proxy); err != nil {
			return err
		}
		return tx.Create(history).Error
	})
}

// GetTypes 获取所有代理类型
func (r *GormProxyRepository) GetTypes(types *[]string) error {
	return r.db.Model(&model.Proxy{}).Distinct("type").Pluck("type", types).Error
}

func (r *GormProxyRepository) PinProxy(id uint, pin bool) error {
	return r.db.Model(&model.Proxy{}).Where("id = ?", id).Update("pinned", pin).Error
}

func (r *GormProxyRepository) CountValidBySubscriptionID(subscriptionID uint) (int64, error) {
	var count int64
	err := r.db.Model(&model.Proxy{}).
		Where("subscription_id = ? AND status != ?", subscriptionID, model.ProxyStatusBanned).
		Count(&count).Error
	return count, err
}

func (r *GormProxyRepository) CountBySubscriptionID(subscriptionID uint) (int64, error) {
	var count int64
	err := r.db.Model(&model.Proxy{}).
		Where("subscription_id = ?", subscriptionID).
		Count(&count).Error
	return count, err
}
