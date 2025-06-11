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
	ProxyTypeSSR       ProxyType = "ssr"
	ProxyTypeTrojan    ProxyType = "trojan"
	ProxyTypeSocks5    ProxyType = "socks5"
	ProxyTypeTuic      ProxyType = "tuic"
	ProxyTypeHysteria  ProxyType = "hysteria"
	ProxyTypeHysteria2 ProxyType = "hysteria2"
	ProxyTypeSnell     ProxyType = "snell"
	ProxyTypeHttp      ProxyType = "http"
	ProxyTypeWireGuard ProxyType = "wireguard"
	ProxyTypeMieru     ProxyType = "mieru"
	ProxyTypeAnyTLS    ProxyType = "anytls"
	ProxyTypeSsh       ProxyType = "ssh"
)

// ProxyStatus 代理状态
type ProxyStatus int

const (
	ProxyStatusPending     ProxyStatus = -1 // 待测试
	ProxyStatusOK          ProxyStatus = 1  // 正常
	ProxyStatusFailed      ProxyStatus = 2  // 连接失败
	ProxyStatusUnknowError ProxyStatus = 3  // 未知错误
	ProxyStatusBanned      ProxyStatus = 4  // 被禁用
)

// Proxy 代理服务器模型
type Proxy struct {
	ID             uint        `json:"id" gorm:"primaryKey;autoIncrement"`
	SubscriptionID *uint       `json:"subscription_id" gorm:"index:idx_proxies_subscription_id"`
	Name           string      `json:"name"`
	Domain         string      `json:"domain" gorm:"uniqueIndex:idx_domain_port"`
	Port           int         `json:"port" gorm:"uniqueIndex:idx_domain_port"`
	Type           ProxyType   `json:"type" gorm:"index:idx_proxies_type;index:idx_filter_sort"`
	Config         string      `json:"config"`                                                                       // JSON格式存储
	Ping           int         `json:"ping" gorm:"index:idx_proxies_ping;index:idx_filter_sort"`                     // 延迟(ms)
	DownloadSpeed  int         `json:"download_speed" gorm:"index:idx_proxies_download_speed;index:idx_filter_sort"` // 下载速度(KB/s)
	UploadSpeed    int         `json:"upload_speed" gorm:"index:idx_proxies_upload_speed;index:idx_filter_sort"`     // 上传速度(KB/s)
	Status         ProxyStatus `json:"status" gorm:"index:idx_proxies_status;index:idx_filter_sort;index:idx_latest_time_status"`
	Pinned         bool        `json:"pinned" gorm:"index:idx_proxies_pinned;default:false"` // 是否置顶
	LatestTestTime *time.Time  `json:"latest_test_time" gorm:"index:idx_proxies_latest_test_time;index:idx_filter_sort;index:idx_latest_time_status"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
}

// SpeedTestResult 测速结果
type SpeedTestResult struct {
	Ping          int   `json:"ping"`
	DownloadSpeed int   `json:"download_speed"`
	UploadSpeed   int   `json:"upload_speed"`
	Error         error `json:"error"`
}

// StringToProxyType string转ProxyType
func StringToProxyType(str string) ProxyType {
	return ProxyType(str)
}
