package repository

import (
	"passwall/internal/model"
	"time"

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

// ProxyRepository 代理服务器仓库接口
type ProxyRepository interface {
	FindByID(id uint) (*model.Proxy, error)
	FindAll(filters map[string]interface{}) ([]*model.Proxy, error)
	FindByStatus(status model.ProxyStatus) ([]*model.Proxy, error)
	Create(proxy *model.Proxy) error
	Update(proxy *model.Proxy) error
	Delete(id uint) error
	SaveSpeedTestResult(proxyID uint, result *model.SpeedTestResult) error
	// 新增分页查询方法
	FindPage(query PageQuery) (*PageResult, error)
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
func (r *GormProxyRepository) FindAll(filters map[string]interface{}) ([]*model.Proxy, error) {
	var proxies []*model.Proxy
	query := r.db

	// 应用过滤条件
	if filters != nil {
		query = query.Where(filters)
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

// Create 创建代理服务器
func (r *GormProxyRepository) Create(proxy *model.Proxy) error {
	return r.db.Create(proxy).Error
}

// Update 更新代理服务器
func (r *GormProxyRepository) Update(proxy *model.Proxy) error {
	return r.db.Save(proxy).Error
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
		if err := tx.Save(proxy).Error; err != nil {
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
			db = db.Where(key, value)
		}
	}

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
