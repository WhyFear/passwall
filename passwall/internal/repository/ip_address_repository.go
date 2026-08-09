package repository

import (
	"errors"
	"passwall/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// IPAddressRepository IP地址仓库接口
type IPAddressRepository interface {
	FindByID(id uint) (*model.IPAddress, error)
	FindByIP(ip string) (*model.IPAddress, error)
	CreateOrIgnore(ipAddress *model.IPAddress) error
}

// GormIPAddressRepository 基于GORM的IP地址仓库实现
type GormIPAddressRepository struct {
	db *gorm.DB
}

// NewIPAddressRepository 创建IP地址仓库
func NewIPAddressRepository(db *gorm.DB) IPAddressRepository {
	return &GormIPAddressRepository{db: db}
}

// FindByID 根据ID查找IP地址
func (r *GormIPAddressRepository) FindByID(id uint) (*model.IPAddress, error) {
	var ipAddress model.IPAddress
	result := r.db.First(&ipAddress, id)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return &ipAddress, nil
}

// FindByIP 根据IP地址查找
func (r *GormIPAddressRepository) FindByIP(ip string) (*model.IPAddress, error) {
	var ipAddress model.IPAddress
	result := r.db.Where("ip = ?", ip).First(&ipAddress)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return &ipAddress, nil
}

// CreateOrUpdate 创建或更新IP地址
func (r *GormIPAddressRepository) CreateOrIgnore(ipAddress *model.IPAddress) error {
	if ipAddress == nil {
		return errors.New("ip address cannot be nil")
	}

	result := r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "ip"}},
		DoNothing: true,
	}).Create(ipAddress)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		return nil
	}

	existing, err := r.FindByIP(ipAddress.IP)
	if err != nil {
		return err
	}
	if existing == nil {
		return errors.New("IP address conflict occurred but existing record was not found")
	}
	*ipAddress = *existing
	return nil
}
