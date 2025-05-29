package model

import (
	"time"
)

// SubscriptionType 订阅类型
type SubscriptionType string

const (
	SubscriptionTypeClash  SubscriptionType = "clash"
	SubscriptionTypeV2ray  SubscriptionType = "v2ray"
	SubscriptionTypeTrojan SubscriptionType = "trojan"
)

// SubscriptionStatus 订阅状态
type SubscriptionStatus int

const (
	SubscriptionStatusPending SubscriptionStatus = 0 // 待处理
	SubscriptionStatusOK      SubscriptionStatus = 1 // 正常可拉取
	SubscriptionStatusInvalid SubscriptionStatus = 2 // 无法处理
	SubscriptionStatusExpired SubscriptionStatus = 3 // 曾经可处理，现在失效
)

// Subscription 订阅源
type Subscription struct {
	ID        uint               `json:"id"`
	URL       string             `json:"url" gorm:"uniqueIndex"` // 订阅URL或文件名，设置为唯一键
	Content   string             `json:"content"`                // 订阅内容，对URL是链接，对文件是内容
	Type      SubscriptionType   `json:"type"`
	Status    SubscriptionStatus `json:"status"`
	CreatedAt time.Time          `json:"created_at"`
	UpdatedAt time.Time          `json:"updated_at"`
}
