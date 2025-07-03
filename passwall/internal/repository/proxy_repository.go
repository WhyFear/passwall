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
	if result.Error != nil {
		return nil, result.Error
	}
	return &proxy, nil
}

// FindAll 查找所有代理服务器
func (r *GormProxyRepository) FindAll() ([]*model.Proxy, error) {
	var proxies []*model.Proxy
	query := r.db

	// 应用过滤条件
	query = query.Where("status != ?", model.ProxyStatusBanned)

	result := query.Find(&proxies)
	if result.Error != nil {
		return nil, result.Error
	}
	return proxies, nil
}

// FindByFilter 根据过滤条件查找代理服务器
func (r *GormProxyRepository) FindByFilter(filter *ProxyFilter) ([]*model.Proxy, error) {
	var proxies []*model.Proxy
	query := r.db

	// 应用过滤条件
	if filter != nil {
		// 按状态过滤
		if len(filter.Status) > 0 {
			query = query.Where("status IN ?", filter.Status)
		} else {
			query = query.Where("status != ?", model.ProxyStatusBanned)
		}

		// 按类型过滤
		if len(filter.Types) > 0 {
			query = query.Where("type IN ?", filter.Types)
		}
	}

	result := query.Find(&proxies)
	if result.Error != nil {
		return nil, result.Error
	}
	return proxies, nil
}

// FindByStatus 根据状态查找代理服务器
func (r *GormProxyRepository) FindByStatus(status model.ProxyStatus) ([]*model.Proxy, error) {
	var proxies []*model.Proxy
	result := r.db.Where("status = ?", status).Find(&proxies)
	if result.Error != nil {
		return nil, result.Error
	}
	return proxies, nil
}

func (r *GormProxyRepository) FindByDomainAndPort(domain string, port int) (*model.Proxy, error) {
	var proxy model.Proxy
	result := r.db.Where("domain = ? AND port = ?", domain, port).First(&proxy)
	// 区分是没记录还是出错
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil // 没有找到记录
		} else {
			return nil, result.Error // 其他错误
		}
	}
	return &proxy, nil
}

// Create 创建代理服务器
func (r *GormProxyRepository) Create(proxy *model.Proxy) error {
	return r.db.Create(proxy).Error
}

// BatchCreate 批量创建代理服务器
func (r *GormProxyRepository) BatchCreate(proxies []*model.Proxy) error {
	// 先对传入的代理服务器列表进行去重，避免多次更新同一行的问题
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
	// 只更新延迟、下载速度、上传速度、测试时间和状态
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

// FindPage 分页查询代理服务器
func (r *GormProxyRepository) FindPage(query PageQuery) (*PageResult, error) {
	var proxies []*model.Proxy
	var total int64

	// 构建查询条件
	db := r.db.Model(&model.Proxy{})

	// 应用过滤条件
	if query.Filters != nil {
		for key, value := range query.Filters {
			// 特殊处理状态数组
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

	// 计算总数
	if err := db.Count(&total).Error; err != nil {
		return nil, err
	}

	// 设置默认值
	if query.Page <= 0 {
		query.Page = 1
	}

	if query.PageSize <= 0 {
		query.PageSize = 10
	}

	// 设置排序
	if query.OrderBy != "" {
		db = db.Order(query.OrderBy)
	} else {
		db = db.Order("updated_at DESC")
	}

	// 执行分页查询
	if err := db.Offset((query.Page - 1) * query.PageSize).
		Limit(query.PageSize).
		Find(&proxies).Error; err != nil {
		return nil, err
	}

	return &PageResult{
		Total: total,
		Items: proxies,
	}, nil
}

// GetTypes 获取所有代理类型
func (r *GormProxyRepository) GetTypes(types *[]string) error {
	var proxyTypes []string
	result := r.db.Model(&model.Proxy{}).Distinct("type").Pluck("type", &proxyTypes)
	if result.Error != nil {
		return result.Error
	}
	*types = proxyTypes
	return nil
}

func (r *GormProxyRepository) PinProxy(id uint, pin bool) error {
	db := r.db.Model(&model.Proxy{})
	result := db.Where("id =?", id).Update("pinned", pin)
	if result.Error != nil {
		log.Errorln("更新代理状态失败: %v", result.Error)
		return result.Error
	}
	return nil
}

func (r *GormProxyRepository) CountValidBySubscriptionID(subscriptionID uint) (int64, error) {
	var count int64
	result := r.db.Model(&model.Proxy{}).Where("subscription_id = ? and status != ?", subscriptionID, model.ProxyStatusBanned).Count(&count)
	if result.Error != nil {
		return 0, result.Error
	}
	return count, nil
}

func (r *GormProxyRepository) CountBySubscriptionID(subscriptionID uint) (int64, error) {
	var count int64
	result := r.db.Model(&model.Proxy{}).Where("subscription_id = ?", subscriptionID).Count(&count)
	if result.Error != nil {
		return 0, result.Error
	}
	return count, nil
}
