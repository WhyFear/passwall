package model

import (
	"time"
)

// IPQualityStatus IP质量状态
type IPQualityStatus string

const (
	IPQualityStatusUnknown IPQualityStatus = "unknown" // 未知
	IPQualityStatusGood    IPQualityStatus = "good"    // 良好
	IPQualityStatusFair    IPQualityStatus = "fair"    // 一般
	IPQualityStatusPoor    IPQualityStatus = "poor"    // 较差
	IPQualityStatusBad     IPQualityStatus = "bad"     // 差
	IPQualityStatusBanned  IPQualityStatus = "banned"  // 被封禁
)

// RiskLevel 风险等级
type RiskLevel string

const (
	RiskLevelUnknown  RiskLevel = "unknown"  // 未知
	RiskLevelLow      RiskLevel = "low"      // 低风险
	RiskLevelMedium   RiskLevel = "medium"   // 中等风险
	RiskLevelHigh     RiskLevel = "high"     // 高风险
	RiskLevelCritical RiskLevel = "critical" // 极高风险
)

// ServiceType 服务类型
type ServiceType string

const (
	ServiceTypeStreaming ServiceType = "streaming" // 流媒体服务
	ServiceTypeAI        ServiceType = "ai"        // AI服务
	ServiceTypeOther     ServiceType = "other"     // 其他服务
)

// UnlockStatus 解锁状态
type UnlockStatus string

const (
	UnlockStatusUnknown     UnlockStatus = "unknown"     // 未知
	UnlockStatusAvailable   UnlockStatus = "available"   // 可用
	UnlockStatusUnavailable UnlockStatus = "unavailable" // 不可用
	UnlockStatusBlocked     UnlockStatus = "blocked"     // 被封禁
	UnlockStatusError       UnlockStatus = "error"       // 检测错误
)

// IPAddress IP地址表
type IPAddress struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	IP        string    `json:"ip" gorm:"uniqueIndex;not null"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// IPQuality IP质量基础信息表
type IPQuality struct {
	ID           uint            `json:"id" gorm:"primaryKey;autoIncrement"`
	IPID         uint            `json:"ip_id" gorm:"not null;index:idx_ip_quality_ip_id"`
	IP           IPAddress       `json:"ip" gorm:"foreignKey:IPID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Status       IPQualityStatus `json:"status" gorm:"index:idx_ip_quality_status"`
	Country      string          `json:"country" gorm:"index:idx_ip_quality_country"`
	City         string          `json:"city" gorm:"index:idx_ip_quality_city"`
	ISP          string          `json:"isp"`
	ASN          string          `json:"asn" gorm:"index:idx_ip_quality_asn"`
	Organization string          `json:"organization"`
	Longitude    float64         `json:"longitude"`
	Latitude     float64         `json:"latitude"`
	IsVPN        bool            `json:"is_vpn" gorm:"index:idx_ip_quality_is_vpn"`
	IsProxy      bool            `json:"is_proxy" gorm:"index:idx_ip_quality_is_proxy"`
	IsHosting    bool            `json:"is_hosting" gorm:"index:idx_ip_quality_is_hosting"`
	IsTor        bool            `json:"is_tor" gorm:"index:idx_ip_quality_is_tor"`
	LastTestAt   *time.Time      `json:"last_test_at"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

// RiskScore 风险评分表
type RiskScore struct {
	ID             uint       `json:"id" gorm:"primaryKey;autoIncrement"`
	IPQualityID    uint       `json:"ip_quality_id" gorm:"not null;index:idx_risk_score_ip_quality_id"`
	IPQuality      IPQuality  `json:"ip_quality" gorm:"foreignKey:IPQualityID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Provider       string     `json:"provider" gorm:"index:idx_risk_score_provider"`
	OverallScore   float64    `json:"overall_score" gorm:"index:idx_risk_score_overall_score"`
	FraudScore     *float64   `json:"fraud_score"`
	SpamScore      *float64   `json:"spam_score"`
	BotScore       *float64   `json:"bot_score"`
	VPNProxyScore  *float64   `json:"vpn_proxy_score"`
	RiskLevel      RiskLevel  `json:"risk_level" gorm:"index:idx_risk_score_risk_level"`
	IsHighRisk     bool       `json:"is_high_risk" gorm:"index:idx_risk_score_is_high_risk"`
	IsRecentAbuse  bool       `json:"is_recent_abuse"`
	LastReportedAt *time.Time `json:"last_reported_at"`
	LastTestAt     *time.Time `json:"last_test_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// ServiceUnlock 服务解锁状态表
type ServiceUnlock struct {
	ID          uint         `json:"id" gorm:"primaryKey;autoIncrement"`
	IPQualityID uint         `json:"ip_quality_id" gorm:"not null;index:idx_service_unlock_ip_quality_id"`
	IPQuality   IPQuality    `json:"ip_quality" gorm:"foreignKey:IPQualityID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	ServiceType ServiceType  `json:"service_type" gorm:"index:idx_service_unlock_service_type"`
	ServiceName string       `json:"service_name" gorm:"index:idx_service_unlock_service_name"`
	Status      UnlockStatus `json:"status" gorm:"index:idx_service_unlock_status"`
	Region      string       `json:"region" gorm:"index:idx_service_unlock_region"`
	ContentType string       `json:"content_type"`
	Details     string       `json:"details"` // JSON格式存储额外信息
	LastTestAt  *time.Time   `json:"last_test_at"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// ProxyIPQuality 代理IP质量关联表
type ProxyIPQuality struct {
	ID          uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	ProxyID     uint      `json:"proxy_id" gorm:"not null;index:idx_proxy_ip_quality_proxy_id"`
	Proxy       Proxy     `json:"proxy" gorm:"foreignKey:ProxyID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	IPQualityID uint      `json:"ip_quality_id" gorm:"not null;index:idx_proxy_ip_quality_ip_quality_id"`
	IPQuality   IPQuality `json:"ip_quality" gorm:"foreignKey:IPQualityID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// IPQualityEvent IP质量检测事件
type IPQualityEvent struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	ProxyID   uint      `json:"proxy_id" gorm:"not null;index:idx_ip_quality_event_proxy_id"`
	Proxy     Proxy     `json:"proxy" gorm:"foreignKey:ProxyID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	EventType string    `json:"event_type" gorm:"index:idx_ip_quality_event_type"`
	Status    string    `json:"status" gorm:"index:idx_ip_quality_event_status"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

// IPQualitySummary IP质量摘要
type IPQualitySummary struct {
	ID                uint            `json:"id" gorm:"primaryKey;autoIncrement"`
	IPQualityID       uint            `json:"ip_quality_id" gorm:"not null;uniqueIndex"`
	IPQuality         IPQuality       `json:"ip_quality" gorm:"foreignKey:IPQualityID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	OverallScore      float64         `json:"overall_score"`
	Status            IPQualityStatus `json:"status"`
	StreamingCount    int             `json:"streaming_count"`
	StreamingUnlocked int             `json:"streaming_unlocked"`
	AICount           int             `json:"ai_count"`
	AIUnlocked        int             `json:"ai_unlocked"`
	LastTestAt        *time.Time      `json:"last_test_at"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}
