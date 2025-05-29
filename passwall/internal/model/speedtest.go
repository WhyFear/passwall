package model

import (
	"time"
)

// SpeedTestHistory 测速历史记录
type SpeedTestHistory struct {
	ID            uint      `json:"id"`
	ProxyID       uint      `json:"proxy_id"`
	Ping          int       `json:"ping"`           // 延迟(ms)
	DownloadSpeed int64     `json:"download_speed"` // 下载速度(KB/s)
	UploadSpeed   int64     `json:"upload_speed"`   // 上传速度(KB/s)
	TestTime      time.Time `json:"test_time"`
	CreatedAt     time.Time `json:"created_at"`
}
