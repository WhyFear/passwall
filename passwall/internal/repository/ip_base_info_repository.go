package repository

import (
	"errors"
	"passwall/internal/model"
	"time"

	"gorm.io/gorm"
)

// IPBaseInfoRepository IP基础信息仓库接口
type IPBaseInfoRepository interface {
	FindByID(id uint) (*model.IPBaseInfo, error)
	FindByIPAddressID(ipAddressID uint) (*model.IPBaseInfo, error)
	CreateOrUpdate(ipBaseInfo *model.IPBaseInfo) error
}

// GormIPBaseInfoRepository 基于GORM的IP基础信息仓库实现
type GormIPBaseInfoRepository struct {
	db *gorm.DB
}

// NewIPBaseInfoRepository 创建IP基础信息仓库
func NewIPBaseInfoRepository(db *gorm.DB) IPBaseInfoRepository {
	return &GormIPBaseInfoRepository{db: db}
}

// FindByID 根据ID查找IP基础信息
func (r *GormIPBaseInfoRepository) FindByID(id uint) (*model.IPBaseInfo, error) {
	var ipBaseInfo model.IPBaseInfo
	result := r.db.First(&ipBaseInfo, id)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return &ipBaseInfo, nil
}

// FindByIPAddressID 根据IP地址ID查找IP基础信息
func (r *GormIPBaseInfoRepository) FindByIPAddressID(ipAddressID uint) (*model.IPBaseInfo, error) {
	var ipBaseInfo model.IPBaseInfo
	result := r.db.Where("ip_addresses_id = ?", ipAddressID).First(&ipBaseInfo)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return &ipBaseInfo, nil
}

// CreateOrUpdate 创建或更新IP基础信息
func (r *GormIPBaseInfoRepository) CreateOrUpdate(ipBaseInfo *model.IPBaseInfo) error {
	if ipBaseInfo == nil {
		return errors.New("ip base info cannot be nil")
	}

	// 先尝试查找是否已存在
	existing, err := r.FindByIPAddressID(ipBaseInfo.IPAddressesID)
	if err != nil {
		return err
	}

	if existing != nil {
		// 更新现有记录
		ipBaseInfo.UpdatedAt = time.Now()
		return r.db.Model(existing).Updates(ipBaseInfo).Error
	}

	// 创建新记录
	ipBaseInfo.CreatedAt = time.Now()
	ipBaseInfo.UpdatedAt = time.Now()
	return r.db.Create(ipBaseInfo).Error
}
