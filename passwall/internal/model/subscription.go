package model

import (
	"time"
)

// SubscriptionType 订阅类型
type SubscriptionType string

const (
	SubscriptionTypeClash    SubscriptionType = "clash"
	SubscriptionTypeShareURL SubscriptionType = "share_url"
)

// SubscriptionStatus 订阅状态
type SubscriptionStatus int

const (
	SubscriptionStatusPending SubscriptionStatus = -1 // 待处理
	SubscriptionStatusOK      SubscriptionStatus = 1  // 正常可拉取
	SubscriptionStatusInvalid SubscriptionStatus = 2  // 无法处理
	SubscriptionStatusDeleted SubscriptionStatus = 3  // 已删除
)

// Subscription 订阅源
type Subscription struct {
	ID        uint               `json:"id" gorm:"primaryKey;autoIncrement"`
	URL       string             `json:"url" gorm:"uniqueIndex:idx_subscriptions_url"` // 订阅URL或文件名，设置为唯一键
	Content   string             `json:"content"`                                      // 订阅内容，对URL是链接，对文件是内容
	Type      SubscriptionType   `json:"type"`
	Status    SubscriptionStatus `json:"status"`
	CreatedAt time.Time          `json:"created_at"`
	UpdatedAt time.Time          `json:"updated_at"`
}

// SubscriptionConfig 订阅自定义配置
type SubscriptionConfig struct {
	SubscriptionID uint      `json:"subscription_id" gorm:"primaryKey"`
	AutoUpdate     bool      `json:"auto_update"`
	UpdateInterval string    `json:"update_interval"`
	UseProxy       bool      `json:"use_proxy"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (SubscriptionConfig) TableName() string {
	return "subscription_configs"
}
