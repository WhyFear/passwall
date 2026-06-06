package repository

import (
	"errors"
	"fmt"
	"passwall/internal/model"
	"strings"
	"time"

	"github.com/metacubex/mihomo/log"

	"gorm.io/gorm/clause"

	"gorm.io/gorm"
)

// PageQuery 分页查询参数
type PageQuery struct {
	Page     int
	PageSize int
	OrderBy  string
	Filters  *NodeFilter
}

// PageResult 分页查询结果
type PageResult struct {
	Total int64
	Items []*model.Proxy
}

// NodeFilter 代理节点过滤条件
type NodeFilter struct {
	Status      []model.ProxyStatus
	Types       []model.ProxyType
	CountryCode []string
	RiskLevel   []string
	AppUnlock   []string
}

// ProxyRepository 代理服务器仓库接口
type ProxyRepository interface {
	FindByID(id uint) (*model.Proxy, error)
	FindAll() ([]*model.Proxy, error)
	FindByStatus(status model.ProxyStatus) ([]*model.Proxy, error)
	FindByFilter(filter *NodeFilter) ([]*model.Proxy, error)
	FindByStatusAndTypesIncludingBanned(statuses []model.ProxyStatus, types []model.ProxyType) ([]*model.Proxy, error)
	//FindBySubscriptionID(subscriptionID uint) ([]*model.Proxy, error)  // 暂时用不上
	FindByDomainPortPassword(domain string, port int, password string) (*model.Proxy, error)
	FindPage(query PageQuery) (*PageResult, error)
	FindByName(name string) (*model.Proxy, error)
	FindNotInIDs(ids []uint) ([]uint, error)
	Create(proxy *model.Proxy) error
	BatchCreate(proxies []*model.Proxy) error
	BatchUpdateProxyStatus(proxyIDs []uint, status model.ProxyStatus) error
	BatchUpdateProxyConfig(proxies []*model.Proxy) error
	Update(proxy *model.Proxy) error
	UpdateSpeedTestInfo(proxy *model.Proxy) error
	UpdateProxyConfig(proxy *model.Proxy) error
	UpdateProxyStatus(proxy *model.Proxy) error
	PinProxy(id uint, pin bool) error
	Delete(id uint) error
	GetTypes(types *[]string) error
	CountValidBySubscriptionID(subscriptionID uint) (int64, error)
	CountBySubscriptionID(subscriptionID uint) (int64, error)
	CountOKBySubscriptionID(subscriptionID uint) (int64, error)
}

// GormProxyRepository 基于GORM的代理服务器仓库实现
type GormProxyRepository struct {
	db *gorm.DB
}

