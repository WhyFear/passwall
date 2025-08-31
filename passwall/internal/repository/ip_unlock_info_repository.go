package repository

import (
	"errors"
	"passwall/internal/model"
	"time"

	"gorm.io/gorm"
)

// IPUnlockInfoRepository IP解锁信息仓库接口
type IPUnlockInfoRepository interface {
	FindByID(id uint) (*model.IPUnlockInfo, error)
	FindByIPAddressID(ipAddressID uint) ([]*model.IPUnlockInfo, error)
	FindByIPAddressIDAndAppName(ipAddressID uint, appName string) (*model.IPUnlockInfo, error)
	CreateOrUpdate(ipUnlockInfo *model.IPUnlockInfo) error
}

// GormIPUnlockInfoRepository 基于GORM的IP解锁信息仓库实现
type GormIPUnlockInfoRepository struct {
	db *gorm.DB
}

// NewIPUnlockInfoRepository 创建IP解锁信息仓库
func NewIPUnlockInfoRepository(db *gorm.DB) IPUnlockInfoRepository {
	return &GormIPUnlockInfoRepository{db: db}
}

// FindByID 根据ID查找IP解锁信息
func (r *GormIPUnlockInfoRepository) FindByID(id uint) (*model.IPUnlockInfo, error) {
	var ipUnlockInfo model.IPUnlockInfo
	result := r.db.First(&ipUnlockInfo, id)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return &ipUnlockInfo, nil
}

// FindByIPAddressID 根据IP地址ID查找所有解锁信息
func (r *GormIPUnlockInfoRepository) FindByIPAddressID(ipAddressID uint) ([]*model.IPUnlockInfo, error) {
	var ipUnlockInfos []*model.IPUnlockInfo
	err := r.db.Where("ip_addresses_id = ?", ipAddressID).Find(&ipUnlockInfos).Error
	if err != nil {
		return nil, err
	}
	return ipUnlockInfos, nil
}

// FindByIPAddressIDAndAppName 根据IP地址ID和应用名称查找解锁信息
func (r *GormIPUnlockInfoRepository) FindByIPAddressIDAndAppName(ipAddressID uint, appName string) (*model.IPUnlockInfo, error) {
	var ipUnlockInfo model.IPUnlockInfo
	result := r.db.Where("ip_addresses_id = ? AND app_name = ?", ipAddressID, appName).First(&ipUnlockInfo)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return &ipUnlockInfo, nil
}

// CreateOrUpdate 创建或更新IP解锁信息
func (r *GormIPUnlockInfoRepository) CreateOrUpdate(ipUnlockInfo *model.IPUnlockInfo) error {
	if ipUnlockInfo == nil {
		return errors.New("ip unlock info cannot be nil")
	}

	// 先尝试查找是否已存在
	existing, err := r.FindByIPAddressIDAndAppName(ipUnlockInfo.IPAddressesID, ipUnlockInfo.AppName)
	if err != nil {
		return err
	}

	if existing != nil {
		// 更新现有记录
		ipUnlockInfo.UpdatedAt = time.Now()
		return r.db.Model(existing).Updates(ipUnlockInfo).Error
	}

	// 创建新记录
	ipUnlockInfo.CreatedAt = time.Now()
	ipUnlockInfo.UpdatedAt = time.Now()
	return r.db.Create(ipUnlockInfo).Error
}
