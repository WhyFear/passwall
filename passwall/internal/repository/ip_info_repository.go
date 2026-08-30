package repository

import (
	"errors"
	"passwall/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
	return r.upsert([]*model.IPInfo{ipInfo})
}

// BatchCreateOrUpdate 批量创建或更新IP信息
func (r *GormIPInfoRepository) BatchCreateOrUpdate(ipInfos []*model.IPInfo) error {
	if len(ipInfos) == 0 {
		return nil
	}

	filtered := make([]*model.IPInfo, 0, len(ipInfos))
	for _, ipInfo := range ipInfos {
		if ipInfo != nil {
			filtered = append(filtered, ipInfo)
		}
	}
	return r.upsert(filtered)
}

func (r *GormIPInfoRepository) upsert(ipInfos []*model.IPInfo) error {
	if len(ipInfos) == 0 {
		return nil
	}
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "ip_addresses_id"}, {Name: "detector"}},
		DoUpdates: clause.AssignmentColumns([]string{"risk", "geo", "raw", "updated_at"}),
	}).Create(&ipInfos).Error
}
