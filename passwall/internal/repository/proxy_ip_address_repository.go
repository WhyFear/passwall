package repository

import (
	"encoding/binary"
	"errors"
	"hash/fnv"
	"passwall/internal/model"
	"time"

	"gorm.io/gorm"
)

// ProxyIPAddressRepository 代理IP关联仓库接口
type ProxyIPAddressRepository interface {
	FindByID(id uint) (*model.ProxyIPAddress, error)
	FindByProxyID(proxyID uint) ([]*model.ProxyIPAddress, error)
	FindLatestByProxyIDList(proxyIDList []uint) ([]*model.ProxyIPAddress, error)
	FindByIPAddressID(ipAddressID uint) ([]*model.ProxyIPAddress, error)
	GetDistinctProxyIDs() ([]uint, error)
	CreateOrUpdate(proxyIPAddress *model.ProxyIPAddress) error
}

// GormProxyIPAddressRepository 基于GORM的代理IP关联仓库实现
type GormProxyIPAddressRepository struct {
	db *gorm.DB
}

func (r *GormProxyIPAddressRepository) GetDistinctProxyIDs() ([]uint, error) {
	var proxyIDs []uint
	result := r.db.Model(&model.ProxyIPAddress{}).Select("DISTINCT proxy_id").Pluck("proxy_id", &proxyIDs)
	if result.Error != nil {
		return nil, result.Error
	}
	return proxyIDs, nil
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

func (r *GormProxyIPAddressRepository) FindLatestByProxyIDList(proxyIDList []uint) ([]*model.ProxyIPAddress, error) {
	if len(proxyIDList) == 0 {
		return []*model.ProxyIPAddress{}, nil
	}

	var proxyIPAddresses []*model.ProxyIPAddress
	err := r.db.
		Preload("IPAddress").
		Preload("IPAddress.IPBaseInfo").
		Where("proxy_id IN ? AND latest = ?", proxyIDList, true).
		Order("proxy_id ASC, ip_type ASC").
		Find(&proxyIPAddresses).Error
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

	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := lockProxyIPAddressWrite(tx, proxyIPAddress.ProxyID, proxyIPAddress.IPType); err != nil {
			return err
		}

		now := time.Now()
		var existing model.ProxyIPAddress
		findErr := tx.Where(
			"proxy_id = ? AND ip_addresses_id = ? AND ip_type = ?",
			proxyIPAddress.ProxyID,
			proxyIPAddress.IPAddressesID,
			proxyIPAddress.IPType,
		).First(&existing).Error
		if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return findErr
		}

		deactivate := tx.Model(&model.ProxyIPAddress{}).
			Where("proxy_id = ? AND ip_type = ? AND latest = ?", proxyIPAddress.ProxyID, proxyIPAddress.IPType, true)
		if findErr == nil {
			deactivate = deactivate.Where("id <> ?", existing.ID)
		}
		if err := deactivate.Updates(map[string]interface{}{
			"latest":     false,
			"updated_at": now,
		}).Error; err != nil {
			return err
		}

		if findErr == nil {
			return tx.Model(&model.ProxyIPAddress{}).
				Where("id = ?", existing.ID).
				Updates(map[string]interface{}{
					"latest":     true,
					"updated_at": now,
				}).Error
		}

		proxyIPAddress.CreatedAt = time.Now()
		proxyIPAddress.UpdatedAt = time.Now()
		proxyIPAddress.Latest = true
		return tx.Create(proxyIPAddress).Error
	})
}

func lockProxyIPAddressWrite(tx *gorm.DB, proxyID uint, ipType uint) error {
	if tx.Dialector == nil || tx.Dialector.Name() != "postgres" {
		return nil
	}
	return tx.Exec("SELECT pg_advisory_xact_lock(?)", proxyIPAddressLockKey(proxyID, ipType)).Error
}

func proxyIPAddressLockKey(proxyID uint, ipType uint) int64 {
	var key [16]byte
	binary.BigEndian.PutUint64(key[:8], uint64(proxyID))
	binary.BigEndian.PutUint64(key[8:], uint64(ipType))

	hash := fnv.New64a()
	_, _ = hash.Write(key[:])
	return int64(hash.Sum64())
}
