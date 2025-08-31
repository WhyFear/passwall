package repository

import (
	"errors"
	"passwall/internal/model"
	"time"

	"gorm.io/gorm"
)

// IPInfoRepository IP信息仓库接口
type IPInfoRepository interface {
	FindByID(id uint) (*model.IPInfo, error)
	FindByIPAddressID(ipAddressID uint) ([]*model.IPInfo, error)
	FindByIPAddressIDAndDetector(ipAddressID uint, detector string) (*model.IPInfo, error)
	CreateOrUpdate(ipInfo *model.IPInfo) error
	BatchCreateOrUpdate(ipInfos []*model.IPInfo) error
}

// GormIPInfoRepository 基于GORM的IP信息仓库实现
type GormIPInfoRepository struct {
	db *gorm.DB
}

// NewIPInfoRepository 创建IP信息仓库
func NewIPInfoRepository(db *gorm.DB) IPInfoRepository {
	return &GormIPInfoRepository{db: db}
}

// FindByID 根据ID查找IP信息
func (r *GormIPInfoRepository) FindByID(id uint) (*model.IPInfo, error) {
	var ipInfo model.IPInfo
	result := r.db.First(&ipInfo, id)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return &ipInfo, nil
}

// FindByIPAddressID 根据IP地址ID查找所有IP信息
func (r *GormIPInfoRepository) FindByIPAddressID(ipAddressID uint) ([]*model.IPInfo, error) {
	var ipInfos []*model.IPInfo
	err := r.db.Where("ip_addresses_id = ?", ipAddressID).Find(&ipInfos).Error
	if err != nil {
		return nil, err
	}
	return ipInfos, nil
}

// FindByIPAddressIDAndDetector 根据IP地址ID和检测器查找IP信息
func (r *GormIPInfoRepository) FindByIPAddressIDAndDetector(ipAddressID uint, detector string) (*model.IPInfo, error) {
	var ipInfo model.IPInfo
	result := r.db.Where("ip_addresses_id = ? AND detector = ?", ipAddressID, detector).First(&ipInfo)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return &ipInfo, nil
}

// CreateOrUpdate 创建或更新IP信息
func (r *GormIPInfoRepository) CreateOrUpdate(ipInfo *model.IPInfo) error {
	if ipInfo == nil {
		return errors.New("ip info cannot be nil")
	}

	// 先尝试查找是否已存在
	existing, err := r.FindByIPAddressIDAndDetector(ipInfo.IPAddressesID, ipInfo.Detector)
	if err != nil {
		return err
	}

	if existing != nil {
		// 更新现有记录
		ipInfo.UpdatedAt = time.Now()
		return r.db.Model(existing).Updates(ipInfo).Error
	}

	// 创建新记录
	ipInfo.CreatedAt = time.Now()
	ipInfo.UpdatedAt = time.Now()
	return r.db.Create(ipInfo).Error
}

// BatchCreateOrUpdate 批量创建或更新IP信息
func (r *GormIPInfoRepository) BatchCreateOrUpdate(ipInfos []*model.IPInfo) error {
	if len(ipInfos) == 0 {
		return nil
	}

	return r.db.Transaction(func(tx *gorm.DB) error {
		for _, ipInfo := range ipInfos {
			if ipInfo == nil {
				continue
			}

			// 先尝试查找是否已存在
			existing, err := r.FindByIPAddressIDAndDetector(ipInfo.IPAddressesID, ipInfo.Detector)
			if err != nil {
				return err
			}

			if existing != nil {
				// 更新现有记录
				ipInfo.UpdatedAt = time.Now()
				if err := tx.Model(existing).Updates(ipInfo).Error; err != nil {
					return err
				}
			} else {
				// 创建新记录
				ipInfo.CreatedAt = time.Now()
				ipInfo.UpdatedAt = time.Now()
				if err := tx.Create(ipInfo).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}
