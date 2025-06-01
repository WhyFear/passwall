package model

import (
	"time"
)

// ProxyType 代理类型
type ProxyType string

const (
	ProxyTypeVMess     ProxyType = "vmess"
	ProxyTypeVLess     ProxyType = "vless"
	ProxyTypeSS        ProxyType = "ss"
	ProxyTypeTrojan    ProxyType = "trojan"
	ProxyTypeSocks5    ProxyType = "socks5"
	ProxyTypeTuic      ProxyType = "tuic"
	ProxyTypeSSR       ProxyType = "ssr"
	ProxyTypeHysteria  ProxyType = "hysteria"
	ProxyTypeHysteria2 ProxyType = "hysteria2"
)

// ProxyStatus 代理状态
type ProxyStatus int

const (
	ProxyStatusPending     ProxyStatus = -1 // 待测试
	ProxyStatusOK          ProxyStatus = 1  // 正常
	ProxyStatusFailed      ProxyStatus = 2  // 连接失败
	ProxyStatusUnknowError ProxyStatus = 3  // 未知错误
)

// Proxy 代理服务器模型
type Proxy struct {
	ID             uint        `json:"id"`
	SubscriptionID *uint       `json:"subscription_id"`
	Name           string      `json:"name"`
	Domain         string      `json:"domain" gorm:"uniqueIndex:idx_domain_port"`
	Port           int         `json:"port" gorm:"uniqueIndex:idx_domain_port"`
	Type           ProxyType   `json:"type"`
	Config         string      `json:"config"`         // JSON格式存储
	Ping           int         `json:"ping"`           // 延迟(ms)
	DownloadSpeed  int64       `json:"download_speed"` // 下载速度(KB/s)
	UploadSpeed    int64       `json:"upload_speed"`   // 上传速度(KB/s)
	Status         ProxyStatus `json:"status"`
	LatestTestTime *time.Time  `json:"latest_test_time"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
}

// SpeedTestResult 测速结果
type SpeedTestResult struct {
	Ping          int   `json:"ping"`
	DownloadSpeed int64 `json:"download_speed"`
	UploadSpeed   int64 `json:"upload_speed"`
	Error         error `json:"error"`
}

// StringToProxyType string转ProxyType
func StringToProxyType(str string) ProxyType {
	return ProxyType(str)
}
