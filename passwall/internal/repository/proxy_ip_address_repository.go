package repository

import (
	"errors"
	"passwall/internal/model"
	"time"

	"gorm.io/gorm"
)

// ProxyIPAddressRepository 代理IP关联仓库接口
type ProxyIPAddressRepository interface {
	FindByID(id uint) (*model.ProxyIPAddress, error)
	FindByProxyID(proxyID uint) ([]*model.ProxyIPAddress, error)
	FindByIPAddressID(ipAddressID uint) ([]*model.ProxyIPAddress, error)
	CreateOrUpdate(proxyIPAddress *model.ProxyIPAddress) error
}

// GormProxyIPAddressRepository 基于GORM的代理IP关联仓库实现
type GormProxyIPAddressRepository struct {
	db *gorm.DB
}

// NewProxyIPAddressRepository 创建代理IP关联仓库
func NewProxyIPAddressRepository(db *gorm.DB) ProxyIPAddressRepository {
	return &GormProxyIPAddressRepository{db: db}
}

// FindByID 根据ID查找代理IP关联
func (r *GormProxyIPAddressRepository) FindByID(id uint) (*model.ProxyIPAddress, error) {
	var proxyIPAddress model.ProxyIPAddress
	result := r.db.First(&proxyIPAddress, id)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return &proxyIPAddress, nil
}

// FindByProxyID 根据代理ID查找关联的IP地址
func (r *GormProxyIPAddressRepository) FindByProxyID(proxyID uint) ([]*model.ProxyIPAddress, error) {
	var proxyIPAddresses []*model.ProxyIPAddress
	err := r.db.Where("proxy_id = ?", proxyID).Find(&proxyIPAddresses).Error
	if err != nil {
		return nil, err
	}
	return proxyIPAddresses, nil
}

// FindByIPAddressID 根据IP地址ID查找关联的代理
func (r *GormProxyIPAddressRepository) FindByIPAddressID(ipAddressID uint) ([]*model.ProxyIPAddress, error) {
	var proxyIPAddresses []*model.ProxyIPAddress
	err := r.db.Where("ip_addresses_id = ?", ipAddressID).Find(&proxyIPAddresses).Error
	if err != nil {
		return nil, err
	}
	return proxyIPAddresses, nil
}

// CreateOrUpdate 创建或更新代理IP关联
func (r *GormProxyIPAddressRepository) CreateOrUpdate(proxyIPAddress *model.ProxyIPAddress) error {
	if proxyIPAddress == nil {
		return errors.New("proxy IP address cannot be nil")
	}

	// 先查proxy关联的所有latest记录，然后
	// 先尝试查找是否已存在
	var existing model.ProxyIPAddress
	result := r.db.Where("proxy_id = ? AND ip_addresses_id = ?",
		proxyIPAddress.ProxyID, proxyIPAddress.IPAddressesID).First(&existing)

	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return result.Error
	}

	if result.Error == nil {
		// 更新现有记录
		proxyIPAddress.UpdatedAt = time.Now()
		return r.db.Model(&existing).Updates(proxyIPAddress).Error
	}

	// 创建新记录
	proxyIPAddress.CreatedAt = time.Now()
	proxyIPAddress.UpdatedAt = time.Now()
	return r.db.Create(proxyIPAddress).Error
}
