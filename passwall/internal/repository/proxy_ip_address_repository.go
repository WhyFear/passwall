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

	// 先查proxy关联的所有latest记录，然后判断是否有关联到ip_addresses_id的记录，如果有就更新updatetime，如果没有则将latest设置为false，然后插入新表
	var existing []*model.ProxyIPAddress
	result := r.db.Where("proxy_id = ? AND latest = ? AND ip_type = ?", proxyIPAddress.ProxyID, true, proxyIPAddress.IPType).Find(&existing)
	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return result.Error
	} else if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		// 创建新记录
		proxyIPAddress.CreatedAt = time.Now()
		proxyIPAddress.UpdatedAt = time.Now()
		return r.db.Create(proxyIPAddress).Error
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
			}
		}()
		// 有记录，判断是否关联到ip_addresses_id的记录
		for _, item := range existing {
			if item.IPAddressesID == proxyIPAddress.IPAddressesID {
				// 更新现有记录
				item.UpdatedAt = time.Now()
				return tx.Model(&item).Updates(item).Error
			}
		}
		// 走到这里，说明没有关联到ip_addresses_id的记录，需要将latest设置为false，然后插入新表
		for _, item := range existing {
			item.Latest = false
			tx.Model(&item).Updates(item)
		}
		// 插入新记录
		proxyIPAddress.CreatedAt = time.Now()
		proxyIPAddress.UpdatedAt = time.Now()
		return tx.Create(proxyIPAddress).Error
	})
}