func (r *GormProxyRepository) FindNotInIDs(ids []uint) ([]uint, error) {
	var proxyIDs []uint
	result := r.db.Model(&model.Proxy{}).Where("id NOT IN ?", ids).Where("status = ?", model.ProxyStatusOK).Pluck("id", &proxyIDs)
	if result.Error != nil {
		return nil, result.Error
	}
	return proxyIDs, nil
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
func (r *GormProxyRepository) FindByFilter(filter *NodeFilter) ([]*model.Proxy, error) {
	var proxies []*model.Proxy
	query, joinedIPMetadata := r.applyNodeFilter(r.db.Model(&model.Proxy{}), filter)
	if joinedIPMetadata {
		query = query.Distinct("proxies.*")
	}
	err := query.Find(&proxies).Error
	if err != nil {
		return nil, err
	}
	return proxies, nil
}

// FindByStatusAndTypesIncludingBanned 根据状态和类型查找代理服务器，允许返回已封禁节点。
func (r *GormProxyRepository) FindByStatusAndTypesIncludingBanned(statuses []model.ProxyStatus, types []model.ProxyType) ([]*model.Proxy, error) {
	var proxies []*model.Proxy
	query := r.db.Model(&model.Proxy{})
	if len(statuses) > 0 {
		query = query.Where("status IN ?", statuses)
	}
	if len(types) > 0 {
		query = query.Where("type IN ?", types)
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

func (r *GormProxyRepository) FindByDomainPortPassword(domain string, port int, password string) (*model.Proxy, error) {
	var proxy model.Proxy
	result := r.db.Where("domain = ? AND port = ? AND password = ?", domain, port, password).First(&proxy)
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
	db, joinedIPMetadata := r.applyNodeFilter(r.db.Model(&model.Proxy{}), query.Filters)

	countDB := db
	if joinedIPMetadata {
		countDB = countDB.Distinct("proxies.id")
	}
	if err := countDB.Count(&total).Error; err != nil {
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

	// 如果有join，需要去重
	if joinedIPMetadata {
		db = db.Distinct("proxies.*")
	}
	if err := db.Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize).Find(&proxies).Error; err != nil {
		return nil, err
	}
	result = PageResult{Total: total, Items: proxies}
	return &result, nil
}

func (r *GormProxyRepository) applyNodeFilter(db *gorm.DB, filter *NodeFilter) (*gorm.DB, bool) {
	joinedIPMetadata := false
	if filter != nil {
		if len(filter.Status) > 0 {
			db = db.Where("proxies.status IN ?", filter.Status)
		}
		if len(filter.Types) > 0 {
			db = db.Where("proxies.type IN ?", filter.Types)
		}
		countryCodeArray := normalizeStringFilterValues(filter.CountryCode)
		riskLevelArray := normalizeStringFilterValues(filter.RiskLevel)
		if len(countryCodeArray) > 0 || len(riskLevelArray) > 0 {
			db = db.Joins("INNER JOIN proxy_ip_addresses ON proxies.id = proxy_ip_addresses.proxy_id AND proxy_ip_addresses.latest = ?", true).
				Joins("INNER JOIN ip_base_infos ON proxy_ip_addresses.ip_addresses_id = ip_base_infos.ip_addresses_id")
			joinedIPMetadata = true
			if len(countryCodeArray) > 0 {
				db = db.Where("ip_base_infos.country_code IN ?", countryCodeArray)
			}
			if len(riskLevelArray) > 0 {
				db = db.Where("ip_base_infos.risk_level IN ?", riskLevelArray)
			}
		}
		appUnlockArray := normalizeStringFilterValues(filter.AppUnlock)
		if len(appUnlockArray) > 0 {
			matchingProxyIDs := r.unlockedAppProxyIDs(appUnlockArray)
			db = db.Where("proxies.id IN (?)", matchingProxyIDs)
		}
	}
	db = db.Where("proxies.status != ?", model.ProxyStatusBanned)
	return db, joinedIPMetadata
}

// normalizeStringFilterValues is defense-in-depth for non-HTTP callers; HTTP filters
// are already normalized by parseNodeFilter.
func normalizeStringFilterValues(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func (r *GormProxyRepository) unlockedAppProxyIDs(appUnlockArray []string) *gorm.DB {
	return r.db.Model(&model.ProxyIPAddress{}).
		Select("proxy_ip_addresses.proxy_id").
		Joins("INNER JOIN ip_unlock_infos ON proxy_ip_addresses.ip_addresses_id = ip_unlock_infos.ip_addresses_id").
		Where("proxy_ip_addresses.latest = ?", true).
		Where("ip_unlock_infos.status = ?", "unlock").
		Where("ip_unlock_infos.app_name IN ?", appUnlockArray).
		Group("proxy_ip_addresses.proxy_id").
		Having("COUNT(DISTINCT ip_unlock_infos.app_name) = ?", len(appUnlockArray))
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
	if len(proxies) == 0 {
		return nil
	}

	uniqueProxies := make([]*model.Proxy, 0)
	exist := make(map[string]bool)
	for _, proxy := range proxies {
		key := proxy.DedupKey()
		if !exist[key] {
			exist[key] = true
			uniqueProxies = append(uniqueProxies, proxy)
		} else {
			log.Infoln("跳过重复的代理服务器：%s:%d:%s", proxy.Domain, proxy.Port, proxy.Password)
		}
	}

	// 分批处理，避免PostgreSQL 65535参数限制
	// 每个代理大约需要8-10个参数，安全批次大小设为500
	batchSize := 500
	for i := 0; i < len(uniqueProxies); i += batchSize {
		end := i + batchSize
		if end > len(uniqueProxies) {
			end = len(uniqueProxies)
		}

		batch := uniqueProxies[i:end]
		if err := r.db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "domain"}, {Name: "port"}, {Name: "password"}},
			DoUpdates: clause.AssignmentColumns([]string{"name", "type", "config", "subscription_id", "status", "updated_at"}),
		}).Create(batch).Error; err != nil {
			return fmt.Errorf("批量创建代理批次 %d-%d 失败: %w", i, end, err)
		}
	}

	return nil
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

// BatchUpdateProxyStatus 批量更新代理服务器的状态
func (r *GormProxyRepository) BatchUpdateProxyStatus(proxyIDs []uint, status model.ProxyStatus) error {
	if len(proxyIDs) == 0 {
		return nil
	}

	return r.db.Model(&model.Proxy{}).
		Where("id IN ?", proxyIDs).
		Updates(map[string]interface{}{
			"status":     status,
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
	if err != nil || proxy == nil {
		return fmt.Errorf("find proxy by id error: %w", err)
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

// CountOKBySubscriptionID 根据订阅ID统计可用代理数量
func (r *GormProxyRepository) CountOKBySubscriptionID(subscriptionID uint) (int64, error) {
	var count int64
	result := r.db.Model(&model.Proxy{}).Where("subscription_id = ?", subscriptionID).Where("status = ?", model.ProxyStatusOK).Count(&count)
	if result.Error != nil {
		return 0, result.Error
	}
	return count, nil
}

// BatchUpdateProxyConfig 在事务中批量更新代理服务器配置
func (r *GormProxyRepository) BatchUpdateProxyConfig(proxies []*model.Proxy) error {
	if len(proxies) == 0 {
		return nil
	}

	return r.db.Transaction(func(tx *gorm.DB) error {
		// 使用批量更新减少数据库IO
		for _, proxy := range proxies {
			if err := tx.Model(proxy).
				Select("name", "type", "config", "subscription_id", "status", "updated_at").
				Updates(map[string]interface{}{
					"name":            proxy.Name,
					"type":            proxy.Type,
					"config":          proxy.Config,
					"subscription_id": proxy.SubscriptionID,
					"status":          proxy.Status,
					"updated_at":      time.Now(),
				}).Error; err != nil {
				log.Errorln("事务中更新代理 %d 配置失败: %v", proxy.ID, err)
				continue
			}
		}
		return nil
	})
}
