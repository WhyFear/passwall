package model

import (
	"time"

	"gorm.io/datatypes"
)

// IPAddress IP地址表模型
// 对应SQL schema中的ip_addresses表
type IPAddress struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	IP        string    `json:"ip" gorm:"uniqueIndex;not null;type:varchar(45)"`
	IPType    uint      `json:"ip_type" gorm:"type:integer"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ProxyIPAddress 代理IP关联表模型
// 对应SQL schema中的proxy_ip_addresses表
type ProxyIPAddress struct {
	ID            uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	ProxyID       uint      `json:"proxy_id" gorm:"not null;index"`
	IPAddressesID uint      `json:"ip_addresses_id" gorm:"not null;index"`
	IPType        uint      `json:"ip_type" gorm:"type:integer;not null"`
	Latest        bool      `json:"latest" gorm:"not null;default:true"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// IPInfo IP信息表模型
// 对应SQL schema中的ip_infos表
type IPInfo struct {
	ID            uint           `json:"id" gorm:"primaryKey;autoIncrement"`
	IPAddressesID uint           `json:"ip_addresses_id" gorm:"not null;index"`
	Detector      string         `json:"detector" gorm:"not null;type:varchar(255);index"`
	Risk          datatypes.JSON `json:"risk" gorm:"type:json"`
	Geo           datatypes.JSON `json:"geo" gorm:"type:json"`
	Raw           string         `json:"raw" gorm:"type:text"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

// IPBaseInfo IP基础信息表模型
// 对应SQL schema中的ip_base_infos表
type IPBaseInfo struct {
	ID            uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	IPAddressesID uint      `json:"ip_addresses_id" gorm:"not null;index"`
	RiskLevel     string    `json:"risk_level" gorm:"type:varchar(10)"`
	CountryCode   string    `json:"country_code" gorm:"type:varchar(5)"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// IPUnlockInfo IP解锁信息表模型
// 对应SQL schema中的ip_unlock_infos表
type IPUnlockInfo struct {
	ID            uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	IPAddressesID uint      `json:"ip_addresses_id" gorm:"not null;index"`
	AppName       string    `json:"app_name" gorm:"type:varchar(50)"`
	Status        string    `json:"status" gorm:"type:varchar(10)"`
	Region        string    `json:"region" gorm:"type:varchar(10)"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
