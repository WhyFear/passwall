package model

import "time"

type ShareConfig struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name" gorm:"type:varchar(255);not null"`
	Slug        string    `json:"slug" gorm:"type:varchar(64);uniqueIndex;not null"`
	Enabled     bool      `json:"enabled" gorm:"not null;default:true"`
	Deleted     bool      `json:"deleted" gorm:"not null;default:false"`
	Type        string    `json:"type" gorm:"type:varchar(32);not null"`
	Status      string    `json:"status" gorm:"type:varchar(64)"`
	ProxyType   string    `json:"proxy_type" gorm:"type:varchar(255)"`
	CountryCode string    `json:"country_code" gorm:"type:varchar(255)"`
	RiskLevel   string    `json:"risk_level" gorm:"type:varchar(255)"`
	Sort        string    `json:"sort" gorm:"type:varchar(64)"`
	SortOrder   string    `json:"sort_order" gorm:"type:varchar(16)"`
	Limit       int       `json:"limit"`
	WithIndex   bool      `json:"with_index" gorm:"not null;default:false"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (ShareConfig) TableName() string {
	return "share_configs"
}
